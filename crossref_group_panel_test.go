//go:build !race

package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// IN THE PANEL: a group's label is larger than a caption and smaller than a
// card's reference, its left edge is the cards', its tap is a full row, and
// opening or closing it moves nothing but the list inside its scroll — the
// panel keeps its size and its place.
func TestAGroupOpensInsideThePanelWithoutMovingIt(t *testing.T) {
	th := appThemeForTest(t)
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	withCrossRefIndex(t, "John.3.16\tRom.5.8\t10\n")
	previousRun, previousLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = previousRun, previousLoad })
	crossRefsRun = func(work func()) { work() }
	crossRefsLoad = func() error { return nil }
	st, _ := groupListState(t)
	w := test.NewWindow(widget.NewLabel("Reading"))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(375, 667))
	st.theme, st.window = th, w
	showCrossRefs(st, "", selSpan{lo: 16, hi: 16})
	top, ok := w.Canvas().Overlays().Top().(*widget.PopUp)
	if !ok {
		t.Fatal("the cross-references panel did not open")
	}
	var groups []*crossRefGroupRow
	var scroll *container.Scroll
	var reference *canvas.Text
	walkTree(top, func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *crossRefGroupRow:
			groups = append(groups, x)
		case *container.Scroll:
			if scroll == nil {
				scroll = x
			}
		case *canvas.Text:
			if x.Text == "Romans 5:8" {
				reference = x
			}
		}
	})
	if len(groups) != 1 || scroll == nil || reference == nil {
		t.Fatalf("%d group rows, scroll %v, the first card's reference %v; want the one group inside the first note", len(groups), scroll != nil, reference != nil)
	}
	g := groups[0]

	// The label: 14 points, between the caption's 11 and the reference's 16,
	// muted, in rows no wider than the label, with no inset of its own.
	rows := test.WidgetRenderer(g.label).(*crossRefPrintedRenderer).rows
	if len(rows) == 0 {
		t.Fatalf("the group's label draws no row")
	}
	for _, row := range rows {
		if got := row.TextSize; got != 14 || got <= g.label.Theme().Size(theme.SizeNameCaptionText) || got >= reference.TextSize || row.Color != th.palette().TextMuted {
			t.Errorf("the group's label is %v points in %v; want 14, muted, above the caption's %v and below the reference's %v", got, row.Color, g.label.Theme().Size(theme.SizeNameCaptionText), reference.TextSize)
		}
		if row.Position().X != 0 || row.MinSize().Width > g.label.Size().Width {
			t.Errorf("a row of the group's label starts %v in and wants %v in a label %v wide", row.Position().X, row.MinSize().Width, g.label.Size().Width)
		}
	}
	drv := fyne.CurrentApp().Driver()
	if lx, rx := drv.AbsolutePositionForObject(g.label).X, drv.AbsolutePositionForObject(reference).X; lx != rx {
		t.Errorf("the group's label starts at %v and the cards' references at %v", lx, rx)
	}
	if h := g.header.Size().Height; h < 44 {
		t.Errorf("the group's tap is %v points tall, want at least 44", h)
	}
	if g.Size().Width > scroll.Size().Width {
		t.Errorf("the group row is %v wide in a list %v wide", g.Size().Width, scroll.Size().Width)
	}

	// Opening and closing: the panel does not move or change size; the list
	// grows inside the scroll and shrinks back.
	size, pos, content := top.Content.Size(), top.Content.Position(), scroll.Content.Size().Height
	test.Tap(g.header)
	if !g.open || len(g.members.Objects) != 3 {
		t.Fatalf("the tap did not open the group: open %v, %d cards", g.open, len(g.members.Objects))
	}
	if top.Content.Size() != size || top.Content.Position() != pos {
		t.Errorf("opening a group moved the panel: %v at %v, was %v at %v", top.Content.Size(), top.Content.Position(), size, pos)
	}
	opened := scroll.Content.Size().Height
	if opened <= content {
		t.Errorf("open, the list is %v tall inside the scroll, no taller than closed (%v): it was not laid out again", opened, content)
	}
	if g.inset.Size().Width > g.Size().Width || g.inset.Size().Height <= 0 {
		t.Errorf("the open group's ground is %v in a row %v", g.inset.Size(), g.Size())
	}
	// The cards on the ground are full taps too, and their previews wrap
	// inside it.
	walkTree(g.inset, func(o fyne.CanvasObject) {
		if card, ok := o.(*tapCard); ok && card.Size().Height < 44 {
			t.Errorf("a card of the open group is %v points tall", card.Size().Height)
		}
		if label, ok := o.(*widget.Label); ok && label.Size().Width > g.inset.Size().Width {
			t.Errorf("a preview of the open group is %v wide on a ground %v wide", label.Size().Width, g.inset.Size().Width)
		}
	})
	test.Tap(g.header)
	if g.open || scroll.Content.Size().Height != content || top.Content.Size() != size || top.Content.Position() != pos {
		t.Errorf("closing did not restore the list: open %v, list %v (was %v), panel %v at %v", g.open, scroll.Content.Size().Height, content, top.Content.Size(), top.Content.Position())
	}
}

