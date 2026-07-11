package bibletext

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// stubClient is an aiClient whose reply depends only on the model it was built
// for — the seam that lets us drive modelResolver without real HTTP.
type stubClient struct {
	model string
	reply map[string]string // model -> answer; a missing model 404s (retired)
	other error             // if set, returned instead (non-model error path)
}

func (s stubClient) generate(_ context.Context, _ string) (string, error) {
	if s.other != nil {
		return "", s.other
	}
	if ans, ok := s.reply[s.model]; ok {
		return ans, nil
	}
	return "", &apiHTTPError{StatusCode: http.StatusNotFound, Details: `model: "` + s.model + `" not found`}
}

func TestIsModelNotFound(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"404 → yes", &apiHTTPError{StatusCode: 404}, true},
		{"400 mentioning model → yes", &apiHTTPError{StatusCode: 400, Details: "The model `x` does not exist"}, true},
		{"400 without model → no", &apiHTTPError{StatusCode: 400, Details: "input too long"}, false},
		{"401 bad key → no", &apiHTTPError{StatusCode: 401, Details: "invalid model? no"}, false},
		{"429 busy → no", &apiHTTPError{StatusCode: 429}, false},
		{"500 → no", &apiHTTPError{StatusCode: 500}, false},
		{"plain error → no", errors.New("model gone"), false},
		{"nil → no", nil, false},
	}
	for _, c := range cases {
		if got := isModelNotFound(c.err); got != c.want {
			t.Errorf("%s: isModelNotFound=%v want %v", c.name, got, c.want)
		}
	}
}

func TestPickInTier(t *testing.T) {
	// Newest in-tier stable wins; non-chat variants and out-of-tier are excluded.
	models := []discoveredModel{
		{"claude-3-5-haiku-20241022", 100},
		{"claude-haiku-4-5", 200},
		{"claude-haiku-5-preview", 250}, // newer but preview → loses to stable
		{"claude-opus-4-8", 300},        // out of tier → never picked
	}
	got, ok := pickInTier(models, "haiku")
	if !ok || got != "claude-haiku-4-5" {
		t.Fatalf("haiku pick = %q,%v; want claude-haiku-4-5", got, ok)
	}

	// OpenAI: "mini" variants that can't chat must be dropped even though they
	// contain the tier keyword.
	oai := []discoveredModel{
		{"gpt-4o-mini", 10},
		{"gpt-4o-mini-tts", 20},
		{"gpt-4o-mini-realtime-preview", 30},
		{"gpt-4o-mini-search-preview", 40},
		{"gpt-5-mini", 50}, // newest real mini → wins
	}
	if got, ok := pickInTier(oai, "mini"); !ok || got != "gpt-5-mini" {
		t.Fatalf("mini pick = %q,%v; want gpt-5-mini", got, ok)
	}

	// Preview used only when nothing stable is in tier.
	if got, ok := pickInTier([]discoveredModel{{"grok-9-beta", 1}}, "grok"); !ok || got != "grok-9-beta" {
		t.Fatalf("beta-only pick = %q,%v; want grok-9-beta", got, ok)
	}

	// No in-tier match → no fallback to an out-of-tier (pricier) model.
	if got, ok := pickInTier([]discoveredModel{{"claude-opus-4-8", 1}}, "haiku"); ok {
		t.Errorf("expected no haiku match, got %q", got)
	}
}

func newTestKeyStore() *keyStore { return newKeyStoreWith(newFakePrefs()) }

func TestResolverSelfHeals(t *testing.T) {
	store := newTestKeyStore()
	listed := false
	r := &modelResolver{
		store: store, id: providerAnthropic, def: "claude-haiku-4-5", tier: "haiku",
		list: func(_ context.Context, _ string) ([]discoveredModel, error) {
			listed = true
			return []discoveredModel{{"claude-haiku-9-1", 900}, {"claude-opus-4-8", 999}}, nil
		},
		// The default is retired (not in reply); only the discovered model answers.
		build: func(m string) aiClient {
			return stubClient{model: m, reply: map[string]string{"claude-haiku-9-1": "OK"}}
		},
	}

	out, err := r.generate(context.Background(), "hi")
	if err != nil || out != "OK" {
		t.Fatalf("expected self-heal to OK, got %q err=%v", out, err)
	}
	if !listed {
		t.Error("expected the models endpoint to be queried on a retired model")
	}
	if got := store.resolvedModel(providerAnthropic); got != "claude-haiku-9-1" {
		t.Errorf("discovered model not cached: %q", got)
	}

	// Second call must reuse the cached model — no re-discovery.
	listed = false
	if out, err := r.generate(context.Background(), "again"); err != nil || out != "OK" {
		t.Fatalf("cached path failed: %q err=%v", out, err)
	}
	if listed {
		t.Error("second call should use the cached model, not re-list")
	}
}

