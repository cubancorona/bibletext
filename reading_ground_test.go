package bibletext

// THE READING GROUND CARRIES NO OUTLINE.
//
// The border this pins was not added on purpose: flattening the reading card
// removed its FILL but not its stroke on the Fyne path, and the styled pane
// inherited the same surface helper.
//
// It survived because nothing failed when it was there. This is that test: it
// walks the real object the desktop reading pane is built from and fails if a
// stroked rectangle reappears around the scripture.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// outerRect returns the first Rectangle found in depth-first order — the ground
// the reading content is stacked on.
func outerRect(o fyne.CanvasObject) *canvas.Rectangle {
	switch v := o.(type) {
	case *canvas.Rectangle:
		return v
	case *fyne.Container:
		for _, child := range v.Objects {
			if r := outerRect(child); r != nil {
				return r
			}
		}
	}
	return nil
}

func TestStyledReadingGroundHasNoOutline(t *testing.T) {
	st := psalm23State()
	st.CurrentVersion = "web"
	area := styledReadingScrollArea(st, st.Bible.GetChapter("Psalms", 23), lightPalette)

	r := outerRect(area)
	if r == nil {
		t.Fatal("the styled reading area has no ground rectangle at all")
	}
	if r.StrokeWidth != 0 {
		t.Errorf("the reading ground is stroked (width %v) — scripture is inside a "+
			"framed card again; iOS, iPadOS, Android and macOS all draw it "+
			"straight onto the page", r.StrokeWidth)
	}
	if r.CornerRadius != 0 {
		t.Errorf("the reading ground has a %vpt corner radius — the card is back", r.CornerRadius)
	}
}

// The padding must survive the change, or the fix quietly tightens every line
// against the pane edge. readingGround applies the same single NewPadded that
// surface() did, so the two wrap their content identically.
func TestReadingGroundKeepsSurfacePadding(t *testing.T) {
	inner := canvas.NewRectangle(lightPalette.Surface)
	ground := readingGround(inner, lightPalette.Background)
	framed := surface(inner, lightPalette.Background, lightPalette.Border, fyne.Size{})

	depth := func(o fyne.CanvasObject) int {
		n := 0
		for {
			c, ok := o.(*fyne.Container)
			if !ok || len(c.Objects) == 0 {
				return n
			}
			// Follow the CONTENT branch (the last child of the stack), not the
			// ground rectangle.
			o = c.Objects[len(c.Objects)-1]
			n++
		}
	}
	if g, f := depth(ground), depth(framed); g != f {
		t.Errorf("readingGround wraps its content %d deep and surface wraps it %d deep — "+
			"the padding changed, so removing the border moved the text", g, f)
	}
	if got := ground.(*fyne.Container).Objects[0].(*canvas.Rectangle).FillColor; got != lightPalette.Background {
		t.Errorf("the ground is filled %v, want the page colour %v", got, lightPalette.Background)
	}
}
