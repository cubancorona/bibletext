package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestVersionSelectorUI(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("Version Selector Test")
	state := sampleState()
	state.window = win

	// Test the version selector widget
	selector := versionSelector(state)
	if selector == nil {
		t.Fatal("versionSelector returned nil")
	}

	// Verify we can tap the selector
	var anchor fyne.Tappable
	switch v := selector.(type) {
	case *versionPickerAnchor:
		anchor = v
	case *fyne.Container:
		anchor = v.Objects[0].(*versionPickerAnchor)
	}

	if anchor == nil {
		t.Fatal("could not find versionPickerAnchor")
	}

	// Tap to open the version picker
	test.Tap(anchor)

	// Verify popup is shown
	if win.Canvas().Overlays().Top() == nil {
		t.Fatal("expected version picker popup to be shown")
	}
}

func TestSwitchVersionInteractive(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("Version Switch Test")
	state := sampleState()
	state.window = win

	// Switch to a known public domain version
	switchVersionInteractive(state, "web")

	if state.currentVersion().ID != "web" {
		t.Errorf("expected version to be 'web', got %q", state.currentVersion().ID)
	}
}
