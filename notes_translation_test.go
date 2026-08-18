package bibletext

// Notes are keyed version|book|chapter, so every path that changes the
// translation — or navigates to a note — has to keep the live mirror
// (ActiveNote/NoteMinimized/NoteVerseLo) in step with the store. Four of these
// paths did not, and the failures were all silent: a note from the wrong
// translation drawn over the text, a note that vanished when you tapped it, a
// deleted note still on screen, and a shared deuterocanon link that did nothing
// at all.

import (
	"errors"
	"testing"

	"fyne.io/fyne/v2/test"
)

func notesXState(t *testing.T) *AppState {
	t.Helper()
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	return &AppState{
		Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1,
		CurrentVersion: "web", loadPhase: loadReady,
		loadedVersions: map[string]*BibleData{"web": bd},
	}
}

// Switching translation CARRIES the note over, renumbered for the translation
// being switched to.
//
// This test used to assert the opposite — that the note was dropped — on the
// reasoning that a remark belongs to the wording it was written against. What a
// reader actually met was a note that disappeared when they changed
// translation, leaving the highlight it had placed behind with nothing to

// note goes with it; where the numbering genuinely does not correspond,
// the derive declines (see the Greek Esther case in
// notes_store_test.go).
func TestSwitchingTranslationCarriesTheNoteOver(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	st := notesXState(t)
	st.CurrentBook, st.CurrentChapter = "John", 3
	// A note that belongs to WEB only.
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Text: "web note"})
	applyNoteForCurrentChapter(st)
	if st.ActiveNote != "web note" {
		t.Fatalf("precondition: the WEB note should be live, got %q", st.ActiveNote)
	}

	other, ok := versionByID("bsb")
	if !ok {
		t.Skip("bsb not registered")
	}
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	applyLoadedVersion(st, other, bd, modeReal)

	if st.ActiveNote != "web note" {
		t.Errorf("the note did not follow the passage into %s: %q", other.ID, st.ActiveNote)
	}
	if st.NoteVerseLo != 16 {
		t.Errorf("the note lost its verse on the way across: %d", st.NoteVerseLo)
	}
}

// ...and the inverse: a note belonging to the translation being switched TO
// must appear without waiting for a navigation.
func TestSwitchingTranslationPicksUpThatTranslationsNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	st := notesXState(t)
	st.CurrentBook, st.CurrentChapter = "John", 3
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "bsb", Book: "John", Chapter: 3, VerseLo: 16, Text: "bsb note"})

	other, ok := versionByID("bsb")
	if !ok {
		t.Skip("bsb not registered")
	}
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	applyLoadedVersion(st, other, bd, modeReal)

	if st.ActiveNote != "bsb note" {
		t.Errorf("the BSB note did not appear on switching to BSB: %q", st.ActiveNote)
	}
}

// Deleting every note must clear the one currently on screen, not just the
// store — the panes render from the live mirror.
func TestDeletingAllNotesClearsTheOneOnScreen(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	st := notesXState(t)
	st.ActiveNote = "still here"
	st.NoteVerseLo = 16
	st.setHL(hlNote, st.CurrentBook, st.CurrentChapter, 16, 0)

	deleteAllNotes(appPrefs())
	clearLiveNote(st)

	if st.ActiveNote != "" || st.NoteVerseLo != 0 {
		t.Errorf("note still live after delete-all: %q verse=%d", st.ActiveNote, st.NoteVerseLo)
	}
	if st.hlOn() {
		t.Error("the note's highlight outlived the note")
	}
}

// The reader's chosen translation must survive a launch where it could not be
// revalidated. The choice lives ONLY in the reading-state blob's Version field,
// which the next navigation rewrites from CurrentVersion — so a fallback that
// does not remember the preference silently converts "could not load it today"
// into "forgot it forever".
func TestForcedFallbackDoesNotEraseTheChosenTranslation(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	origLoad := loadVersionForRestore
	defer func() { loadVersionForRestore = origLoad }()
	loadVersionForRestore = func(BibleVersion, *BibleData) (*BibleData, dataMode, error) {
		return nil, modeReal, errOfflineForTest
	}
	t.Setenv("BIBLE_API_KEY", "")
	fake := withFakeSharedKeys(t)
	fake.setBibleAPIKey("unlocks-the-licensed-version")

	licensed := ""
	for _, v := range bibleVersions() {
		if isLicensedSource(v) && v.canSelect() {
			licensed = v.ID
			break
		}
	}
	if licensed == "" {
		t.Fatal("no selectable licensed version — the test would not exercise the fallback")
	}

	base := fullValidBible()
	st := &AppState{Bible: base, CurrentVersion: defaultVersionID,
		loadedVersions: map[string]*BibleData{defaultVersionID: base}}

	if _, err := restoreReadingState(st, readingState{Version: licensed, Book: "Genesis", Chapter: 1}, base); err != nil {
		t.Fatalf("fallback errored: %v", err)
	}
	if st.CurrentVersion != defaultVersionID {
		t.Fatalf("precondition: should be showing the default, got %q", st.CurrentVersion)
	}

	// The reader navigates; this is what rewrites the saved blob.
	snap := snapshotReadingState(st, 0, 0, 0, 0, 0)
	if snap.Version != licensed {
		t.Errorf("persisting after a forced fallback saved %q — the reader's choice of %q is gone for good",
			snap.Version, licensed)
	}
}

// An explicit switch must clear the remembered preference: the reader has now
// chosen, and their choice outranks what we fell back from.
func TestAnExplicitSwitchClearsTheFallbackPreference(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	st := notesXState(t)
	st.preferredVersion = "nkjv"

	other, ok := versionByID("bsb")
	if !ok {
		t.Skip("bsb not registered")
	}
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	applyLoadedVersion(st, other, bd, modeReal)

	if st.preferredVersion != "" {
		t.Errorf("preference %q survived an explicit switch", st.preferredVersion)
	}
	if snapshotReadingState(st, 0, 0, 0, 0, 0).Version != other.ID {
		t.Error("the reader's new choice was not what got saved")
	}
}

var errOfflineForTest = errors.New("offline")

// A note that goes away must take ITS highlight with it. Clearing the note
// alone left the passage marked with nothing to explain the mark — the reader
// sees their verse highlighted and the message gone, which reads as data loss

// where a note genuinely cannot follow: Greek Esther, whose numbering does not
// correspond.
func TestLosingANoteAlsoClearsItsHighlight(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	st := notesXState(t)
	st.CurrentVersion = "web"
	st.CurrentBook, st.CurrentChapter = "Esther", 4
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Esther", Chapter: 4, VerseLo: 1,
		Text: "for such a time as this"})
	applyNoteForCurrentChapter(st)
	if st.ActiveNote == "" || !st.hlOn() {
		t.Fatalf("precondition: note and its highlight should be live (note=%q highlight=%v)",
			st.ActiveNote, st.hlOn())
	}

	// The Catholic edition carries Greek Esther, so this note has nowhere to go.
	st.CurrentVersion = "webc"
	applyNoteForCurrentChapter(st)

	if st.ActiveNote != "" {
		t.Errorf("the note survived into a book its numbering does not describe: %q", st.ActiveNote)
	}
	if st.hlOn() {
		t.Error("the note's highlight was left behind — a marked verse with nothing explaining it")
	}
}
