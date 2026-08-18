package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// The cross-chapter note, and the verb that can now reach it.
//
// HISTORY — why X13 existed. There is exactly one passage in the shipping
// translations where a note's passage moves to another CHAPTER, and it is
// worth naming because it is easy to assume the arm is theoretical. The
// doxology "Now to him who is able to establish you…" closes chapter 14 in
// the manuscript tradition the WEB follows and chapter 16 in the one the BSB
// and NKJV follow (versification_data.go:26-28, :36-38). Same three sentences
// of scripture, two filing addresses. Measured over the real text of all four
// translations, that is the ONLY cross-chapter case: 24 mappings, all Romans.
// Joel, Malachi, 3 John and the psalm superscriptions do NOT diverge here —
// every shipping translation uses the same English chapter division — so any
// design note claiming otherwise is describing a problem this app does not
// have.
//
// The note itself was always handled correctly: the derive renumbers it and
// shows it on the chapter the passage actually lives on. The defect (X13) was
// that the VERBS could not address it, because Hide and Delete rebuilt the
// key's book and chapter from state.CurrentBook / state.CurrentChapter —
// where the READER was standing — while the note was stored under the chapter
// it came from. That was X1's mechanism surviving in a dimension the X1 fix
// (31bc97630, which made NoteVersionID carry the VERSION third of the key)
// did not touch: the other two thirds were still reconstructed. It was the
// argument for an identity carried whole rather than for a third patch — and
// the scrapbook store (S5) is that identity: every verb takes StoredNote.ID,
// handed to it by the surface that drew the note, and nothing rebuilds an
// address from the reader's position. X13 was struck from docs/NOTES_STATE.md
// when this landed.
func TestCrossChapterNoteIsReachedByItsVerb(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	// Filed by a WEB reader against Romans 14:24.
	stored, ok := addNote(appPrefs(), StoredNote{
		Kind: noteKindReceived, VersionID: "web", Book: "Romans", Chapter: 14, VerseLo: 24, VerseHi: 26,
		Text: "the doxology — read this last bit",
	})
	if !ok {
		t.Fatal("the note was not stored")
	}

	// Sanity: the note is INVISIBLE on the BSB's Romans 14 and VISIBLE,
	// correctly renumbered, on the BSB's Romans 16. This half always worked.
	if _, ok := noteForChapter(appPrefs(), "bsb", "Romans", 14, nil); ok {
		t.Error("the note showed on Romans 14 in a translation that files it at 16")
	}
	n, ok := noteForChapter(appPrefs(), "bsb", "Romans", 16, nil)
	if !ok {
		t.Fatal("the note did not follow the doxology into Romans 16")
	}
	if n.VerseLo != 25 || n.VerseHi != 27 {
		t.Errorf("renumbered wrong: got %d-%d, want 25-27", n.VerseLo, n.VerseHi)
	}
	if n.VersionID != "web" {
		t.Errorf("the note lost track of where it is stored: %q", n.VersionID)
	}
	if n.ID != stored.ID {
		t.Fatalf("the followed note lost its identity: %d, want %d", n.ID, stored.ID)
	}

	// Now the reader — standing in the BSB on Romans 16, looking at the note —
	// deletes it. The mirror is populated by the REAL derive, not by hand: a
	// hand-set st.NoteID would keep passing if applyNoteForCurrentChapter ever
	// stopped handing the followed note's ID to the mirror, which is exactly
	// the wiring this test exists to hold (implementation verification on the first
	// version).
	st := &AppState{CurrentVersion: "bsb", CurrentBook: "Romans", CurrentChapter: 16}
	applyNoteForCurrentChapter(st)
	if st.ActiveNote != n.Text {
		t.Fatalf("the derive did not surface the followed note: %q", st.ActiveNote)
	}
	if st.NoteID != stored.ID {
		t.Fatalf("the derive handed the mirror ID %d, want %d — the verb below would "+
			"address the wrong object", st.NoteID, stored.ID)
	}
	dropCurrentNote(st)

	if _, back := noteForChapter(appPrefs(), "bsb", "Romans", 16, nil); back {
		t.Error("X13 is BACK: the reader watched the note go and it is still there — " +
			"a verb rebuilt an address instead of using the note's own identity")
	}
	if _, filed := findStoredNote(appPrefs(), "web", "Romans", 14); filed {
		t.Error("the record survives under its filing address after its deletion")
	}
}
