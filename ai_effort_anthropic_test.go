package bibletext

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestAnthropicEffortOnWire(t *testing.T) {
	for _, tc := range []struct {
		model, choice, expected string
		budget                  int
	}{
		{"claude-opus-5", "", "", 0},
		{"claude-haiku-5-5", "xhigh", "xhigh", 0},
		{"claude-opus-5", "max", "max", 0},
		{"claude-opus-4-8", "high", "high", 0},
		{"claude-opus-4-8", "", "", 0},
		{"claude-sonnet-4-6", "xhigh", "", 0},
		{"claude-mythos-preview", "max", "max", 0},
		{"claude-opus-4-5-20251101", "medium", "medium", 0},
		{"claude-opus-4-5", "max", "", 0},
		{"claude-haiku-4-5-20251001", "4096", "", 4096},
		{"claude-sonnet-4-5", "1024", "", 1024},
		{"claude-sonnet-4-5", "", "", 0},
		{"claude-unknown", "high", "", 0},
	} {
		t.Run(tc.model+"/"+tc.choice, func(t *testing.T) {
			store := newTestKeyStore()
			store.setOverrideModel(providerAnthropic, tc.model)
			store.prefs.SetString(reasoningEffortKey(providerAnthropic, tc.model), tc.choice)
			mock := &mockHTTP{statusCode: http.StatusOK, body: `{"content":[{"type":"thinking","thinking":"internal"},{"type":"redacted_thinking","data":"hidden"},{"type":"text","text":"Synthetic answer."}],"stop_reason":"end_turn"}`}
			resolver := effortTestResolver(t, store, providerAnthropic, map[string]*mockHTTP{tc.model: mock})
			out, err := resolver.generate(context.Background(), "Synthetic prompt.")
			if err != nil || out != "Synthetic answer." || mock.calls != 1 {
				t.Fatalf("generate returned %q, %v; sends %d", out, err, mock.calls)
			}
			var req struct {
				Model        string                   `json:"model"`
				OutputConfig *struct{ Effort string } `json:"output_config"`
				Thinking     *struct {
					Type         string
					BudgetTokens int `json:"budget_tokens"`
				} `json:"thinking"`
			}
			if err := json.NewDecoder(mock.lastReq.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if req.Model != tc.model || (req.OutputConfig != nil) != (tc.expected != "") || req.OutputConfig != nil && req.OutputConfig.Effort != tc.expected {
				t.Fatalf("unexpected named effort for %s: %+v", tc.model, req.OutputConfig)
			}
			if tc.model == "claude-opus-4-8" && tc.choice == "high" {
				if req.Thinking == nil || req.Thinking.Type != "adaptive" || req.Thinking.BudgetTokens != 0 {
					t.Fatal("the selected effort did not turn on adaptive reasoning")
				}
			} else if (req.Thinking != nil) != (tc.budget > 0) || req.Thinking != nil && (req.Thinking.Type != "enabled" || req.Thinking.BudgetTokens != tc.budget) {
				t.Fatalf("unexpected manual budget for %s: %+v", tc.model, req.Thinking)
			}
		})
	}
}

func TestAnthropicModelListSuppliesEffortCapabilities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.URL.Query().Get("limit") != "1000" {
			t.Errorf("unexpected discovery path %s", r.URL)
		}
		switch r.URL.Query().Get("after_id") {
		case "":
			fmt.Fprint(w, `{"data":[{"id":"new-model","capabilities":{"effort":{"supported":true,"low":{"supported":true},"high":{"supported":false},"max":{"supported":true},"ultra":{"supported":true}}}}],"has_more":true,"last_id":"new-model"}`)
		case "new-model":
			fmt.Fprint(w, `{"data":[{"id":"manual","capabilities":{"effort":{"supported":false},"thinking":{"types":{"enabled":{"supported":true}}}}},{"id":"fixed","capabilities":{"effort":{"supported":false},"thinking":{"types":{"enabled":{"supported":false}}}}},{"id":"unreported","capabilities":null}]}`)
		default:
			t.Errorf("unexpected cursor %q", r.URL.Query().Get("after_id"))
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	models, err := listAnthropicModels(server.URL)(context.Background(), "synthetic-key")
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string][]string)
	for _, m := range models {
		got[m.id] = m.efforts
	}
	want := map[string][]string{"new-model": {"low", "max", "ultra"}, "manual": {"1024", "4096", "8192"}, "fixed": {}, "unreported": nil}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listed effort capabilities = %#v, want %#v", got, want)
	}
}
