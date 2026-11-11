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
	if len(groups) != 3 || scroll == nil || reference == nil {
		t.Fatalf("%d group rows, scroll %v, the first card's reference %v", len(groups), scroll != nil, reference != nil)
	}
	g := groups[0]

	// The label: 14 points, between the caption's 11 and the reference's 16,
	// wrapping, with no inset of its own inside the list's scope.
	if got := g.label.Theme().Size(sizeNameCrossRefGroup); got != 14 || got <= g.label.Theme().Size(theme.SizeNameCaptionText) || got >= reference.TextSize {
		t.Errorf("the group's label is %v points; want 14, above the caption's %v and below the reference's %v", got, g.label.Theme().Size(theme.SizeNameCaptionText), reference.TextSize)
	}
	if g.label.Wrapping != fyne.TextWrapWord || g.label.Theme().Size(theme.SizeNameInnerPadding) != 0 {
		t.Errorf("the group's label does not wrap, or keeps an inset of its own")
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
	if len(notes) != 1 || scroll == nil || reference == nil {
		t.Fatalf("%d note rows, scroll %v, the first card's reference %v", len(notes), scroll != nil, reference != nil)
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

	// The note's row: a full tap, its printed line at the list's edge and
	// the references under it set in beside the rule (crossref_section.go).
	if h := n.header.Size().Height; h < 44 {
		t.Errorf("the note's tap is %v points tall, want at least 44", h)
	}
	if got := n.label.Theme().Size(sizeNameCrossRefGroup); got != 14 || n.label.Wrapping != fyne.TextWrapWord {
		t.Errorf("the note's printed line is %v points, wrapping %v; want 14 and wrapping", got, n.label.Wrapping)
	}
	if lx, rx := drv.AbsolutePositionForObject(n.label).X, drv.AbsolutePositionForObject(reference).X; rx-lx != crossRefUnderInset {
		t.Errorf("the note's printed line starts at %v and the references under it at %v; want them %v further in", lx, rx, crossRefUnderInset)
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

// WHAT IS UNDER A LETTER IS SEEN TO BE UNDER IT: the note's own row stands
// at the list's edge; every row under it is set in beside a rule that runs
// their whole height; one rule the width of the list closes the note; and
// the next note's row stands back at the edge. A group that is its whole
// note opens into a ground set in the same way.
func TestWhatIsUnderALetterIsSetInBesideItsRule(t *testing.T) {
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
	var groups []*crossRefGroupRow
	walkTree(top, func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *crossRefNoteRow:
			notes = append(notes, x)
		case *crossRefGroupRow:
			groups = append(groups, x)
		}
	})
	if len(notes) != 1 || len(groups) != 3 {
		t.Fatalf("%d note rows and %d group rows; want the first note's row, its group, and the two notes that are each a group", len(notes), len(groups))
	}
	n, inner, lone := notes[0], groups[0], groups[1]
	drv := fyne.CurrentApp().Driver()
	x := func(o fyne.CanvasObject) float32 { return drv.AbsolutePositionForObject(o).X }
	y := func(o fyne.CanvasObject) float32 { return drv.AbsolutePositionForObject(o).Y }
	foot := func(o fyne.CanvasObject) float32 { return y(o) + o.Size().Height }
	edge := x(n)

	// The note's own row at the edge, and every row under its letter set in.
	if x(n.header) != edge {
		t.Errorf("the note's own row starts %v in from the list's edge", x(n.header)-edge)
	}
	if len(n.pieces) != 4 {
		t.Fatalf("%d rows under the letter, want three cards and the group", len(n.pieces))
	}
	for i, piece := range n.pieces {
		if got := x(piece) - edge; got != crossRefUnderInset {
			t.Errorf("row %d under the letter stands %v in from the list's edge, want %v", i, got, crossRefUnderInset)
		}
	}
	// The rule: the letter's colour, beside the rows and clear of them, from
	// the top of the first to the foot of the last.
	first, last := n.pieces[0], n.pieces[len(n.pieces)-1]
	rule, ok := n.body.Objects[0].(*canvas.Rectangle)
	if !ok {
		t.Fatalf("the rows under the letter stand beside a %T, want the rule", n.body.Objects[0])
	}
	if x(rule)-edge != crossRefUnderRuleAt || rule.Size().Width != crossRefUnderRuleWidth || x(rule)+rule.Size().Width > x(first) {
		t.Errorf("the rule is %v wide, %v in from the edge, beside rows %v in", rule.Size().Width, x(rule)-edge, x(first)-edge)
	}
	if y(rule) != y(first) || foot(rule) != foot(last) || rule.Size().Height <= 0 {
		t.Errorf("the rule runs from %v to %v; the rows under the letter from %v to %v", y(rule), foot(rule), y(first), foot(last))
	}
	if rule.FillColor != crossRefUnderRuleColour(th.palette()) {
		t.Errorf("the rule is %v, want the letter's colour made quiet, %v", rule.FillColor, crossRefUnderRuleColour(th.palette()))
	}
	// One rule the width of the list closes the note, at the edge, under
	// the last row, which has none of its own.
	if !n.foot.Visible() || x(n.foot) != edge || n.foot.Size().Width != n.Size().Width || y(n.foot) < foot(last) {
		t.Errorf("the note's closing rule: visible %v, %v in from the edge, %v wide in a note %v wide, at %v under rows that end at %v", n.foot.Visible(), x(n.foot)-edge, n.foot.Size().Width, n.Size().Width, y(n.foot), foot(last))
	}
	if _, own := inner.box.Objects[len(inner.box.Objects)-1].(*widget.Separator); own || last != fyne.CanvasObject(inner) {
		t.Errorf("the last row under the letter is not the group, or it closes with a rule of its own (%v)", own)
	}
	// The next letter stands back at the edge, under that rule.
	if x(lone) != edge || y(lone) < y(n.foot) {
		t.Errorf("the next note's row stands %v in from the edge, at %v; the rule above it at %v", x(lone)-edge, y(lone), y(n.foot))
	}

	// A group under a letter keeps to the rows' inset: its ground stands
	// where the rows do, with no rule of its own beside it.
	test.Tap(inner.header)
	if _, again := inner.inset.Objects[0].(*canvas.Rectangle); again || !inner.inset.Visible() || x(inner.shade)-edge != crossRefUnderInset {
		t.Errorf("the group under the letter: open %v, a rule of its own %v, its ground %v in from the edge", inner.inset.Visible(), again, x(inner.shade)-edge)
	}
	if foot(rule) != foot(inner) {
		t.Errorf("with its group open, the letter's rule ends at %v and the rows under it at %v", foot(rule), foot(inner))
	}
	// A group that is its whole note: closed, nothing under its letter;
	// open, its ground set in beside a rule of its own.
	if lone.inset.Visible() {
		t.Fatalf("the group that is its whole note is open before a tap")
	}
	test.Tap(lone.header)
	own, ok := lone.inset.Objects[0].(*canvas.Rectangle)
	if !ok || !lone.inset.Visible() {
		t.Fatalf("open, the group that is its whole note stands beside a %T (visible %v), want the rule", lone.inset.Objects[0], lone.inset.Visible())
	}
	if x(lone.shade)-edge != crossRefUnderInset || x(own)-edge != crossRefUnderRuleAt || own.Size().Height != lone.inset.Size().Height || own.Size().Height <= 0 {
		t.Errorf("the group's ground stands %v in from the edge beside a rule %v in and %v tall, on a ground %v tall", x(lone.shade)-edge, x(own)-edge, own.Size().Height, lone.inset.Size().Height)
	}

	// Closed, the note is its own row and the rule that closes it.
	test.Tap(n.header)
	if n.open || n.body.Visible() || !n.foot.Visible() || x(n.foot) != edge {
		t.Errorf("closed: open %v, the rows under the letter visible %v, the closing rule visible %v", n.open, n.body.Visible(), n.foot.Visible())
	}
}
