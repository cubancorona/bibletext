package bibletext

import (
	"strings"
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
	// A locked version carries the note for the REASON it is locked, and the two
	// reasons are different: a bring-your-own-key translation is waiting for the
	// reader's key, an unlicensed one is waiting for a licence. Asserting the
	// evaluation wording for every locked version was only ever right while an
	// unlicensed one shipped — once the NRSV and the LSB moved behind build
	// tags, the only locked version left was the NKJV and the test failed
	// demanding the wrong sentence.
	var lockedBYOK, lockedEval int
	for _, v := range bibleVersions() {
		if v.canSelect() {
			continue
		}
		if byokCapable(v) {
			lockedBYOK++
		} else {
			lockedEval++
		}
	}
	// CONTAINMENT, not equality: treeHasText matches a whole node, and these
	// sentences embed the version names, which change as the catalogue does.
	hasPhrase := func(want string) bool {
		for _, s := range treeTexts(popup) {
			if strings.Contains(s, want) {
				return true
			}
		}
		return false
	}
	if lockedBYOK > 0 && !hasPhrase("with your own free API.Bible key") {
		t.Error("a locked bring-your-own-key version must say a key unlocks it")
	}
	if lockedEval > 0 && !treeHasText(popup, "Evaluation in progress — not yet available") {
		t.Error("a locked unlicensed version must carry the evaluation-in-progress note")
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

// TestVersionPickerOrder pins the picker's display order: a licence-configured
// NKJV leads, the public-domain versions follow in registry order, and
// everything not selectable sinks to the bottom (where the footer note names
// it). Without a licence the NKJV is itself locked and must sink too.
func TestVersionPickerOrder(t *testing.T) {
	t.Setenv("BIBLE_API_KEY", "test-key")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "1")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "test-bible")

	got := versionPickerOrder()
	if len(got) != len(bibleVersions()) {
		t.Fatalf("picker order dropped versions: %d != %d", len(got), len(bibleVersions()))
	}
	if got[0].ID != "nkjv" {
		t.Errorf("licensed NKJV should lead the picker, got %q first", got[0].ID)
	}
	// THE ZONE RULE IS ABOUT WHAT THE READER CAN ACT ON, not about selectability.
	// It used to be "no selectable version may follow a locked one", which was
	// right while every locked version was awaiting a licence. A
	// bring-your-own-key translation is deliberately placed ABOVE the
	// public-domain ones even when it has no key, so that invariant now reports
	// the intended layout as a fault. What must still hold: a version NOBODY can
	// unlock never precedes one the reader can use or unlock.
	deadZone := false
	for i, v := range got {
		unlockableByNobody := !v.canSelect() && !byokCapable(v)
		if unlockableByNobody {
			deadZone = true
		} else if deadZone {
			t.Errorf("%q at %d is usable or unlockable, yet follows a version awaiting a licence", v.ID, i)
		}
	}
	for _, name := range append(lockedVersionNames(true), lockedVersionNames(false)...) {
		if name == "NKJV" {
			t.Error("licence-configured NKJV must not be in the locked footer note")
		}
	}

	// Without a key it STILL leads: the row tells the reader how to unlock it,
	// and burying it is what this ordering exists to prevent (owner, 22 Aug
	// 2026). See versions_picker_order_test.go for the dedicated cases.
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "")
	t.Setenv("BIBLE_API_KEY", "")
	got = versionPickerOrder()
	if got[0].ID != "nkjv" {
		t.Errorf("keyless NKJV should still lead the picker, got %q", got[0].ID)
	}
	found := false
	for _, name := range lockedVersionNames(true) {
		if name == "NKJV" {
			found = true
		}
	}
	if !found {
		t.Error("unlicensed NKJV missing from the BYOK footer note")
	}
	for _, name := range lockedVersionNames(false) {
		if name == "NKJV" {
			t.Error("BYOK-capable NKJV must not be in the evaluation footer note")
		}
	}
}

func TestJoinNatural(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"NRSV"}, "NRSV"},
		{[]string{"NRSV", "LSB"}, "NRSV and LSB"},
		{[]string{"NRSV", "LSB", "NKJV"}, "NRSV, LSB and NKJV"},
	}
	for _, c := range cases {
		if got := joinNatural(c.in); got != c.want {
			t.Errorf("joinNatural(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
