package bibletext

// A model whose completion-token cap is below aiMaxOutputTokens refuses the
// budget with a 400 whose message names the model ("This model supports at
// most 16384 completion tokens"). Filed as model-not-found, that refusal made
// a pinned model read as gone: the faster-model offer for ChatGPT wrote
// gpt-4o-mini, whose cap is 16384, and every request on it was answered with
// "isn't available with your key". These tests drive the real OpenAI client
// and the model resolver against a stand-in for /chat/completions that
// answers as OpenAI does, so the retry, the request count and the budget of
// each request are what the provider would see.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// openAIStandIn imitates OpenAI's /chat/completions. caps maps each model it
// serves to that model's completion-token cap: a request over the cap gets
// the provider's refusal, one at or under it the answer, and a model not in
// the map the 404 an unknown or retired id gets. It records each request's
// model and max_completion_tokens in order.
type openAIStandIn struct {
	srv     *httptest.Server
	mu      sync.Mutex
	models  []string
	budgets []int
}

func newOpenAIStandIn(t *testing.T, caps map[string]int) *openAIStandIn {
	t.Helper()
	s := &openAIStandIn{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model               string `json:"model"`
			MaxCompletionTokens int    `json:"max_completion_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("stand-in: undecodable request: %v", err)
		}
		s.mu.Lock()
		s.models = append(s.models, req.Model)
		s.budgets = append(s.budgets, req.MaxCompletionTokens)
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		limit, ok := caps[req.Model]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"error":{"message":"The model '%s' does not exist or you do not have access to it.","type":"invalid_request_error","param":null,"code":"model_not_found"}}`, req.Model)
			return
		}
		if req.MaxCompletionTokens > limit {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":{"message":"max_tokens is too large: %d. This model supports at most %d completion tokens, whereas you provided %d.","type":"invalid_request_error","param":"max_tokens","code":null}}`,
				req.MaxCompletionTokens, limit, req.MaxCompletionTokens)
			return
		}
		fmt.Fprint(w, `{"choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}]}`)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *openAIStandIn) requests() (models []string, budgets []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.models...), append([]int(nil), s.budgets...)
}

// openAIResolverAgainst is the ChatGPT entry's resolver, as aiProviders builds
// it, pointed at the stand-in, with a model list the test supplies and a flag
// that records whether self-heal asked for it.
func openAIResolverAgainst(store *keyStore, s *openAIStandIn, listed []discoveredModel, asked *bool) *modelResolver {
	return &modelResolver{
		store: store, id: providerOpenAI, def: openAIModel, tier: "gpt-5", apiKey: "test-key",
		extraExclude: openAIChatOnlyExclude,
		list: func(context.Context, string) ([]discoveredModel, error) {
			*asked = true
			return listed, nil
		},
		build: func(m string) aiClient { return newOpenAIClient("test-key", s.srv.URL, m) },
	}
}

// A pinned model that refuses the budget as more than its cap is sent the
// same request once more at the cap it named, and answers: two requests, the
// second at 16384, no self-heal and the pin kept. The pin here is the id the
// faster-model offer wrote to devices until 2026-10-15.
func TestAPinnedModelWithALowerCompletionCapAnswersAtItsCap(t *testing.T) {
	const capped = "gpt-4o-mini"
	s := newOpenAIStandIn(t, map[string]int{capped: 16384})
	store := newTestKeyStore()
	store.setOverrideModel(providerOpenAI, capped)
	asked := false
	r := openAIResolverAgainst(store, s, []discoveredModel{{"gpt-5.2", 200}}, &asked)

	out, err := r.generate(context.Background(), "Reply with the single word: OK")
	var mg modelGoneError
	if errors.As(err, &mg) {
		t.Fatalf("a budget the model's cap refuses read as the model being gone: %v", err)
	}
	if err != nil || out != "OK" {
		t.Fatalf("generate = %q, %v; want the answer", out, err)
	}
	models, budgets := s.requests()
	if len(models) != 2 {
		t.Fatalf("%d requests, want 2 (the refusal and the retry at the cap): models %v budgets %v", len(models), models, budgets)
	}
	if budgets[0] != aiMaxOutputTokens || budgets[1] != 16384 {
		t.Errorf("budgets sent = %v, want [%d 16384]", budgets, aiMaxOutputTokens)
	}
	if models[0] != capped || models[1] != capped {
		t.Errorf("models sent = %v, want the pinned %q both times", models, capped)
	}
	if asked {
		t.Error("self-heal listed the models for a model that exists")
	}
	if got := store.overrideModel(providerOpenAI); got != capped {
		t.Errorf("pin = %q after the retry, want %q kept", got, capped)
	}

	// Control: a model whose cap is the budget answers in one request.
	roomy := newOpenAIStandIn(t, map[string]int{capped: aiMaxOutputTokens})
	r = openAIResolverAgainst(store, roomy, nil, &asked)
	if out, err := r.generate(context.Background(), "Reply with the single word: OK"); err != nil || out != "OK" {
		t.Fatalf("control: generate = %q, %v", out, err)
	}
	if models, _ := roomy.requests(); len(models) != 1 {
		t.Errorf("control: %d requests to a model that takes the budget, want 1", len(models))
	}
}

