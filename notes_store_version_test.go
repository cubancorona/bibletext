package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// ActiveNote, NoteMinimized, NoteVerseLo and NoteVersionID are ONE VALUE, and
// every writer has to write all four. Two of them wrote three.
//
// NoteVersionID exists so Hide and Delete reach a note that FOLLOWED from
// another translation. Left stale it does the opposite of its job: the incoming
// link path set the first three and left the fourth naming whatever the derive
// had just found — a different note whenever the chapter already carried one.
// The reader saw the arriving note, pressed Delete, and the store lost the OTHER
// person's message while the one on screen survived and came back on the next
// navigation. Deleting the wrong message is the one mistake with no undo.
//
// Found by the notes state enumeration (docs/NOTES_STATE.md, X1) rather than by
// anybody using the app, which is the argument for that document existing.
func TestDeleteReachesTheNoteOnScreenNotAStaleOne(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	st := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3,
		CurrentVersion: "web", loadPhase: loadReady}

	// Friend's note is stored under the BSB only. The reader is in the WEB, where it
	// FOLLOWS — so the derive sets NoteVersionID to "bsb", correctly.
	saveNote(appPrefs(), SharedNote{VersionID: "bsb", Book: "John", Chapter: 3, VerseLo: 16, Text: "Friend's note"})
	applyNoteForCurrentChapter(st)
	if st.NoteVersionID != "bsb" {
		t.Fatalf("precondition: the bsb note should have followed, got %q", st.NoteVersionID)
	}

	// Now a link arrives carrying a NEW note. rememberIncomingNote files it under
	// the LINK's translation (web), and the three-field write leaves
	// NoteVersionID still saying "bsb" — the note that is no longer on screen.
	applyShareTarget(st, ShareTarget{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16,
		Note: "a friend's note"})

	t.Logf("on screen: %q   but NoteVersionID says: %q", st.ActiveNote, st.NoteVersionID)

	// The reader deletes what is in front of them.
	dropCurrentNote(st)

	_, mumSurvives := readNotes(appPrefs())[noteKey("bsb", "John", 3)]
	_, friendSurvives := readNotes(appPrefs())[noteKey("web", "John", 3)]
	if !mumSurvives {
		t.Error("REPRODUCED: deleting the friend's note destroyed Friend's note instead")
	}
	if friendSurvives {
		t.Error("REPRODUCED: the note the reader deleted is still in the store")
	}
}
