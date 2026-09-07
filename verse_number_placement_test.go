package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// The verse number must not be placed above the top of the line it opens, in
// EITHER of the pane's two leadings.
//
// The number is already a superscript before the pane touches it
// (superscriptNumber writes ¹²³), so how high it sits inside its own box is the
// reading face's decision. The shipped face draws those glyphs 0.232 em higher
// than the system serif the pane used to borrow, measured from both faces'
// outlines, and lifting them again floated them clear of the capitals beside
// them and into the interline space. A marked verse paints its wash from the
// line top down, so the words were coloured and the number left hanging above.
//
// This checks the placement rather than the pixels because the two are the same
// question here and the placement can be stated exactly: the box may not begin
// above its line. Both leadings are exercised, at the SHIPPING body size, which
// is what the first attempt missed — it was calibrated against the cozy column
// alone, and the reporter page has a shorter line, so the same offset put the
// numeral further out.
func TestVerseNumberIsNotPlacedAboveItsLine(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	// The shipping sizes. The reporter gate is a multiple of the body size, so
	// a bare test theme puts every width on the same side of it and only one
	// of the two leadings is ever exercised.
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
			w := fyne.CurrentApp().NewWindow(tc.name)
			w.SetContent(p)
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

			checked := 0
			for i, dr := range p.drawRuns {
				if dr.Kind != runVerseNum || i >= len(r.texts) {
					continue
				}
				ln := p.lay.Lines[dr.Line]
				rel := r.texts[i].Position().Y - ln.Y
				if rel < 0 {
					t.Errorf("verse number %q starts %.2fpx above the top of its own line "+
						"(line %d at y=%.2f, body %.1fpt, leading %.2f)",
						dr.Text, -rel, dr.Line, ln.Y, p.textSize, p.lh)
				}
				checked++
			}
			if checked == 0 {
				t.Fatal("no verse number was examined — the test proved nothing")
			}
			t.Logf("%d verse numbers checked at %.0fpt body, leading %.2f, styledNumRaise=%.3f", checked, p.textSize, p.lh, styledNumRaise)
		})
	}
}
