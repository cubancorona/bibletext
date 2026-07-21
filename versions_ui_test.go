package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// lockUnlicensedVersions pins every env switch that could make the NRSV/LSB
// licensed sources available, so these tests always see the default build:
// locked rows in the picker, refusals from switchVersion — and, crucially, no
// async download path (a QA machine with the license trio set would otherwise
// send switchVersionInteractive down the goroutine/network branch mid-test).
func lockUnlicensedVersions(t *testing.T) {
	t.Helper()
	for _, v := range []string{
		"BIBLETEXT_ENABLE_TESTING", "BIBLE_API_KEY",
		"BIBLETEXT_LICENSE_NRSV", "BIBLETEXT_PROVIDER_ID_NRSV",
		"BIBLETEXT_LICENSE_LSB", "BIBLETEXT_PROVIDER_ID_LSB",
	} {
		t.Setenv(v, "")
	}
}

func TestVersionSelectorUI(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	lockUnlicensedVersions(t)

	win := app.NewWindow("Version Selector Test")
	state := sampleState()
	state.CurrentVersion = defaultVersionID
	state.window = win

	selector := versionSelector(state)
	var anchor *versionPickerAnchor
	switch v := selector.(type) {
	case *versionPickerAnchor:
		anchor = v
	case *fyne.Container:
		anchor, _ = v.Objects[0].(*versionPickerAnchor)
	}
	if anchor == nil {
		t.Fatal("could not find versionPickerAnchor")
	}
	// The header subtitle names the ACTIVE translation.
	if want := state.currentVersion().Name + "  ▾"; anchor.text != want {
		t.Errorf("selector reads %q, want %q", anchor.text, want)
	}

	// Tap to open the version picker.
	test.Tap(anchor)
	popup, ok := win.Canvas().Overlays().Top().(*widget.PopUp)
	if !ok {
		t.Fatalf("expected the version picker popup, got %T", win.Canvas().Overlays().Top())
	}
	if !treeHasText(popup, "Translation") {
		t.Error("picker missing its title")
	}

	// Every registered version appears; only selectable ones get a tappable row;
	// locked ones carry the evaluation note instead.
	var selectable int
	for _, v := range bibleVersions() {
		if !treeHasText(popup, v.Name+"  ("+v.Abbrev+")") {
			t.Errorf("picker missing the row for %q", v.Name)
		}
		if v.canSelect() {
			selectable++
		}
	}
	var tappable int
	walkTree(popup, func(n fyne.CanvasObject) {
		if _, ok := n.(*tapBox); ok {
			tappable++
		}
	})
	if tappable != selectable {
		t.Errorf("%d tappable rows, want %d — locked versions must be inert", tappable, selectable)
	}
	if selectable < len(bibleVersions()) && !treeHasText(popup, "Evaluation in progress — not yet available") {
		t.Error("locked versions must carry the evaluation-in-progress note")
	}

	// Exactly the active version carries the check mark.
	var checks int
	walkTree(popup, func(n fyne.CanvasObject) {
		if txt, ok := n.(*canvas.Text); ok && txt.Text == "✓" {
			checks++
		}
	})
	if checks != 1 {
		t.Errorf("expected exactly one active-version check, got %d", checks)
	}

	// Close dismisses the picker.
	closeBtn := findTreeButton(popup, "Close")
	if closeBtn == nil {
		t.Fatal("picker missing the Close button")
	}
	test.Tap(closeBtn)
	if win.Canvas().Overlays().Top() != nil {
		t.Fatal("Close must dismiss the version picker")
	}
}

// TestSwitchVersionInteractive drives the picker's switch entry point down its
// synchronous path (target already in memory) and its refusal paths — never the
// network. (The old test asserted currentVersion().ID == "web" on a state whose
// CurrentVersion was still "", which the default-version fallback satisfies even
// when nothing switches — while the call it made kicked off a real background
// download of the WEB text.)
func TestSwitchVersionInteractive(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	lockUnlicensedVersions(t)

	state := sampleState()
	state.CurrentVersion = defaultVersionID
	web := state.Bible
	bsb := NewBibleData()
	bsb.PopulateWithSampleVerses()
	bsb.PrepareSearchIndex()
	state.loadedVersions = map[string]*BibleData{defaultVersionID: web, "bsb": bsb}

	// In-memory target → synchronous swap of both the id and the data.
	switchVersionInteractive(state, "bsb")
	if state.CurrentVersion != "bsb" {
		t.Fatalf("expected CurrentVersion 'bsb', got %q", state.CurrentVersion)
	}
	if state.Bible != bsb {
		t.Fatal("switch must point AppState.Bible at the loaded BSB data")
	}

	// Re-selecting the active version is a no-op.
	switchVersionInteractive(state, "bsb")
	if state.CurrentVersion != "bsb" || state.Bible != bsb {
		t.Fatal("re-selecting the active version must change nothing")
	}

	// A not-yet-licensed version is refused outright (the backstop behind the
	// picker's inert rows), as is an unknown id.
	switchVersionInteractive(state, "nrsv")
	if state.CurrentVersion != "bsb" || state.Bible != bsb {
		t.Fatal("an unlicensed version must be refused")
	}
	switchVersionInteractive(state, "no-such-version")
	if state.CurrentVersion != "bsb" {
		t.Fatal("an unknown version id must be refused")
	}
}
