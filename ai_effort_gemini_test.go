package bibletext

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestGeminiModelEffortLevelsAndBudgets(t *testing.T) {
	pre := time.Date(2026, 11, 30, 23, 59, 59, 0, time.UTC)
	post := pre.Add(time.Second)
	for _, tc := range []struct {
		model string
		want  []string
	}{
		{"gemini-4-neon", []string{"low", "medium", "high", "xhigh"}},
		{"gemini-4-hydrogen", []string{"low", "medium", "high", "xhigh"}},
		{"gemini-4-carbon", []string{"low", "medium", "high"}},
		{"gemini-3.8-flash", []string{"low", "medium", "high"}},
		{"gemini-3.5-flash-lite", []string{"minimal", "low", "medium", "high"}},
		{"gemini-3-pro-preview", []string{"low", "high"}},
		{"gemini-2.5-pro", []string{"1024", "8192", "24576"}},
		{"gemini-2.5-flash", []string{"0", "1024", "8192", "24576"}},
		{"gemini-2.5-flash-lite", []string{"0", "1024", "8192", "24576"}},
		{"gemini-3.8-flash-tts", nil},
		{"gemini-flash-latest", nil},
	} {
		if got := geminiEffortsAt(tc.model, pre); !slices.Equal(got, tc.want) {
			t.Errorf("%s: choices %v, want %v", tc.model, got, tc.want)
		}
	}
	for alias, target := range map[string]string{"gemini-2.5-flash": "gemini-3.5-flash-lite", "gemini-2.5-pro": "gemini-3.8-flash"} {
		if got, want := geminiEffortsAt(alias, post), geminiEffortsAt(target, post); !slices.Equal(got, want) {
			t.Errorf("after redirect %s: choices %v, want %v", alias, got, want)
		}
	}
	if !slices.Equal(geminiEffortsAt("gemini-2.5-flash-lite", pre), geminiEffortsAt("gemini-2.5-flash-lite", post)) {
		t.Error("Flash-Lite changed on another model's redirect date")
	}
}

func TestGeminiAliasLearnsOnlyFromItsOwnSuccessfulResponse(t *testing.T) {
	const alias, target = "gemini-pro-latest", "gemini-9-synthetic"
	for _, tc := range []struct {
		name, requested, modelVersion string
		visible, learn                bool
	}{
		{"actual alias target", alias, target, true, true},
		{"resource ID", alias, "models/" + target, true, true},
		{"no visible answer", alias, target, false, false},
		{"missing", alias, "", true, false},
		{"ordinal version", alias, "001", true, false},
		{"another alias", alias, "gemini-flash-latest", true, false},
		{"path", alias, "../" + target, true, false},
		{"pinned model", "gemini-4-carbon", target, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newTestKeyStore()
			store.setOverrideModel(providerGemini, tc.requested)
			parts := []map[string]any{{"thought": true, "text": "Synthetic private summary."}}
			if tc.visible {
				parts = append(parts, map[string]any{"text": "Visible."})
			}
			body, err := json.Marshal(map[string]any{"modelVersion": tc.modelVersion, "candidates": []any{map[string]any{"content": map[string]any{"parts": parts}}}})
			if err != nil {
				t.Fatal(err)
			}
			mock := &mockHTTP{statusCode: 200, body: string(body)}
			resolver := effortTestResolver(t, store, providerGemini, map[string]*mockHTTP{tc.requested: mock})
			answer, err := resolver.generate(context.Background(), "A synthetic question.")
			if mock.calls != 1 || (err == nil) != tc.visible || (tc.visible && answer != "Visible.") {
				t.Fatalf("calls = %d, answer = %q, error = %v", mock.calls, answer, err)
			}
			want := ""
			if tc.learn {
				want = target
			}
			if got := recentGeminiResponseTarget(store.prefs, tc.requested); got != want {
				t.Fatalf("observed target = %q, want %q", got, want)
			}
			saved := store.prefs.String(geminiResponseTargetKey(tc.requested))
			if strings.Contains(saved, "Visible") || strings.Contains(saved, "private") {
				t.Fatal("response content was stored with the model ID")
			}
		})
	}
}

func TestGeminiAliasEvidenceExpiresAndRemainsScoped(t *testing.T) {
	const alias, target = "gemini-pro-latest", "gemini-9-synthetic"
	prefs := newFakePrefs()
	rememberGeminiResponseTarget(prefs, alias, target)
	if got := recentGeminiResponseTarget(prefs, "gemini-flash-latest"); got != "" {
		t.Fatalf("alias evidence leaked to another alias: %q", got)
	}
	for _, offset := range []time.Duration{-25 * time.Hour, time.Hour} {
		data, err := json.Marshal(geminiAliasObservation{Target: target, Checked: time.Now().Add(offset).Unix()})
		if err != nil {
			t.Fatal(err)
		}
		prefs.SetString(geminiResponseTargetKey(alias), string(data))
		if got := recentGeminiResponseTarget(prefs, alias); got != "" {
			t.Fatalf("target accepted with clock offset %s: %q", offset, got)
		}
	}
}

