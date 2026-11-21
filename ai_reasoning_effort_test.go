package bibletext

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"testing"
)

func TestAIReasoningEffortCapabilitiesAndPreferences(t *testing.T) {
	original := []string{"minimal", "low", "medium", "high"}
	newer := []string{"none", "low", "medium", "high", "xhigh"}
	for _, tc := range []struct {
		provider, model string
		want            []string
	}{
		{providerOpenAI, "gpt-5", original},
		{providerOpenAI, "gpt-5-2025-08-07", original},
		{providerOpenAI, "gpt-5-mini", original},
		{providerOpenAI, "gpt-5-mini-2025-08-07", original},
		{providerOpenAI, "gpt-5-nano", original},
		{providerOpenAI, "gpt-5-nano-2025-08-07", original},
		{providerOpenAI, "gpt-5.4", newer},
		{providerOpenAI, "gpt-5.4-2026-03-05", newer},
		{providerOpenAI, "gpt-4.1", nil},
		{providerOpenAI, "gpt-5-2099-01-01", nil},
		{providerOpenAI, "gpt-5.4-pro", nil},
		{providerOpenAI, "gpt-5.4-unknown", nil},
		{providerGrok, "gpt-5", nil},
		{providerGemini, "gpt-5-mini", nil},
		{providerGemini, "gemini-4-carbon", []string{"low", "medium", "high"}},
		{providerGemini, "gemini-pro-latest", nil},
		{providerGemini, "gemini-4-carbon-unknown", nil},
		{providerOpenAI, "gemini-4-carbon", nil},
		{providerAnthropic, "gpt-5.4", nil},
	} {
		if got := aiReasoningEfforts(tc.provider, tc.model); !slices.Equal(got, tc.want) {
			t.Errorf("levels for %s/%s = %v, want %v", tc.provider, tc.model, got, tc.want)
		}
	}

	store := newTestKeyStore()
	model := activeModelFor(store, providerOpenAI)
	setAIReasoningEffort(store.prefs, providerOpenAI, model, "high")
	setAIReasoningEffort(store.prefs, providerOpenAI, "gpt-5-mini", "minimal")
	store.setOverrideModel(providerOpenAI, model)
	if got := aiReasoningEffort(store.prefs, providerOpenAI, activeModelFor(store, providerOpenAI)); got != "high" {
		t.Fatalf("pin of the recommended model lost its choice: %q", got)
	}
	setAIReasoningEffort(store.prefs, providerOpenAI, model, "xhigh")
	if got := aiReasoningEffort(store.prefs, providerOpenAI, model); got != "high" {
		t.Errorf("unavailable new choice replaced the saved choice: %q", got)
	}
	setAIReasoningEffort(store.prefs, providerOpenAI, model, "")
	store.setOverrideModel(providerOpenAI, "")
	if got := aiReasoningEffort(store.prefs, providerOpenAI, activeModelFor(store, providerOpenAI)); got != "" {
		t.Errorf("Recommended did not clear the model's override: %q", got)
	}
	if got := aiReasoningEffort(store.prefs, providerOpenAI, "gpt-5-mini"); got != "minimal" {
		t.Errorf("reset also changed a different model's choice: %q", got)
	}
	store.prefs.SetString(reasoningEffortKey(providerOpenAI, model), "retired-level")
	if got := aiReasoningEffort(store.prefs, providerOpenAI, model); got != "" {
		t.Errorf("unavailable stored effort = %q, want provider default", got)
	}
}

// Keep the registry's real resolver, client and serializer. Replace only the
// transport so these tests inspect the JSON without a network connection.
func effortTestResolver(t *testing.T, store *keyStore, provider string, replies map[string]*mockHTTP) *modelResolver {
	t.Helper()
	info, ok := providerByID(provider)
	if !ok {
		t.Fatalf("missing provider %s", provider)
	}
	resolver := info.New(store, "synthetic-key").(*modelResolver)
	build := resolver.build
	resolver.build = func(model, effort string) aiClient {
		mock := replies[model]
		if mock == nil {
			t.Fatalf("unexpected model %q", model)
		}
		client := build(model, effort)
		switch c := client.(type) {
		case *openAIClient:
			c.http = mock
		case *geminiClient:
			c.http = mock
		case *anthropicClient:
			c.http = mock
		default:
			t.Fatalf("unexpected client for %s", provider)
		}
		return client
	}
	resolver.list = func(context.Context, string) ([]discoveredModel, error) {
		t.Fatal("unexpected model discovery")
		return nil, nil
	}
	return resolver
}

