package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// Notes default to ON: a link carrying one is useless without it, and a reader
// who has never opened Settings should get what the link assumes.
func TestNotesDefaultOn(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	if !notesEnabled() {
		t.Error("shared notes should default to on")
	}
}

func TestNotesSwitchRoundTrips(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(false)
	if notesEnabled() {
		t.Error("off did not stick")
	}
	setNotesEnabled(true)
	if !notesEnabled() {
		t.Error("on did not stick")
	}
}

// Off must mean "not shown", never "quietly destroyed" — the saved notes have
// to still be there when the reader changes their mind.
func TestOffKeepsTheNotesUnlessAskedToDelete(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	p := fyne.CurrentApp().Preferences()
	saveNote(p, SharedNote{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Text: "kept"})

	setNotesEnabled(false)
	if len(readNotes(p)) != 1 {
		t.Error("turning notes off destroyed the stored note")
	}

	st := psalm23State()
	st.CurrentBook, st.CurrentChapter = "John", 3
	applyNoteForCurrentChapter(st)
	if st.ActiveNote != "" {
		t.Errorf("a note surfaced while the feature is off: %q", st.ActiveNote)
	}

	setNotesEnabled(true)
	applyNoteForCurrentChapter(st)
	if st.ActiveNote != "kept" {
		t.Errorf("the note did not come back when switched on: %q", st.ActiveNote)
	}
}

func TestDeleteAllNotesEmptiesTheStore(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	p := fyne.CurrentApp().Preferences()
	saveNote(p, SharedNote{VersionID: "web", Book: "John", Chapter: 3, Text: "a"})
	saveNote(p, SharedNote{VersionID: "web", Book: "John", Chapter: 4, Text: "b"})
	if len(readNotes(p)) != 2 {
		t.Fatalf("setup: expected 2, got %d", len(readNotes(p)))
	}
	deleteAllNotes(p)
	if got := len(readNotes(p)); got != 0 {
		t.Errorf("expected the store emptied, got %d", got)
	}
}

// An incoming link must not store a note while the feature is off — otherwise
// "off" would quietly accumulate other people's messages on the device.
func TestOffDoesNotStoreAnIncomingNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(false)

	st := psalm23State()
	st.loadPhase = loadReady
	target, ok := ParseShareLink("https://bibletext.co.uk/bsb/psalms/23/#v1-4&n=" + EncodeNote("should not be kept"))
	if !ok {
		t.Fatal("link did not parse")
	}
	applyShareTarget(st, target)

	if st.ActiveNote != "" {
		t.Errorf("note surfaced while off: %q", st.ActiveNote)
	}
	if n := len(readNotes(fyne.CurrentApp().Preferences())); n != 0 {
		t.Errorf("note was stored while off (%d in the store)", n)
	}
}
