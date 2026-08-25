package bibletext

// An explicit arrival must always place the view.
//
// The reading panes skip their entire push — HTML rebuild and the scroll cadence
// that rides on it — when the render fingerprint is unchanged. That is right for
// a repaint and wrong for a navigation: tapping a note for the passage already on
// screen produces a byte-identical fingerprint, so nothing moved and the tap
// looked broken.

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestOpeningANoteAlwaysAsksToReposition(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	n := StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms", Chapter: 23, VerseLo: 1, VerseHi: 4, Text: "n"}

	// The hard case: ALREADY on the chapter, with that very range already lit —
	// so nothing about the render would change and the skip gate would swallow it.
	st := psalm23State()
	st.CurrentBook, st.CurrentChapter = "Psalms", 23
	st.setHL(hlNote, "Psalms", 23, 1, 4)
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
	st.setHL(hlNote, "Psalms", 23, 4, 0)
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

// The Go-to box, a cross-reference and the verse of the day all arrive through
// goToVerseRange, and it is the one explicit arrival that never said so.
//
// It got away with it while the Apple panes folded the WASH into their single
// render fingerprint: a new mark changed the fingerprint, the push rebuilt, and
// the rebuild's scroll cadence happened to land on the verse. A wash change is a
// live attribute mutation now (reading_tint_apple.go) and deliberately does not
// scroll — a highlight arriving under the reader's eye must not move the page —
// so the intent has to be declared, exactly as the three arrivals above declare
// it. The hard case is the same one they pin: the verse is already lit, so
// nothing about the render changes and only forceReposition can say "go there".
//
// Every shipping pane now consumes forceReposition: Apple pairs it with the
// live wash mutation, while Android and the styled Windows/Linux pane use it to
// suppress same-chapter carry. The declaration therefore belongs to the verb,
// not to a platform predicate.
func TestGoToVerseRangeAlwaysAsksToReposition(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	st.CurrentBook, st.CurrentChapter = "Psalms", 23
	st.setHL(hlVerseOfDay, "Psalms", 23, 4, 4)
	st.forceReposition = false

	goToVerseRange(st, "Psalms", 23, 4, 4)
	if !st.forceReposition {
		t.Error("a Go-to for the verse already lit must ask every renderer to reposition")
	}
}
