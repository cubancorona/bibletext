//go:build !race

// Skipped under -race: see the note in ui_focus_test.go about Fyne's test app
// clearing its font cache on a background goroutine.

package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// TestFullUIBuildsAndExercisesPaths renders the whole UI on an in-memory test
// canvas and drives the main flows. It fails if any widget builder panics
// (sidebar list rows, reading paragraphs, search results, history bar, theme).
func TestFullUIBuildsAndExercisesPaths(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := sampleState()
	win := app.NewWindow("render")
	win.SetContent(CreateMainUI(app, state, win))
	win.Resize(fyne.NewSize(1100, 800))

	// Reading view is the default; switch to search results.
	executeSearch(state, "god")
	if !state.IsSearching {
		t.Fatal("expected search mode after executeSearch")
	}
	if len(state.SearchResults) == 0 {
		t.Fatal("expected results for 'god' from sample data")
	}
	// The live window content must actually swap to the results view: one
	// tappable card per match in the reading pane.
	if got := len(collectResultCards(win.Canvas().Content())); got != len(state.SearchResults) {
		t.Fatalf("window renders %d result cards, want %d", got, len(state.SearchResults))
	}

	// Open a result -> reading view with a highlighted, scrolled-to verse.
	openSearchResult(state, state.SearchResults[0])
	if !state.hlOn() {
		t.Fatal("expected a highlighted verse after opening a result")
	}
	if left := len(collectResultCards(win.Canvas().Content())); left != 0 {
		t.Fatalf("reading view must replace the results, still %d cards rendered", left)
	}

	// Navigate so the recent-history bar has something to render.
	selectBook(state, "Genesis", true)
	state.refresh()
	selectBook(state, "Psalms", true)
	state.refresh()
	if recentJumpTargets(state, 6) == nil {
		t.Fatal("expected recent history after navigating between books")
	}

	// There's no in-app dark-mode toggle any more (we follow the OS variant).
	// Rebuild the window once more to make sure CreateMainUI is idempotent.
	win.SetContent(CreateMainUI(app, state, win))
}

// TestRebuildWindowHidesOverlayPopups pins the variant-flip fix: rebuildWindow
// must DRAIN the overlay stack (an open popup would otherwise survive
// SetContent with its build-time palette colors — the field-reported
// dark-on-dark Settings sheet after an overnight dark→light switch) and it
// must HIDE each popup rather than bare-Remove it, because the popup watchdog
// timers poll Visible() to run their close/restore duties.
func TestRebuildWindowHidesOverlayPopups(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := sampleState()
	win := app.NewWindow("rebuild")
	state.app = app
	state.window = win
	win.SetContent(CreateMainUI(app, state, win))

	pop := widget.NewModalPopUp(widget.NewLabel("settings stand-in"), win.Canvas())
	pop.Show()
	if len(win.Canvas().Overlays().List()) == 0 {
		t.Fatal("popup must be on the overlay stack before the rebuild")
	}

	rebuildWindow(state)

	if n := len(win.Canvas().Overlays().List()); n != 0 {
		t.Errorf("rebuildWindow must drain the overlay stack, %d left", n)
	}
	if pop.Visible() {
		t.Error("popup must be hidden, not bare-removed — watchdogs poll Visible() and would spin forever")
	}
}
