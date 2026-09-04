package bibletext

// FULL-SCREEN READING IS THE READING PANE ALONE, ON EVERY PLATFORM.
//
// The chapter toolbar's focus button flips state.IsFullScreen and rebuilds.
// On the desktop's shipped rail layout that rebuild used to land in a layout
// that never read the flag, so the header and rail stayed and only the button's
// icon changed. The shared compact layout now owns the full-screen tree, and
// this holds it on the real entry point (CreateMainUI, which on the host is the
// desktop one) and on the shared builder: no navigation cell and no app header
// in the built tree, with a control that the ordinary tree carries them.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func countTabCells(root fyne.CanvasObject) int {
	n := 0
	walkTree(root, func(o fyne.CanvasObject) {
		if _, ok := o.(*tabCell); ok {
			n++
		}
	})
	return n
}

// countIconButtons counts the buttons drawn with a given theme icon; text-only
// buttons carry no icon.
func countIconButtons(root fyne.CanvasObject, iconName string) int {
	n := 0
	walkTree(root, func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && b.Icon != nil && b.Icon.Name() == iconName {
			n++
		}
	})
	return n
}

func TestFullScreenReadingDropsTheHeaderAndNavigation(t *testing.T) {
	t.Setenv("BIBLETEXT_DESKTOP_TABS", "") // the shipped desktop navigation
	app := test.NewApp()
	defer app.Quit()
	st := sampleState()
	st.loadPhase = loadReady
	w := app.NewWindow("full-screen")
	defer w.Close()
	w.Resize(fyne.NewSize(1100, 760))

	enter := theme.ViewFullScreenIcon().Name()
	restore := theme.ViewRestoreIcon().Name()

	// Control: the ordinary tree carries the destinations, the app header and
	// the way in, through the entry point the focus button's rebuild takes.
	st.IsFullScreen = false
	ordinary := CreateMainUI(app, st, w)
	if got := countTabCells(ordinary); got < 3 {
		t.Fatalf("ordinary tree has %d tab cells, want at least 3 — the control cannot fail", got)
	}
	if !treeHasText(ordinary, "BibleText") {
		t.Fatal("ordinary tree carries no app header — the control cannot fail")
	}
	if countIconButtons(ordinary, enter) != 1 || countIconButtons(ordinary, restore) != 0 {
		t.Fatalf("ordinary tree: %d enter / %d restore buttons, want 1 / 0",
			countIconButtons(ordinary, enter), countIconButtons(ordinary, restore))
	}

	st.IsFullScreen = true
	for _, entry := range []struct {
		name  string
		build func() fyne.CanvasObject
	}{
		{"CreateMainUI", func() fyne.CanvasObject { return CreateMainUI(app, st, w) }},
		{"buildCompactUI", func() fyne.CanvasObject { return buildCompactUI(st) }},
	} {
		name, root := entry.name, entry.build()
		if got := countTabCells(root); got != 0 {
			t.Errorf("%s: full-screen tree still carries %d tab cells — the bar or rail survived", name, got)
		}
		// The tree is the page ground plus the reading host and nothing else:
		// no header above it, no navigation beside it.
		stack, ok := root.(*fyne.Container)
		if !ok || len(stack.Objects) != 2 {
			t.Fatalf("%s: full-screen root = %T with %d objects, want a Stack of ground + reading host", name, root, len(stack.Objects))
		}
		host, ok := stack.Objects[1].(*fyne.Container)
		if !ok || len(host.Objects) != 1 {
			t.Fatalf("%s: full-screen content = %T, want the reading host holding one reading view", name, stack.Objects[1])
		}
		if st.showReading == nil {
			t.Errorf("%s: full-screen tree left showReading unset — a chapter change would have no pane to render into", name)
		}
		if treeHasText(root, "BibleText") {
			t.Errorf("%s: the app header survived into full-screen", name)
		}
		// The way back must be on screen: the chapter toolbar's restore button,
		// and no second way in.
		if countIconButtons(root, restore) != 1 || countIconButtons(root, enter) != 0 {
			t.Errorf("%s: full-screen tree has %d restore / %d enter buttons, want 1 / 0 — the reader would be stranded",
				name, countIconButtons(root, restore), countIconButtons(root, enter))
		}
	}
}

// The phone-landscape presentation is the same tree with the reader's own
// choice untouched: the seam pinned on, IsFullScreen off, and the shared
// builder returns the full-screen tree on the Read tab — and only there: a
// rotation while browsing Books or Search keeps their ordinary layout, and
// the overlay rule follows the same answer.
func TestPhoneLandscapePresentationIsTheFullScreenTree(t *testing.T) {
	t.Setenv("BIBLETEXT_DESKTOP_TABS", "")
	app := test.NewApp()
	defer app.Quit()
	st := sampleState()
	st.loadPhase = loadReady
	w := app.NewWindow("landscape")
	defer w.Close()
	st.window = w
	w.Resize(fyne.NewSize(874, 402))
	// Through the entry point once, as the app does: it installs the theme
	// and the hooks the tab builders read.
	_ = CreateMainUI(app, st, w)

	orig := phoneLandscapeReading
	phoneLandscapeReading = func() bool { return true }
	defer func() { phoneLandscapeReading = orig }()

	st.IsFullScreen = false
	root := buildCompactUI(st)
	if got := countTabCells(root); got != 0 {
		t.Errorf("presentation tree carries %d tab cells", got)
	}
	if treeHasText(root, "BibleText") {
		t.Error("presentation tree carries the app header")
	}
	if st.IsFullScreen {
		t.Error("the presentation wrote the reader's own full-screen choice")
	}
	if !overlayShouldShow(st) {
		t.Error("the overlay rule does not follow the presentation on the Read tab")
	}
	st.CurrentTab = 1
	if got := countTabCells(buildCompactUI(st)); got < 3 {
		t.Errorf("Books tab in landscape lost its navigation (%d tab cells) — the presentation is a reading mode", got)
	}
	if overlayShouldShow(st) {
		t.Error("the overlay would show over the Books tab in landscape")
	}
}