// A pinned model the provider does not have still reads as gone, in one
// request and with no self-heal: the budget retry is for the cap refusal
// alone.
func TestAPinnedModelTheProviderLacksStillReadsAsGone(t *testing.T) {
	s := newOpenAIStandIn(t, map[string]int{})
	store := newTestKeyStore()
	store.setOverrideModel(providerOpenAI, "gpt-5.5-pro")
	asked := false
	r := openAIResolverAgainst(store, s, []discoveredModel{{"gpt-5.2", 200}}, &asked)

	out, err := r.generate(context.Background(), "hi")
	var mg modelGoneError
	if !errors.As(err, &mg) {
		t.Fatalf("a 404 on a pinned model must surface modelGoneError, got out=%q err=%v", out, err)
	}
	if mg.tried != "gpt-5.5-pro" {
		t.Errorf("modelGoneError names %q, want the pinned model", mg.tried)
	}
	if models, _ := s.requests(); len(models) != 1 {
		t.Errorf("%d requests, want 1: a 404 is not retried", len(models))
	}
	if asked {
		t.Error("self-heal must not list the models for an explicit pin")
	}
}

// On the Recommended path a 404 on the default still self-heals: the live
// list is asked, the in-tier pick answers and is cached.
func TestTheRecommendedPathStillSelfHealsOnA404(t *testing.T) {
	s := newOpenAIStandIn(t, map[string]int{"gpt-5.2": aiMaxOutputTokens})
	store := newTestKeyStore()
	asked := false
	r := openAIResolverAgainst(store, s, []discoveredModel{{openAIModel, 100}, {"gpt-5.2", 200}}, &asked)

	out, err := r.generate(context.Background(), "hi")
	if err != nil || out != "OK" {
		t.Fatalf("generate = %q, %v; want the healed answer", out, err)
	}
	if !asked {
		t.Error("a 404 on the default must ask the provider for its current models")
	}
	models, _ := s.requests()
	if strings.Join(models, ",") != openAIModel+",gpt-5.2" {
		t.Errorf("models sent = %v, want the default then the healed pick", models)
	}
	if got := store.resolvedModel(providerOpenAI); got != "gpt-5.2" {
		t.Errorf("healed pick cached as %q, want gpt-5.2", got)
	}
}

// The economy model the waiting screens offer for ChatGPT is a model of the
// capable default's own family, so it takes the shared output budget and the
// fixed-temperature rule as the default does, and it is not the default,
// else the offer would never appear.
func TestTheChatGPTFastModelIsInTheCapableDefaultsFamily(t *testing.T) {
	if !strings.HasPrefix(openAIFastModel, "gpt-5") {
		t.Errorf("openAIFastModel = %q, want a gpt-5 family model", openAIFastModel)
	}
	if !openAIFixedTemperature(openAIFastModel) {
		t.Errorf("openAIFixedTemperature(%q) = false; the reasoning families reject an explicit temperature", openAIFastModel)
	}
	if openAIFastModel == openAIModel {
		t.Errorf("the fast model %q is the default; the offer would never show", openAIFastModel)
	}
	info, ok := providerByID(providerOpenAI)
	if !ok || info.FastModel != openAIFastModel {
		t.Errorf("the ChatGPT entry offers %q, want %q", info.FastModel, openAIFastModel)
	}
}
