package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// AN ARRIVAL SURVIVES THE DOUBLE REBUILD.
//
// Tapping a note link on another tab runs TWO rebuilds in one event handler:
// HandleShareLink itself surfaces the Read tab (that wire consumes
// forceReposition and arms the placement), and the tapping row's own
// switch-to-read rebuilds again. No layout runs between them — Fyne lays out on
// the next frame — so the second wire used to capture the first pane's
// never-laid-out offset 0, read it as "the reader is at the top", claim the
// placement for it (carryTop) and CEDE the highlight. The note the reader
// tapped then never scrolled into view.
//
// Found live on the styled mimic: the dev Links tab's "second note on the same
// passage" row, with the reader scrolled to the bottom of John 11. The pin
// replays the two rebuilds with no layout between, then lets layout run once,
// and requires the view to land on the note.
func TestArrivalSurvivesTheDoubleRebuild(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())
	defer setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	setNotesEnabled(true)

	origPane := useStyledPane
	useStyledPane = func() bool { return true }
	defer func() { useStyledPane = origPane }()

	st := planTestState(t)
	long := make([]Verse, 0, 57)
	for i := 1; i <= 57; i++ {
		txt := "A verse long enough that the paragraph splitter works with real material, running on toward its threshold."
		if i%5 == 0 {
			txt += " And here the sentence closes so a paragraph may end."
		}
		long = append(long, Verse{BookName: "John", Book: "John", Chapter: 3, Verse: i, Text: txt})
	}
	st.Bible.Verses["John"][3] = long
	win := app.NewWindow("double")
	win.Resize(fyne.NewSize(420, 700))
	st.app = app
	st.window = win
	win.SetContent(CreateMainUI(app, st, win))

	// The reader has BEEN on the reading tab (so a same-chapter pane exists to
	// carry from), is at the top, then goes to the Links tab.
	st.CurrentTab = 0
	rebuildWindow(st)
	st.CurrentTab = 3
	rebuildWindow(st)

	// The tap. HandleShareLink surfaces the Read tab itself (rebuild #1);
	// the row's switchToRead runs rebuild #2 — with NO layout between, which is
	// exactly how one event handler runs them live.
	url := ShareLinkURLWithNote("web", "John", 3, 35, 35, "Second note on the same passage.")
	if !HandleShareLink(st, url) {
		t.Fatal("declined")
	}
	// LIVE, NO LAYOUT HAS RUN YET between the two rebuilds — Fyne lays out on
	// the next frame, and both rebuilds happen inside one event handler. The
	// test canvas cannot reproduce that (it lays out synchronously on
	// SetContent), so the intermediate state is imposed exactly as the live
	// trace showed it: rebuild #1's pane wired, at offset zero, its Layout
	// never having settled a viewport.
	if styledScroll == nil {
		t.Fatal("rebuild #1 wired no pane")
	}
	styledScroll.Offset.Y = 0
	styledViewportSettled = false
	// Live, nothing has PLACED yet either — and the explicit flag is consumed
	// at placement now, not on wiring — so between the two rebuilds it is
	// still armed. The test canvas's synchronous layout placed-and-consumed
	// during rebuild #1 (the same artifact the offset imposition corrects).
	st.forceReposition = true
	st.CurrentTab = 0
	leaveSearchForRead(st, 0)
	rebuildWindow(st)

	// NOW layout runs — the next frame's worth.
	for i := 0; i < 3; i++ {
		win.Canvas().Content().Refresh()
		if styledScroll != nil {
			styledScroll.Resize(styledScroll.Size())
		}
	}

	if styledScroll == nil || styledPane == nil {
		t.Fatal("no styled pane wired")
	}
	noteY := styledPane.highlightY()
	if noteY <= 0 {
		t.Fatalf("the fixture note raised no band (%v); nothing to land on", noteY)
	}
	target := noteY - noteMetrics().Lead
	if diff := styledScroll.Offset.Y - target; diff > 40 || diff < -40 {
		t.Errorf("the view sits at %.1f; the tapped note's band is at %.1f. The "+
			"second rebuild carried the first pane's never-laid-out zero as 'the "+
			"reader is at the top' and ceded the highlight (ceded=%v settled=%v).",
			styledScroll.Offset.Y, noteY, styledHighlightCeded, styledViewportSettled)
	}
}
