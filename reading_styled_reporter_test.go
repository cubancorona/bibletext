package bibletext


// U.S. Reports typesetting). Width-gated in the styled pane: a pane wide enough
// for the 27.5em measure centres it at 1.3 leading with indented, gapless
// paragraphs; a narrow pane keeps the cozy 1.55/gap layout unchanged.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func reporterTestState() *AppState {
	bd := &BibleData{
		Books: []string{"Romans"},
		Verses: map[string]map[int][]Verse{"Romans": {8: {
			{BookName: "Romans", Chapter: 8, Verse: 1, Text: "Therefore there is now no condemnation for those who are in Christ Jesus."},
			{BookName: "Romans", Chapter: 8, Verse: 2, Text: "For in Christ Jesus the law of the Spirit of life set you free from the law of sin and death."},
		}}},
	}
	bd.PrepareSearchIndex()
	return &AppState{Bible: bd, CurrentBook: "Romans", CurrentChapter: 8}
}

// poetryTestState opens on an authored poem line, which must never be indented.
func poetryTestState() *AppState {
	bd := &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {23: {
			{BookName: "Psalms", Chapter: 23, Verse: 1, Text: "The LORD is my shepherd;\nI shall not want."},
		}}},
	}
	bd.PrepareSearchIndex()
	return &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 23}
}

// The engine's indent: first line of a prose paragraph starts Indent in and
// loses Indent of budget; later lines sit flush left; poetry is never indented.
func TestLayoutChapterFirstLineIndent(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := reporterTestState()
	p := styledLayoutParams{Width: 300, LineHeight: 20, ParaGap: 0, SpaceW: 7, Indent: 22}
	lay := layoutChapter(st, st.Bible.GetChapter("Romans", 8), p, fixedMeasure)
	if len(lay.Lines) < 2 {
		t.Fatalf("expected a wrapped paragraph, got %d lines", len(lay.Lines))
	}
	first, second := lay.Lines[0], lay.Lines[1]
	if !first.ParaFirst {
		t.Fatal("line 0 should open the paragraph")
	}
	if got := first.Runs[0].X; got != 22 {
		t.Errorf("paragraph's first run starts at %v, want the 22pt indent", got)
	}
	if got := second.Runs[0].X; got != 0 {
		t.Errorf("continuation line starts at %v, want flush left", got)
	}
	// The indented line's budget shrank: its content must fit Width like every
	// other line's.
	for i, ln := range lay.Lines {
		last := ln.Runs[len(ln.Runs)-1]
		if end := last.X + last.W; end > p.Width+0.5 {
			t.Errorf("line %d overruns the measure: ends at %v of %v", i, end, p.Width)
		}
	}

	// Poetry: no indent, ever.
	ps := poetryTestState()
	lay = layoutChapter(ps, ps.Bible.GetChapter("Psalms", 23), p, fixedMeasure)
	if got := lay.Lines[0].Runs[0].X; got != 0 {
		t.Errorf("a poem-opening paragraph was indented to %v", got)
	}
}

// The pane's width gate: wide panes centre the measure at reporter metrics,
// narrow panes keep the legacy layout byte-for-byte.
func TestStyledPaneReporterGate(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	th := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)

	st := reporterTestState()
	p := newStyledReadingPane(st, st.Bible.GetChapter("Romans", 8))
	m := reporterMeasureEm * p.textSize

	// WIDE: the reporter page.
	wide := m + 2*styledPaneInset + 200
	p.relayout(wide)
	if p.extraInset <= 0 {
		t.Fatalf("wide pane (%.0fpt) did not centre the measure (extraInset=%v)", wide, p.extraInset)
	}
	if got, want := p.lh, p.textSize*1.3; got != want {
		t.Errorf("reporter leading %v, want %v (1.3 × body)", got, want)
	}
	// Centred means: inset + column + inset ≈ pane width.
	col := wide - 2*styledPaneInset - 2*p.extraInset
	if col > m+1 {
		t.Errorf("column %vpt exceeds the %vpt measure", col, m)
	}
	// Gapless paragraphs: line Ys advance by exactly the leading.
	for i := 1; i < len(p.lay.Lines); i++ {
		if gap := p.lay.Lines[i].Y - p.lay.Lines[i-1].Y; gap != p.lh {
			t.Errorf("line %d gap %v, want %v (no paragraph gap in reporter)", i, gap, p.lh)
		}
	}
	// The indent reached the engine.
	if got := p.lay.Lines[0].Runs[0].X; got != 1.5*p.textSize {
		t.Errorf("first-line indent %v, want %v", got, 1.5*p.textSize)
	}

	// NARROW: the legacy pane, exactly as before.
	p.relayout(m) // pane width == measure → avail < measure → legacy
	if p.extraInset != 0 {
		t.Errorf("narrow pane grew an extraInset of %v", p.extraInset)
	}
	if got, want := p.lh, p.textSize*1.55; got != want {
		t.Errorf("narrow leading %v, want the cozy %v", got, want)
	}
	if got := p.lay.Lines[0].Runs[0].X; got != 0 {
		t.Errorf("narrow pane gained an indent of %v", got)
	}
}

// Selection X mapping must use the LIVE inset: in reporter mode a click on a
// glyph's x must resolve to that glyph, not one extraInset to its left.
func TestStyledPaneSelectionUsesLiveInset(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	th := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)

	st := reporterTestState()
	p := newStyledReadingPane(st, st.Bible.GetChapter("Romans", 8))
	p.relayout(reporterMeasureEm*p.textSize + 2*styledPaneInset + 300)
	if p.extraInset <= 0 {
		t.Fatal("precondition: reporter mode")
	}
	// Round-trip: the X reported for an offset, fed back as a click, must land
	// on the same offset.
	ln := p.lay.Lines[0]
	mid := (ln.StartOffset + ln.EndOffset) / 2
	x := p.xForOffset(0, mid)
	back := p.offsetAtPos(fyne.NewPos(x+0.5, ln.Y+1))
	if back != mid {
		t.Errorf("offset %d maps to x=%v which maps back to %d — draw and hit-test disagree by the inset", mid, x, back)
	}
}
