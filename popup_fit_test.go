package bibletext

// Regression net for popup sizing: every dialog here once could put
// content — in two cases its ONLY dismissal — outside the reachable screen.
// Each test lays the real popup out on a small phone canvas and asserts the box
// it occupies lands inside the glass, reusing sheetBox/findScroll from
// sheet_fit_test.go.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// smallPhone is an iPhone SE 1st-gen canvas — the tightest screen supported.
func smallPhone(t *testing.T) (*AppState, fyne.Window) {
	t.Helper()
	app := test.NewApp()
	t.Cleanup(app.Quit)
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	win := app.NewWindow("popup")
	win.Resize(fyne.NewSize(320, 568))
	st := sampleState()
	st.window = win
	st.theme = th
	return st, win
}

func topPopup(t *testing.T, win fyne.Window) *widget.PopUp {
	t.Helper()
	p, ok := win.Canvas().Overlays().Top().(*widget.PopUp)
	if !ok || p == nil {
		t.Fatalf("expected a popup overlay, got %T", win.Canvas().Overlays().Top())
	}
	// Hide on the way out: several of these popups arm 40ms re-measure timers
	// gated on Visible(), and a popup left visible past its test lets that
	// timer's font measurement race the next test's (go-text's glyph cache is
	// single-thread-only; the real app measures on one UI thread).
	t.Cleanup(p.Hide)
	return p
}

// assertBoxOnScreen fails if the popup's painted box leaves the canvas.
func assertBoxOnScreen(t *testing.T, p *widget.PopUp, cnv fyne.Canvas, label string) (top, bottom float32) {
	t.Helper()
	top, bottom = sheetBox(t, p)
	if top < 0 {
		t.Errorf("%s starts above the screen at y=%v", label, top)
	}
	if h := cnv.Size().Height; bottom > h {
		t.Errorf("%s runs off the bottom: box %v..%v on a %vpt canvas (%vpt unreachable)",
			label, top, bottom, h, bottom-h)
	}
	return top, bottom
}

// The version-load-error dialog is modal and OK is its only way out. Un-Resized
// it floored at the OK button's width, re-wrapping the message into a ribbon
// hundreds of points tall that pushed OK off every phone.
func TestVersionLoadErrorDialogIsDismissable(t *testing.T) {
	st, win := smallPhone(t)
	showVersionLoadError(st, "World English Bible (Catholic)")
	p := topPopup(t, win)
	test.WidgetRenderer(p).Layout(p.Size())

	assertBoxOnScreen(t, p, win.Canvas(), "load-error dialog")
	// The ribbon symptom: a card as narrow as a button. Real width proves the
	// explicit Resize took.
	if wd := p.Content.Size().Width; wd < 200 {
		t.Errorf("dialog is %vpt wide — the one-word-per-line ribbon is back", wd)
	}
	// And OK must be inside the laid-out card.
	ok := findTreeButton(p.Content, "OK")
	if ok == nil {
		t.Fatal("no OK button in the dialog")
	}
}

// The download spinner's title is the version's display name; as a canvas.Text
// it could not wrap, so long names clipped at both screen edges.
func TestVersionLoadingCardFitsNarrowPhone(t *testing.T) {
	st, win := smallPhone(t)
	dismiss := showVersionLoading(st, "World English Bible (Catholic)")
	defer dismiss()
	p := topPopup(t, win)
	test.WidgetRenderer(p).Layout(p.Size())

	if wd := p.Content.Size().Width; wd > win.Canvas().Size().Width {
		t.Errorf("loading card is %vpt wide on a %vpt canvas", wd, win.Canvas().Size().Width)
	}
	assertBoxOnScreen(t, p, win.Canvas(), "loading card")
}

