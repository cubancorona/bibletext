package bibletext

// Tests for the Settings → Assistant "None" choice, which turns every AI feature
// off at runtime: the Find toggle disappears, leftover AI search state can't
// reach the AI result views, and the native-menu dispatch drops AI actions.

import (
	"context"
	"testing"
	"time"

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
	// Notes off TOO. The row owns the Search/Find pair AND the notes bubble, so
	// what collapses it is having no choice left at all — with notes on there is
	// still a bubble to press, and it should be there.
	setNotesEnabled(false)
	defer setNotesEnabled(true)
	row := buildSearchModeControls(state, func(searchMode) { t.Fatal("onSelect must never fire with nothing to choose") })
	if row == nil {
		t.Fatal("expected a placeholder object")
	}
	// Fyne floors every canvas object's MinSize at 1×1, so "collapsed" means at
	// most that floor — anything larger would be a visible control.
	if got := row.MinSize(); got.Width > 1 || got.Height > 1 {
		t.Fatalf("with nothing to choose the row must collapse, got MinSize %v", got)
	}
	if texts := collectText(row); len(texts) != 0 {
		t.Fatalf("the collapsed row must render no text, got %v", texts)
	}
}

// With AI off but notes ON the row still exists — the bubble is the one control
// left — and it must carry no "Find".
func TestNotesBubbleSurvivesAIOff(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := aiOffState()
	setNotesEnabled(true)
	row := buildSearchModeControls(state, func(searchMode) {})
	if got := row.MinSize(); got.Height <= 1 {
		t.Errorf("with notes on the row must still show the bubble, got MinSize %v", got)
	}
	for _, txt := range collectText(row) {
		if txt == "Find" || txt == "Search" {
			t.Errorf("no scripture-search control may exist with AI off, found %q", txt)
		}
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
	for _, txt := range collectText(buildSearchModeControls(state, func(searchMode) {})) {
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
	dispatchAIAction(onState, aiActionAsk, "In the beginning", selSpan{})
	if onWin.Canvas().Overlays().Top() == nil {
		t.Fatal("control failed: with AI on, an Ask dispatch must open the question sheet")
	}
	onWin.Canvas().Overlays().Remove(onWin.Canvas().Overlays().Top())

	// With the assistant on "None", the same dispatches must be dropped by the
	// aiFeaturesEnabled guard: no panel, no sheet, no overlay of any kind.
	state := aiOffState()
	win := app.NewWindow("ai off")
	state.window = win
	dispatchAIAction(state, aiActionExplain, "In the beginning", selSpan{})
	if top := win.Canvas().Overlays().Top(); top != nil {
		t.Fatalf("AI-off Explain dispatch opened an overlay: %T", top)
	}
	dispatchAIAction(state, aiActionAsk, "In the beginning", selSpan{})
	if top := win.Canvas().Overlays().Top(); top != nil {
		t.Fatalf("AI-off Ask dispatch opened an overlay: %T", top)
	}
}

// The selection's span reaches the request seam as the pane resolved it, for
// a fixed action through the answer panel and for Ask through the question
// sheet, whose header names the verses. Control: the zero span arrives as the
// zero span.
func TestDispatchAIActionCarriesSpan(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	spans := make(chan selSpan, 4)
	never := make(chan struct{}) // requests park, so no completion touches a panel mid-test
	prev := aiActionRun
	aiActionRun = func(ctx context.Context, _ *AppState, _, _, _ string, span selSpan) (string, error) {
		spans <- span
		<-never
		return "", ctx.Err()
	}
	t.Cleanup(func() { aiActionRun = prev })
	next := func(what string) selSpan {
		t.Helper()
		select {
		case s := <-spans:
			return s
		case <-time.After(2 * time.Second):
			t.Fatalf("%s never reached the request seam", what)
			return selSpan{}
		}
	}

	// The refrain chapter: selected in verse 2 the words match every verse,
	// so only the span can say verse 2 (selection_span_test.go).
	state := refrainChapterState()
	state.aiKeys = newKeyStoreWith(newFakePrefs())
	win := app.NewWindow("ai on")
	state.window = win

	dispatchAIAction(state, aiActionExplain, refrain, selSpanFromNative(2, 2))
	if got := next("the Explain dispatch"); got != (selSpan{lo: 2, hi: 2}) {
		t.Errorf("the seam saw %+v for a verse-2 dispatch", got)
	}
	win.Canvas().Overlays().Remove(win.Canvas().Overlays().Top())

	dispatchAIAction(state, aiActionExplain, refrain, selSpan{})
	if got := next("the zero-span dispatch"); got != (selSpan{}) {
		t.Errorf("control: the zero span arrived as %+v", got)
	}
	win.Canvas().Overlays().Remove(win.Canvas().Overlays().Top())

	dispatchAIAction(state, aiActionAsk, refrain, selSpanFromNative(2, 2))
	sheet := win.Canvas().Overlays().Top()
	if sheet == nil || !treeHasText(sheet, "Ask about this passage") {
		t.Fatalf("the Ask dispatch did not open the question sheet; texts %v", treeTexts(sheet))
	}
	if !treeHasText(sheet, "Psalms 136:2") {
		t.Errorf("the question sheet does not name the verse the span resolved to; texts %v", treeTexts(sheet))
	}
}