func TestGeminiThinkingBudgetRequest(t *testing.T) {
	const model = "gemini-2.5-flash-lite"
	for _, saved := range []string{"", "0", "1024", "8192", "24576"} {
		t.Run(saved, func(t *testing.T) {
			store := newTestKeyStore()
			store.setOverrideModel(providerGemini, model)
			setAIReasoningEffort(store.prefs, providerGemini, model, saved)
			mock := &mockHTTP{statusCode: http.StatusOK, body: `{"candidates":[{"content":{"parts":[{"text":"OK"}]}}]}`}
			resolver := effortTestResolver(t, store, providerGemini, map[string]*mockHTTP{model: mock})
			if _, err := resolver.generate(context.Background(), "Synthetic question."); err != nil {
				t.Fatal(err)
			}
			var req struct {
				Config map[string]json.RawMessage `json:"generationConfig"`
			}
			if err := json.NewDecoder(mock.lastReq.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			cfg, present := req.Config["thinkingConfig"]
			if saved == "" {
				if present {
					t.Fatalf("Recommended sent %s", cfg)
				}
				return
			}
			var thinking map[string]int
			want, _ := strconv.Atoi(saved)
			if err := json.Unmarshal(cfg, &thinking); err != nil {
				t.Fatal(err)
			}
			if budget, ok := thinking["thinkingBudget"]; !ok || budget != want || len(thinking) != 1 {
				t.Fatalf("thinkingConfig=%s, want only budget %d", cfg, want)
			}
		})
	}
}

func TestGeminiCatalogReportsEffortEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" || r.Header.Get("x-goog-api-key") != "synthetic" {
			t.Error("unexpected catalog request")
		}
		_, _ = w.Write([]byte(`{"models":[
			{"name":"models/gemini-pro-latest","baseModelId":"gemini-4-neon","thinking":true,"supportedGenerationMethods":["generateContent"]},
			{"name":"models/gemini-flash-latest","version":"gemini-3.8-flash","thinking":true,"supportedGenerationMethods":["generateContent"]},
			{"name":"models/gemini-synthetic-latest","baseModelId":"gemini-4-carbon","version":"models/gemini-9-synthetic","thinking":true,"supportedGenerationMethods":["generateContent"]},
			{"name":"models/gemini-unresolved-latest","baseModelId":"gemini-pro-latest","version":"001","supportedGenerationMethods":["generateContent"]},
			{"name":"models/synthetic-new","version":"4","thinking":true,"supportedGenerationMethods":["generateContent"]},
			{"name":"models/synthetic-exposed","supportedThinkingLevels":["LOW","THINKING_LEVEL_HIGH","HIGH","NEW_DEPTH","THINKING_LEVEL_NEW_DEPTH","UNSPECIFIED","THINKING_LEVEL_UNSPECIFIED","4096","not a level"],"supportedGenerationMethods":["generateContent"]},
			{"name":"models/synthetic-no-thinking","thinking":false,"supportedGenerationMethods":["generateContent"]},
			{"name":"models/synthetic-not-text","supportedGenerationMethods":["embedContent"]}]}`))
	}))
	t.Cleanup(server.Close)
	got, err := listGeminiModels(server.URL)(context.Background(), "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"gemini-pro-latest": nil, "gemini-flash-latest": nil, "synthetic-new": nil, "synthetic-exposed": {"low", "high", "new_depth"}, "synthetic-no-thinking": {}}
	want["gemini-synthetic-latest"] = nil
	want["gemini-unresolved-latest"] = nil
	docs := map[string]string{"gemini-pro-latest": "gemini-4-neon", "gemini-flash-latest": "gemini-3.8-flash", "gemini-synthetic-latest": "gemini-9-synthetic"}
	if len(got) != len(want) {
		t.Fatalf("catalog returned %d text models, want %d", len(got), len(want))
	}
	for _, m := range got {
		if m.docsModel != docs[m.id] {
			t.Errorf("%s: documentation model %q, want %q", m.id, m.docsModel, docs[m.id])
		}
		levels, ok := want[m.id]
		if !ok || (levels == nil) != (m.efforts == nil) || !slices.Equal(levels, m.efforts) {
			t.Errorf("%s: efforts %#v, want %#v", m.id, m.efforts, levels)
		}
	}
	if config := geminiThinkingOptions("new_depth"); config.ThinkingLevel != "NEW_DEPTH" || config.ThinkingBudget != nil {
		t.Fatalf("future provider-reported enum lost in wire conversion: %+v", config)
	}
}
