package bibletext

// THE RAIL STANDS BESIDE A SIDEWAYS PHONE'S SIDE INSETS, NEVER UNDER THEM.
//
// A phone held sideways has the rail (mobileRailWanted), on the leading edge,
// and on an iPhone that edge can be the Dynamic Island's. Nothing in the app
// moves the rail clear of it: both mobile drivers lay the window's whole tree
// inside the canvas's safe area, so the rail and the content beside it are
// there by the rule Android's rail has always had (tab_rail.go). These lay
// the real window out as a phone's driver does (insetWindow,
// sheet_side_insets_test.go) and read where everything in it lies. The host
// builds the tree with the desktop's compactNavRail, so each case first asks
// the phone's rule at its geometry and points BIBLETEXT_DESKTOP_TABS at the
// answer, and asks railForWindow again of the live window.

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

// canvasBox is one object of the window's tree and where it lies on the canvas.
type canvasBox struct {
	obj            fyne.CanvasObject
	x0, x1, y0, y1 float32
}

func (p canvasBox) String() string {
	return fmt.Sprintf("%T at x %.1f..%.1f, y %.1f..%.1f", p.obj, p.x0, p.x1, p.y0, p.y1)
}

// windowBoxes is every visible object of the window's tree with a size, at
// its place on the canvas, through containers and widgets' renderers. A
// scroll is read as its own box: what it holds lies wherever the scroll
// brings it. The canvas's content is insetWindow's stand-in for the driver,
// which places the window's tree (safeAreaLayout); the walk starts at what it
// holds.
func windowBoxes(c fyne.Canvas) []canvasBox {
	var out []canvasBox
	var walk func(o fyne.CanvasObject, at fyne.Position)
	walk = func(o fyne.CanvasObject, at fyne.Position) {
		if o == nil || !o.Visible() {
			return
		}
		if sz := o.Size(); sz.Width > 0 && sz.Height > 0 {
			out = append(out, canvasBox{o, at.X, at.X + sz.Width, at.Y, at.Y + sz.Height})
		}
		switch v := o.(type) {
		case *fyne.Container:
			for _, ch := range v.Objects {
				walk(ch, at.Add(ch.Position()))
			}
		case *container.Scroll:
		case fyne.Widget:
			for _, ch := range test.WidgetRenderer(v).Objects() {
				walk(ch, at.Add(ch.Position()))
			}
		}
	}
	for _, o := range c.Content().(*fyne.Container).Objects {
		walk(o, o.Position())
	}
	return out
}

// navCells is the navigation's cells in parts, in the order drawn.
func navCells(parts []canvasBox) []canvasBox {
	var out []canvasBox
	for _, p := range parts {
		if _, ok := p.obj.(*tabCell); ok {
			out = append(out, p)
		}
	}
	return out
}

// underASide lists the parts that reach under one of s's side insets.
func underASide(s insetScreen, parts []canvasBox) []canvasBox {
	var out []canvasBox
	for _, p := range parts {
		if p.x0 < s.left || p.x1 > s.w-s.right {
			out = append(out, p)
		}
	}
	return out
}

// navWindow lays the real window out on s, on tab, with the navigation the
// rule chooses for a phone there.
func navWindow(t *testing.T, s insetScreen, tab int) (*AppState, fyne.Window, bool) {
	t.Helper()
	rail := mobileRailWanted(false, s.w, s.h)
	if rail {
		t.Setenv("BIBLETEXT_DESKTOP_TABS", "rail")
	} else {
		t.Setenv("BIBLETEXT_DESKTOP_TABS", "bar")
	}
	st, w := insetWindow(t, s, tab)
	if got := railForWindow(st); got != rail {
		t.Fatalf("control: %s: the live window's rule says rail = %v, the geometry's %v", s.name, got, rail)
	}
	return st, w, rail
}

