package bibletext

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// TestDenseGridWrapConstantGap guards the Goto chapter-grid "changes size when you pick a
// book" bug: the inter-cell gap must be the fixed denseGridPadding no matter how many cells
// are laid out. The old theme-override approach read theme.Padding() from the rendering
// stack, so a book swap (which re-lays the grid out without the override's render push) made
// the gap revert to the app default and the grid visibly spread. A constant layout can't.
func TestDenseGridWrapConstantGap(t *testing.T) {
	mk := func(n int) []fyne.CanvasObject {
		objs := make([]fyne.CanvasObject, n)
		for i := range objs {
			objs[i] = canvas.NewRectangle(color.Black)
		}
		return objs
	}
	// Horizontal gap between cell 0 and cell 1 of the first row, after laying out n cells in
	// a pane wide enough for several columns.
	gap := func(n int) float32 {
		g := &denseGridWrapLayout{cell: fyne.NewSize(34, 34)}
		objs := mk(n)
		g.Layout(objs, fyne.NewSize(200, 500))
		return objs[1].Position().X - (objs[0].Position().X + 34)
	}
	for _, n := range []int{2, 5, 21, 50, 150} {
		if got := gap(n); got != denseGridPadding {
			t.Errorf("inter-cell gap for %d cells = %v, want %v (constant)", n, got, float32(denseGridPadding))
		}
	}
}
