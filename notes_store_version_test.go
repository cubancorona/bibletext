package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// The live note's IDENTITY is one value, and every writer of the mirror has to
// write it. Its ancestor was four loose fields (ActiveNote / NoteMinimized /
// NoteVerseLo / NoteVersionID) of which two writers wrote three — so Delete
// followed a stale field to a DIFFERENT note whenever the chapter already
// carried one, and the store lost the other person's message while the one on
// screen survived (X1, found by the notes state enumeration). The mirror now
// carries StoredNote.ID, handed to it by the derive or the arrival path, and
// the verbs address that and nothing else.
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

	// Friend's note is stored under the BSB only. The reader is in the WEB, where
	// it FOLLOWS — so the derive hands the mirror Friend's note's identity.
	Friend, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "bsb", Book: "John", Chapter: 3, VerseLo: 16, Text: "Friend's note"})
	if !ok {
		t.Fatal("precondition: Friend's note was not stored")
	}
	applyNoteForCurrentChapter(st)
	if st.NoteID != Friend.ID {
		t.Fatalf("precondition: the bsb note should have followed, got id %d want %d", st.NoteID, Friend.ID)
	}

	// Now a link arrives carrying a NEW note. The arrival stores it and points
	// the mirror at ITS identity — the note actually on screen.
	applyShareTarget(st, ShareTarget{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16,
		Note: "a friend's note"})

	t.Logf("on screen: %q   mirror id: %d", st.ActiveNote, st.NoteID)
	if st.NoteID == Friend.ID {
		t.Fatal("the arrival left the mirror addressing the note that is no longer on screen — X1's tear")
	}

	// The reader deletes what is in front of them.
	dropCurrentNote(st)

	if _, mumSurvives := findStoredNote(appPrefs(), "bsb", "John", 3); !mumSurvives {
		t.Error("deleting the friend's note destroyed Friend's note instead")
	}
	if _, friendSurvives := findStoredNote(appPrefs(), "web", "John", 3); friendSurvives {
		t.Error("the note the reader deleted is still in the store")
	}
}