// IN THE PANEL: the selected text heads the list, no wider than the list,
// every row of it inside its width, a letter raised and smaller than the
// words; a note's own row is a full tap whose printed line starts where the
// cards' references start; and closing or opening a note moves nothing but
// the list inside its scroll.
func TestANoteClosesInsideThePanelWithoutMovingIt(t *testing.T) {
	th := appThemeForTest(t)
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	withCrossRefIndex(t, "John.3.16\tRom.5.8\t10\n")
	previousRun, previousLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = previousRun, previousLoad })
	crossRefsRun = func(work func()) { work() }
	crossRefsLoad = func() error { return nil }
	st, _ := groupListState(t)
	w := test.NewWindow(widget.NewLabel("Reading"))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(375, 667))
	st.theme, st.window = th, w
	showCrossRefs(st, "", selSpan{lo: 16, hi: 16})
	top, ok := w.Canvas().Overlays().Top().(*widget.PopUp)
	if !ok {
		t.Fatal("the cross-references panel did not open")
	}
	var notes []*crossRefNoteRow
	var scroll *container.Scroll
	var reference *canvas.Text
	var passage *readingParagraph
	walkTree(top, func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *readingParagraph:
			if passage == nil {
				passage = x
			}
		case *crossRefNoteRow:
			notes = append(notes, x)
		case *container.Scroll:
			if scroll == nil {
				scroll = x
			}
		case *canvas.Text:
			if x.Text == "Romans 5:8" {
				reference = x
			}
		}
	})
	if len(notes) != 2 || scroll == nil || reference == nil {
		t.Fatalf("%d note rows, scroll %v, the first card's reference %v; want the first note's row and the row of the note that is one group", len(notes), scroll != nil, reference != nil)
	}
	n := notes[0]

	// The passage: the list's first row, inside the list's width, its rows
	// inside its own; three letters, each smaller than the words and at the
	// top of its row.
	if passage == nil {
		t.Fatal("no passage in the list")
	}
	drv := fyne.CurrentApp().Driver()
	if py, sy := drv.AbsolutePositionForObject(passage).Y, drv.AbsolutePositionForObject(scroll).Y; py < sy || py > sy+8 {
		t.Errorf("the passage starts %v points below the list's top: it does not head the list", py-sy)
	}
	if passage.Size().Width > scroll.Size().Width || passage.Size().Width <= 0 {
		t.Errorf("the passage is %v wide in a list %v wide", passage.Size().Width, scroll.Size().Width)
	}
	r := test.WidgetRenderer(passage).(*readingParagraphRenderer)
	for i, rw := range r.rowWidths() {
		if rw > passage.Size().Width+0.5 {
			t.Errorf("row %d of the passage is %v wide in %v", i, rw, passage.Size().Width)
		}
	}
	// Raised as the page raises a superscript: a letter is readingMarkScale
	// of the words' size, its baseline readingMarkLift of that size above
	// theirs, and it is given the height of its own glyphs. Given its row's
	// whole height the toolkit centres it there, a small letter among the
	// words. (The fixture's third note is keyed past the text's end, so its
	// letter closes the row: the words of its row are the ones before it.)
	_, bodyBase := drv.RenderedTextSize("Ag", passage.size, fyne.TextStyle{}, passage.regular)
	letters := 0
	for ri, row := range r.rows {
		var word *canvas.Text
		for _, tx := range row {
			if tx.Color != th.palette().Accent && tx.TextSize == passage.size {
				word = tx
				break
			}
		}
		for _, tx := range row {
			if tx.Color != th.palette().Accent {
				continue
			}
			letters++
			if want := passage.size * readingMarkScale; tx.TextSize != want {
				t.Errorf("the letter %q is %v points beside words of %v, want %v", tx.Text, tx.TextSize, passage.size, want)
			}
			if word == nil {
				t.Errorf("the letter %q stands on row %d with no word", tx.Text, ri)
				continue
			}
			own, base := drv.RenderedTextSize(tx.Text, tx.TextSize, fyne.TextStyle{}, tx.FontSource)
			lift := (word.Position().Y + bodyBase) - (tx.Position().Y + base)
			if want := passage.size * readingMarkLift; lift < want-0.01 || lift > want+0.01 || tx.Size().Height > own.Height {
				t.Errorf("the letter %q stands %v above the line of its row's words in a box %v tall (its glyphs are %v): want %v above, in a box no taller than its glyphs", tx.Text, lift, tx.Size().Height, own.Height, want)
			}
		}
	}
	if letters != 3 {
		t.Errorf("%d letters in the passage, want one for each of the verse's three notes", letters)
	}

	// The note's row: a full tap, its printed line beside the letter's
	// margin, where the references under it start (crossref_section.go).
	if h := n.header.Size().Height; h < 44 {
		t.Errorf("the note's tap is %v points tall, want at least 44", h)
	}
	for _, row := range test.WidgetRenderer(n.label).(*crossRefPrintedRenderer).rows {
		if row.TextSize != 14 || row.Color != th.palette().TextMuted || row.MinSize().Width > n.label.Size().Width {
			t.Errorf("a row of the note's printed line is %v points in %v and wants %v in a line %v wide; want 14, muted, inside it", row.TextSize, row.Color, row.MinSize().Width, n.label.Size().Width)
		}
	}
	if lx, rx := drv.AbsolutePositionForObject(n.label).X, drv.AbsolutePositionForObject(reference).X; lx != rx {
		t.Errorf("the note's printed line starts at %v and the references under it at %v", lx, rx)
	}
	if mx, lx := drv.AbsolutePositionForObject(n.letter).X, drv.AbsolutePositionForObject(n.label).X; lx-mx != crossRefLetterMargin || n.letter.Text != "a" {
		t.Errorf("the letter %q stands at %v and the printed line at %v; want the line %v beside it", n.letter.Text, mx, lx, crossRefLetterMargin)
	}
	if n.Size().Width > scroll.Size().Width {
		t.Errorf("the note row is %v wide in a list %v wide", n.Size().Width, scroll.Size().Width)
	}

	// Closing and opening: the panel does not move or change size; the list
	// shrinks inside the scroll and grows back.
	size, pos, content := top.Content.Size(), top.Content.Position(), scroll.Content.Size().Height
	test.Tap(n.header)
	if n.open || n.body.Visible() {
		t.Fatalf("the tap did not close the note")
	}
	if top.Content.Size() != size || top.Content.Position() != pos {
		t.Errorf("closing a note moved the panel: %v at %v, was %v at %v", top.Content.Size(), top.Content.Position(), size, pos)
	}
	if closed := scroll.Content.Size().Height; closed >= content {
		t.Errorf("closed, the list is %v tall inside the scroll, no shorter than open (%v): it was not laid out again", closed, content)
	}
	test.Tap(n.header)
	if !n.open || scroll.Content.Size().Height != content || top.Content.Size() != size || top.Content.Position() != pos {
		t.Errorf("opening did not restore the list: open %v, list %v (was %v), panel %v at %v", n.open, scroll.Content.Size().Height, content, top.Content.Size(), top.Content.Position())
	}
}

