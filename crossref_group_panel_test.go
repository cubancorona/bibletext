//go:build !race

package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// IN THE PANEL: a group's label is larger than a caption and smaller than a
// card's reference, its left edge is the cards', its tap is a full row, and
// opening or closing it moves nothing but the list inside its scroll — the
// panel keeps its size and its place.
func TestAGroupOpensInsideThePanelWithoutMovingIt(t *testing.T) {
	th := appThemeForTest(t)
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	withCrossRefIndex(t, "John.3.16\tRom.5.8\t10\n")
	previousRun, previousLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = previousRun, previousLoad })
	crossRefsRun = func(work func()) { work() }
	crossRefsLoad = func() error { return nil }
	st, _ := groupListState(t)
	w := test.NewWindow(widget.NewLabel("Reading"))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(375, 667))
	st.theme, st.window = th, w
	showCrossRefs(st, "", selSpan{lo: 16, hi: 16})
	top, ok := w.Canvas().Overlays().Top().(*widget.PopUp)
	if !ok {
		t.Fatal("the cross-references panel did not open")
	}
	var groups []*crossRefGroupRow
	var scroll *container.Scroll
	var reference *canvas.Text
	walkTree(top, func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *crossRefGroupRow:
			groups = append(groups, x)
		case *container.Scroll:
			if scroll == nil {
				scroll = x
			}
		case *canvas.Text:
			if x.Text == "Romans 5:8" {
				reference = x
			}
		}
	})
	if len(groups) != 3 || scroll == nil || reference == nil {
		t.Fatalf("%d group rows, scroll %v, the first card's reference %v", len(groups), scroll != nil, reference != nil)
	}
	g := groups[0]

	// The label: 14 points, between the caption's 11 and the reference's 16,
	// wrapping, with no inset of its own inside the list's scope.
	if got := g.label.Theme().Size(sizeNameCrossRefGroup); got != 14 || got <= g.label.Theme().Size(theme.SizeNameCaptionText) || got >= reference.TextSize {
		t.Errorf("the group's label is %v points; want 14, above the caption's %v and below the reference's %v", got, g.label.Theme().Size(theme.SizeNameCaptionText), reference.TextSize)
	}
	if g.label.Wrapping != fyne.TextWrapWord || g.label.Theme().Size(theme.SizeNameInnerPadding) != 0 {
		t.Errorf("the group's label does not wrap, or keeps an inset of its own")
	}
	drv := fyne.CurrentApp().Driver()
	if lx, rx := drv.AbsolutePositionForObject(g.label).X, drv.AbsolutePositionForObject(reference).X; lx != rx {
		t.Errorf("the group's label starts at %v and the cards' references at %v", lx, rx)
	}
	if h := g.header.Size().Height; h < 44 {
		t.Errorf("the group's tap is %v points tall, want at least 44", h)
	}
	if g.Size().Width > scroll.Size().Width {
		t.Errorf("the group row is %v wide in a list %v wide", g.Size().Width, scroll.Size().Width)
	}

	// Opening and closing: the panel does not move or change size; the list
	// grows inside the scroll and shrinks back.
	size, pos, content := top.Content.Size(), top.Content.Position(), scroll.Content.Size().Height
	test.Tap(g.header)
	if !g.open || len(g.members.Objects) != 3 {
		t.Fatalf("the tap did not open the group: open %v, %d cards", g.open, len(g.members.Objects))
	}
	if top.Content.Size() != size || top.Content.Position() != pos {
		t.Errorf("opening a group moved the panel: %v at %v, was %v at %v", top.Content.Size(), top.Content.Position(), size, pos)
	}
	opened := scroll.Content.Size().Height
	if opened <= content {
		t.Errorf("open, the list is %v tall inside the scroll, no taller than closed (%v): it was not laid out again", opened, content)
	}
	if g.inset.Size().Width > g.Size().Width || g.inset.Size().Height <= 0 {
		t.Errorf("the open group's ground is %v in a row %v", g.inset.Size(), g.Size())
	}
	// The cards on the ground are full taps too, and their previews wrap
	// inside it.
	walkTree(g.inset, func(o fyne.CanvasObject) {
		if card, ok := o.(*tapCard); ok && card.Size().Height < 44 {
			t.Errorf("a card of the open group is %v points tall", card.Size().Height)
		}
		if label, ok := o.(*widget.Label); ok && label.Size().Width > g.inset.Size().Width {
			t.Errorf("a preview of the open group is %v wide on a ground %v wide", label.Size().Width, g.inset.Size().Width)
		}
	})
	test.Tap(g.header)
	if g.open || scroll.Content.Size().Height != content || top.Content.Size() != size || top.Content.Position() != pos {
		t.Errorf("closing did not restore the list: open %v, list %v (was %v), panel %v at %v", g.open, scroll.Content.Size().Height, content, top.Content.Size(), top.Content.Position())
	}
}
