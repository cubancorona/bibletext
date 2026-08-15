package bibletext

// The verse highlight must cover the highlighted verses' WORDS — and nothing
// else.
//
// The bug this pins: the styled pane drew the highlight as ONE full-column
// rectangle spanning a LINE RANGE. Lines routinely carry more than one verse
// (measured on John 3 at 560pt: 26 of 63 lines carry two or more), so the band
// lit every neighbouring verse that happened to share the highlight's first or
// last line. With a single highlight nobody noticed; the moment two adjacent
// verses carry DIFFERENT tints, the spill is exactly the difference between
// them.
//
// The assertion is deliberately written against PAINTED RECTANGLES rather than
// any particular field, so it holds whatever the renderer's internals become:
// collect every visible rectangle wearing the highlight wash, and require that
// none of them overlaps the box an unhighlighted neighbour's runs occupy.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
)

// sharedLineState is three short prose verses — short enough to sit on ONE
// laid-out line at any sane pane width, which is the shape that shows the
// defect.
func sharedLineState() *AppState {
	bd := &BibleData{
		Books: []string{"Ruth"},
		Verses: map[string]map[int][]Verse{"Ruth": {1: {
			{BookName: "Ruth", Book: "Ruth", Chapter: 1, Verse: 1, Text: "Alpha one."},
			{BookName: "Ruth", Book: "Ruth", Chapter: 1, Verse: 2, Text: "Beta two."},
			{BookName: "Ruth", Book: "Ruth", Chapter: 1, Verse: 3, Text: "Gamma three."},
		}}},
	}
	return &AppState{Bible: bd, CurrentBook: "Ruth", CurrentChapter: 1}
}

// paintedBox is a drawn rectangle in widget coordinates.
type paintedBox struct{ x0, y0, x1, y1 float32 }

func (b paintedBox) overlaps(o paintedBox) bool {
	return b.x0 < o.x1 && o.x0 < b.x1 && b.y0 < o.y1 && o.y0 < b.y1
}

// styledHighlightBoxes returns every VISIBLE rectangle the renderer paints in
// the highlight wash — one band or many, the test does not care which.
func styledHighlightBoxes(p *styledReadingPane, r fyne.WidgetRenderer) []paintedBox {
	wantR, wantG, wantB, wantA := p.pal.Highlight.RGBA()
	var out []paintedBox
	for _, o := range r.Objects() {
		rect, ok := o.(*canvas.Rectangle)
		if !ok || !rect.Visible() {
			continue
		}
		gr, gg, gb, ga := rect.FillColor.RGBA()
		if gr != wantR || gg != wantG || gb != wantB || ga != wantA {
			continue
		}
		pos, sz := rect.Position(), rect.Size()
		out = append(out, paintedBox{pos.X, pos.Y, pos.X + sz.Width, pos.Y + sz.Height})
	}
	return out
}

// verseBoxOnLine is the box the given verse's runs occupy on one line, in the
// same widget coordinates the renderer draws in, shrunk by a hair so a rect
// that merely ABUTS the neighbour does not read as covering it.
func verseBoxOnLine(p *styledReadingPane, li, verse int) (paintedBox, bool) {
	ln := p.lay.Lines[li]
	first := true
	var box paintedBox
	for _, run := range ln.Runs {
		if run.Verse != verse {
			continue
		}
		x0, x1 := p.insetX()+run.X, p.insetX()+run.X+run.W
		if first {
			box = paintedBox{x0, ln.Y, x1, ln.Y + ln.H}
			first = false
			continue
		}
		if x0 < box.x0 {
			box.x0 = x0
		}
		if x1 > box.x1 {
			box.x1 = x1
		}
	}
	if first {
		return paintedBox{}, false
	}
	box.x0 += 0.5
	box.x1 -= 0.5
	box.y0 += 0.5
	box.y1 -= 0.5
	return box, true
}

// TestStyledPaneHighlightDoesNotSpillOntoLineNeighbours: a highlight on verse 2
// must not wash verses 1 and 3, which share its line.
func TestStyledPaneHighlightDoesNotSpillOntoLineNeighbours(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := sharedLineState()
	st.setHL(hlSearch, "Ruth", 1, 2, 0)
	p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
	p.Resize(fyne.NewSize(560, 400))
	r := p.CreateRenderer()

	// The fixture only means anything if the three verses really do share one
	// line — otherwise the test proves nothing.
	if len(p.lay.Lines) != 1 {
		t.Fatalf("fixture must lay out as ONE line, got %d", len(p.lay.Lines))
	}
	seen := map[int]bool{}
	for _, run := range p.lay.Lines[0].Runs {
		seen[run.Verse] = true
	}
	if !seen[1] || !seen[2] || !seen[3] {
		t.Fatalf("fixture must put verses 1-3 on the same line, got %v", seen)
	}

	boxes := styledHighlightBoxes(p, r)
	if len(boxes) == 0 {
		t.Fatal("no highlight painted for the highlighted verse")
	}
	for _, neighbour := range []int{1, 3} {
		nb, ok := verseBoxOnLine(p, 0, neighbour)
		if !ok {
			t.Fatalf("verse %d has no runs on line 0", neighbour)
		}
		for i, b := range boxes {
			if b.overlaps(nb) {
				t.Errorf("highlight rect %d (%.1f..%.1f) covers unhighlighted verse %d (%.1f..%.1f)",
					i, b.x0, b.x1, neighbour, nb.x0, nb.x1)
			}
		}
	}

	// And it really does cover the verse it is FOR — a renderer that paints
	// nothing would pass the spill check trivially.
	hb, ok := verseBoxOnLine(p, 0, 2)
	if !ok {
		t.Fatal("verse 2 has no runs on line 0")
	}
	covered := false
	for _, b := range boxes {
		if b.overlaps(hb) {
			covered = true
		}
	}
	if !covered {
		t.Error("the highlighted verse's own words are not washed")
	}
}

// A relayout that shrinks the span list without a rebuild must not leave the
// surplus wash rectangles painted where they were.
//
// The single full-width band this replaced was immune by construction: one
// rectangle, with an explicit Hide on the else arm. Per-line rects removed that
// safety net, and position() merely dropped out of its loop when it ran past
// the spans — so clearing a mark and relaying out left every rect on screen at
// its old geometry. It is not reachable through today's one production caller,
// which Refreshes (and so rebuilds) immediately after every relayout. That is a
// property of that call site, not of this code, and the next caller inherits
// nothing from it.
func TestStyledPaneHidesSurplusWashRects(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := sharedLineState()
	st.setHL(hlSearch, "Ruth", 1, 2, 2)
	p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
	r := p.CreateRenderer().(*styledPaneRenderer)
	p.relayout(560)
	r.Layout(fyne.NewSize(560, p.MinSize().Height))
	if len(r.tintRects) == 0 {
		t.Fatal("precondition: expected at least one wash rect while a verse is marked")
	}
	before := len(r.tintRects)

	// Clear the mark and relayout WITHOUT rebuilding, exactly as a future caller
	// that forgets the Refresh would.
	st.clearMark()
	p.relayout(560)
	r.Layout(fyne.NewSize(560, p.MinSize().Height))

	if got := len(p.tintSpans); got != 0 {
		t.Fatalf("precondition: %d spans after clearing the mark, want 0", got)
	}
	for i, rect := range r.tintRects {
		if rect.Visible() {
			t.Errorf("wash rect %d of %d is still painted after the mark was cleared", i, before)
		}
	}
}
