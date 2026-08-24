//go:build darwin

package bibletext

// The NATIVE consumer pin: reverting the "Clear
// highlight" export's body from clearHighlightAndRederive back to the bare
// clearHighlightedVerse left the whole suite green — the shared verb and both
// Fyne call sites were pinned, but the iOS/macOS tap where the symptom was
// FOUND (bubble re-opens, verse unwashed) was not. This exercises the real
// //export body: activeAIState is the package seam the native side dispatches
// through, and the test driver runs the export's fyne.Do inline.

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestNativeClearHighlightRestoresTheNoteWash(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	defer deleteAllNotes(appPrefs())

	stored, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Psalms", Chapter: 23, VerseLo: 2, VerseHi: 3, Text: "n"})
	if !ok {
		t.Fatal("the note was not stored")
	}
	st := psalm23State()
	applyNoteForCurrentChapter(st)
	if st.NoteID != stored.ID || !st.mark.fromNote() {
		t.Fatal("precondition: the note owns the page")
	}
	// A Go-to wash replaces the note's mark — the note stands down.
	goToVerseRange(st, "Psalms", 23, 1, 1)
	if !notesSuppressed(st) {
		t.Fatal("precondition: the foreign mark suppresses the note")
	}

	orig := activeAIState
	activeAIState = st
	defer func() { activeAIState = orig }()

	bibleTextHighlightCleared()

	if st.mark.live() && !st.mark.fromNote() {
		t.Fatal("the foreign mark must be gone")
	}
	if !st.mark.fromNote() {
		t.Error("the note's OWN wash must be re-raised — the export must end on " +
			"clearHighlightAndRederive, not the bare clear")
	}
	if notesSuppressed(st) || st.ActiveNote == "" {
		t.Error("the note must stand back up expanded once the foreign mark is cleared")
	}
}
