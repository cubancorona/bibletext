package bibletext


// icon in the search box is still highlighted and clicking it does not take me
// back from reader to notes and also then causes search to be highlighted").
// Three symptoms, one state model: on desktop the Notes MODE survives opening
// a note (the reading pane takes the results pane, NotesMode stays true), and
// the bubble's toggle-out used to fire on the mode alone — so the tap that
// should have returned the reader to the list silently flipped the mode to
// keyword and lit Search. The rule now: the bubble toggles OUT only while the
// list is actually on screen; from the reader it goes BACK to the list.
//
// Pinned by driving the REAL sidebar's mode buttons, laid out and visible the
// screen_seen_test.go way.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// seenButtons collects every visible, laid-out button under root.
func seenButtons(t *testing.T, root fyne.CanvasObject, size fyne.Size) []*widget.Button {
	t.Helper()
	w := test.NewWindow(root)
	t.Cleanup(w.Close)
	w.Resize(size)
	var out []*widget.Button
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if o == nil || !o.Visible() {
			return
		}
		if b, ok := o.(*widget.Button); ok {
			if sz := b.Size(); sz.Width > 0 && sz.Height > 0 {
				out = append(out, b)
			}
			return
		}
		if sc, ok := o.(*container.Scroll); ok {
			walk(sc.Content)
			return
		}
		if c, ok := o.(*fyne.Container); ok {
			for _, ch := range c.Objects {
				walk(ch)
			}
			return
		}
		if wd, ok := o.(fyne.Widget); ok {
			if r := test.WidgetRenderer(wd); r != nil {
				for _, ch := range r.Objects() {
					walk(ch)
				}
			}
		}
	}
	walk(root)
	return out
}

// modeRowTestApp is themedTestApp's race-safe twin: that helper lives in a
// //go:build !race file, and this suite must run under -race too. Same body,
// same reason — the sidebar's section labels use Bold+Monospace, which the
// bare test theme leaves undefined (nil-font panic on measure).
func modeRowTestApp() fyne.App {
	app := test.NewApp()
	app.Settings().SetTheme(&bibleTheme{})
	return app
}

func TestDesktopModeRowStaysHonestThroughANoteOpen(t *testing.T) {
	app := modeRowTestApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	stored, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Psalms", Chapter: 23, VerseLo: 1, Text: "a note"})
	if !ok {
		t.Fatal("the note was not stored")
	}

	st := psalm23State()
	st.Bible.PrepareSearchIndex()
	st.aiKeys = newKeyStoreWith(newFakePrefs())
	st.aiKeys.setAIEnabled(true) // the Search/Find pair must exist for the phantom-highlight half
	st.surfaceSearch = nil       // desktop

	size := fyne.NewSize(300, 800)
	buttons := seenButtons(t, buildSidebar(st), size)
	var notesBtn, kwBtn *widget.Button
	for _, b := range buttons {
		switch {
		case b.Icon != nil && b.Icon.Name() == iconNoteBubble.Name():
			notesBtn = b
		case b.Text == "Search":
			kwBtn = b
		}
	}
	if notesBtn == nil || kwBtn == nil {
		t.Fatalf("the real sidebar's mode row is missing controls: notes=%v search=%v", notesBtn != nil, kwBtn != nil)
	}

	// Into Notes: the list claims the pane, the bubble lights.
	test.Tap(notesBtn)
	if searchModeOf(st) != modeNotes || !st.IsSearching {
		t.Fatalf("tapping the bubble should show the list: mode=%v searching=%v", searchModeOf(st), st.IsSearching)
	}
	if notesBtn.Importance != widget.HighImportance {
		t.Error("the bubble should light while the list is up")
	}

	// Open a note: the reader lands on the passage; the MODE is still Notes,
	// and the row must say so — the highlight is honest, not stale.
	openNote(st, stored)
	if st.IsSearching {
		t.Fatal("precondition: the reading pane should own the screen after the tap-through")
	}
	if searchModeOf(st) != modeNotes {
		t.Fatal("precondition: the mode survives the tap-through on desktop")
	}
	if notesBtn.Importance != widget.HighImportance {
		t.Error("the bubble's highlight must reflect the mode the pane belongs to")
	}


	// BACK TO THE LIST — not silently flip the mode and light Search.
	test.Tap(notesBtn)
	if !st.IsSearching || searchModeOf(st) != modeNotes {
		t.Errorf("the bubble must return the reader to the notes list: searching=%v mode=%v",
			st.IsSearching, searchModeOf(st))
	}
	if notesBtn.Importance != widget.HighImportance {
		t.Error("the bubble must stay lit — the reader is back on the list")
	}
	if kwBtn.Importance == widget.HighImportance {
		t.Error("Search lit up without anyone choosing it — the phantom highlight, pinned")
	}

	// And the way OUT still works: from the list, the bubble toggles back to
	// scripture, exactly as before.
	test.Tap(notesBtn)
	if searchModeOf(st) == modeNotes || st.NotesMode {
		t.Error("from the list, the bubble is still the way out of Notes mode")
	}
	if notesBtn.Importance == widget.HighImportance {
		t.Error("the bubble must dim once the reader has left Notes")
	}
}

// The mobile row lives inside the Search tab, so it is only ever visible with
// the list on screen — there the bubble's toggle-out stays unconditional.
func TestMobileModeRowBubbleStillTogglesOut(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	st := psalm23State()
	st.surfaceSearch = func() {} // the phones' signal
	st.NotesMode = true

	var selected []searchMode
	row := buildSearchModeControls(st, func(m searchMode) { selected = append(selected, m) })
	var notesBtn *widget.Button
	for _, b := range seenButtons(t, row, fyne.NewSize(320, 80)) {
		if b.Icon != nil && b.Icon.Name() == iconNoteBubble.Name() {
			notesBtn = b
		}
	}
	if notesBtn == nil {
		t.Fatal("no notes bubble on the mobile mode row")
	}
	test.Tap(notesBtn)
	if len(selected) != 1 || selected[0] != modeKeyword {
		t.Errorf("on the phones the bubble toggles out of Notes: got %v", selected)
	}
}
