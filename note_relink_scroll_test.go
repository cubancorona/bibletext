package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// Re-tapping a link must scroll to its note again — reported once from the dev
// Links tab on the styled mimic, and not yet reproduced (all five variants here
// pass, and the same tap is fine on iOS). Pinned anyway: the five sequences are
// each a real reader's path, and whichever gate eventually refuses one of them
// will fail this test by name.
//
// The variants: a plain re-tap; a re-tap after going back to the Links tab; one
// after the reader scrolled away (the styledUserScrolled latch); one with the
// note minimized to the pill; and one with per-paragraph pills on. What they
// share is the tap body of the dev tab's "Open in app" row, verbatim.
func TestReTappingALinkScrollsToItsNoteAgain(t *testing.T) {
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
	st.Bible.Verses["John"][3] = enumerationChapter()
	win := app.NewWindow("probe")
	win.Resize(fyne.NewSize(420, 700))
	st.app = app
	st.window = win
	win.SetContent(CreateMainUI(app, st, win))

	url := ShareLinkURLWithNote("web", "John", 3, 16, 16, "A note deep enough to need a scroll.")

	tap := func(label string) float32 {
		if !HandleShareLink(st, url) {
			t.Fatalf("%s: link declined", label)
		}
		st.CurrentTab = 0
		leaveSearchForRead(st, 0)
		rebuildWindow(st)
		// Let the layout settle the same way the app's frame loop would.
		for i := 0; i < 3; i++ {
			win.Canvas().Content().Refresh()
			if styledScroll != nil {
				styledScroll.Resize(styledScroll.Size())
			}
		}
		if styledScroll == nil {
			t.Fatalf("%s: no styled scroll wired", label)
		}
		_ = func() {} // trace point: offset and latches, if this ever fails
		return styledScroll.Offset.Y
	}

	first := tap("first")

	// Back to the Links tab, exactly as the reader goes.
	st.CurrentTab = 3
	rebuildWindow(st)

	second := tap("second")

	// VARIANT C: the reader SCROLLS AWAY on the reading pane, goes to Links,
	// taps the same row.
	styledScroll.Offset.Y = 0
	styledScroll.Refresh()
	styledUserScrolled = true
	st.CurrentTab = 3
	rebuildWindow(st)
	third := tap("after scrolling away")

	// VARIANT D: the reader MINIMIZES the note first (the pill), then re-taps.
	fyneDo := func(f func()) { f() }
	fyneDo(func() { hideCurrentNote(st); st.refreshReadingOnly() })
	st.CurrentTab = 3
	rebuildWindow(st)
	fourth := tap("after minimizing")

	// VARIANT E: pills ON (the mimic's dev toggle), note minimized, re-tap.
	origPills := notesPillPerParagraph
	notesPillPerParagraph = true
	defer func() { notesPillPerParagraph = origPills }()
	fyneDo(func() { hideCurrentNote(st); st.refreshReadingOnly() })
	st.CurrentTab = 3
	rebuildWindow(st)
	fifth := tap("pills on, minimized")

	for name, off := range map[string]float32{
		"third (scrolled away)": third, "fourth (minimized)": fourth,
		"fifth (pills+minimized)": fifth,
	} {
		if off <= 0 {
			t.Errorf("%s: view left at %.1f — not scrolled to the note", name, off)
		}
	}

	if first <= 0 {
		t.Fatalf("the FIRST tap did not scroll (offset %.1f); the fixture is wrong "+
			"or the first arrival is broken too", first)
	}
	if second <= 0 {
		t.Errorf("the SECOND tap of the same link left the view at %.1f — the "+
			"reported defect", second)
	}
}
