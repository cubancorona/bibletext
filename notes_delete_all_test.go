package bibletext

// Settings must offer a way to delete every stored note, and must ask first.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// The Settings sheet must offer a way to delete every stored note, enabled only
// when there is something to delete.
func TestSettingsOffersDeleteAllNotes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	th := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	setNotesEnabled(true)
	win := app.NewWindow("s")
	win.Resize(fyne.NewSize(430, 932))
	st := sampleState()
	st.window = win
	st.theme = th

	// No notes yet: the control exists but must not offer a no-op.
	showAISettings(st)
	p := win.Canvas().Overlays().Top().(*widget.PopUp)
	btn := findTreeButton(p.Content, "Delete all notes")
	if btn == nil {
		t.Fatal("Settings has no 'Delete all notes' control")
	}
	if !btn.Disabled() {
		t.Error("with no notes stored the button should be disabled")
	}
	p.Hide()

	// With a note stored it must be enabled, and deleting must clear the store.
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 11, VerseLo: 35, Text: "hi"})
	if storedNoteCount(appPrefs()) != 1 {
		t.Fatal("precondition: one stored note")
	}
	showAISettings(st)
	p2 := win.Canvas().Overlays().Top().(*widget.PopUp)
	btn2 := findTreeButton(p2.Content, "Delete all notes")
	if btn2 == nil || btn2.Disabled() {
		t.Fatal("with a note stored the button should be present and enabled")
	}
	btn2.OnTapped()
	// The confirmation must be asked, not bypassed.
	confirm, ok := win.Canvas().Overlays().Top().(*widget.PopUp)
	if !ok || confirm == nil {
		t.Fatal("deleting did not ask for confirmation")
	}
	if storedNoteCount(appPrefs()) != 1 {
		t.Error("notes were deleted before the reader confirmed")
	}
	del := findTreeButton(confirm.Content, "Delete them")
	if del == nil {
		t.Fatal("confirmation has no 'Delete them'")
	}
	del.OnTapped()
	if n := storedNoteCount(appPrefs()); n != 0 {
		t.Errorf("after confirming, %d notes remain", n)
	}
}
