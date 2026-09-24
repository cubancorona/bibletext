package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// The Windows and Linux pane draws the page the spec gives it (reading_page.go).

// In a wide window the book page is reached at EVERY text size. The column used
// to be capped at 760 units, which the widest book page — 750.75 plus 15 each
// side at Extra large — does not fit in; once the pane took the reading size
// the Extra large page could never be the book page there.
func TestCanvasPaneReachesTheBookPageAtEverySize(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(&bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()})
	t.Cleanup(func() { setReadingTextSizeID("normal") })

	for _, size := range []string{"normal", "large", "xl"} {
		t.Run(size, func(t *testing.T) {
			setReadingTextSizeID(size)
			st := reporterTestState()
			area := styledReadingScrollArea(st, st.Bible.GetChapter("Romans", 8), lightPalette)
			w := test.NewWindow(area)
			defer w.Close()
			w.Resize(fyne.NewSize(1400, 700))

			p := findStyledPane(area)
			if p == nil {
				t.Fatal("no styled pane in the reading area")
			}
			if !p.page.Book() || p.extraInset <= 0 {
				t.Errorf("at %s in a 1400-wide window the pane is on the %v page (extraInset %v, pane %.1f wide)",
					size, p.page.Kind, p.extraInset, p.Size().Width)
			}
		})
	}
}

// The page changes where the spec says, and the phone page's ink starts the
// spec's side minimum from the pane's edge.
func TestCanvasPaneTakesItsPageFromTheSpec(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(&bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()})

	st := reporterTestState()
	p := newStyledReadingPane(st, st.Bible.GetChapter("Romans", 8))
	sw := float32(reporterMeasureEm*float64(p.referenceSize()) + 2*readingPageSideMin)

	p.relayout(sw - 1)
	if p.page.Book() {
		t.Errorf("a pane one unit short of the switch (%.1f) is on the book page", sw-1)
	}
	if got := p.insetX(); got != float32(readingPageSideMin) {
		t.Errorf("the phone page's ink starts %v from the edge, want %v", got, readingPageSideMin)
	}
	p.relayout(sw)
	if !p.page.Book() {
		t.Errorf("a pane at the switch (%.1f) is on the phone page", sw)
	}
	if got := p.insetX(); got < float32(readingPageSideMin) {
		t.Errorf("the book page's ink starts %v from the edge, under the minimum", got)
	}
}

// findStyledPane walks a reading area for its styled pane.
func findStyledPane(o fyne.CanvasObject) *styledReadingPane {
	if p, ok := o.(*styledReadingPane); ok {
		return p
	}
	var kids []fyne.CanvasObject
	switch c := o.(type) {
	case *fyne.Container:
		kids = c.Objects
	case fyne.Widget:
		if r := test.WidgetRenderer(c); r != nil {
			kids = r.Objects()
		}
	}
	for _, k := range kids {
		if p := findStyledPane(k); p != nil {
			return p
		}
	}
	return nil
}