// Verse of the day on a short canvas: the button row must stay inside the card
// and the passage must scroll rather than push it out.
//
// THE FIXTURE OVERFLOWS ON PURPOSE. The suite's sample verses never did — the
// tallest rotation passage they carry laid out 12pt short of the cap on this
// canvas — so the scroll and the height cap were never exercised, and a test
// that read only the modal's frame could not fail anyway: the modal renderer
// clamps its frame to the canvas whatever the content does. One long verse,
// the only entry the fixture can show, is what makes the cap and the scroll
// load-bearing here.
//
// Mutations this guards: dropping the VScroll (the buttons leave the canvas);
// dropping the height cap (same); wrapping the passage at a width other than
// the one it is drawn at (a row is wider than its column and clips).
func TestVerseOfDayFitsShortCanvas(t *testing.T) {
	st, win := smallPhone(t)
	win.Resize(fyne.NewSize(320, 400)) // shorter than any phone: the split-screen shape
	long := strings.Repeat("Jesus said to him, I am the way, the truth, and the life. ", 40)
	st.Bible = &BibleData{
		Books: []string{"John"},
		Verses: map[string]map[int][]Verse{"John": {14: {
			{BookName: "John", Chapter: 14, Verse: 6, Text: long},
		}}},
	}
	votdSynchronousRemeasure(t)
	showVerseOfDay(st)
	p := topPopup(t, win)
	test.WidgetRenderer(p).Layout(p.Size())

	scroll := findScroll(p.Content)
	if scroll == nil {
		t.Fatal("no scroll in the verse-of-day card")
	}
	body := findReadingParagraph(p.Content)
	if body == nil {
		t.Fatal("the card sets no reading paragraph")
	}
	if body.MinSize().Height <= scroll.Size().Height {
		t.Fatalf("the fixture does not overflow (passage %vpt in a %vpt scroll): this test "+
			"cannot see the defect it exists for", body.MinSize().Height, scroll.Size().Height)
	}
	top, bottom := assertBoxOnScreen(t, p, win.Canvas(), "verse-of-day card")
	pos, sz := win.Canvas().InteractiveArea()
	if maxH := sheetMaxHeight(win.Canvas().Size().Height, pos.Y, sz.Height, pos.Y+16); bottom-top > maxH+0.5 {
		t.Errorf("card is %vpt tall against a %vpt cap", bottom-top, maxH)
	}
	rb := findTreeButton(p.Content, "Read in context")
	if findTreeButton(p.Content, "Close") == nil || rb == nil {
		t.Fatal("button row missing from the laid-out card")
	}
	bp := fyne.CurrentApp().Driver().AbsolutePositionForObject(rb)
	if bot := bp.Y + rb.Size().Height; bot > win.Canvas().Size().Height {
		t.Errorf("button row ends at y=%v on a %vpt canvas", bot, win.Canvas().Size().Height)
	}
	// Every drawn row fits the column it was given: the passage was wrapped at
	// the width it is really drawn at, not at a guess.
	r := test.WidgetRenderer(body).(*readingParagraphRenderer)
	for i, wd := range r.rowWidths() {
		if wd > body.Size().Width+0.5 {
			t.Errorf("row %d is %vpt wide in a %vpt column", i, wd, body.Size().Width)
		}
	}
}

// votdSynchronousRemeasure runs the card's deferred second fit inline for the
// rest of the test, so the layout the assertions read is the settled one and no
// timer outlives the test (see votdRemeasure).
func votdSynchronousRemeasure(t *testing.T) {
	t.Helper()
	prev := votdRemeasure
	votdRemeasure = func(fit func()) { fit() }
	t.Cleanup(func() { votdRemeasure = prev })
}

func findReadingParagraph(o fyne.CanvasObject) *readingParagraph {
	switch v := o.(type) {
	case *readingParagraph:
		return v
	case *fyne.Container:
		for _, c := range v.Objects {
			if p := findReadingParagraph(c); p != nil {
				return p
			}
		}
	case *container.Scroll:
		return findReadingParagraph(v.Content)
	case fyne.Widget:
		for _, c := range test.WidgetRenderer(v).Objects() {
			if p := findReadingParagraph(c); p != nil {
				return p
			}
		}
	}
	return nil
}

// The compose sheet's counter line is the only content wider than a small
// phone; as an unwrappable canvas.Text it set the whole card's minimum width
// and pushed the Share button past the right edge below ~357pt.
func TestComposeSheetFitsNarrowPhone(t *testing.T) {
	st, win := smallPhone(t)
	promptShareNote(st, "For God so loved the world", selSpan{})
	p := topPopup(t, win)
	test.WidgetRenderer(p).Layout(p.Size())

	cw := win.Canvas().Size().Width
	if wd := p.Content.Size().Width; wd > cw {
		t.Errorf("compose card is %vpt wide on a %vpt canvas — Share is off-screen", wd, cw)
	}
	if findTreeButton(p.Content, "Share") == nil {
		t.Fatal("Share button missing")
	}
}
