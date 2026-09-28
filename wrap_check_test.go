package bibletext

// THE SETTINGS SWITCHES' LABELS FIT THEIR CARDS.
//
// "Show the translators' footnotes", "Show notes people share with you" and
// "Show the words of King Jesus in red" were widget.Check labels, drawn as one
// canvas.Text at full width whatever width the check was given. At 320pt all
// three ran past their cards into the scroll's edge, which cut them off, at
// 360pt the second and third did, and at 375pt the third. This holds that
// every word of each label is drawn inside its card's row and reached by a
// tap at every sheet width, and that where the label fits, the check is
// exactly the widget.Check it was.

import (
	"math"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var settingsSwitchLabels = []string{
	"Show the translators' footnotes",
	"Show notes people share with you",
	"Show the words of King Jesus in red",
}

// drawnCheckLines is what c draws of its label, top to bottom: its visible,
// non-empty texts.
func drawnCheckLines(c fyne.Widget) []*canvas.Text {
	var out []*canvas.Text
	for _, o := range test.WidgetRenderer(c).Objects() {
		if tx, ok := o.(*canvas.Text); ok && tx.Visible() && tx.Text != "" {
			out = append(out, tx)
		}
	}
	return out
}

// plainCheckAt is the widget.Check the switch used to be, laid out at size,
// with its label.
func plainCheckAt(text string, size fyne.Size) (*widget.Check, *canvas.Text) {
	plain := widget.NewCheck(text, nil)
	plain.Resize(size)
	lines := drawnCheckLines(plain)
	if len(lines) != 1 {
		return plain, nil
	}
	return plain, lines[0]
}

func TestSettingsSwitchLabelsFitTheirCardAtEverySheetWidth(t *testing.T) {
	for _, sc := range []struct {
		name string
		w, h float32
		// wrap is how many of the three break at this width: the control that
		// the test sees both shapes.
		wrap int
	}{
		{"320pt phone", 320, 568, 3},
		{"360dp phone", 360, 780, 2},
		{"375pt phone", 375, 812, 1},
		{"393pt phone", 393, 852, 0},
		{"440pt phone", 440, 956, 0},
		{"11-inch iPad", 834, 1194, 0},
		{"13-inch iPad", 1032, 1376, 0},
		{"desktop", 1280, 800, 0},
	} {
		t.Run(sc.name, func(t *testing.T) {
			_, win, popup := settingsWithIncludedKey(t, sc.w, sc.h, 1, nil)
			drv := fyne.CurrentApp().Driver()
			scroll := findScroll(popup.Content)
			byText := map[string]*wrapCheck{}
			walkTree(popup, func(o fyne.CanvasObject) {
				if c, ok := o.(*wrapCheck); ok {
					byText[c.Text] = c
				}
			})
			wrapped := 0
			for _, label := range settingsSwitchLabels {
				c := byText[label]
				if c == nil {
					t.Fatalf("no %q switch in the Settings sheet", label)
				}
				card := keySectionCard(t, popup, c)
				contentTop := drv.AbsolutePositionForObject(scroll.Content).Y
				scroll.ScrollToOffset(fyne.NewPos(0, drv.AbsolutePositionForObject(c).Y-contentTop-60))
				test.WidgetRenderer(popup).Layout(popup.Size())

				cardAt := drv.AbsolutePositionForObject(card)
				inner := float32(settingsCardChromeWidth) / 2
				left, right := cardAt.X+inner, cardAt.X+card.Size().Width-inner
				at := drv.AbsolutePositionForObject(c)
				size, asks := c.Size(), c.MinSize()
				if math.Abs(float64(at.X-left)) > 0.5 || math.Abs(float64(at.X+size.Width-right)) > 0.5 {
					t.Fatalf("%q: control: the switch spans %.1f–%.1f, the card's rows %.1f–%.1f",
						label, at.X, at.X+size.Width, left, right)
				}
				if size.Width < asks.Width-0.01 || size.Height < asks.Height-0.01 {
					t.Errorf("%q is laid out %v, smaller than the %v it asks for", label, size, asks)
				}

				lines := drawnCheckLines(c)
				var words []string
				for i, tx := range lines {
					words = append(words, tx.Text)
					p := drv.AbsolutePositionForObject(tx)
					end := tx.MinSize()
					if p.X+end.Width > right+0.01 {
						t.Errorf("%q: %q ends at %.1f, past the card's rows ending at %.1f", label, tx.Text, p.X+end.Width, right)
					}
					if p.Y < at.Y-0.01 || p.Y+end.Height > at.Y+size.Height+0.01 {
						t.Errorf("%q: %q spans %.1f–%.1f down, outside the switch's %.1f–%.1f",
							label, tx.Text, p.Y, p.Y+end.Height, at.Y, at.Y+size.Height)
					}
					if i > 0 {
						if prev := lines[i-1]; tx.Position().Y < prev.Position().Y+prev.MinSize().Height-0.01 {
							t.Errorf("%q: %q starts over the line above it", label, tx.Text)
						}
					}
				}
				if got := strings.Join(words, " "); got != label {
					t.Errorf("the switch draws %q, want the whole of %q", got, label)
				}
				if len(lines) > 1 {
					wrapped++
					// The lines as a block centred down the switch, as one line
					// is.
					first, last := lines[0], lines[len(lines)-1]
					above := first.Position().Y
					below := size.Height - last.Position().Y - last.MinSize().Height
					if math.Abs(float64(above-below)) > 0.5 {
						t.Errorf("%q: the lines sit %.1fpt from the top and %.1fpt from the bottom", label, above, below)
					}
				} else if len(lines) == 1 {
					// One line: exactly the widget.Check it used to be, at this
					// size — the same text in the same place, and the same size
					// asked for wherever the row has it.
					plain, plainText := plainCheckAt(label, size)
					if r, ok := test.WidgetRenderer(c).(*wrapCheckRenderer); !ok || lines[0] != r.label {
						t.Errorf("%q: on one line the switch does not draw the check's own label", label)
					}
					if plainText != nil && (lines[0].Position() != plainText.Position() || lines[0].Size() != plainText.Size()) {
						t.Errorf("%q: the label is at %v, %v; widget.Check draws it at %v, %v",
							label, lines[0].Position(), lines[0].Size(), plainText.Position(), plainText.Size())
					}
					if own := plain.MinSize(); size.Width >= own.Width && asks != own {
						t.Errorf("%q asks for %v; widget.Check asks %v", label, asks, own)
					}
				}

				// Tappable across the box and every word: a tap at the box, at
				// the middle of the last line and at the end of the widest
				// reaches it. The switch's own effect is set aside for the tap
				// and its state put back after.
				prevChanged, prevChecked := c.OnChanged, c.Checked
				var widest *canvas.Text
				for _, tx := range lines {
					if widest == nil || tx.MinSize().Width > widest.MinSize().Width {
						widest = tx
					}
				}
				last := lines[len(lines)-1]
				lp, wp := drv.AbsolutePositionForObject(last), drv.AbsolutePositionForObject(widest)
				for _, spot := range []struct {
					name string
					pos  fyne.Position
				}{
					{"the box", fyne.NewPos(at.X+theme.Padding()+2, at.Y+size.Height/2)},
					{"the last line", lp.Add(fyne.NewPos(last.MinSize().Width/2, last.MinSize().Height/2))},
					{"the end of the widest line", wp.Add(fyne.NewPos(widest.MinSize().Width-1, widest.MinSize().Height/2))},
				} {
					tapped := 0
					c.OnChanged = func(bool) { tapped++ }
					test.TapCanvas(win.Canvas(), spot.pos)
					c.OnChanged = nil
					c.SetChecked(prevChecked)
					if tapped != 1 {
						t.Errorf("%q: a tap at %s does not reach it", label, spot.name)
					}
				}
				c.OnChanged = prevChanged
			}
			if wrapped != sc.wrap {
				t.Errorf("control: %d of the three labels break at %.0fpt, want %d", wrapped, sc.w, sc.wrap)
			}
		})
	}
}

func TestWrapCheckBreaksOnlyWhenItsLabelIsTooWide(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(&bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()})
	const label = "Show the words of King Jesus in red"
	c := newWrapCheck(label, nil)
	plain := widget.NewCheck(label, nil)
	own := plain.MinSize()
	if got := c.MinSize(); got != own {
		t.Errorf("not yet laid out, it asks for %v; widget.Check asks %v", got, own)
	}
	r := test.WidgetRenderer(c).(*wrapCheckRenderer)
	plain.Resize(own)
	plainText := drawnCheckLines(plain)[0]
	// Control on the one assumption about widget.Check's renderer: its label
	// starts where lead says.
	if got := plainText.Position().X; got != r.lead() {
		t.Fatalf("widget.Check starts its label at %.1f; lead is %.1f", got, r.lead())
	}
	lineH := plainText.MinSize().Height

	// Its own width: the check's one label, where widget.Check draws it.
	c.Resize(own)
	if lines := drawnCheckLines(c); len(lines) != 1 || lines[0].Position() != plainText.Position() || c.MinSize() != own {
		t.Errorf("at its own width it draws %d lines and asks %v", len(lines), c.MinSize())
	}
	// Just wide enough for the words, without the room the check keeps after
	// them: still one line, and it asks for no more than it was given.
	fits := r.lead() + plainText.MinSize().Width
	c.Resize(fyne.NewSize(fits, own.Height))
	if lines := drawnCheckLines(c); len(lines) != 1 {
		t.Errorf("at %.1fpt, wide enough for the words, it broke into %d lines", fits, len(lines))
	}
	if got := c.MinSize(); got.Width != fits || got.Height != own.Height {
		t.Errorf("at %.1fpt it asks for %v, want %.1f x %.1f", fits, got, fits, own.Height)
	}
	// With some of that room: it asks for what it has, so taps reach across
	// it as they do across widget.Check's.
	c.Resize(fyne.NewSize(fits+3, own.Height))
	if got := c.MinSize(); len(drawnCheckLines(c)) != 1 || got.Width != fits+3 {
		t.Errorf("at %.1fpt it asks for %v, want %.1f wide on one line", fits+3, got, fits+3)
	}
	// A hair narrower: two lines, the words split between them, and it asks
	// for the second.
	narrow := fits - 0.5
	c.Resize(fyne.NewSize(narrow, own.Height))
	c.Resize(fyne.NewSize(narrow, c.MinSize().Height))
	lines := drawnCheckLines(c)
	var words []string
	for _, tx := range lines {
		words = append(words, tx.Text)
		if w := tx.MinSize().Width; tx.Position().X+w > narrow+0.01 {
			t.Errorf("at %.1fpt the line %q ends at %.1f", narrow, tx.Text, tx.Position().X+w)
		}
	}
	if len(lines) != 2 || strings.Join(words, " ") != label {
		t.Errorf("at %.1fpt it draws %q, want the label on two lines", narrow, words)
	}
	if got := c.MinSize(); got.Height != own.Height+lineH || got.Width > narrow {
		t.Errorf("on two lines it asks for %v, want at most %.1f wide and %.1f tall", got, narrow, own.Height+lineH)
	}
	// Taps reach the end of the first line, which runs past where the check's
	// words would have been measured as one.
	c.MinSize()
	toggled := false
	c.OnChanged = func(bool) { toggled = true }
	first := lines[0]
	c.Tapped(&fyne.PointEvent{Position: first.Position().Add(fyne.NewPos(first.MinSize().Width-1, first.MinSize().Height/2))})
	if !toggled || !c.Checked {
		t.Error("a tap at the end of the first line does not reach the check")
	}
	// A refresh, as ticking it makes, keeps the lines.
	if got := drawnCheckLines(c); len(got) != 2 {
		t.Errorf("after the tick's refresh it draws %d lines", len(got))
	}
	// Wide again: the check's own label, and widget.Check's size.
	c.Resize(fyne.NewSize(own.Width+40, own.Height))
	if lines := drawnCheckLines(c); len(lines) != 1 || c.MinSize() != own {
		t.Errorf("wide again it draws %d lines and asks %v", len(lines), c.MinSize())
	}
}
