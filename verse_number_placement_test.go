package bibletext

import (
	"image"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// A verse number on the Windows and Linux pane is drawn as the Apple panes draw
// it — ordinary figures, in the bold cut, at 0.66 of the body, with the baseline
// lifted a third of the body above the text's — and its INK stays inside the
// line it opens, in EITHER of the pane's two leadings.
//
// The pane used to draw the run's own text, the Unicode superscript figures
// (superscriptNumber writes ¹²³), which the face draws small and raised already;
// setting them at 0.66 on top made a numeral 29% of a capital's height where the
// iPad's is 59%. The ink half is what the old version of this test guarded: a
// marked verse paints its wash over the line, so a numeral that climbed out of
// its line was left hanging over the colour with the words coloured beneath it.
// It is asked of the pixels now rather than of the text object's box, because
// the box carries the face's declared ascent, which is not ink.
//
// Both leadings are exercised at the SHIPPING body size: the reporter gate is a
// multiple of the body size, so a bare test theme puts every width on the same
// side of it and only one of the two leadings is ever tried.
func TestVerseNumbersAreDrawnAsTheApplePanesDrawThem(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(&bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()})

	for _, tc := range []struct {
		name  string
		width float32
		cozy  bool
	}{
		{"cozy column", 420, true},
		{"reporter page", 1100, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := sampleState()
			p := newStyledReadingPane(st, st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter))
			p.pal = lightPalette
			w := test.NewWindow(p)
			defer w.Close()
			w.SetPadded(false) // pane coordinates ARE capture coordinates
			w.Resize(fyne.NewSize(tc.width, 620))
			p.Resize(fyne.NewSize(tc.width, 620))
			p.Refresh()

			if cozy := p.lh >= p.textSize*1.4; cozy != tc.cozy {
				t.Fatalf("wanted the %s leading and got lh=%.2f at body %.1f — the case did not exercise what it names",
					tc.name, p.lh, p.textSize)
			}
			r, ok := test.WidgetRenderer(p).(*styledPaneRenderer)
			if !ok {
				t.Fatal("the pane's renderer is not the styled one")
			}

			drv := fyne.CurrentApp().Driver()
			bodyS, bodyBase := drv.RenderedTextSize("Ag", p.textSize, fyne.TextStyle{}, p.font)
			img := w.Canvas().Capture()
			scale := float32(img.Bounds().Dx()) / w.Canvas().Size().Width

			checked := 0
			for i, dr := range p.drawRuns {
				if dr.Kind != runVerseNum || i >= len(r.texts) {
					continue
				}
				txt := r.texts[i]
				for _, c := range txt.Text {
					if c < '0' || c > '9' {
						t.Fatalf("verse number drawn as %q — the superscript figures, which the face already shrinks", txt.Text)
					}
				}
				if txt.FontSource == nil || txt.FontSource.Name() != p.numeralFace().Name() {
					t.Errorf("verse number %q is not set in the bold cut the Apple panes use", txt.Text)
				}
				if want := p.textSize * styledNumRatio; txt.TextSize < want-0.01 || txt.TextSize > want+0.01 {
					t.Errorf("verse number %q drawn at %.2f, want %.2f (0.66 of the body)", txt.Text, txt.TextSize, want)
				}

				// The baseline: where the Apple panes put it. AppKit, laying
				// the Apple dialect out in the reading face at the Normal
				// size, raises a verse number's baseline 8.0pt on a 24pt
				// body; the figure is written here rather than read from the
				// pane's constant, so a changed constant cannot agree with
				// itself.
				const appleLift = float32(8.0 / 24.0)
				ln := p.lay.Lines[dr.Line]
				_, numBase := drv.RenderedTextSize(txt.Text, txt.TextSize, fyne.TextStyle{}, txt.FontSource)
				textBaseline := ln.Y + (p.lh-bodyS.Height)/2 + bodyBase
				numBaseline := txt.Position().Y + numBase
				if lift := textBaseline - numBaseline; lift < p.textSize*appleLift-0.05 || lift > p.textSize*appleLift+0.05 {
					t.Errorf("verse number %q sits %.2f above the text's baseline, want %.2f (a third of the body, as AppKit sets it)",
						txt.Text, lift, p.textSize*appleLift)
				}

				// The ink: every pixel of the number's colour in its column, from
				// the line above to the line below, must lie within the line it
				// opens. (Wider than that and the column meets the numbers that
				// open other lines at the same x.)
				x0 := int((p.insetX()+dr.X)*scale) - 2
				x1 := int((p.insetX()+dr.X+txt.MinSize().Width)*scale) + 6 // a text object inks a few px in from its origin
				y0, y1 := int((ln.Y-p.lh/2)*scale), int((ln.Y+ln.H+p.lh/2)*scale)
				top, bot, n := inkRows(img, x0, x1, y0, y1)
				if n == 0 {
					wt, wb, wn := inkRows(img, 0, img.Bounds().Dx(), y0, y1)
					t.Errorf("verse number %q drew no ink in columns %d..%d rows %d..%d (text at %v size %v; blue in the band: rows %d..%d, %d px)",
						txt.Text, x0, x1, y0, y1, txt.Position(), txt.MinSize(), wt, wb, wn)
					continue
				}
				if lnTop, lnBot := int(ln.Y*scale), int((ln.Y+ln.H)*scale); top < lnTop || bot >= lnBot {
					t.Errorf("verse number %q inks rows %d..%d, outside its line %d..%d (body %.1f, leading %.2f)",
						txt.Text, top, bot, lnTop, lnBot-1, p.textSize, p.lh)
				}
				checked++
			}
			if checked == 0 {
				t.Fatal("no verse number was examined — the test proved nothing")
			}
			t.Logf("%d verse numbers checked at %.1f body, leading %.2f, lift %.3f", checked, p.textSize, p.lh, styledNumLift)
		})
	}
}

// inkRows is the first and last row in [y0,y1), within columns [x0,x1), holding
// a pixel of the verse number's ink, and how many such pixels there are. The ink
// is told by its BLUE: the light palette's number colour is a slate blue, and
// neither the paper nor the body text nor their anti-aliased edges lean blue, so
// even the thin blended strokes of a small numeral are found and nothing else is.
func inkRows(img image.Image, x0, x1, y0, y1 int) (top, bot, n int) {
	top, bot = -1, -1
	b := img.Bounds()
	if y0 < b.Min.Y {
		y0 = b.Min.Y
	}
	for y := y0; y < y1 && y < b.Max.Y; y++ {
		for x := x0; x < x1 && x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r, g, bl := int(r>>8), int(g>>8), int(bl>>8); bl > r+20 && bl > g+8 {
				if top < 0 {
					top = y
				}
				bot = y
				n++
			}
		}
	}
	return top, bot, n
}
