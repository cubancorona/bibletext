package bibletext

// Reopening the app on a chapter that has a note.
//
// The bug these pin: the restore path sets book/chapter directly and never went
// through addRecentChapter, so the chapter the reader last had open came back
// with no note — while every other chapter's note appeared correctly as soon as
// they navigated. Field-reported.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// resumeState is a state sitting where the launch restore leaves one: version,
// book and chapter set, no highlight, nothing having gone through navigation.
func resumeState(t *testing.T) *AppState {
	t.Helper()
	st := psalm23State()
	st.CurrentBook, st.CurrentChapter = "Psalms", 23
	st.HasHighlightedVerse = false
	st.ActiveNote = ""
	st.restore = nil
	return st
}

func TestNoteComesBackOnReopen(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	p := fyne.CurrentApp().Preferences()
	setNotesEnabled(true)
	saveNote(p, SharedNote{VersionID: "web", Book: "Psalms", Chapter: 23,
		VerseLo: 1, VerseHi: 4, Text: "This got me through last night."})

	st := resumeState(t)
	applyNoteOnResume(st)

	if st.ActiveNote != "This got me through last night." {
		t.Errorf("reopening did not bring the note back: %q", st.ActiveNote)
	}
	// Nothing to protect (never scrolled), so the note keeps its highlight and
	// the reader lands on it — same as arriving on the link.
	if !st.HasHighlightedVerse || st.HighlightedVerse != 1 || st.HighlightedVerseEnd != 4 {
		t.Errorf("expected the note's passage highlighted, got has=%v %d-%d",
			st.HasHighlightedVerse, st.HighlightedVerse, st.HighlightedVerseEnd)
	}
}

// A reader who had read on past the note must come back to where they stopped,
// not be yanked up to the note. The iOS scroller keys off the highlight, so the
// note must not set one when there is a saved position.
func TestReopenKeepsTheSavedPositionOverTheNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	p := fyne.CurrentApp().Preferences()
	setNotesEnabled(true)
	saveNote(p, SharedNote{VersionID: "web", Book: "Psalms", Chapter: 23,
		VerseLo: 1, VerseHi: 4, Text: "read this bit"})

	for _, tc := range []struct {
		name   string
		anchor *restoreAnchor
	}{
		{"verse anchor", &restoreAnchor{Book: "Psalms", Chapter: 23, Verse: 5}},
		{"fraction fallback", &restoreAnchor{Book: "Psalms", Chapter: 23, Frac: 0.7}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := resumeState(t)
			st.restore = tc.anchor
			applyNoteOnResume(st)

			if st.ActiveNote != "read this bit" {
				t.Errorf("the note should still come back: %q", st.ActiveNote)
			}
			if st.HasHighlightedVerse {
				t.Errorf("a note highlight overrode the saved reading position (verse %d)",
					st.HighlightedVerse)
			}
			if st.restore == nil {
				t.Error("the saved position was dropped")
			}
		})
	}
}

// A highlight the reader arrived with — a link, a search result — is theirs and
// outranks both. Giving it up would be the old "note steals the jump" bug.
func TestReopenNeverTouchesAnExistingHighlight(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	p := fyne.CurrentApp().Preferences()
	setNotesEnabled(true)
	saveNote(p, SharedNote{VersionID: "web", Book: "Psalms", Chapter: 23,
		VerseLo: 1, VerseHi: 4, Text: "note"})

	st := resumeState(t)
	st.restore = &restoreAnchor{Book: "Psalms", Chapter: 23, Verse: 5}
	st.HasHighlightedVerse = true
	st.HighlightedBook, st.HighlightedChapter = "Psalms", 23
	st.HighlightedVerse = 6

	applyNoteOnResume(st)

	if !st.HasHighlightedVerse || st.HighlightedVerse != 6 {
		t.Errorf("the reader's own highlight was discarded: has=%v verse=%d",
			st.HasHighlightedVerse, st.HighlightedVerse)
	}
}

// With the feature off, reopening must stay silent.
func TestReopenSurfacesNothingWhenNotesAreOff(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	p := fyne.CurrentApp().Preferences()
	saveNote(p, SharedNote{VersionID: "web", Book: "Psalms", Chapter: 23,
		VerseLo: 1, VerseHi: 4, Text: "hidden"})
	setNotesEnabled(false)
	defer setNotesEnabled(true)

	st := resumeState(t)
	applyNoteOnResume(st)

	if st.ActiveNote != "" {
		t.Errorf("a note surfaced on reopen with the feature off: %q", st.ActiveNote)
	}
	if st.HasHighlightedVerse {
		t.Error("a highlight appeared on reopen with the feature off")
	}
}
