package bibletext

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestGrokReasoningEfforts(t *testing.T) {
	for _, tc := range []struct {
		model string
		want  []string
	}{
		{"grok-4.3", []string{"none", "low", "medium", "high"}},
		{"grok-4.5", []string{"low", "medium", "high"}},
		{"grok-4.5-latest", []string{"low", "medium", "high"}},
		{"grok-4.6", []string{"low", "medium", "high", "xhigh"}},
		{"grok-4.7", []string{"low", "medium", "high", "xhigh"}},
		{"grok-4.20-multi-agent", nil},
		{"grok-4.20-0309-non-reasoning", nil},
		{"grok-synthetic-future", nil},
	} {
		if got := grokEfforts(tc.model); !slices.Equal(got, tc.want) {
			t.Errorf("offline efforts for %s = %v, want %v", tc.model, got, tc.want)
		}
	}

	live := []string{"low", "high", "xhigh"}
	for _, model := range []string{"grok-4.3", "grok-4.5-latest"} {
		if got := grokCatalogEfforts(model, live); !slices.Equal(got, []string{"low", "high"}) {
			t.Errorf("%s offers a value that runs as high: %v", model, got)
		}
	}
	if got := grokCatalogEfforts("grok-synthetic-future", live); !slices.Equal(got, []string{"low", "high", "xhigh"}) {
		t.Errorf("new model lost its advertised depths: %v", got)
	}
	for _, model := range []string{"grok-4.20-multi-agent-0309", "grok-4.20-0309-non-reasoning", "grok-voice-latest", "grok-imagine-image-2.0"} {
		if got := grokCatalogEfforts(model, live); got == nil || len(got) != 0 {
			t.Errorf("model with a distinct purpose %s has levels %v", model, got)
		}
	}
}

func TestGrokReasoningEffortWire(t *testing.T) {
	for _, tc := range []struct{ model, saved, want string }{
		{"grok-4.3", "none", "none"},
		{"grok-4.5", "high", "high"},
		{"grok-4.5", "xhigh", ""},
		{"grok-4.7", "xhigh", "xhigh"},
		{"grok-4.7", "", ""},
		{"grok-4.20-multi-agent", "high", ""},
	} {
		t.Run(tc.model+"/"+tc.saved, func(t *testing.T) {
			store := newTestKeyStore()
			store.setOverrideModel(providerGrok, tc.model)
			store.prefs.SetString(reasoningEffortKey(providerGrok, tc.model), tc.saved)
			mock := &mockHTTP{statusCode: 200, body: `{"choices":[{"message":{"content":"OK"}}]}`}
			r := effortTestResolver(t, store, providerGrok, map[string]*mockHTTP{tc.model: mock})
			if _, err := r.generate(context.Background(), "A synthetic question."); err != nil {
				t.Fatal(err)
			}
			checkEffortInRequest(t, mock, tc.model, tc.want)
		})
	}
}

func TestCompatibleModelListReadsOptionalEffortMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/models" || r.Header.Get("Authorization") != "Bearer synthetic-key" {
			t.Errorf("unexpected model-list request")
		}
		io.WriteString(w, `{"data":[
			{"id":"grok-synthetic","created":11,"aliases":["grok-synthetic-latest"],"capabilities":{"reasoning_effort":["medium","xhigh"],"default_reasoning_effort":"xhigh"}},
			{"id":"synthetic-empty","capabilities":{"reasoning_effort":[]}},
			{"id":"synthetic-absent"},
			{"id":"synthetic-malformed","capabilities":{"reasoning_effort":["low",4]}}
		]}`)
	}))
	defer srv.Close()
	models, err := listOpenAIModels(srv.URL)(context.Background(), "synthetic-key")
	if err != nil || len(models) != 4 {
		t.Fatalf("models = %v, error %v", models, err)
	}
	first := models[0]
	if first.id != "grok-synthetic" || first.rank != 11 || !slices.Equal(first.aliases, []string{"grok-synthetic-latest"}) ||
		!slices.Equal(first.efforts, []string{"medium", "xhigh"}) {
		t.Errorf("incomplete provider model metadata: %+v", first)
	}
	catalog := effortCatalogFromModels(providerGrok, models)
	for _, id := range []string{first.id, first.aliases[0]} {
		if got := verifiedDefaultEffort(nil, providerGrok, id, catalog, catalog.Levels[id]); got != "xhigh" {
			t.Errorf("default reported by /models for %s = %q, want xhigh", id, got)
		}
	}
	if models[1].efforts == nil || len(models[1].efforts) != 0 || models[2].efforts != nil || models[3].efforts != nil {
		t.Errorf("explicit empty, missing, or malformed capabilities changed meaning: %+v", models)
	}
}
