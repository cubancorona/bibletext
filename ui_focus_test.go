//go:build !race

// These tests render real widgets via fyne.io/fyne/v2/test. Fyne's test app
// clears its font cache on a background goroutine when settings change, which
// the race detector flags against synchronous text measurement during layout.
// That race lives in Fyne's test harness (not the real driver or our code), so
// we skip these under -race; the pure-logic tests stay fully race-checked.

package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// findEntry walks a canvas object tree for an Entry with the given placeholder.
func findEntry(obj fyne.CanvasObject, placeholder string) *widget.Entry {
	switch o := obj.(type) {
	case *widget.Entry:
		if o.PlaceHolder == placeholder {
			return o
		}
	case *fyne.Container:
		for _, child := range o.Objects {
			if e := findEntry(child, placeholder); e != nil {
				return e
			}
		}
	}
	return nil
}

// TestBookFilterKeepsFocusWhileTyping guards against the original bug where each
// keystroke rebuilt the sidebar and stole focus. The fix is that the filter only
// refreshes the list, leaving the entry (and its focus) intact.
func TestBookFilterKeepsFocusWhileTyping(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := sampleState()
	win := app.NewWindow("focus")
	state.window = win

	sidebar := buildSidebar(state)
	win.SetContent(sidebar)
	win.Resize(fyne.NewSize(320, 640))

	filter := findEntry(sidebar, "Filter books")
	if filter == nil {
		t.Fatal("could not find the book filter entry")
	}

	win.Canvas().Focus(filter)
	test.Type(filter, "gen")

	if win.Canvas().Focused() != filter {
		t.Fatal("book filter lost focus while typing")
	}
	if state.BookFilterQuery != "gen" {
		t.Fatalf("expected filter query 'gen', got %q", state.BookFilterQuery)
	}
}

// TestSidebarHasSearchAndFilterEntries is a light smoke test that the persistent
// sidebar builds with both entry widgets present.
func TestSidebarHasSearchAndFilterEntries(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := sampleState()
	win := app.NewWindow("smoke")
	state.window = win
	sidebar := buildSidebar(state)
	win.SetContent(sidebar)

	if findEntry(sidebar, "Filter books") == nil {
		t.Error("missing book filter entry")
	}
	if findEntry(sidebar, "Search…") == nil {
		t.Error("missing search entry")
	}
}
