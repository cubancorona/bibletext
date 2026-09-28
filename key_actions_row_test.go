package bibletext

// THE KEY BUTTONS FIT THEIR CARD.
//
// Under each API key field in Settings, the assistant's and API.Bible's, sit
// Paste, Test key and Clear. On one line they need 305.9pt. At 320pt, the
// iPhone SE's width, the key card gives its rows 252pt, and Clear ran past the
// card into the scroll's edge, which cut it off; at 375pt it ran 6.9pt past
// the key field, over the card's padding to its border. This holds, in both
// sections, that every button is whole, inside the card and reachable by a tap
// at every sheet width, and that where the three fit on one line they are
// exactly where the HBox they used to sit in put them.

import (
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// inWalkedTree reports whether target is root or lies under it, through
// everything walkTree descends.
func inWalkedTree(root, target fyne.CanvasObject) bool {
	found := false
	walkTree(root, func(o fyne.CanvasObject) {
		if o == target {
			found = true
		}
	})
	return found
}

// keySectionCard is the card of the Settings body that holds o: the child of
// the scrolling form with o somewhere under it.
func keySectionCard(t *testing.T, popup *widget.PopUp, o fyne.CanvasObject) fyne.CanvasObject {
	t.Helper()
	scroll := findScroll(popup.Content)
	if scroll == nil {
		t.Fatal("no scroll in the Settings sheet")
	}
	body, ok := scroll.Content.(*fyne.Container)
	if !ok || len(body.Objects) != 1 {
		t.Fatal("the Settings body is not the one form it should be")
	}
	form, ok := body.Objects[0].(*fyne.Container)
	if !ok {
		t.Fatal("the Settings body holds no form")
	}
	for _, c := range form.Objects {
		if inWalkedTree(c, o) {
			return c
		}
	}
	t.Fatal("the button is in no card of the Settings body")
	return nil
}

func TestKeyButtonsFitTheirCardAtEverySheetWidth(t *testing.T) {
	labels := []string{"Paste", "Test key", "Clear"}
	for _, sc := range []struct {
		name  string
		w, h  float32
		wraps bool // the three are wider than the card's rows here
	}{
		{"320pt phone", 320, 568, true},
		{"375pt phone", 375, 812, true},
		{"393pt phone", 393, 852, false},
		{"440pt phone", 440, 956, false},
		{"11-inch iPad", 834, 1194, false},
		{"13-inch iPad", 1032, 1376, false},
		{"desktop", 1280, 800, false},
	} {
		t.Run(sc.name, func(t *testing.T) {
			_, win, popup := settingsWithIncludedKey(t, sc.w, sc.h, 1, nil)
			drv := fyne.CurrentApp().Driver()
			scroll := findScroll(popup.Content)

			// Both sections' buttons, the assistant's first.
			byLabel := map[string][]*widget.Button{}
			walkTree(popup, func(o fyne.CanvasObject) {
				if b, ok := o.(*widget.Button); ok {
					byLabel[b.Text] = append(byLabel[b.Text], b)
				}
			})
			for _, l := range labels {
				if len(byLabel[l]) != 2 {
					t.Fatalf("found %d %q buttons, want the assistant's and API.Bible's", len(byLabel[l]), l)
				}
			}

			for i, section := range []string{"assistant", "API.Bible"} {
				row := []*widget.Button{byLabel["Paste"][i], byLabel["Test key"][i], byLabel["Clear"][i]}
				card := keySectionCard(t, popup, row[0])
				var field *widget.Entry
				var status *statusLine
				walkTree(card, func(o fyne.CanvasObject) {
					switch v := o.(type) {
					case *widget.Entry:
						field = v
					case *statusLine:
						status = v
					}
				})
				if field == nil || status == nil {
					t.Fatalf("%s: the card holds no key field or no status line", section)
				}

				// Bring the row into the body's view, then measure it on the
				// canvas.
				contentTop := drv.AbsolutePositionForObject(scroll.Content).Y
				scroll.ScrollToOffset(fyne.NewPos(0, drv.AbsolutePositionForObject(row[0]).Y-contentTop-60))
				test.WidgetRenderer(popup).Layout(popup.Size())

				at := func(o fyne.CanvasObject) (fyne.Position, fyne.Position) {
					p := drv.AbsolutePositionForObject(o)
					return p, p.Add(fyne.NewPos(o.Size().Width, o.Size().Height))
				}
				cardMin, cardMax := at(card)
				fieldMin, fieldMax := at(field)
				statusMin, _ := at(status)
				viewMin, viewMax := at(scroll)
				// The width the card gives its rows: inside its border and its
				// padding. Control: the key field is exactly that wide.
				inner := float32(settingsCardChromeWidth) / 2
				left, right := cardMin.X+inner, cardMax.X-inner
				if math.Abs(float64(fieldMin.X-left)) > 0.5 || math.Abs(float64(fieldMax.X-right)) > 0.5 {
					t.Fatalf("%s: control: the key field spans %.1f–%.1f, the card's rows %.1f–%.1f",
						section, fieldMin.X, fieldMax.X, left, right)
				}

				for j, b := range row {
					lo, hi := at(b)
					if !b.Visible() {
						t.Errorf("%s: %s is hidden", section, b.Text)
					}
					// Whole and unchanged: laid out at its own size, neither
					// squeezed nor stretched, on a line of its own or not.
					if want := b.MinSize(); math.Abs(float64(b.Size().Width-want.Width)) > 0.01 ||
						math.Abs(float64(b.Size().Height-want.Height)) > 0.01 {
						t.Errorf("%s: %s is %v, not its own %v", section, b.Text, b.Size(), want)
					}
					if lo.X < left-0.01 || hi.X > right+0.01 {
						t.Errorf("%s: %s spans %.1f–%.1f, outside the card's rows at %.1f–%.1f (the card %.1f–%.1f)",
							section, b.Text, lo.X, hi.X, left, right, cardMin.X, cardMax.X)
					}
					if lo.Y < fieldMax.Y-0.01 || hi.Y > statusMin.Y+0.01 || hi.Y > cardMax.Y-inner+0.01 {
						t.Errorf("%s: %s spans %.1f–%.1f down, not between the field ending at %.1f and the status starting at %.1f",
							section, b.Text, lo.Y, hi.Y, fieldMax.Y, statusMin.Y)
					}
					if lo.X < viewMin.X || hi.X > viewMax.X || lo.Y < viewMin.Y || hi.Y > viewMax.Y {
						t.Errorf("%s: %s is not wholly in the body's view", section, b.Text)
					}
					for _, other := range row[:j] {
						omin, omax := at(other)
						if lo.X < omax.X && omin.X < hi.X && lo.Y < omax.Y && omin.Y < hi.Y {
							t.Errorf("%s: %s lies over %s", section, b.Text, other.Text)
						}
					}

					// Tappable: a tap at its centre reaches it and nothing
					// else. A disabled button ignores taps, so it is enabled
					// for the tap and put back after.
					tapped := false
					prevTap, wasDisabled := b.OnTapped, b.Disabled()
					b.OnTapped = func() { tapped = true }
					b.Enable()
					test.TapCanvas(win.Canvas(), fyne.NewPos((lo.X+hi.X)/2, (lo.Y+hi.Y)/2))
					b.OnTapped = prevTap
					if wasDisabled {
						b.Disable()
					}
					if !tapped {
						t.Errorf("%s: a tap at the centre of %s does not reach it", section, b.Text)
					}
				}

				paste, testKey, clear := row[0], row[1], row[2]
				pMin, pMax := at(paste)
				tMin, tMax := at(testKey)
				cMin, _ := at(clear)
				pad := theme.Padding()
				if sc.wraps {
					// Control: here the three do not fit on one line.
					need := paste.MinSize().Width + testKey.MinSize().Width + clear.MinSize().Width + 2*pad
					if need <= right-left {
						t.Fatalf("control: the three need %.1fpt and the card's rows have %.1fpt; they should not fit",
							need, right-left)
					}
					// Test key follows Paste on the first line, and Clear,
					// the one that did not fit, starts the next under Paste.
					if math.Abs(float64(tMin.X-(pMax.X+pad))) > 0.01 || math.Abs(float64(tMin.Y-pMin.Y)) > 0.01 {
						t.Errorf("%s: Test key is at %v; want it after Paste, at %.1f, %.1f",
							section, tMin, pMax.X+pad, pMin.Y)
					}
					if math.Abs(float64(cMin.X-pMin.X)) > 0.01 || math.Abs(float64(cMin.Y-(pMax.Y+pad))) > 0.01 {
						t.Errorf("%s: Clear is at %v; want it under Paste, at %.1f, %.1f",
							section, cMin, pMin.X, pMax.Y+pad)
					}
					continue
				}
				// One line, exactly as the HBox laid them out: from the start
				// of the row, each a padding after the one before, all at
				// the row's top, and the status a padding under the row.
				want := []fyne.Position{
					{X: left, Y: fieldMax.Y + pad},
					{X: pMax.X + pad, Y: pMin.Y},
					{X: tMax.X + pad, Y: tMin.Y},
				}
				for j, b := range row {
					if got, _ := at(b); math.Abs(float64(got.X-want[j].X)) > 0.01 || math.Abs(float64(got.Y-want[j].Y)) > 0.01 {
						t.Errorf("%s: %s is at %.1f, %.1f; on one line it belongs at %.1f, %.1f",
							section, b.Text, got.X, got.Y, want[j].X, want[j].Y)
					}
				}
				if got := statusMin.Y; math.Abs(float64(got-(pMax.Y+pad))) > 0.01 {
					t.Errorf("%s: the status starts at %.1f; one line of buttons ends at %.1f and a padding follows",
						section, got, pMax.Y)
				}
			}
		})
	}
}

func TestWrapRowLayoutBreaksOnlyWhenARowIsTooWide(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	pad := theme.Padding()
	a, b, c := widget.NewButton("Paste", nil), widget.NewButton("Test key", nil), widget.NewButton("Clear", nil)
	row := keyActionsRow(a, b, c)
	one := a.MinSize().Width + b.MinSize().Width + c.MinSize().Width + 2*pad
	h := a.MinSize().Height

	if got := row.MinSize(); got.Height != h {
		t.Errorf("not yet laid out, the row is one line, %.1fpt; it asks for %.1fpt", h, got.Height)
	}
	// Exactly wide enough: one line.
	row.Resize(fyne.NewSize(one, h))
	if c.Position().Y != 0 || row.MinSize().Height != h {
		t.Errorf("at %.1fpt, its own width, the row broke: Clear at %v, height %.1f", one, c.Position(), row.MinSize().Height)
	}
	// A hair narrower: Clear starts a second line, and the row asks for it.
	row.Resize(fyne.NewSize(one-0.5, h))
	if c.Position() != fyne.NewPos(0, h+pad) {
		t.Errorf("at %.1fpt Clear is at %v, want the start of a second line at 0, %.1f", one-0.5, c.Position(), h+pad)
	}
	// Alone on its line it keeps its own size, as the other two keep theirs.
	for _, o := range []*widget.Button{a, b, c} {
		if o.Size() != o.MinSize() {
			t.Errorf("broken in two, %s is %v, not its own %v", o.Text, o.Size(), o.MinSize())
		}
	}
	if got := row.MinSize().Height; got != 2*h+pad {
		t.Errorf("broken in two, the row asks for %.1fpt, want %.1f", got, 2*h+pad)
	}
	// Narrower than any two: one button a line. The row is never wider than
	// its widest button.
	row.Resize(fyne.NewSize(b.MinSize().Width, h))
	if got := row.MinSize(); got.Height != 3*h+2*pad || got.Width != b.MinSize().Width {
		t.Errorf("one button a line, the row asks for %v, want %.1f x %.1f", got, b.MinSize().Width, 3*h+2*pad)
	}
	// A hidden button takes no room.
	b.Hide()
	row.Resize(fyne.NewSize(one, h))
	if c.Position().X != a.MinSize().Width+pad {
		t.Errorf("with Test key hidden, Clear is at %v; want it after Paste", c.Position())
	}
}
