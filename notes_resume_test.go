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
	st.clearMark()
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
	if !st.hlOn() || st.hlLo() != 1 || st.hlHi() != 4 {
		t.Errorf("expected the note's passage highlighted, got has=%v %d-%d",
			st.hlOn(), st.hlLo(), st.hlHi())
	}
}

// The note comes back WHOLE — bubble and highlight — even when a saved reading
// position exists. Dropping the highlight to protect the scroll was the first
// attempt and it was wrong: a bubble pointing at nothing reads as a fault, and
// was reported as one. The scroll is protected in the reading panes instead,
// where a pending restore outranks the highlight.
func TestReopenRestoresTheNoteWhole(t *testing.T) {
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
			if !st.hlOn() || st.hlLo() != 1 || st.hlHi() != 4 {
				t.Errorf("the note came back without its highlight: has=%v %d-%d",
					st.hlOn(), st.hlLo(), st.hlHi())
			}
			// The saved position must survive too — the panes use it to outrank
			// the highlight, so losing it here would hand the scroll to the note.
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
	st.setHL(hlSearch, "Psalms", 23, 6, 0)

	applyNoteOnResume(st)

	if !st.hlOn() || st.hlLo() != 6 {
		t.Errorf("the reader's own highlight was discarded: has=%v verse=%d",
			st.hlOn(), st.hlLo())
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
	if st.hlOn() {
		t.Error("a highlight appeared on reopen with the feature off")
	}
}

// An explicit arrival clears the saved position, because the panes now let a
// pending restore outrank the highlight — leaving one armed would send a tapped
// link to where the reader last was instead of the verse they asked for.
func TestArrivalsClearTheSavedPosition(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	for name, arrive := range map[string]func(*AppState){
		"tapped link": func(st *AppState) {
			applyShareTarget(st, ShareTarget{VersionID: "web", Book: "Psalms", Chapter: 23,
				VerseLo: 1, VerseHi: 4, Note: "hello"})
		},
		"search result": func(st *AppState) {
			openSearchResult(st, Verse{BookName: "Psalms", Chapter: 23, Verse: 2})
		},
		"note from the browser": func(st *AppState) {
			openNote(st, SharedNote{VersionID: "web", Book: "Psalms", Chapter: 23,
				VerseLo: 1, VerseHi: 4, Text: "n"})
		},
	} {
		t.Run(name, func(t *testing.T) {
			st := psalm23State()
			st.restore = &restoreAnchor{Book: "Psalms", Chapter: 23, Verse: 5}
			arrive(st)
			if st.restore != nil {
				t.Errorf("a stale saved position survived an arrival (verse %d) — it would win the scroll",
					st.restore.Verse)
			}
			if !st.forceReposition {
				t.Error("an arrival must also ask the pane to reposition")
			}
		})
	}
}
