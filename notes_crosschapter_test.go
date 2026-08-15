package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// X13 — a note whose passage moves to another CHAPTER cannot be reached by any
// verb.
//
// There is exactly one passage in the shipping translations where this happens,
// and it is worth naming because it is easy to assume the arm is theoretical.
// The doxology "Now to him who is able to establish you…" closes chapter 14 in
// the manuscript tradition the WEB follows and chapter 16 in the one the BSB and
// NKJV follow (versification_data.go:26-28, :36-38). Same three sentences of
// scripture, two filing addresses. Measured over the real text of all four
// translations, that is the ONLY cross-chapter case: 24 mappings, all Romans.
// Joel, Malachi, 3 John and the psalm superscriptions do NOT diverge here —
// every shipping translation uses the same English chapter division — so any
// design note claiming otherwise is describing a problem this app does not have.
//
// The note itself is handled correctly: noteFromAnotherTranslation renumbers it
// and shows it on the chapter the passage actually lives on. Nothing is lost and
// nothing is misplaced. The defect is that the VERBS then cannot address it,
// because Hide and Delete rebuild the key's book and chapter from
// state.CurrentBook / state.CurrentChapter — where the READER is standing —
// while the note is stored under the chapter it came from.
//
// This is X1's mechanism surviving in a dimension the X1 fix did not touch.
// 31bc97630 made NoteVersionID carry the VERSION third of the key; the book and
// chapter thirds are still reconstructed from the reader's position. It is the
// argument for NoteKey — an identity carried whole — rather than for a third
// patch.
func TestX13CrossChapterNoteCannotBeReachedByAVerb(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	// Filed by a WEB reader against Romans 14:24.
	saveNote(appPrefs(), SharedNote{
		VersionID: "web", Book: "Romans", Chapter: 14, VerseLo: 24, VerseHi: 26,
		Text: "the doxology — read this last bit",
	})

	// Sanity: the note is INVISIBLE on the BSB's Romans 14 and VISIBLE, correctly
	// renumbered, on the BSB's Romans 16. This half already works.
	if _, ok := loadNote(appPrefs(), "bsb", "Romans", 14); ok {
		t.Error("the note showed on Romans 14 in a translation that files it at 16")
	}
	n, ok := loadNote(appPrefs(), "bsb", "Romans", 16)
	if !ok {
		t.Fatal("the note did not follow the doxology into Romans 16")
	}
	if n.VerseLo != 25 || n.VerseHi != 27 {
		t.Errorf("renumbered wrong: got %d-%d, want 25-27", n.VerseLo, n.VerseHi)
	}
	if n.VersionID != "web" {
		t.Errorf("the note lost track of where it is stored: %q", n.VersionID)
	}

	// Now the reader — standing in the BSB on Romans 16, looking at the note —
	// deletes it.
	st := &AppState{CurrentVersion: "bsb", CurrentBook: "Romans", CurrentChapter: 16}
	st.ActiveNote = n.Text
	st.NoteVerseLo = n.VerseLo
	st.NoteVersionID = n.VersionID
	dropCurrentNote(st)

	if _, back := loadNote(appPrefs(), "bsb", "Romans", 16); !back {
		t.Log("X13 is FIXED: Delete reached a note stored under another chapter.")
		t.Error("Strike X13 from docs/NOTES_STATE.md and delete this test's expectation.")
		return
	}
	// The defect, stated: the reader watched the note go and it is still there.
	// dropCurrentNote called deleteNote(..., "Romans", 16) because that is the
	// chapter the reader is on; the note is filed at "web|Romans|14".
	t.Log("X13 confirmed: the note survives its own deletion, and returns on the " +
		"next navigation. Delete addressed Romans 16; the note is filed at Romans 14.")
}
