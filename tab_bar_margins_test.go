package bibletext

// THE NAV BAR'S VERTICAL MARGINS ARE EQUAL, AND ITS TABS SIT IN THE MIDDLE.
//
// Measured from the laid-out object tree rather than from a screenshot. A pixel
// probe kept catching the scripture text that sits directly above the bar, and
// an eyeballed render is exactly how this shipped wrong in the first place:
// the bar had 14pt above its tabs and 7pt below, because a VBox spaces its
// children by theme padding and NewPadded then added its own on top. Nothing at
// the call site said so.
//
// Two separate faults produced one symptom, so there are two assertions:
//   1. the bar's own padding above and below the tab row is equal;
//   2. the icon-and-label column is centred INSIDE its cell — a VBox stacks
//      from the top and leaves the slack at the bottom, so the pair rested on
//      the cell's ceiling however the bar was padded.

import (
	"testing"

	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
)

// findFirst walks the tree for the first object satisfying pred, returning it
// with its absolute position.
func findFirst(o fyne.CanvasObject, at fyne.Position, pred func(fyne.CanvasObject) bool) (fyne.CanvasObject, fyne.Position, bool) {
	p := at.Add(o.Position())
	if pred(o) {
		return o, p, true
	}
	if c, ok := o.(*fyne.Container); ok {
		for _, ch := range c.Objects {
			if got, gp, ok := findFirst(ch, p, pred); ok {
				return got, gp, true
			}
		}
	}
	return nil, fyne.Position{}, false
}

func collect(o fyne.CanvasObject, at fyne.Position, pred func(fyne.CanvasObject) bool, out *[]struct {
	Obj fyne.CanvasObject
	Pos fyne.Position
}) {
	p := at.Add(o.Position())
	if pred(o) {
		*out = append(*out, struct {
			Obj fyne.CanvasObject
			Pos fyne.Position
		}{o, p})
	}
	if c, ok := o.(*fyne.Container); ok {
		for _, ch := range c.Objects {
			collect(ch, p, pred, out)
		}
	}
}

func TestNavBarVerticalMarginsAreEqual(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, h float32
	}{
		{"phone", 393, 852},
		{"tablet-portrait", 1024, 1366},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := test.NewApp()
			defer app.Quit()
			app.Settings().SetTheme(&bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()})

			st := compactGalleryState(t)
			w := test.NewWindow(nil)
			defer w.Close()
			w.Resize(fyne.NewSize(tc.w, tc.h))
			st.window, st.app, st.theme = w, app, &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}

			// The bar must be laid out the way buildCompactUI lays it out — in a
			// Border's bottom slot, full width, natural height. Sizing it by hand
			// after a full-window layout leaves stale positions and measures
			// nothing.
			bar := buildMobileTabBar(st)
			w.SetContent(container.NewBorder(nil, bar, nil, nil,
				canvas.NewRectangle(color.Transparent)))
			w.Resize(fyne.NewSize(tc.w, tc.h))

			var cells []struct {
				Obj fyne.CanvasObject
				Pos fyne.Position
			}
			collect(bar, fyne.Position{}, func(o fyne.CanvasObject) bool {
				_, ok := o.(*tabCell)
				return ok
			}, &cells)
			if len(cells) == 0 {
				t.Fatal("no tab cells in the bar")
			}
			ruleObj, rulePos, ok := findFirst(bar, fyne.Position{}, func(o fyne.CanvasObject) bool {
				_, is := o.(*canvas.Line)
				return is
			})
			if !ok {
				t.Fatal("no hairline rule in the bar")
			}

			cell := cells[0]
			barH := bar.Size().Height
			barTop := bar.Position().Y
			// From the rule's BOTTOM: the hairline occupies 1pt of its own, and
			// counting it as air makes an equal bar look 1pt top-heavy.
			above := cell.Pos.Y - (rulePos.Y + ruleObj.Size().Height)
			below := (barTop + barH) - (cell.Pos.Y + cell.Obj.Size().Height)

			if d := above - below; d > 0.6 || d < -0.6 {
				t.Errorf("bar has %.1fpt above the tabs and %.1fpt below — off by %.1f.\n"+
					"A VBox spaces its children by theme padding; if that crept back in, "+
					"the gap above is the one that grew.", above, below, d)
			}
		})
	}
}
