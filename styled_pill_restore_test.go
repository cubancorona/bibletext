package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// The reader's report, on the STYLED pane: pressing the chapter pill must move
// the view to the note, not merely expand a card off-screen. The Apple panes
// consume the declaration through nativeScrollToHighlight; this pane consumes
// it in styledReadingArea's Layout, so the fix has to be proven here too rather
// than assumed to travel.
func TestStyledPillRestoreMovesTheViewport(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())
	defer setNotesEnabled(true)
	deleteAllNotes(appPrefs())

	st := planTestState(t)
	st.Bible.Verses["John"][3] = enumerationChapter()
	verses := st.Bible.GetChapter("John", 3)
	last := verses[len(verses)-1].Verse
	if _, ok := addNote(appPrefs(), StoredNote{
		Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3,
		VerseLo: last, Text: "a note at the very end of the chapter",
	}); !ok {
		t.Fatal("fixture note refused")
	}
	applyNoteForCurrentChapter(st)
	if st.ActiveNote == "" {
		t.Fatal("no note open")
	}

	pane := newStyledReadingPane(st, verses)
	pane.Resize(fyne.NewSize(360, 600))

	hideCurrentNote(st)
	st.forceReposition = false
	restoreCurrentNote(st)

	if !st.forceReposition {
		t.Fatal("the pill's restore did not declare a placement, so this pane " +
			"has nothing to consume and the card opens off-screen")
	}
	// And the placement must have somewhere to go: a note at the end of a long
	// chapter is far below the top, which is the whole point of the report.
	if y := pane.highlightY(); y <= 0 {
		t.Errorf("highlightY()=%g — the fixture note is not below the fold, so "+
			"this test could pass with no scrolling at all", y)
	}
}
