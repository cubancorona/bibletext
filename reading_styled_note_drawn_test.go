package bibletext

// WHAT WAS MEASURED IS WHAT WAS DRAWN.
//
// This pane measures first and draws second: relayout() fills a geometry table
// (p.noteGeom, p.pillGeoms) saying where the card, its text and each pill belong,
// and the renderer then builds canvas objects and places them from that table.
//
// Nearly every assertion this surface had read the TABLE. That confirms the
// arithmetic and says nothing about the drawing, so the suite stayed green with
// the card image never appended, the pill chips never appended, the pill frames
// and their live hit targets moved 400pt away from their labels, and the note's
// message drawn 400pt below the card on top of the scripture. Every one of those
// leaves the table perfect and the page wrong.
//
// The tests here close that gap by comparing the two: for each thing the table
// measured, the object the renderer built must exist, be visible, and sit at the
// measured rect. Each carries the mutation it was written against.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
)

func inDrawList(r *styledPaneRenderer, want fyne.CanvasObject) bool {
	for _, o := range r.Objects() {
		if o == want {
			return true
		}
	}
	return false
}

func nearlyPos(a fyne.Position, r styledNoteRect, tol float32) bool {
	dx, dy := a.X-r.X, a.Y-r.Y
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	return dx <= tol && dy <= tol
}

// Mutations that used to pass: drop the card image from the renderer's object
// list; offset every body line's drawn position by +400pt while leaving the
// geometry untouched.
func TestTheOpenNoteIsDrawnWhereItWasMeasured(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	defer setNotesEnabled(true)

	st, verses, _, _, _ := twoNotedParagraphs(t, 3)
	// Pills deliberately OFF: the single sticker is the open state, and the
	// collapsed one has its own test below.
	st.resetNoteFocus()
	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	groups := chapterNoteGroups(st, plan, verses)
	if len(groups) == 0 {
		t.Fatal("the fixture produced no note groups")
	}
	focusNoteAtGroup(st, groups[0].Key)

	pane := newStyledReadingPane(st, verses)
	rend, ok := pane.CreateRenderer().(*styledPaneRenderer)
	if !ok {
		t.Fatalf("unexpected renderer type")
	}
	rend.Layout(fyne.NewSize(420, 900))

	g := pane.noteGeom
	if !g.present || g.pill || g.card.W <= 0 || g.card.H <= 0 {
		t.Fatalf("the fixture drew no open card (present=%v pill=%v card=%+v) — this "+
			"test cannot see the defect it exists for", g.present, g.pill, g.card)
	}

	if rend.noteCard == nil {
		t.Fatal("the note was measured but no card image reached the renderer — the " +
			"reader sees the reserved band and nothing in it")
	}
	if !rend.noteCard.Visible() {
		t.Error("the card image was built and hidden")
	}
	// BUILT IS NOT DRAWN. The renderer keeps a reference to the card whether or
	// not it appends it, so a check on the field alone passes for a pane that
	// never paints the note — which is exactly the mutation this test failed to
	// catch when it was first written.
	if !inDrawList(rend, rend.noteCard) {
		t.Error("the card image was built and placed but never appended to the " +
			"renderer's objects, so it is never painted: the reader sees the reserved " +
			"band and nothing in it")
	}
	if !nearlyPos(rend.noteCard.Position(), g.card, 0.5) {
		t.Errorf("the card is drawn at %v but was measured for (%.1f,%.1f)",
			rend.noteCard.Position(), g.card.X, g.card.Y)
	}
	if sz := rend.noteCard.Size(); sz.Width < g.card.W-0.5 || sz.Height < g.card.H-0.5 {
		t.Errorf("the card is drawn %v but was measured %.1fx%.1f", sz, g.card.W, g.card.H)
	}

	// The message. Its rects are the ones a mutation moved onto the scripture.
	if len(g.bodyLines) == 0 {
		t.Fatal("the note measured no body lines, so this fixture cannot see the defect")
	}
	drawn := 0
	for _, txt := range rend.noteTexts {
		for _, want := range g.bodyLines {
			if nearlyPos(txt.Position(), want, 1.0) {
				drawn++
				break
			}
		}
	}
	if drawn < len(g.bodyLines) {
		var at []fyne.Position
		for _, txt := range rend.noteTexts {
			at = append(at, txt.Position())
		}
		t.Errorf("%d of %d body lines are drawn where they were measured; the message "+
			"is not in the card.\nmeasured: %+v\ndrawn: %v", drawn, len(g.bodyLines),
			g.bodyLines, at)
	}
}

