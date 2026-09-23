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
			app.Settings().SetTheme(&bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()})

			st := tabBarTestState()
			w := test.NewWindow(nil)
			defer w.Close()
			w.Resize(fyne.NewSize(tc.w, tc.h))
			st.window, st.app, st.theme = w, app, &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}

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

			// Measured on what is DRAWN, the icon and its label, not on the cell:
			// the cell is the tap target and fills the bar (see
			// TestNavBarIsTappableAcrossItsHeight), so its edges say nothing
			// about the air a reader sees.
			cell := cells[0]
			iconTop, labelBottom := drawnSpan(t, cell.Obj.(*tabCell), cell.Pos)
			barH := bar.Size().Height
			barTop := bar.Position().Y
			// From the rule's BOTTOM: the hairline occupies 1pt of its own, and
			// counting it as air makes an equal bar look 1pt top-heavy.
			above := iconTop - (rulePos.Y + ruleObj.Size().Height)
			below := (barTop + barH) - labelBottom

			if d := above - below; d > 0.6 || d < -0.6 {
				t.Errorf("bar has %.1fpt above the tabs and %.1fpt below — off by %.1f.\n"+
					"A VBox spaces its children by theme padding; if that crept back in, "+
					"the gap above is the one that grew.", above, below, d)
			}
		})
	}
}

// drawnSpan returns the absolute top of a tab's icon and bottom of its label.
// at is the cell's own absolute position, as collect reports it.
func drawnSpan(t *testing.T, c *tabCell, at fyne.Position) (top, bottom float32) {
	t.Helper()
	top, bottom = -1, -1
	var walk func(o fyne.CanvasObject, p fyne.Position)
	walk = func(o fyne.CanvasObject, p fyne.Position) {
		p = p.Add(o.Position())
		switch o.(type) {
		case *canvas.Image:
			top = p.Y
		case *canvas.Text:
			bottom = p.Y + o.Size().Height
		}
		if cont, ok := o.(*fyne.Container); ok {
			for _, ch := range cont.Objects {
				walk(ch, p)
			}
		}
	}
	for _, o := range test.WidgetRenderer(c).Objects() {
		walk(o, at)
	}
	if top < 0 || bottom < 0 {
		t.Fatal("a tab cell no longer draws an icon and a label")
	}
	return top, bottom
}

// THE WHOLE BAR IS THE TAP TARGET.
//
// The mobile driver moves every touch up 8pt (tapYOffset) to allow for how a
// finger lands. When the bar's air sat outside the tabs, a tab answered only
// from the top of its icon down, so a tap on the icon's upper half landed in
// that air and hit nothing: the tab had to be tapped twice. The air now lives
// inside each tab, so a tap anywhere in a tab's column of the bar selects it.
// Taps go through the toolkit's own hit test (test.TapCanvas uses the same
// FindObjectAtPositionMatching the mobile canvas does).
func TestNavBarIsTappableAcrossItsHeight(t *testing.T) {
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
			app.Settings().SetTheme(&bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()})
			st := tabBarTestState()
			w := test.NewWindow(nil)
			defer w.Close()
			w.Resize(fyne.NewSize(tc.w, tc.h))
			st.window, st.app, st.theme = w, app, &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
			bar := buildMobileTabBar(st)
			w.SetContent(container.NewBorder(nil, bar, nil, nil,
				canvas.NewRectangle(color.Transparent)))
			w.Resize(fyne.NewSize(tc.w, tc.h))

			// Taps land on the canvas, so every coordinate here is the one the
			// toolkit's hit test uses: positions summed down from the canvas
			// content. (The test driver's AbsolutePositionForObject disagrees
			// with it by a few points, so it cannot be used to aim a tap.)
			var cells, rules, bars []struct {
				Obj fyne.CanvasObject
				Pos fyne.Position
			}
			root := w.Canvas().Content()
			collect(root, fyne.Position{}, func(o fyne.CanvasObject) bool {
				_, ok := o.(*tabCell)
				return ok
			}, &cells)
			collect(root, fyne.Position{}, func(o fyne.CanvasObject) bool {
				_, ok := o.(*canvas.Line)
				return ok
			}, &rules)
			collect(root, fyne.Position{}, func(o fyne.CanvasObject) bool { return o == bar }, &bars)
			if len(cells) == 0 || len(rules) == 0 || len(bars) != 1 {
				t.Fatalf("bar not found as expected: %d cells, %d rules, %d bars", len(cells), len(rules), len(bars))
			}
			ruleBottom := rules[0].Pos.Y + rules[0].Obj.Size().Height
			barBottom := bars[0].Pos.Y + bar.Size().Height

			hit := -1
			for i, c := range cells {
				i := i
				c.Obj.(*tabCell).onTapped = func() { hit = i }
			}
			for i, c := range cells {
				iconTop, labelBottom := drawnSpan(t, c.Obj.(*tabCell), c.Pos)
				x := c.Pos.X + c.Obj.Size().Width/2
				for where, y := range map[string]float32{
					"just under the rule":         ruleBottom + 1,
					"in the air above the icon":   (ruleBottom + iconTop) / 2,
					"on the icon's top edge":      iconTop + 0.5,
					"in the air below the label":  (labelBottom + barBottom) / 2,
					"just above the bar's bottom": barBottom - 1,
				} {
					hit = -1
					test.TapCanvas(w.Canvas(), fyne.NewPos(x, y))
					if hit != i {
						t.Errorf("tab %d: a tap %s (y=%.1f) selected %d, want %d", i, where, y, hit, i)
					}
				}
			}
		})
	}
}

// tabBarTestState is all the bar reads: the palette, the current tab and the
// canvas width. It deliberately needs no Bible on disk — the gallery state does,
// and skips without one, which would leave these tests running nowhere but on a
// machine that has already run the app.
func tabBarTestState() *AppState {
	return &AppState{CurrentVersion: "web", loadPhase: loadReady}
}
