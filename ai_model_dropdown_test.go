package bibletext

// Tests for the settings model dropdown's data shaping: the provider's live
// model list filtered to chat-capable ids, stable-first, newest-first, capped.

import (
	"fmt"
	"testing"
)

func TestDropdownModelIDs(t *testing.T) {
	models := []discoveredModel{
		{id: "gemini-2.5-flash", rank: 2_500},
		{id: "gemini-2.5-flash", rank: 2_500}, // duplicate — must collapse
		{id: "gemini-2.0-flash", rank: 2_000},
		{id: "gemini-2.5-pro", rank: 2_500},
		{id: "gemini-2.5-flash-preview-05-20", rank: 2_501}, // unstable → after stable
		{id: "gemini-2.5-flash-image", rank: 2_500},         // non-chat → dropped
		{id: "gemini-embedding-001", rank: 1},               // non-chat → dropped
		{id: "gemini-2.5-flash-tts", rank: 2_500},           // non-chat → dropped
		{id: "", rank: 9},                                   // empty → dropped
	}

	got := dropdownModelIDs(models, nil, "gemini")
	want := []string{
		"gemini-2.5-pro",   // rank tie with flash → id desc tiebreak
		"gemini-2.5-flash", // stable, newest
		"gemini-2.0-flash",
		"gemini-2.5-flash-preview-05-20", // unstable last despite higher rank
	}
	if len(got) != len(want) {
		t.Fatalf("dropdownModelIDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dropdownModelIDs = %v, want %v", got, want)
		}
	}
}

func TestDropdownModelIDsCap(t *testing.T) {
	var models []discoveredModel
	for i := 0; i < dropdownModelCap+20; i++ {
		models = append(models, discoveredModel{id: fmt.Sprintf("model-%03d", i), rank: int64(i)})
	}
	got := dropdownModelIDs(models, nil, "gemini")
	if len(got) != dropdownModelCap {
		t.Fatalf("dropdown must cap at %d, got %d", dropdownModelCap, len(got))
	}
	// Newest (highest rank) first.
	if got[0] != fmt.Sprintf("model-%03d", dropdownModelCap+19) {
		t.Fatalf("newest model must lead, got %s", got[0])
	}
}

func TestDropdownModelIDsExtraExclude(t *testing.T) {
	// OpenAI's list: the "-pro" Responses-only tier and codex coding models must
	// be dropped (extra + global), leaving the real chat models.
	models := []discoveredModel{
		{id: "gpt-5.5", rank: 5_500},
		{id: "gpt-5.5-pro", rank: 5_500},   // Responses-only → extra "-pro"
		{id: "gpt-5.2-codex", rank: 5_200}, // coding → global "codex"
		{id: "gpt-4o-mini", rank: 4_000},
	}
	got := dropdownModelIDs(models, []string{"-pro"}, "gpt")
	want := []string{"gpt-5.5", "gpt-4o-mini"}
	if len(got) != len(want) {
		t.Fatalf("dropdown = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dropdown = %v, want %v", got, want)
		}
	}
}

func TestExtraExcludeIsProviderScoped(t *testing.T) {
	// The "-pro" exclude is on OpenAI ONLY — Gemini's own dropdown must keep
	// gemini-2.5-pro (a valid chat model), because each provider lists only its
	// own models and applies only its own ExtraModelExclude.
	gemModels := []discoveredModel{{id: "gemini-2.5-pro", rank: 2_500}, {id: "gemini-2.5-flash", rank: 2_500}}
	for _, p := range aiProviders() {
		if p.ID == providerGemini {
			got := dropdownModelIDs(gemModels, p.ExtraModelExclude, modelFamilyOf(p.Model))
			if !containsExact(got, "gemini-2.5-pro") {
				t.Fatalf("Gemini dropdown must keep gemini-2.5-pro, got %v", got)
			}
		}
		if p.ID == providerOpenAI && !containsExact(p.ExtraModelExclude, "-pro") {
			t.Fatalf("OpenAI must exclude -pro from its dropdown; ExtraModelExclude=%v", p.ExtraModelExclude)
		}
	}
}

func TestDropdownModelIDsFamilyFirst(t *testing.T) {
	// A higher-ranked non-family model must not jump ahead of the provider's own
	// family (the real Gemini bug: gemma rank 40 > gemini rank 25).
	models := []discoveredModel{
		{id: "gemma-4-31b-it", rank: 4_000},
		{id: "gemini-2.5-flash", rank: 2_500},
	}
	got := dropdownModelIDs(models, nil, "gemini")
	if len(got) != 2 || got[0] != "gemini-2.5-flash" {
		t.Fatalf("provider family must lead over higher-ranked other family, got %v", got)
	}
}

func TestModelFamilyOf(t *testing.T) {
	for in, want := range map[string]string{
		"gemini-2.5-flash": "gemini", "gpt-4o-mini": "gpt",
		"claude-haiku-4-5": "claude", "grok-2-latest": "grok", "solo": "solo",
	} {
		if got := modelFamilyOf(in); got != want {
			t.Errorf("modelFamilyOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDropdownModelIDsEmpty(t *testing.T) {
	if got := dropdownModelIDs(nil, nil, ""); len(got) != 0 {
		t.Fatalf("nil input must yield no options, got %v", got)
	}
	only := []discoveredModel{{id: "text-embedding-3-small", rank: 5}}
	if got := dropdownModelIDs(only, nil, ""); len(got) != 0 {
		t.Fatalf("all-excluded input must yield no options, got %v", got)
	}
}
