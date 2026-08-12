package bibletext

// Regression net for the popup-audit fixes: every dialog here once could put
// content — in two cases its ONLY dismissal — outside the reachable screen.
// Each test lays the real popup out on a small phone canvas and asserts the box
// it occupies lands inside the glass, reusing sheetBox/findScroll from
// sheet_fit_test.go.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// smallPhone is an iPhone SE 1st-gen canvas — the tightest screen supported.
func smallPhone(t *testing.T) (*AppState, fyne.Window) {
	t.Helper()
	app := test.NewApp()
	t.Cleanup(app.Quit)
	th := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
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
// hundreds of points tall that pushed OK off every phone (audit, upheld 3/3).
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

// The arriving-note card is modal and "Read the passage" is its only closer. A
// 280-rune note with blank lines pushed it off a small phone, stranding the
// reader with the reading overlay latched hidden.
func TestSharedNoteCardKeepsItsButtonOnScreen(t *testing.T) {
	st, win := smallPhone(t)
	long := strings.Repeat("A line of somebody's message.\n", 12) +
		strings.Repeat("x", 40)
	showSharedNote(st, long)
	p := topPopup(t, win)
	test.WidgetRenderer(p).Layout(p.Size())

	assertBoxOnScreen(t, p, win.Canvas(), "shared-note card")
	// Fitting by truncation is not fitting: the overflow must be scrollable.
	if findScroll(p.Content) == nil {
		t.Fatal("no scroll in the note card — a long note would be unreachable")
	}
	btn := findTreeButton(p.Content, "Read the passage")
	if btn == nil {
		t.Fatal("the only dismissal is missing")
	}
	// The button's ABSOLUTE position, not just its existence: the original bug
	// laid it out below the frame the modal renderer clamps to — present in the
	// tree, painted off the glass. The Border structure is what pins it inside.
	bp := fyne.CurrentApp().Driver().AbsolutePositionForObject(btn)
	if bot := bp.Y + btn.Size().Height; bot > win.Canvas().Size().Height {
		t.Errorf("the only dismissal ends at y=%v on a %vpt canvas", bot, win.Canvas().Size().Height)
	}
}

// Verse of the day on a short canvas: the button row must stay inside the card.
func TestVerseOfDayFitsShortCanvas(t *testing.T) {
	st, win := smallPhone(t)
	win.Resize(fyne.NewSize(320, 400)) // shorter than any phone: the split-screen shape
	showVerseOfDay(st)
	p := topPopup(t, win)
	test.WidgetRenderer(p).Layout(p.Size())

	assertBoxOnScreen(t, p, win.Canvas(), "verse-of-day card")
	if findScroll(p.Content) == nil {
		t.Fatal("no scroll in the verse-of-day card")
	}
	rb := findTreeButton(p.Content, "Read in context")
	if findTreeButton(p.Content, "Close") == nil || rb == nil {
		t.Fatal("button row missing from the laid-out card")
	}
	bp := fyne.CurrentApp().Driver().AbsolutePositionForObject(rb)
	if bot := bp.Y + rb.Size().Height; bot > win.Canvas().Size().Height {
		t.Errorf("button row ends at y=%v on a %vpt canvas", bot, win.Canvas().Size().Height)
	}
}

// The compose sheet's counter line is the only content wider than a small
// phone; as an unwrappable canvas.Text it set the whole card's minimum width
// and pushed the Share button past the right edge below ~357pt.
func TestComposeSheetFitsNarrowPhone(t *testing.T) {
	st, win := smallPhone(t)
	promptShareNote(st, "For God so loved the world")
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