func checkEffortInRequest(t *testing.T, mock *mockHTTP, model, effort string) bool {
	t.Helper()
	if mock.calls != 1 || mock.lastReq == nil {
		t.Fatalf("requests for %s = %d, want one", model, mock.calls)
	}
	var req map[string]json.RawMessage
	if err := json.NewDecoder(mock.lastReq.Body).Decode(&req); err != nil {
		t.Fatal(err)
	}
	if string(req["model"]) != strconv.Quote(model) {
		t.Errorf("wire model = %s, want %q", req["model"], model)
	}
	raw, present := req["reasoning_effort"]
	if wantPresent := effort != ""; present != wantPresent || (present && string(raw) != strconv.Quote(effort)) {
		t.Errorf("reasoning_effort = %s, present=%v, want %q (Recommended must omit the field)", raw, present, effort)
	}
	_, hasTemperature := req["temperature"]
	return hasTemperature
}

func TestAIReasoningEffortOnWire(t *testing.T) {
	for _, tc := range []struct{ provider, model, saved, want string }{
		{providerOpenAI, "gpt-5", "", ""},
		{providerOpenAI, "gpt-5", "high", "high"},
		{providerOpenAI, "gpt-5-mini", "minimal", "minimal"},
		{providerOpenAI, "gpt-5-nano-2025-08-07", "medium", "medium"},
		{providerOpenAI, "gpt-5.4", "none", "none"},
		{providerOpenAI, "gpt-5.4", "", ""},
		{providerOpenAI, "gpt-5.4-2026-03-05", "xhigh", "xhigh"},
		{providerOpenAI, "gpt-5.4", "minimal", ""},
		{providerOpenAI, "gpt-4.1", "high", ""},
		{providerGrok, "gpt-5", "high", ""},
	} {
		t.Run(tc.provider+"/"+tc.model+"/"+tc.saved, func(t *testing.T) {
			store := newTestKeyStore()
			store.setOverrideModel(tc.provider, tc.model)
			store.prefs.SetString(reasoningEffortKey(tc.provider, tc.model), tc.saved)
			mock := &mockHTTP{statusCode: 200, body: `{"choices":[{"message":{"content":"OK"}}]}`}
			r := effortTestResolver(t, store, tc.provider, map[string]*mockHTTP{tc.model: mock})
			if got, err := r.generate(context.Background(), "A synthetic question."); err != nil || got != "OK" {
				t.Fatalf("generate = %q, %v", got, err)
			}
			if hasTemp := checkEffortInRequest(t, mock, tc.model, tc.want); hasTemp == openAIFixedTemperature(tc.model) {
				t.Errorf("wrong temperature presence %v for model %s", hasTemp, tc.model)
			}
		})
	}
}

func TestAIReasoningEffortFollowsRediscoveredModel(t *testing.T) {
	for _, replacement := range []struct{ model, effort, label string }{
		{"gpt-5.4", "xhigh", "Extra high"},
		{"gpt-5-future", "", ""},
	} {
		t.Run(replacement.model, func(t *testing.T) {
			store := newTestKeyStore()
			setAIReasoningEffort(store.prefs, providerOpenAI, openAIModel, "high")
			setAIReasoningEffort(store.prefs, providerOpenAI, replacement.model, replacement.effort)
			gone := &mockHTTP{statusCode: http.StatusNotFound, body: `{"error":{"message":"Unknown model"}}`}
			good := &mockHTTP{statusCode: http.StatusOK, body: `{"choices":[{"message":{"content":"OK"}}]}`}
			r := effortTestResolver(t, store, providerOpenAI, map[string]*mockHTTP{openAIModel: gone, replacement.model: good})
			r.list = func(context.Context, string) ([]discoveredModel, error) {
				return []discoveredModel{{id: replacement.model, rank: 1}}, nil
			}
			var reported []aiModelInUse
			ctx := withAIModelObserver(context.Background(), func(m aiModelInUse) { reported = append(reported, m) })
			if got, err := r.generate(ctx, "A synthetic question."); err != nil || got != "OK" {
				t.Fatalf("generate = %q, %v", got, err)
			}
			checkEffortInRequest(t, gone, openAIModel, "high")
			checkEffortInRequest(t, good, replacement.model, replacement.effort)
			want := []aiModelInUse{
				{provider: providerOpenAI, model: openAIModel, effortLabel: "High", lowerEffort: "medium"},
				{provider: providerOpenAI, model: replacement.model, effortLabel: replacement.label},
			}
			if replacement.effort == "xhigh" {
				want[1].lowerEffort = "high"
			}
			if !slices.Equal(reported, want) {
				t.Errorf("progress reports = %+v, want %+v", reported, want)
			}
		})
	}
}

