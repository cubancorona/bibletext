package bibletext

import (
	"context"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// A Find already using its provider's fast model can still lower an explicit
// effort. Follow the actual tap through both layouts and into the resolver.
func TestFindRetriesAtLowerEffortOnlyForThisSearch(t *testing.T) {
	armAISpeedClock(t)
	for _, layout := range []string{"Search tab", "sidebar"} {
		t.Run(layout, func(t *testing.T) {
			sends := routeFind(t, "gemini-3-pro")
			var state *AppState
			var pane func() fyne.CanvasObject
			var ask func(string)
			var rebuild func()
			if layout == "Search tab" {
				h := liveFindTab(t)
				state, pane, ask, rebuild = h.state, h.pane, h.ask, h.flip
			} else {
				a := test.NewApp()
				t.Cleanup(a.Quit)
				state = sidebarFindState(t)
				var current fyne.CanvasObject
				pane = func() fyne.CanvasObject { return current }
				rebuild = func() { current = buildSearchResultsView(state) }
				state.showReading = rebuild
				ask = func(q string) {
					state.aiSearchQuery = q
					state.retryAISearch(aiSpeedChoice{})
				}
			}
			store := state.keys()
			store.setOverrideModel(providerGemini, geminiFastModel)
			rememberModelEfforts(store.prefs, providerGemini, []discoveredModel{
				{id: geminiFastModel, efforts: []string{"low", "medium", "high"}},
			})
			setAIReasoningEffort(store.prefs, providerGemini, geminiFastModel, "high")

			checkSend := func(want string) {
				t.Helper()
				got := nextSend(t, sends)
				if got.model != geminiFastModel || got.effort != want {
					t.Fatalf("sent to %q at %q, want %q at %q", got.model, got.effort, geminiFastModel, want)
				}
				wantModelLine(t, pane(), "Gemini (Google) · "+geminiFastModel+" · "+effortDisplayName(want), "actual Find attempt")
			}
			ask("First synthetic request")
			checkSend("high")
			retired := waitingAction(pane(), "Use less reasoning this time")
			if retired == nil || retired.detail != "Medium effort" {
				t.Fatalf("missing lower-effort action, displayed %v", treeTexts(pane()))
			}
			rebuild()
			gen := state.askSession.gen
			retired.Tapped(nil)
			if !state.askSession.Current(gen) {
				t.Fatal("a superseded Find control restarted the search")
			}
			lower := waitingAction(pane(), "Use less reasoning this time")
			if lower == nil || lower.detail != "Medium effort" {
				t.Fatalf("a rebuild lost the lower-effort action, displayed %v", treeTexts(pane()))
			}
			lower.Tapped(nil)
			checkSend("medium")
			if got := aiReasoningEffort(store.prefs, providerGemini, geminiFastModel); got != "high" {
				t.Fatalf("retry changed the saved effort to %q", got)
			}
			if low := waitingAction(pane(), "Use less reasoning this time"); low == nil || low.detail != "Low effort" {
				t.Fatalf("retry still offers options from the old attempt, displayed %v", treeTexts(pane()))
			}

			ask("Second synthetic request")
			checkSend("high")
			abandonAISearch(state)
		})
	}
}

func TestFindOneTimeEffortUsesItsOwnCachedAnswers(t *testing.T) {
	isolateAICache(t)
	store := newKeyStoreWith(newFakePrefs())
	store.setActiveProvider(providerOpenAI)
	store.setAPIKey(providerOpenAI, "synthetic-key")
	grantAIPermission(store, providerOpenAI)
	const model = "gpt-5-mini"
	store.setOverrideModel(providerOpenAI, model)
	setAIReasoningEffort(store.prefs, providerOpenAI, model, "high")
	state := &AppState{aiKeys: store}
	prev := aiSearchGenerate
	t.Cleanup(func() { aiSearchGenerate = prev })
	var sends []string
	aiSearchGenerate = func(ctx context.Context, _ providerInfo, _ *keyStore, _, _ string) (string, error) {
		sends = append(sends, ctx.Value(aiRequestIdentityKey{}).(*aiRequestIdentity).effort)
		return "Acts 2:1", nil
	}
	normal := context.Background()
	lowered := withAISpeedChoice(normal, aiSpeedChoice{provider: providerOpenAI, model: model, effort: "medium"})
	for _, ctx := range []context.Context{normal, lowered, lowered, normal} {
		if _, err := runAISearch(ctx, state, "Synthetic query"); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(sends, []string{"high", "medium"}) {
		t.Fatalf("uncached Find efforts = %v, want high and medium once each", sends)
	}
}
