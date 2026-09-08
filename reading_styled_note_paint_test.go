package bibletext

// Two assertions that this surface believed it already had, and did not.
//
// Mutation testing found the pane's note coverage almost entirely vacuous: the
// package stayed green with every per-paragraph pill wired to the first group's
// note, and with the selection rectangles painted BENEATH the opaque wash. The
// cause was uniform — the existing checks read p.pillGeoms and p.noteGeom, the
// geometry table relayout() computes BEFORE the renderer runs, so they pass for
// a renderer that builds nothing, wires nothing, or stacks its layers backwards.
//
// These two read the renderer instead: the live buttons it wired, and the order
// it appended its objects in. Both carry a mutation note saying what was broken
// when they were written, so a later reader can re-break it and watch them fail.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// PRESSING THE PILL, not calling what the pill is supposed to call. Every test
// that claimed this pressed focusNoteAtGroup directly with a key re-derived from
// the model, so the one link that can actually be miswired — this pill's button
// carries THIS group's key — was never exercised.
//
// Mutation that used to pass: in buildNote's pill loop, capture the first
// group's key for every button instead of its own. Reader symptom: every pill on
// the page opens the same note, and the notes on every other paragraph are
// unreachable by the only control drawn for them.
func TestEachDrawnPillOpensItsOwnNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	defer setNotesEnabled(true)
	withPillsOn(t)

	st, verses, _, _, _ := twoNotedParagraphs(t, 3)
	pane := newStyledReadingPane(st, verses)
	rend, ok := pane.CreateRenderer().(*styledPaneRenderer)
	if !ok {
		t.Fatalf("unexpected renderer type")
	}
	rend.Layout(fyne.NewSize(320, 900))

	if len(rend.pillBtns) < 2 {
		t.Fatalf("the renderer wired %d pill buttons; this test needs at least two "+
			"to tell 'each opens its own' from 'they all open one'", len(rend.pillBtns))
	}

	opened := map[uint64]int{}
	for i, btn := range rend.pillBtns {
		if btn == nil || btn.OnTapped == nil {
			t.Errorf("pill %d has no live button — it is drawn and does nothing", i)
			continue
		}
		st.resetNoteFocus()
		btn.OnTapped()
		if st.NoteID == 0 {
			t.Errorf("pressing pill %d opened nothing", i)
			continue
		}
		if prev, seen := opened[st.NoteID]; seen {
			t.Errorf("pressing pill %d opened the same note as pill %d — the pill drawn "+
				"for one paragraph reaches another paragraph's note, so at least one "+
				"note has no control that reaches it", i, prev)
			continue
		}
		opened[st.NoteID] = i
	}
	if len(opened) != len(rend.pillBtns) {
		t.Errorf("%d pills opened %d distinct notes", len(rend.pillBtns), len(opened))
	}
}

// THE SELECTION MUST BE PAINTED OVER THE WASH. This is the defect class that
// needed a BTWashView on iOS and a LineBackgroundSpan on Android: where the wash
// is opaque and the selection translucent, drawing them the wrong way round
// leaves a reader dragging across a noted verse with no visible selection at all
// — the text simply looks unresponsive.
//
// On this pane the ordering is nothing but the order rebuild() appends to
// r.objects, so that is what this reads. Fyne paints objects in slice order, so
// LATER is ABOVE.
//
// Mutation that used to pass: build the selection rectangles before the wash
// rectangles in rebuild().
func TestTheSelectionIsPaintedOverTheWashNotUnderIt(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	defer setNotesEnabled(true)
	withPillsOn(t)

	st, verses, firstVerse, _, _ := twoNotedParagraphs(t, 3)
	// The wash is the verse MARK, not the note — a note reserves a band, a mark
	// paints the colour the selection has to stay visible against.
	st.setHL(hlSearch, st.CurrentBook, st.CurrentChapter, firstVerse, 0)
	pane := newStyledReadingPane(st, verses)
	rend, ok := pane.CreateRenderer().(*styledPaneRenderer)
	if !ok {
		t.Fatalf("unexpected renderer type")
	}
	rend.Layout(fyne.NewSize(320, 900))

	// A selection over the whole chapter, so it is guaranteed to cross whatever
	// the wash covers. Set on the pane rather than dragged: this test is about
	// the paint order, and the drag path has its own tests.
	if pane.lay == nil || len(pane.lay.Lines) == 0 {
		t.Fatal("the pane laid out no lines")
	}
	end := pane.lay.Lines[len(pane.lay.Lines)-1].EndOffset
	pane.selAnchor, pane.selStart, pane.selEnd = 0, 0, end
	rend.Refresh()
	rend.Layout(fyne.NewSize(320, 900))

	if len(rend.tintRects) == 0 {
		t.Fatal("the fixture painted no wash, so this test cannot see the ordering")
	}
	if len(rend.selRects) == 0 {
		t.Fatal("the fixture painted no selection, so this test cannot see the ordering")
	}

	index := func(want fyne.CanvasObject) int {
		for i, o := range rend.Objects() {
			if o == want {
				return i
			}
		}
		return -1
	}
	lastWash := -1
	for _, r := range rend.tintRects {
		if !r.Visible() {
			continue
		}
		if i := index(r); i > lastWash {
			lastWash = i
		}
	}
	firstSel := len(rend.Objects())
	for _, r := range rend.selRects {
		if i := index(r); i >= 0 && i < firstSel {
			firstSel = i
		}
	}
	if lastWash < 0 {
		t.Fatal("no visible wash rectangle reached the renderer's object list")
	}
	if firstSel <= lastWash {
		t.Errorf("the selection is painted at object %d, beneath the wash at %d — "+
			"a reader dragging across a noted verse would see no selection there, "+
			"because the wash is opaque and the selection is not", firstSel, lastWash)
	}

	// The control: prove the wash really is the opaque layer, so the ordering
	// above is load-bearing rather than a preference. A translucent wash would
	// let the selection show through either way and this test would be theatre.
	var opaque bool
	for _, r := range rend.tintRects {
		if !r.Visible() {
			continue
		}
		if _, _, _, a := r.FillColor.RGBA(); a == 0xffff {
			opaque = true
		}
	}
	if !opaque {
		t.Error("no wash rectangle is fully opaque any more — if the wash has become " +
			"translucent this test no longer guards anything and should be re-thought")
	}
}
