package bibletext

// Tests for the Settings → Assistant "None" choice, which turns every AI feature
// off at runtime: the Find toggle disappears, leftover AI search state can't
// reach the AI result views, and the native-menu dispatch drops AI actions.

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestKeyStoreAIEnabled(t *testing.T) {
	ks := newKeyStoreWith(newFakePrefs())
	if !ks.aiEnabled() {
		t.Fatal("aiEnabled must default to true")
	}
	ks.setAIEnabled(false)
	if ks.aiEnabled() {
		t.Fatal("aiEnabled must be false after setAIEnabled(false)")
	}
	ks.setAIEnabled(true)
	if !ks.aiEnabled() {
		t.Fatal("aiEnabled must be true again after setAIEnabled(true)")
	}
	// Keys survive the off/on round trip — "None" hides AI, it doesn't reset it.
	ks.setAPIKey("gemini", "k")
	ks.setAIEnabled(false)
	if ks.apiKey("gemini") != "k" {
		t.Fatal("stored keys must be kept while AI is off")
	}

	// An inert store (no prefs bound) reports the default.
	var nilStore *keyStore
	if !nilStore.aiEnabled() {
		t.Fatal("nil keyStore must report the enabled default")
	}
}

// aiOffState is sampleState with the assistant switched to "None".
func aiOffState() *AppState {
	state := sampleState()
	state.aiKeys = newKeyStoreWith(newFakePrefs())
	state.aiKeys.setAIEnabled(false)
	return state
}

func TestSearchModeToggleHiddenWhenAIOff(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := aiOffState()
	// The toggle is the SCRIPTURE pair (Search/Find). With no assistant there is
	// no second way to search the Bible, so it collapses to nothing — the notes
	// bubble is a separate control and does not keep it alive.
	toggle := buildSearchModeToggle(state, func(searchMode) { t.Fatal("onSelect must never fire with AI off") })
	if toggle == nil {
		t.Fatal("expected a placeholder object")
	}
	// Fyne floors every canvas object's MinSize at 1×1, so "collapsed" means at
	// most that floor — anything larger would be a visible control.
	if got := toggle.MinSize(); got.Width > 1 || got.Height > 1 {
		t.Fatalf("with AI off the toggle must collapse to nothing, got MinSize %v", got)
	}
	if texts := collectText(toggle); len(texts) != 0 {
		t.Fatalf("with AI off the toggle must render no text (no Find button), got %v", texts)
	}
}

func TestSearchResultsViewIgnoresAIStateWhenOff(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := aiOffState()
	// Leftover AI-search state from before the reader switched to "None": with the
	// feature off this must fall through to the keyword view, never aiSearchingView.
	state.aiSearchActive = true
	state.aiSearchLoading = true
	state.aiSearchQuery = "leftover"

	view := buildSearchResultsView(state)
	if view == nil {
		t.Fatal("expected a results view")
	}
	for _, txt := range collectText(view) {
		if txt == "Searching with AI…" {
			t.Fatal("AI-off results view must not render the AI searching state")
		}
	}
}

func TestSidebarForcesKeywordModeWhenAIOff(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := aiOffState()
	state.aiSearchMode = true // persisted-in-session Find mode from before the switch
	state.aiSearchActive = true

	if sb := buildSidebar(state); sb == nil {
		t.Fatal("expected the sidebar to build")
	}
	if state.aiSearchMode || state.aiSearchActive {
		t.Fatal("building the sidebar with AI off must force keyword mode")
	}
	for _, txt := range collectText(buildSearchModeToggle(state, func(searchMode) {})) {
		if txt == "Find" {
			t.Fatal("no Find control may exist with AI off")
		}
	}
}

func TestDispatchAIActionNoOpWhenOff(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	// Positive control on a live window: with AI ON, an Ask dispatch opens the
	// question sheet as a canvas overlay. (Ask is the safe control — unlike the
	// fixed actions it makes no provider call until the question is submitted.)
	// This proves the overlay channel observes the dispatch, so the AI-off
	// assertions below can genuinely fail if the guard is ever lost.
	onState := sampleState()
	onState.aiKeys = newKeyStoreWith(newFakePrefs())
	onWin := app.NewWindow("ai on")
	onState.window = onWin
	dispatchAIAction(onState, aiActionAsk, "In the beginning")
	if onWin.Canvas().Overlays().Top() == nil {
		t.Fatal("control failed: with AI on, an Ask dispatch must open the question sheet")
	}
	onWin.Canvas().Overlays().Remove(onWin.Canvas().Overlays().Top())

	// With the assistant on "None", the same dispatches must be dropped by the
	// aiFeaturesEnabled guard: no panel, no sheet, no overlay of any kind.
	state := aiOffState()
	win := app.NewWindow("ai off")
	state.window = win
	dispatchAIAction(state, aiActionExplain, "In the beginning")
	if top := win.Canvas().Overlays().Top(); top != nil {
		t.Fatalf("AI-off Explain dispatch opened an overlay: %T", top)
	}
	dispatchAIAction(state, aiActionAsk, "In the beginning")
	if top := win.Canvas().Overlays().Top(); top != nil {
		t.Fatalf("AI-off Ask dispatch opened an overlay: %T", top)
	}
}