// Mutations that used to pass: never append the pill chips; move every pill
// frame and its live button 400pt down while leaving the label at its measured
// position, so an empty chip sits on the verses and a naked word floats above it.
func TestEachPillIsDrawnWhereItWasMeasured(t *testing.T) {
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

	if len(pane.pillGeoms) < 2 {
		t.Fatalf("the fixture measured %d pills; this test needs at least two", len(pane.pillGeoms))
	}
	if len(rend.pillFrames) != len(pane.pillGeoms) {
		t.Fatalf("%d pills were measured and %d chips drawn — a measured pill with no "+
			"chip is a note the reader cannot see or reach",
			len(pane.pillGeoms), len(rend.pillFrames))
	}

	for i, want := range pane.pillGeoms {
		frame := rend.pillFrames[i]
		if !frame.Visible() {
			t.Errorf("pill %d's chip is hidden", i)
		}
		if !inDrawList(rend, frame) {
			t.Errorf("pill %d's chip was built but never appended to the renderer's "+
				"objects, so it is never painted", i)
		}
		if !nearlyPos(frame.Position(), want.card, 0.5) {
			t.Errorf("pill %d's chip is drawn at %v but was measured for (%.1f,%.1f)",
				i, frame.Position(), want.card.X, want.card.Y)
		}
		// The label must sit INSIDE the chip it belongs to, not merely at some
		// position of its own: the two were moved apart and nothing noticed.
		lbl := rend.pillLabels[i]
		lc := fyne.NewPos(lbl.Position().X+lbl.MinSize().Width/2, lbl.Position().Y+lbl.MinSize().Height/2)
		if !want.card.contains(lc) {
			t.Errorf("pill %d's label centres at %v, outside its own chip %+v — the "+
				"reader sees an empty chip and a loose word", i, lc, want.card)
		}
		// And the live hit target must cover the chip, or the pill is decoration.
		btn := rend.pillBtns[i]
		if !nearlyPos(btn.Position(), want.card, 1.0) {
			t.Errorf("pill %d's hit target is at %v, not on its chip at (%.1f,%.1f) — "+
				"pressing the pill does nothing and pressing empty text opens a note",
				i, btn.Position(), want.card.X, want.card.Y)
		}
	}
}

// Mutation that used to pass: disable the hitsAnyPill guard, so a press meant for
// a pill also plants a text selection under it.
func TestAPressOnAPillDoesNotAlsoStartASelection(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	defer setNotesEnabled(true)
	withPillsOn(t)

	st, verses, _, _, _ := twoNotedParagraphs(t, 3)
	pane := newStyledReadingPane(st, verses)
	// Resize, not a bare renderer Layout: the press path reads the pane's own
	// size, and a pane that was never sized selects nothing anywhere.
	pane.Resize(fyne.NewSize(320, 900))
	if len(pane.pillGeoms) == 0 {
		t.Fatal("no pill was measured")
	}

	pane.selAnchor, pane.selStart, pane.selEnd = -1, -1, -1
	card := pane.pillGeoms[0].card
	at := fyne.NewPos(card.X+card.W/2, card.Y+card.H/2)
	pane.MouseDown(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: at}, Button: desktop.MouseButtonPrimary})

	if pane.selStart != -1 || pane.selAnchor != -1 {
		t.Errorf("a press on the pill at %v started a selection (anchor=%d start=%d) — "+
			"the reader gets a caret and a highlight they did not ask for on top of "+
			"opening the note", at, pane.selAnchor, pane.selStart)
	}

	// The control: the same press one line below the pill MUST start one, or this
	// test would pass on a pane whose selection is broken everywhere.
	pane.selAnchor, pane.selStart, pane.selEnd = -1, -1, -1
	if pane.lay == nil || len(pane.lay.Lines) == 0 {
		t.Fatal("the pane laid out no lines")
	}
	last := pane.lay.Lines[len(pane.lay.Lines)-1]
	below := fyne.NewPos(card.X+card.W/2, last.Y+last.H/2)
	pane.MouseDown(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: below}, Button: desktop.MouseButtonPrimary})
	if pane.selAnchor == -1 {
		t.Errorf("a press in the running text at %v started no selection either, so the "+
			"check above proves nothing — either the fixture or the selection path is "+
			"broken", below)
	}
}
