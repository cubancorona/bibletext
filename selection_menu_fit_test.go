package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
)

// A selection menu pressed near the bottom of the window must open high
// enough for its submenus: Fyne clamps a child menu flush against the canvas
// edge, which read as "butting against the bottom, slightly cut off".
func TestSelectionMenuOpensWithRoomForItsSubmenus(t *testing.T) {
	study := fyne.NewMenu("", fyne.NewMenuItem("Explain", nil),
		fyne.NewMenuItem("Analyze context", nil), fyne.NewMenuItem("Analyze translation", nil))
	share := fyne.NewMenu("", fyne.NewMenuItem("a", nil), fyne.NewMenuItem("b", nil),
		fyne.NewMenuItem("c", nil), fyne.NewMenuItem("d", nil))
	ai := fyne.NewMenuItem("Study with AI", nil)
	ai.ChildMenu = study
	sh := fyne.NewMenuItem("Share", nil)
	sh.ChildMenu = share
	items := []*fyne.MenuItem{fyne.NewMenuItem("Copy", nil), fyne.NewMenuItem("Select all", nil), ai, sh,
		fyne.NewMenuItem("Cross-references", nil)}
	childH := func(m *fyne.Menu) float32 { return float32(len(m.Items)) * 30 }
	const menuH, canvasH, rowH = 150, 600, 30

	// Plenty of room: the menu opens where it was pressed.
	if got := selectionMenuFitY(100, menuH, canvasH, items, childH); got != 100 {
		t.Errorf("with room to spare the menu moved from 100 to %v", got)
	}
	// Pressed near the bottom: every child must fit with the margin.
	got := selectionMenuFitY(560, menuH, canvasH, items, childH)
	for i, it := range items {
		if it.ChildMenu == nil {
			continue
		}
		bottom := got + float32(i)*rowH + childH(it.ChildMenu)
		if bottom > canvasH-16 {
			t.Errorf("submenu of %q would end at %v on a %v canvas — flush or cut off", it.Label, bottom, canvasH)
		}
	}
	if got >= 560 {
		t.Errorf("a bottom press must move the menu up, got %v", got)
	}
	// Never above the canvas.
	if got := selectionMenuFitY(5, menuH, 100, items, childH); got < 0 {
		t.Errorf("menu pushed above the canvas: %v", got)
	}
}
