package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The list's scrollbar is an OVERLAY, not a reserved column: it is drawn on top
// of the rows at the scroll's right edge, 3pt wide at rest and 16pt while the
// pointer is over it. A row whose trailing control stops 3pt short of that edge
// is therefore flush at rest and 13pt UNDER the bar on hover — which is most of
// a 19pt icon button, so the reader reaches for the bin and hits the scrollbar.
//
// Every row control must clear the widest the bar can get. Measured against the
// enclosing scrollable's right edge rather than the window's, because the
// browser is width-limited and the two are far apart.
func TestNoteRowControlsClearTheScrollbar(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	seedNoteStore(t, 6)
	st := psalm23State()
	st.NotesMode = true

	view := buildNotesBrowseView(st)
	w := test.NewWindow(view)
	defer w.Close()
	w.Resize(fyne.NewSize(900, 700))

	want := fyneTheme.Size(fyneTheme.SizeNameScrollBar)

	var worst float32 = 1e9
	seen := 0
	var walk func(o fyne.CanvasObject, offX, scrollRight float32)
	walk = func(o fyne.CanvasObject, offX, scrollRight float32) {
		if o == nil || !o.Visible() {
			return
		}
		x := offX + o.Position().X
		switch o.(type) {
		case *widget.List, *container.Scroll:
			scrollRight = x + o.Size().Width
		}
		if b, ok := o.(*widget.Button); ok && b.Text == "" && b.Icon != nil {
			seen++
			if gap := scrollRight - (x + b.Size().Width); gap < worst {
				worst = gap
			}
		}
		switch v := o.(type) {
		case *container.Scroll:
			walk(v.Content, x, scrollRight)
		case *fyne.Container:
			for _, ch := range v.Objects {
				walk(ch, x, scrollRight)
			}
		case fyne.Widget:
			if r := test.WidgetRenderer(v); r != nil {
				for _, ch := range r.Objects() {
					walk(ch, x, scrollRight)
				}
			}
		}
	}
	walk(view, 0, 1e9)

	if seen == 0 {
		t.Fatalf("no row delete controls found — the test is measuring nothing")
	}
	if worst < want {
		t.Errorf("the row's delete control clears the scroll edge by %.1fpt; the "+
			"hovered scrollbar is %.0fpt wide, so it covers %.1fpt of the control",
			worst, want, want-worst)
	}
}
