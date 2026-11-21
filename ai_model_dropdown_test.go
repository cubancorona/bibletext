package bibletext

// Tests for the settings model dropdown's data shaping: the provider's live
// model list filtered to chat-capable ids, stable-first, newest-first.

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"fyne.io/fyne/v2"
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

func TestDropdownModelIDsKeepsEveryEligibleModel(t *testing.T) {
	const count = 250
	var models []discoveredModel
	for i := 0; i < count; i++ {
		models = append(models, discoveredModel{id: fmt.Sprintf("model-%03d", i), rank: int64(i)})
	}
	models = append(models, discoveredModel{id: "test-embedding"}, models[0])
	got := dropdownModelIDs(models, nil, "gemini")
	if len(got) != count {
		t.Fatalf("dropdown has %d choices for %d distinct eligible models", len(got), count)
	}
	for i, id := range got {
		if want := fmt.Sprintf("model-%03d", count-1-i); id != want {
			t.Fatalf("choice %d = %s, want %s", i, id, want)
		}
	}
}

func TestGeminiModelListPaginates(t *testing.T) {
	const cursor = "next+&/=?"
	for _, tc := range []struct {
		name, second string
		wantErr      bool
	}{
		{"two pages", `{"models":[{"name":"models/gemini-second","supportedGenerationMethods":["generateContent"]}]}`, false},
		{"repeated token", `{"nextPageToken":"next+&/=?"}`, true},
		{"invalid second page", `{"models":`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query()
				if r.Method != http.MethodGet || r.URL.Path != "/v1beta/models" || q.Get("pageSize") != "1000" || r.Header.Get("x-goog-api-key") != "synthetic-key" {
					t.Errorf("invalid model-list request")
				}
				switch requests.Add(1) {
				case 1:
					if q.Has("pageToken") {
						t.Error("first request should have no page token")
					}
					io.WriteString(w, `{"models":[{"name":"models/gemini-first","supportedGenerationMethods":["generateContent"]},{"name":"models/embed","supportedGenerationMethods":["embedContent"]}],"nextPageToken":"next+&/=?"}`)
				case 2:
					if q.Get("pageToken") != cursor || len(q) != 2 {
						t.Errorf("opaque page token did not round-trip")
					}
					io.WriteString(w, tc.second)
				default:
					t.Error("requested another page after an invalid or repeated cursor")
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer server.Close()
			models, err := listGeminiModels(server.URL+"/v1beta")(context.Background(), "synthetic-key")
			if requests.Load() != 2 {
				t.Fatalf("got %d requests, want two", requests.Load())
			}
			if tc.wantErr {
				if err == nil || models != nil {
					t.Fatalf("incomplete catalog returned %v, %v", models, err)
				}
				return
			}
			if err != nil || len(models) != 2 || models[0].id != "gemini-first" || models[1].id != "gemini-second" {
				t.Fatalf("paginated catalog returned %v, %v", models, err)
			}
		})
	}
}

func TestLongModelPickerScrollsToAndSelectsLastModel(t *testing.T) {
	const count = 150
	state, _ := desktopWindow(t, fyne.NewSize(600, 650))
	var initialHeight float32
	settingsWithListedModels(t, state, count, func(settings fyne.CanvasObject) {
		model := findTreeButton(settings, "Recommended")
		if model == nil {
			t.Fatal("missing Model button")
		}
		model.OnTapped()
		list := settingsPickerList(t, state)
		if got := list.Length(); got != 1 {
			t.Fatalf("picker has %d options while loading, want only Recommended", got)
		}
		initialHeight = list.Size().Height
	})
	list := settingsPickerList(t, state)
	if got := list.Length(); got != count+1 {
		t.Fatalf("picker has %d rows, want Recommended and all %d eligible models", got, count)
	}
	if height := list.CreateItem().MinSize().Height; height != 44 {
		t.Fatalf("model row height is %g, want 44", height)
	}
	if height := list.Size().Height; height <= initialHeight || height >= state.window.Canvas().Size().Height {
		t.Fatalf("open list did not grow to fit the window: height %g, previously %g", height, initialHeight)
	}
	list.ScrollToBottom()
	if offset := list.GetScrollOffset(); offset <= 44 {
		t.Fatalf("last model was not reached by scrolling: offset %g", offset)
	}
	list.Select(count)
	if got := state.keys().overrideModel(defaultProviderID); got != "gemini-sample-00" {
		t.Fatalf("selecting the final model saved %q", got)
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
