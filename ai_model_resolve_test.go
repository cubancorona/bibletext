package bibletext

import (
	"context"
	"errors"
	"net/http"
	"strings"
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

func TestResolverDoesNotHealAnExplicitPin(t *testing.T) {
	store := newTestKeyStore()
	store.setOverrideModel(providerOpenAI, "gpt-5.5-pro") // user pinned this in Settings
	listed := false
	r := &modelResolver{
		store: store, id: providerOpenAI, def: "gpt-4o-mini", tier: "mini",
		list: func(_ context.Context, _ string) ([]discoveredModel, error) {
			listed = true
			return []discoveredModel{{"gpt-4o-mini", 400}}, nil
		},
		// The pinned gpt-5.5-pro 404s on the chat endpoint (Responses-only). The
		// mini-tier default WOULD answer — but we must NOT silently substitute it.
		build: func(m string) aiClient {
			return stubClient{model: m, reply: map[string]string{"gpt-4o-mini": "OK"}}
		},
	}

	out, err := r.generate(context.Background(), "hi")
	var mg modelGoneError
	if !errors.As(err, &mg) {
		t.Fatalf("a failed PIN must surface modelGoneError, got out=%q err=%v", out, err)
	}
	if listed {
		t.Error("must not re-list / self-heal a user's explicit pin")
	}
	if got := store.overrideModel(providerOpenAI); got != "gpt-5.5-pro" {
		t.Errorf("pin must be preserved for the user to change, got %q", got)
	}

	// Clearing the pin (choosing Recommended) restores self-heal to the default.
	store.setOverrideModel(providerOpenAI, "")
	if out, err := r.generate(context.Background(), "hi"); err != nil || out != "OK" {
		t.Fatalf("after unpinning, default must answer: out=%q err=%v", out, err)
	}
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
	got, ok := pickInTier(models, "haiku", nil)
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
	if got, ok := pickInTier(oai, "mini", nil); !ok || got != "gpt-5-mini" {
		t.Fatalf("mini pick = %q,%v; want gpt-5-mini", got, ok)
	}

	// Preview used only when nothing stable is in tier.
	if got, ok := pickInTier([]discoveredModel{{"grok-9-beta", 1}}, "grok", nil); !ok || got != "grok-9-beta" {
		t.Fatalf("beta-only pick = %q,%v; want grok-9-beta", got, ok)
	}

	// No in-tier match → no fallback to an out-of-tier (pricier) model.
	if got, ok := pickInTier([]discoveredModel{{"claude-opus-4-8", 1}}, "haiku", nil); ok {
		t.Errorf("expected no haiku match, got %q", got)
	}
}

// TestPickInTierHonorsExtraExclude: the capable-tier keyword "gpt-5" also
// matches OpenAI's "-pro" Responses-only family, which 404s on
// /chat/completions. Self-heal caches its pick, so selecting one would wedge
// the Recommended path (every retry rediscovers the same broken id). The
// provider's chat-endpoint exclusions must therefore reach pickInTier, not
// just the settings dropdown.
func TestPickInTierHonorsExtraExclude(t *testing.T) {
	models := []discoveredModel{
		{"gpt-5", 100},
		{"gpt-5.2-pro", 300}, // newest in tier, but Responses-API-only
		{"gpt-5.2", 200},
	}
	if got, ok := pickInTier(models, "gpt-5", openAIChatOnlyExclude); !ok || got != "gpt-5.2" {
		t.Fatalf("gpt-5 pick = %q,%v; want gpt-5.2 (the -pro id must be excluded)", got, ok)
	}
	// And the registry actually threads the exclusion into the resolver — the
	// dropdown filter and the self-heal filter must not drift apart.
	info, ok := providerByID(providerOpenAI)
	if !ok {
		t.Fatal("no OpenAI provider registered")
	}
	r, ok := info.New(newTestKeyStore(), "test-key").(*modelResolver)
	if !ok {
		t.Fatal("OpenAI client is not a modelResolver")
	}
	if len(r.extraExclude) == 0 {
		t.Fatal("OpenAI's modelResolver carries no extraExclude — self-heal could cache a -pro pick")
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

// The model-gone message must NAME the failing model and point at the model
// picker (with the self-healing "Recommended" escape) — a pinned model a key
// can't invoke is the common case (field-reported: a pinned gemini-2.5-pro on
// an un-entitled key), and "update the app" advice would be stale now that
// models are discovered live.
func TestFriendlyModelGoneNamesModelAndFix(t *testing.T) {
	msg := friendlyAIError(modelGoneError{provider: providerGemini, tried: "gemini-2.5-pro"})
	for _, want := range []string{"gemini-2.5-pro", "Recommended", "AI settings"} {
		if !strings.Contains(msg, want) {
			t.Errorf("model-gone message missing %q: %q", want, msg)
		}
	}
}

// Google rejects an invalid key with 400 + "API key not valid…" (not 401) —
// the friendly message must say key-rejected, not "selection too long"
// (field-reported via the settings sheet's Test key).
func TestFriendlyBadKey400SaysKeyRejected(t *testing.T) {
	err := &apiHTTPError{StatusCode: http.StatusBadRequest,
		Details: "API key not valid. Please pass a valid API key."}
	if msg := friendlyAIError(err); !strings.Contains(msg, "key was rejected") {
		t.Errorf("400 bad-key message = %q, want key-rejected", msg)
	}
	// A genuine 400 (oversized input) still reads as the selection message.
	err2 := &apiHTTPError{StatusCode: http.StatusBadRequest, Details: "input too long"}
	if msg := friendlyAIError(err2); !strings.Contains(msg, "shorter passage") {
		t.Errorf("400 too-long message = %q, want shorter-passage", msg)
	}
}

// TestPickInTierPrefersLightOverReasoning locks the backstop for tier keywords
// that cannot discriminate. SpaceXAI's keyword is "grok" — every model they
// sell matches it — so newest-wins can hand a reader a heavy reasoning model
// (measured 2026-08-07: a reasoning variant took ~48s on a broad Find against
// ~4s for the non-reasoning one, past the 35s timeout).
//
// LIMIT, deliberately locked here: this only works when the id SAYS so. Names
// like "grok-4.5" vs "grok-4.3" are opaque — a model list carries no cost or
// latency signal — so the primary defense against a slow self-heal stays the
// PINNED default (kept alive by TestLivePinnedDefaultsExist), with this as a
// backstop for the explicitly-marked variants.
func TestPickInTierPrefersLightOverReasoning(t *testing.T) {
	// Real SpaceXAI ids and creation ranks (2026-08-07), newest first.
	models := []discoveredModel{
		{"grok-4.20-0309-reasoning", 1773014402},     // newest, marked heavy
		{"grok-4.20-multi-agent-0309", 1773014401},   // marked heavy
		{"grok-4.20-0309-non-reasoning", 1773014400}, // oldest, but light
	}
	got, ok := pickInTier(models, "grok", nil)
	if !ok {
		t.Fatal("expected an in-tier pick")
	}
	if got != "grok-4.20-0309-non-reasoning" {
		t.Errorf("a light in-tier model must beat NEWER marked-heavy ones, got %q", got)
	}

	// Opaque ids are not guessable — newest still wins, which is exactly why the
	// pin matters. Locked so the limitation is visible rather than assumed away.
	opaque := []discoveredModel{{"grok-4.5", 1782691200}, {"grok-4.3", 1776384000}}
	if got, ok := pickInTier(opaque, "grok", nil); !ok || got != "grok-4.5" {
		t.Errorf("opaque ids fall back to newest-wins: got %q ok=%v", got, ok)
	}

	// The discriminating tiers keep pure newest-wins.
	anth := []discoveredModel{{"claude-haiku-4-5", 2}, {"claude-haiku-3-5", 1}}
	if got, ok := pickInTier(anth, "haiku", nil); !ok || got != "claude-haiku-4-5" {
		t.Errorf("haiku tier should still take the newest: got %q ok=%v", got, ok)
	}

	// A tier holding ONLY heavy models still resolves (better than nothing).
	only := []discoveredModel{{"grok-9-reasoning", 3}, {"grok-8-reasoning", 2}}
	if got, ok := pickInTier(only, "grok", nil); !ok || got != "grok-9-reasoning" {
		t.Errorf("all-heavy tier must still resolve to the newest: got %q ok=%v", got, ok)
	}
}
