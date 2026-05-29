//go:build !race

// Skipped under -race: see the note in ui_focus_test.go about Fyne's test app
// clearing its font cache on a background goroutine.

package holybible

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
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

	// Open a result -> reading view with a highlighted, scrolled-to verse.
	openSearchResult(state, state.SearchResults[0])
	if !state.HasHighlightedVerse {
		t.Fatal("expected a highlighted verse after opening a result")
	}

	// Navigate so the recent-history bar has something to render.
	selectBook(state, "Genesis", true)
	state.refresh()
	selectBook(state, "Psalms", true)
	state.refresh()
	if recentJumpTargets(state, 6) == nil {
		t.Fatal("expected recent history after navigating between books")
	}

	// Toggle dark mode and back; both rebuild the full tree.
	toggleTheme(state)
	if !state.theme.dark {
		t.Fatal("expected dark mode enabled")
	}
	toggleTheme(state)
	if state.theme.dark {
		t.Fatal("expected light mode after toggling back")
	}
}
