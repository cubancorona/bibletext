package bibletext

// Settings → "Delete all notes (N)": the count says how much "all" means, and
// taps through to the list. And the list keeps the reader's place while they
// stay in Notes mode.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestDeleteAllNotesShowsAndLinksTheCount(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	th := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	st := sampleState()
	st.theme = th

	link := newNotesCountLink(st, nil)

	// No notes: nothing to show and nothing to tap through to.
	link.setCount(0)
	if link.label() != "" {
		t.Errorf("with no notes the count should be blank, got %q", link.label())
	}
	tapped := 0
	link.onTapped = func() { tapped++ }
	link.Tapped(&fyne.PointEvent{})
	if tapped != 0 {
		t.Error("an empty count must not navigate — there is nothing to look at")
	}

	// With notes: the number is shown in parentheses and taps through.
	link.setCount(3)
	if link.label() != "(3)" {
		t.Errorf("count label = %q, want %q", link.label(), "(3)")
	}
	link.Tapped(&fyne.PointEvent{})
	if tapped != 1 {
		t.Error("tapping the count did not navigate")
	}
}

// showNotesList must actually put the reader in front of the notes, on either
// layout: the desktop results pane only exists while IsSearching, and on mobile
// the Search tab has to be brought forward.
func TestShowNotesListReachesTheNotes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)

	st := sampleState()
	surfaced := 0
	st.surfaceSearch = func() { surfaced++ }

	showNotesList(st)

	if searchModeOf(st) != modeNotes {
		t.Error("did not switch into Notes mode")
	}
	if !st.IsSearching {
		t.Error("desktop: Notes must claim the results pane, or nothing is shown")
	}
	if surfaced != 1 {
		t.Errorf("mobile: the Search tab was surfaced %d times, want 1", surfaced)
	}
}

// The list's scroll survives EVERY route out and back (owner, 2026-08-19:
// "the notes browser should remember its scroll position"): a rebuild while in
// Notes keeps it, and leaving the mode HARVESTS the live list's offset rather
// than forgetting it — the reader returns to the same neighbourhood. Only the
// reader func (which aliases the torn-down list) is dropped on exit.
func TestNotesScrollRemembersAcrossLeavingTheMode(t *testing.T) {
	st := sampleState()
	setNotesMode(st, true)
	st.notesScroll = 240

	setNotesMode(st, true) // still in Notes (e.g. a sort change rebuild)
	if st.notesScroll != 240 {
		t.Errorf("scroll lost while staying in Notes mode: %v", st.notesScroll)
	}

	// The live list has scrolled since the last harvest; leaving the mode must
	// ask IT, not trust the stale value.
	st.notesScrollRead = func() float32 { return 512 }
	setNotesMode(st, false) // reader leaves Notes
	if st.notesScroll != 512 {
		t.Errorf("leaving Notes must harvest the live offset: got %v, want 512", st.notesScroll)
	}
	if st.notesScrollRead != nil {
		t.Error("the reader func aliases the torn-down list and must be dropped on exit")
	}

	setNotesMode(st, true) // and coming back finds the memory intact
	if st.notesScroll != 512 {
		t.Errorf("re-entering Notes lost the remembered offset: %v", st.notesScroll)
	}
}
