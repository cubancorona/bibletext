package bibletext

// THE BOOKS GRID SITS ON THE SCREEN'S CENTRE LINE.
//
// A wrapping grid fits a whole number of fixed-width columns and has a
// remainder by definition. Left-packing gives that remainder entirely to the
// right-hand side, which on an iPhone is 46pt — enough that the canon reads as
// nudged over rather than centred, and enough that nobody looking at code would
// notice, because 0 is the obvious x to start at.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

func gridCells(n int) []fyne.CanvasObject {
	objs := make([]fyne.CanvasObject, n)
	for i := range objs {
		objs[i] = canvas.NewRectangle(nil)
	}
	return objs
}

func TestBooksGridCentresItsColumns(t *testing.T) {
	// The iPhone case that prompted this: 385pt of pane, 168pt cells.
	const pane float32 = 385
	g := &denseGridWrapLayout{cell: fyne.NewSize(bookCellW, bookCellH), centre: true}
	objs := gridCells(6)
	g.Layout(objs, fyne.NewSize(pane, 400))

	cols := g.cols(pane)
	if cols != 2 {
		t.Fatalf("expected 2 columns in %.0fpt, got %d", pane, cols)
	}
	used := float32(cols)*bookCellW + float32(cols-1)*denseGridPadding
	wantLeft := (pane - used) / 2

	left := objs[0].Position().X
	if left != wantLeft {
		t.Errorf("first column starts at %.1f, want %.1f", left, wantLeft)
	}
	// The real assertion: equal air on both sides.
	right := pane - (objs[1].Position().X + bookCellW)
	if diff := left - right; diff > 0.5 || diff < -0.5 {
		t.Errorf("the grid has %.1fpt of air on the left and %.1fpt on the right — "+
			"it is off-centre by %.1fpt", left, right, diff)
	}
}

// Every row must start on the same x, or the columns stagger.
func TestBooksGridRowsShareTheSameLeftEdge(t *testing.T) {
	g := &denseGridWrapLayout{cell: fyne.NewSize(bookCellW, bookCellH), centre: true}
	objs := gridCells(5) // two full rows plus a short one
	g.Layout(objs, fyne.NewSize(385, 400))

	first := objs[0].Position().X
	for _, i := range []int{2, 4} { // starts of rows 2 and 3
		if got := objs[i].Position().X; got != first {
			t.Errorf("row starting at object %d begins at x=%.1f, want %.1f — the "+
				"columns stagger down the page", i, got, first)
		}
	}
	// The short last row stays left-aligned WITHIN the block, as a grid should:
	// centring it on its own width would float one book in the middle.
	if objs[4].Position().X != first {
		t.Error("the short final row was centred on itself instead of aligning to the columns")
	}
}

// The chapter picker shares this layout and must be unaffected.
func TestUncentredGridStillPacksLeft(t *testing.T) {
	g := &denseGridWrapLayout{cell: fyne.NewSize(34, 34)}
	objs := gridCells(4)
	g.Layout(objs, fyne.NewSize(385, 200))
	if got := objs[0].Position().X; got != 0 {
		t.Errorf("an uncentred grid starts at x=%.1f, want 0 — the chapter picker moved", got)
	}
}
