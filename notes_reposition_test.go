package bibletext

// An explicit arrival must always place the view.
//
// The reading panes skip their entire push — HTML rebuild and the scroll cadence
// that rides on it — when the render fingerprint is unchanged. That is right for
// a repaint and wrong for a navigation: tapping a note for the passage already on
// screen produces a byte-identical fingerprint, so nothing moved and the tap
// looked broken. observed in practice.

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestOpeningANoteAlwaysAsksToReposition(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	n := SharedNote{VersionID: "web", Book: "Psalms", Chapter: 23, VerseLo: 1, VerseHi: 4, Text: "n"}

	// The hard case: ALREADY on the chapter, with that very range already lit —
	// so nothing about the render would change and the skip gate would swallow it.
	st := psalm23State()
	st.CurrentBook, st.CurrentChapter = "Psalms", 23
	st.HighlightedBook, st.HighlightedChapter = "Psalms", 23
	st.HighlightedVerse, st.HighlightedVerseEnd = 1, 4
	st.HasHighlightedVerse = true
	st.forceReposition = false

	openNote(st, n)
	if !st.forceReposition {
		t.Error("re-opening the note already on screen must still ask to reposition")
	}
}

func TestOpeningASearchResultAlwaysAsksToReposition(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	st.CurrentBook, st.CurrentChapter = "Psalms", 23
	st.HighlightedBook, st.HighlightedChapter = "Psalms", 23
	st.HighlightedVerse = 4
	st.HasHighlightedVerse = true
	st.forceReposition = false

	openSearchResult(st, Verse{BookName: "Psalms", Chapter: 23, Verse: 4})
	if !st.forceReposition {
		t.Error("re-opening the verse already lit must still ask to reposition")
	}
}

func TestTappedLinkAlwaysAsksToReposition(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	st := psalm23State()
	st.CurrentBook, st.CurrentChapter = "Psalms", 23
	st.forceReposition = false

	applyShareTarget(st, ShareTarget{VersionID: "web", Book: "Psalms", Chapter: 23,
		VerseLo: 1, VerseHi: 4, Note: "hello"})
	if !st.forceReposition {
		t.Error("a tapped link must place the view even when it names the current passage")
	}
}