func TestResolverModelGoneWhenNoReplacement(t *testing.T) {
	store := newTestKeyStore()
	r := &modelResolver{
		store: store, id: providerAnthropic, def: "claude-haiku-4-5", tier: "haiku",
		list: func(_ context.Context, _ string) ([]discoveredModel, error) {
			return []discoveredModel{{"claude-opus-4-8", 1}}, nil // nothing in-tier
		},
		build: func(m string) aiClient { return stubClient{model: m, reply: map[string]string{}} },
	}
	_, err := r.generate(context.Background(), "hi")
	var mg modelGoneError
	if !errors.As(err, &mg) {
		t.Fatalf("expected modelGoneError, got %v", err)
	}
}

func TestResolverSurfacesDiscoveryError(t *testing.T) {
	// The default model is retired (self-heal fires), but the models-list call
	// fails — e.g. the account has no credits (xAI 403). The resolver must surface
	// THAT error, not a misleading "model unavailable".
	store := newTestKeyStore()
	listErr := &apiHTTPError{StatusCode: http.StatusForbidden, Details: "no credits or licenses"}
	r := &modelResolver{
		store: store, id: providerGrok, def: "grok-2-latest", tier: "grok",
		list:  func(_ context.Context, _ string) ([]discoveredModel, error) { return nil, listErr },
		build: func(m string) aiClient { return stubClient{model: m, reply: map[string]string{}} },
	}
	_, err := r.generate(context.Background(), "hi")
	if !errors.Is(err, error(listErr)) {
		t.Fatalf("expected the discovery (403) error to surface, got %v", err)
	}
	var mg modelGoneError
	if errors.As(err, &mg) {
		t.Error("should not report modelGoneError when the real problem is the list call failing")
	}
}

func TestResolverPassesNonModelErrorsThrough(t *testing.T) {
	store := newTestKeyStore()
	authErr := &apiHTTPError{StatusCode: http.StatusUnauthorized, Details: "bad key"}
	discovered := false
	r := &modelResolver{
		store: store, id: providerAnthropic, def: "claude-haiku-4-5", tier: "haiku",
		list: func(_ context.Context, _ string) ([]discoveredModel, error) {
			discovered = true
			return nil, nil
		},
		build: func(m string) aiClient { return stubClient{model: m, other: authErr} },
	}
	_, err := r.generate(context.Background(), "hi")
	if !errors.Is(err, error(authErr)) {
		t.Fatalf("expected the auth error to pass through, got %v", err)
	}
	if discovered {
		t.Error("a non-model error must NOT trigger model discovery")
	}
}

func TestResolverResolutionOrder(t *testing.T) {
	store := newTestKeyStore()
	r := &modelResolver{store: store, id: providerAnthropic, def: "default-model"}

	if got := r.currentModel(); got != "default-model" {
		t.Errorf("with nothing set, want default; got %q", got)
	}
	store.setResolvedModel(providerAnthropic, "healed-model")
	if got := r.currentModel(); got != "healed-model" {
		t.Errorf("cached discovered model should win over default; got %q", got)
	}
	store.setOverrideModel(providerAnthropic, "pinned-model")
	if got := r.currentModel(); got != "pinned-model" {
		t.Errorf("user override must win over everything; got %q", got)
	}
}

func TestParseVersionRank(t *testing.T) {
	cases := map[string]int64{
		"gemini-2.5-flash": 25,
		"gemini-2.0-flash": 20,
		"gemini-1.5-flash": 15,
		"gemini-flash":     0,
		"flash-3":          30,
	}
	for id, want := range cases {
		if got := parseVersionRank(id); got != want {
			t.Errorf("parseVersionRank(%q)=%d want %d", id, got, want)
		}
	}
}