// THE RAIL KEEPS CLEAR OF THE SIDE INSETS ON EVERY PHONE HELD SIDEWAYS. On an
// iPhone 17 Pro Max and an iPhone 16 Pro held sideways, with the insets UIKit
// reports there (62 points at each side, the island's and the one opposite,
// whichever way the phone is turned); with the island's side alone inset, on
// the left and on the right, as Android reports a cutout; and on an Android
// phone with its cutout on the left or its three-button navigation bar on the
// right: on Books and on Search, the rail is drawn, a column of every
// destination standing right at the safe area's leading edge, inside the
// window's padding, so neither under an inset nor held further in than the
// driver puts the window; and nothing in the window, the rail, the header or
// the content beside the rail, reaches under either side inset. The control
// lays the same tree across the whole canvas, as a driver that reported no
// safe area would, and the check finds it under the inset.
func TestTheRailKeepsClearOfTheSideInsets(t *testing.T) {
	for _, s := range []insetScreen{
		proMaxSideways,
		{"iPhone 16 Pro, landscape", 874, 402, 0, 20, 62, 62},
		{"iPhone 17 Pro Max, landscape, the island's side alone inset, left", 956, 440, 0, 20, 62, 0},
		{"iPhone 17 Pro Max, landscape, the island's side alone inset, right", 956, 440, 0, 20, 0, 62},
		{"Android phone, landscape, cutout on the left", 803, 360, 0, 0, 21, 0},
		{"Android phone, landscape, three-button navigation on the right", 803, 360, 21, 0, 0, 42},
	} {
		for _, tab := range []int{1, 2} {
			t.Run(fmt.Sprintf("%s, tab %d", s.name, tab), func(t *testing.T) {
				_, w, rail := navWindow(t, s, tab)
				if !rail {
					t.Fatalf("control: %s is wider than tall and has no rail", s.name)
				}
				pad := theme.Padding()
				parts := windowBoxes(w.Canvas())
				cells := navCells(parts)
				if len(cells) != len(tabDestinations()) {
					t.Fatalf("control: %d navigation cells drawn, want %d", len(cells), len(tabDestinations()))
				}
				for i, c := range cells {
					if c.x0 != s.left+pad {
						t.Errorf("rail cell %d starts at x %.1f; the safe area's leading edge inside the window's padding is %.1f",
							i, c.x0, s.left+pad)
					}
					if i > 0 && c.y0 < cells[i-1].y1 {
						t.Errorf("rail cell %d (%v) is not below cell %d (%v): the navigation is not a column", i, c, i-1, cells[i-1])
					}
				}
				for _, p := range underASide(s, parts) {
					t.Errorf("%v reaches under a side inset (the safe area runs from x %.0f to %.0f)", p, s.left, s.w-s.right)
				}

				// The control: the tree laid across the whole canvas.
				root := w.Canvas().Content().(*fyne.Container)
				root.Layout = safeAreaLayout{insetScreen{w: s.w, h: s.h}, pad}
				root.Refresh()
				if len(underASide(s, windowBoxes(w.Canvas()))) == 0 {
					t.Fatalf("control: laid across the whole canvas, nothing reaches under %s's side insets", s.name)
				}
			})
		}
	}
}

// THE BOTTOM BAR, AND ITS CENTRED DRESS, ARE AN UPRIGHT WINDOW'S. Every phone
// and tablet has the rail held sideways and the bar upright. Upright, a phone's
// bar spreads its tabs across the width as before, and an iPad's or an Android
// tablet's, wider than tabBarSpreadMaxWidth, centres them at their designed
// slots. So no phone wears the centred dress: a sideways iPhone's bar, the one
// place a phone did, is now the rail. Shown to fail on the rule before it,
// where the three iPhones held sideways drew the centred bar.
func TestTheCentredBarIsAnUprightTablets(t *testing.T) {
	test.NewTempApp(t)
	for _, d := range navDevices {
		for _, g := range []struct {
			how  string
			w, h float32
		}{{"upright", d.w, d.h}, {"sideways", d.h, d.w}} {
			st := sampleState()
			win := test.NewWindow(nil)
			win.Resize(fyne.NewSize(g.w, g.h))
			st.window = win
			got := "the rail"
			if !railRule(d, g.w, g.h) {
				switch tabBarStyleFor(st) {
				case tabBarEdgeSpread:
					got = "the spread bar"
				case tabBarEdgeCentred:
					got = "the centred bar"
				default:
					got = "the pill"
				}
			}
			want := "the rail"
			switch {
			case g.how == "upright" && d.tablet:
				want = "the centred bar"
			case g.how == "upright":
				want = "the spread bar"
			}
			if got != want {
				t.Errorf("%s held %s (%vx%v) draws %s, want %s", d.name, g.how, g.w, g.h, got, want)
			}
			win.Close()
		}
	}
}