func TestGeminiReasoningEffortOnWire(t *testing.T) {
	for _, tc := range []struct{ model, saved, want string }{
		{"gemini-4-carbon", "", ""},
		{"gemini-4-carbon", "low", "LOW"},
		{"gemini-4-carbon", "medium", "MEDIUM"},
		{"gemini-4-carbon", "high", "HIGH"},
		{"gemini-4-carbon", "minimal", ""},
		{"gemini-4-carbon", "xhigh", ""},
		{"gemini-pro-latest", "high", ""},
	} {
		t.Run(tc.model+"/"+tc.saved, func(t *testing.T) {
			store := newTestKeyStore()
			store.setOverrideModel(providerGemini, tc.model)
			store.prefs.SetString(reasoningEffortKey(providerGemini, tc.model), tc.saved)
			mock := &mockHTTP{statusCode: http.StatusOK, body: `{"candidates":[{"content":{"parts":[{"thought":true,"text":"Synthetic summary."},{"text":"Synthetic "},{"text":"answer."}]},"finishReason":"STOP"}]}`}
			r := effortTestResolver(t, store, providerGemini, map[string]*mockHTTP{tc.model: mock})
			if got, err := r.generate(context.Background(), "Synthetic question."); err != nil || got != "Synthetic answer." {
				t.Fatalf("generate = %q, %v", got, err)
			}
			if mock.calls != 1 || mock.lastReq == nil || mock.lastReq.URL.Path != "/v1beta/models/"+tc.model+":generateContent" {
				t.Fatalf("expected one generateContent request for %s", tc.model)
			}
			var req struct {
				Config map[string]json.RawMessage `json:"generationConfig"`
			}
			if err := json.NewDecoder(mock.lastReq.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if string(req.Config["maxOutputTokens"]) != strconv.Itoa(aiMaxOutputTokens) {
				t.Fatal("missing original output allowance")
			}
			config, present := req.Config["thinkingConfig"]
			if tc.want == "" {
				if present {
					t.Fatalf("unexpected thinkingConfig: %s", config)
				}
				return
			}
			var thinking map[string]string
			if err := json.Unmarshal(config, &thinking); err != nil || len(thinking) != 1 || thinking["thinkingLevel"] != tc.want {
				t.Fatalf("thinkingConfig = %s, want only thinkingLevel %q", config, tc.want)
			}
		})
	}
}

func TestGeminiThinkingOnlyAtTokenLimit(t *testing.T) {
	out, err := new(geminiResponse).parseText([]byte(`{"candidates":[{"content":{"parts":[{"thought":true,"text":"Synthetic summary."}]},"finishReason":"MAX_TOKENS"}]}`))
	if out != "" || !errors.Is(err, errBudgetExhausted) {
		t.Fatalf("thinking-only completion produced %q, %v", out, err)
	}
}

func TestRejectedGeminiThinkingLevelDoesNotReplaceModel(t *testing.T) {
	store := newTestKeyStore()
	const model = "gemini-4-carbon"
	store.setResolvedModel(providerGemini, model)
	setAIReasoningEffort(store.prefs, providerGemini, model, "high")
	mock := &mockHTTP{statusCode: http.StatusBadRequest, body: `{"error":{"message":"Invalid thinkingLevel for this model.","status":"INVALID_ARGUMENT"}}`}
	r := effortTestResolver(t, store, providerGemini, map[string]*mockHTTP{model: mock})
	_, err := r.generate(context.Background(), "Synthetic question.")
	if err == nil || isModelNotFound(err) || mock.calls != 1 {
		t.Fatalf("thinking-level refusal produced %v with %d calls", err, mock.calls)
	}
}

func TestRejectedReasoningEffortDoesNotReplaceModel(t *testing.T) {
	store := newTestKeyStore()
	setAIReasoningEffort(store.prefs, providerOpenAI, openAIModel, "high")
	mock := &mockHTTP{statusCode: http.StatusBadRequest, body: `{"error":{"message":"This value is not supported with this model.","param":"reasoning_effort"}}`}
	r := effortTestResolver(t, store, providerOpenAI, map[string]*mockHTTP{openAIModel: mock})
	_, err := r.generate(context.Background(), "A synthetic question.")
	if err == nil || isModelNotFound(err) {
		t.Fatalf("parameter rejection produced %v", err)
	}
	checkEffortInRequest(t, mock, openAIModel, "high")
}
