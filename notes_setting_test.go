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
	addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Text: "kept"})

	setNotesEnabled(false)
	if storedNoteCount(p) != 1 {
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
	addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, Text: "a"})
	addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 4, Text: "b"})
	if storedNoteCount(p) != 2 {
		t.Fatalf("setup: expected 2, got %d", storedNoteCount(p))
	}
	deleteAllNotes(p)
	if got := storedNoteCount(p); got != 0 {
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
	if n := storedNoteCount(fyne.CurrentApp().Preferences()); n != 0 {
		t.Errorf("note was stored while off (%d in the store)", n)
	}
}

// Turning notes off puts the note's mark out AT THAT MOMENT — not at the next
// navigation.
//
// The review mutation that demanded this pin: deleting clearMarkFromNote from
// turnNotesOff left the ENTIRE suite green, including all 1,280 enumeration
// cells, because the harness observes after the next derive — where the
// off-branch backstop rescues the mark. On the real Settings route the switch
// is followed by a refresh, not a derive, so under that mutation the note's
// tint genuinely stayed lit on screen until the reader navigated. The moment
// half and the backstop half are two different promises; this holds the first.
func TestNotesOffClearsTheNoteMarkImmediately(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Psalms", Chapter: 23, VerseLo: 1, Text: "lit by me"})

	st := psalm23State()
	applyNoteForCurrentChapter(st)
	if !st.mark.fromNote() {
		t.Fatal("precondition: the note should have raised its mark")
	}

	turnNotesOff(st)

	if st.hasMark() {
		t.Error("the note's mark survived the OFF switch itself — it must not wait " +
			"for the next navigation to be rescued")
	}

	// ...and a mark the note does NOT own survives the same switch.
	setNotesEnabled(true)
	applyNoteForCurrentChapter(st)
	st.setHL(hlSearch, "Psalms", 23, 4, 0)
	turnNotesOff(st)
	if sp, ok := st.markSpan(); !ok || st.mark.Origin != hlSearch || sp.Lo != 4 {
		t.Error("the reader's own search mark must survive notes going off")
	}
}

// Turning notes off clears the MIRROR at that moment too — the field the mark
// pin above deliberately leaves one field over. appleStickerPush gates on the
// mirror, not on the feature switch, so a mirror that outlived the switch kept
// the sticker on the iOS/macOS page — expanded, verbs and all, tint gone —
// until the next navigation ran the projection's off-branch. The switch's own
// route is a refresh, not a derive, which is why the verb itself must end on
// the projection.
func TestNotesOffClearsTheMirrorImmediately(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Psalms", Chapter: 23, VerseLo: 1, Text: "on the sticker"})

	st := psalm23State()
	applyNoteForCurrentChapter(st)
	if st.ActiveNote == "" || st.NoteID == 0 {
		t.Fatal("precondition: the note should be on the mirror")
	}

	turnNotesOff(st)

	if st.ActiveNote != "" || st.NoteID != 0 || st.NoteVerseLo != 0 {
		t.Errorf("the mirror survived the OFF switch (text=%q id=%d lo=%d) — the sticker "+
			"push reads the mirror, so this is the sticker still on the page",
			st.ActiveNote, st.NoteID, st.NoteVerseLo)
	}
	text, who, _, _ := appleStickerPush(st, buildChapterPlan(st, appPrefs(), st.Bible))
	if text != "" || who != "" {
		t.Errorf("the pushed tuple must carry no sticker with notes off, got text=%q who=%q", text, who)
	}
}

// The way back ON re-projects at that moment: the off-route's derive cleared
// the mirror, so a bare preference write left the noted chapter bare on the
// Apple panes (appleStickerPush returns early on an empty mirror) and its
// verse unwashed on the banner platforms (only the projection sets hlNote)
// until the next navigation.
func TestNotesOnReprojectsTheChapterImmediately(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Psalms", Chapter: 23, VerseLo: 2, Text: "waiting"})

	st := psalm23State()
	setNotesEnabled(false)
	applyNoteForCurrentChapter(st) // the navigation-while-off that empties the mirror
	if st.ActiveNote != "" {
		t.Fatal("precondition: off leaves the mirror empty")
	}

	turnNotesOn(st)

	if st.ActiveNote != "waiting" || st.NoteID == 0 {
		t.Errorf("the stored note must be back on the mirror the moment notes come on, got text=%q id=%d",
			st.ActiveNote, st.NoteID)
	}
	if !st.mark.fromNote() {
		t.Error("the note's own wash must be re-raised with it — the banner platforms tint from state.mark")
	}
	text, _, pill, _ := appleStickerPush(st, buildChapterPlan(st, appPrefs(), st.Bible))
	if text != "waiting" || pill {
		t.Errorf("the pushed tuple must carry the note expanded, got text=%q pill=%v", text, pill)
	}
}