// EACH NOTE STANDS ON A GROUND OF ITS OWN, ITS LETTER ALONE IN A MARGIN, so
// that a reader sees which rows are a letter's and where the next begins.
// Every note opens the same way, a note that is one parenthesised group
// included; a group inside a note starts closed and opens onto the panel's
// own ground, a well in the note's.
func TestEachNoteStandsOnAGroundOfItsOwn(t *testing.T) {
	th := appThemeForTest(t)
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	withCrossRefIndex(t, "John.3.16\tRom.5.8\t10\n")
	previousRun, previousLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = previousRun, previousLoad })
	crossRefsRun = func(work func()) { work() }
	crossRefsLoad = func() error { return nil }
	st, _ := groupListState(t)
	w := test.NewWindow(widget.NewLabel("Reading"))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(375, 667))
	st.theme, st.window = th, w
	showCrossRefs(st, "", selSpan{lo: 16, hi: 16})
	top, ok := w.Canvas().Overlays().Top().(*widget.PopUp)
	if !ok {
		t.Fatal("the cross-references panel did not open")
	}
	var sections []*fyne.Container
	var notes []*crossRefNoteRow
	var groups []*crossRefGroupRow
	var scroll *container.Scroll
	var place *fyne.Container // the list's place in the panel: the scroll reaches past it by the lane
	walkTree(top, func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *container.Scroll:
			if scroll == nil {
				scroll = x
			}
		case *fyne.Container:
			if _, section := x.Layout.(crossRefSectionLayout); section {
				sections = append(sections, x)
			}
			if _, lane := x.Layout.(crossRefScrollLaneLayout); lane {
				place = x
			}
		case *crossRefNoteRow:
			notes = append(notes, x)
		case *crossRefGroupRow:
			groups = append(groups, x)
		}
	})
	if len(sections) != 3 || len(notes) != 2 || len(groups) != 1 {
		t.Fatalf("%d grounds, %d note rows, %d group rows; want a ground for each of the three notes, a row for the first two, and the first note's group", len(sections), len(notes), len(groups))
	}
	a, b, inner := notes[0], notes[1], groups[0]
	pal := th.palette()
	drv := fyne.CurrentApp().Driver()
	x := func(o fyne.CanvasObject) float32 { return drv.AbsolutePositionForObject(o).X }
	y := func(o fyne.CanvasObject) float32 { return drv.AbsolutePositionForObject(o).Y }
	right := func(o fyne.CanvasObject) float32 { return x(o) + o.Size().Width }
	foot := func(o fyne.CanvasObject) float32 { return y(o) + o.Size().Height }
	ground := func(i int) *canvas.Rectangle { return sections[i].Objects[0].(*canvas.Rectangle) }
	on := func(o fyne.CanvasObject, g *canvas.Rectangle) bool {
		return x(o) >= x(g) && right(o) <= right(g) && y(o) >= y(g) && foot(o) <= foot(g)
	}

	// The grounds: the note's colour, which is not the panel's; rounded; one
	// under another, the same width, with the gap between them.
	if crossRefSectionGround(pal) == pal.SurfaceAlt || crossRefSectionGround(pal) == crossRefGroupGround(pal) {
		t.Fatalf("a note's ground is %v, the panel's own colour: it sets nothing apart", crossRefSectionGround(pal))
	}
	for i := range sections {
		g := ground(i)
		if g.FillColor != crossRefSectionGround(pal) || g.CornerRadius != crossRefSectionRadius || g.Size().Height <= 0 {
			t.Errorf("ground %d is %v, radius %v, %v tall", i, g.FillColor, g.CornerRadius, g.Size().Height)
		}
		if x(g) != x(ground(0)) || g.Size().Width != ground(0).Size().Width {
			t.Errorf("ground %d stands at %v, %v wide; the first at %v, %v wide", i, x(g), g.Size().Width, x(ground(0)), ground(0).Size().Width)
		}
		if i > 0 {
			if gap := y(g) - foot(ground(i-1)); gap != crossRefSectionGap {
				t.Errorf("ground %d starts %v under the ground before it, want %v", i, gap, crossRefSectionGap)
			}
		}
	}

	// The scroll's indicator runs in a lane beside the list: the scroll
	// reaches that far past the grounds' right edge, and no further than
	// the panel's own padding beside the list.
	if scroll == nil || place == nil {
		t.Fatalf("the list's scroll %v, its place in the panel %v", scroll != nil, place != nil)
	}
	if lane := right(scroll) - right(ground(0)); lane != crossRefScrollLane || crossRefScrollLane <= 0 {
		t.Errorf("the scroll ends %v past the grounds' right edge; want a lane of %v", lane, crossRefScrollLane)
	}
	// The lane is not taken from the list: a note's ground is as wide as
	// the list's place in the panel, and the scroll reaches past that place
	// into the panel's padding, staying inside the panel.
	if ground(0).Size().Width != place.Size().Width || right(scroll)-right(place) != crossRefScrollLane {
		t.Errorf("a note's ground is %v wide in a place %v wide, and the scroll ends %v past it; want the place's width and %v", ground(0).Size().Width, place.Size().Width, right(scroll)-right(place), crossRefScrollLane)
	}
	if right(scroll) > x(top.Content)+top.Content.Size().Width {
		t.Errorf("the scroll ends at %v, past the panel's edge at %v", right(scroll), x(top.Content)+top.Content.Size().Width)
	}
	// A row's disclosure mark is drawn in a box of its own size.
	for _, icon := range []*widget.Icon{a.icon, b.icon, inner.icon} {
		if icon.Size() != fyne.NewSize(crossRefFoldIconSize, crossRefFoldIconSize) {
			t.Errorf("a disclosure mark is %v, want %v square", icon.Size(), crossRefFoldIconSize)
		}
	}

	// Every row of a note stands on that note's ground, and on no other.
	rows := func(n *crossRefNoteRow) []fyne.CanvasObject {
		return append([]fyne.CanvasObject{n.header}, n.pieces...)
	}
	if len(a.pieces) != 4 || len(b.pieces) != 1 {
		t.Fatalf("%d rows under the first letter and %d under the second; want three cards and the group, and one card", len(a.pieces), len(b.pieces))
	}
	for i, row := range rows(a) {
		if !on(row, ground(0)) || on(row, ground(1)) {
			t.Errorf("row %d of the first note does not stand on its note's ground and no other", i)
		}
	}
	for i, row := range rows(b) {
		if !on(row, ground(1)) || on(row, ground(0)) {
			t.Errorf("row %d of the second note does not stand on its note's ground and no other", i)
		}
	}

	// The letter alone in the margin, on the first line of the printed
	// note; the printed note and every row under it beside the margin.
	for _, n := range []*crossRefNoteRow{a, b} {
		margin := x(n.letter)
		g := ground(0)
		if n == b {
			g = ground(1)
		}
		if margin-x(g) != crossRefSectionPadX {
			t.Errorf("the letter %q stands %v in from its ground's edge, want %v", n.letter.Text, margin-x(g), crossRefSectionPadX)
		}
		if x(n.label)-margin != crossRefLetterMargin {
			t.Errorf("the printed note stands %v beside the letter %q, want %v", x(n.label)-margin, n.letter.Text, crossRefLetterMargin)
		}
		for i, piece := range n.pieces {
			if x(piece)-margin != crossRefLetterMargin {
				t.Errorf("row %d under the letter %q stands %v beside it, want %v", i, n.letter.Text, x(piece)-margin, crossRefLetterMargin)
			}
		}
		_, own := drv.RenderedTextSize(n.letter.Text, n.letter.TextSize, n.letter.TextStyle, nil)
		_, line := drv.RenderedTextSize("Ag", crossRefGroupTextSize, fyne.TextStyle{}, nil)
		if d := (y(n.letter) + own) - (y(n.label) + line); d < -0.5 || d > 0.5 {
			t.Errorf("the letter %q stands %v points off the line of the printed note", n.letter.Text, d)
		}
		// From under the letter a rule runs down the margin beside the
		// note's rows: the letter's colour, under the letter and clear of
		// the rows, from a little under the line the letter stands on to a
		// little above the foot of the last row.
		stem := n.stem
		first, last := n.pieces[0], n.pieces[len(n.pieces)-1]
		if !stem.Visible() || stem.FillColor != crossRefStemColour(pal) || stem.Size().Width != crossRefStemWidth || x(stem)-margin != crossRefStemAt {
			t.Errorf("the rule under %q: visible %v, %v, %v wide, %v into the margin", n.letter.Text, stem.Visible(), stem.FillColor, stem.Size().Width, x(stem)-margin)
		}
		if x(stem) < x(n.letter) || right(stem) > right(n.letter) || right(stem) > x(first) {
			t.Errorf("the rule under %q spans %v to %v; the letter %v to %v, the rows from %v", n.letter.Text, x(stem), right(stem), x(n.letter), right(n.letter), x(first))
		}
		if d := y(stem) - (y(n.letter) + own + crossRefStemGap); d < -0.01 || d > 0.01 || y(stem) >= y(first) {
			t.Errorf("the rule under %q starts at %v; the letter stands on the line at %v and the rows start at %v", n.letter.Text, y(stem), y(n.letter)+own, y(first))
		}
		if d := foot(stem) - (foot(last) - crossRefStemFoot); d < -0.01 || d > 0.01 {
			t.Errorf("the rule under %q ends at %v; the rows at %v", n.letter.Text, foot(stem), foot(last))
		}
	}
	if a.letter.Text != "a" || b.letter.Text != "b" || a.letter.Color != pal.Accent {
		t.Errorf("the letters are %q and %q in %v", a.letter.Text, b.letter.Text, a.letter.Color)
	}

	// Every note opens the same way: its own citations shown. The second
	// note is one parenthesised group; its row carries the group as
	// printed and its citation stands under it.
	if !a.open || !a.body.Visible() || !b.open || !b.body.Visible() {
		t.Errorf("as the panel opens the first note is open %v and the second %v", a.open, b.open)
	}
	if got := joined(textsIn(b.label)); got != "(Is. 9:6)" {
		t.Errorf("the second note's row reads %q, want the group as printed", got)
	}
	if got := joined(cardLabelsIn(b.pieces)); got != "Isaiah 9:6" {
		t.Errorf("under the second letter stand %q, want the group's citation", got)
	}
	// The third is a group nothing of which can be followed: as printed,
	// under its letter, with nothing to tap.
	if got := joined(textsIn(sections[2])); got != "c|(Hab. 2:4)" || len(cardsIn([]fyne.CanvasObject{sections[2]})) != 0 {
		t.Errorf("the third note reads %q with %d taps; want its letter and the group as printed, and none", got, len(cardsIn([]fyne.CanvasObject{sections[2]})))
	}

	// A group inside a note starts closed. Open, its cards stand on the
	// panel's own ground, inside the note's, and the next note's ground
	// keeps its distance.
	if inner.open || inner.inset.Visible() {
		t.Fatalf("the group inside the first note is open before a tap")
	}
	test.Tap(inner.header)
	if !inner.inset.Visible() || inner.shade.FillColor != crossRefGroupGround(pal) || inner.shade.FillColor == ground(0).FillColor || !on(inner.shade, ground(0)) {
		t.Errorf("the opened group: visible %v, its ground %v on the note's %v, inside it %v", inner.inset.Visible(), inner.shade.FillColor, ground(0).FillColor, on(inner.shade, ground(0)))
	}
	if gap := y(ground(1)) - foot(ground(0)); gap != crossRefSectionGap {
		t.Errorf("with its group open, the first note's ground ends %v above the second's, want %v", gap, crossRefSectionGap)
	}
	if d := foot(a.stem) - (foot(inner) - crossRefStemFoot); d < -0.01 || d > 0.01 {
		t.Errorf("with its group open, the rule under the first letter ends at %v and its rows at %v", foot(a.stem), foot(inner))
	}

	// Closed, a note is its own row on its ground, and the next ground
	// follows it.
	test.Tap(a.header)
	if a.open || a.body.Visible() || a.stem.Visible() || !on(a.header, ground(0)) || foot(ground(0))-foot(a.header) != crossRefSectionPadY {
		t.Errorf("closed: open %v, rows visible %v, the rule visible %v, the ground ends %v under the row", a.open, a.body.Visible(), a.stem.Visible(), foot(ground(0))-foot(a.header))
	}
	if gap := y(ground(1)) - foot(ground(0)); gap != crossRefSectionGap {
		t.Errorf("with the first note closed, its ground ends %v above the second's, want %v", gap, crossRefSectionGap)
	}
}
