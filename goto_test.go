package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestShowChapterPicker(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("Chapter Picker Test")
	state := sampleState()
	state.window = win

	showChapterPicker(state)

	if win.Canvas().Overlays().Top() == nil {
		t.Fatal("expected chapter picker popup to be shown")
	}
}

func TestShowGotoPicker(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("Goto Picker Test")
	state := sampleState()
	state.window = win

	showGotoPicker(state)

	if win.Canvas().Overlays().Top() == nil {
		t.Fatal("expected goto picker popup to be shown")
	}
}
