package bibletext

// JUSTIFICATION on the Windows and Linux pane (readingJustifyProse,
// spreadLine). The layout spreads each line a prose paragraph breaks for width
// to the measure and moves nothing but X; the drawing puts a justified line's
// words down one object each at those X; and everything that reads positions
// — the washes, selection, hit-testing — follows because it reads the same X.

import (
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// justifyState is one chapter holding every case: a long prose paragraph; a
// prose paragraph that turns to poetry, with a poem line long enough to wrap;
// and a paragraph that opens on a poem line and turns to prose.
func justifyState() *AppState {
	bd := NewBibleData()
	bd.Verses = map[string]map[int][]Verse{"Romans": {8: {
		{BookName: "Romans", Chapter: 8, Verse: 1, ParaStart: true,
			Text: "Therefore, there is now no condemnation for those who are in Christ Jesus."},
		{BookName: "Romans", Chapter: 8, Verse: 2,
			Text: "For in Christ Jesus the law of the Spirit of life set you free from the law of sin and death."},
		{BookName: "Romans", Chapter: 8, Verse: 3,
			Text: "For what the law was powerless to do in that it was weakened by the flesh, God did."},
		{BookName: "Romans", Chapter: 8, Verse: 4, ParaStart: true,
			Text: "So that the righteous standard of the law might be fulfilled in us, who do not walk according to the flesh:"},
		{BookName: "Romans", Chapter: 8, Verse: 5,
			Text: "Those who live according to the flesh set their minds on the things of the flesh and of the world,\nbut those who live by the Spirit set theirs on the things of the Spirit and of life."},
		{BookName: "Romans", Chapter: 8, Verse: 6, ParaStart: true,
			Text: "The mind of the flesh is death, but the mind of the Spirit is life and peace, because the mind of the flesh is hostile to God.\nIt does not submit."},
		{BookName: "Romans", Chapter: 8, Verse: 7,
			Text: "And those who are in the flesh cannot please God, for the mind set on the flesh does not submit to the law of God, nor can it do so."},
	}}}
	bd.Books = []string{"Romans"}
	bd.PrepareSearchIndex()
	return &AppState{Bible: bd, CurrentBook: "Romans", CurrentChapter: 8, CurrentVersion: "web"}
}

const justifyIndent = 30

func justifyLayouts(t *testing.T, width float32) (ragged, justified *chapterLayout) {
	t.Helper()
	st := justifyState()
	verses := st.Bible.GetChapter("Romans", 8)
	p := testLayoutParams
	p.Width = width
	p.Indent = justifyIndent
	ragged = layoutChapter(st, verses, p, fixedMeasure)
	p.Justify = true
	justified = layoutChapter(justifyState(), verses, p, fixedMeasure)
	return ragged, justified
}

// Justifying moves X and nothing else: the text, the breaks, the offsets and
// every line's place are the ragged layout's. A justified line ends on the
// measure with its first run where it was — an indent stays an indent — and
// the slack shared evenly among its gaps, the one after a verse number
// included. A paragraph's last line, a line ending in a poem break, every row
// of a poetic verse, every line of a paragraph that opens on a poem line, and a
// line of one run are left exactly as they were.
func TestJustifyingMovesOnlyX(t *testing.T) {
	const width = 400
	ragged, just := justifyLayouts(t, width)
	if ragged.Text != just.Text || len(ragged.Lines) != len(just.Lines) {
		t.Fatalf("justifying changed the text or the line count: %d lines against %d", len(just.Lines), len(ragged.Lines))
	}
	poetic := map[int]bool{}      // verse -> poetic
	opensOnPoem := map[int]bool{} // verse -> its paragraph opens on a poem line
	para := 0
	for i, v := range justifyState().Bible.GetChapter("Romans", 8) {
		poetic[v.Verse] = verseIsPoetic(v.Text)
		if i == 0 || v.ParaStart {
			para = v.Verse
		}
		opensOnPoem[v.Verse] = para == 6
	}

	justified, flushBefore, indented := 0, 0, 0
	// Lines that WOULD be justified — two runs or more, room to spare — but
	// for the exemption, by kind: each must occur, or its check proves nothing.
	exempt := map[string]int{}
	for li, ln := range just.Lines {
		was := ragged.Lines[li]
		if ln.Y != was.Y || ln.H != was.H || ln.StartOffset != was.StartOffset || ln.EndOffset != was.EndOffset || len(ln.Runs) != len(was.Runs) {
			t.Fatalf("line %d moved: %+v against %+v", li, ln, was)
		}
		for ri, r := range ln.Runs {
			if r.Text != was.Runs[ri].Text || r.Offset != was.Runs[ri].Offset || r.W != was.Runs[ri].W {
				t.Fatalf("line %d run %d changed beyond its X: %+v against %+v", li, ri, r, was.Runs[ri])
			}
		}
		if len(ln.Runs) == 0 {
			continue
		}
		wasLast := was.Runs[len(was.Runs)-1]
		slack := float32(width) - (wasLast.X + wasLast.W)
		kind := ""
		switch {
		case li+1 == len(just.Lines) || just.Lines[li+1].ParaFirst:
			kind = "a paragraph's last line"
		case ln.PoemBreakAfter:
			kind = "a line before a poem break"
		case poetic[ln.Runs[0].Verse]:
			kind = "a row of a poetic verse"
		case opensOnPoem[ln.Runs[0].Verse]:
			kind = "prose in a paragraph that opens on a poem line"
		}
		if !ln.Justified {
			for ri, r := range ln.Runs {
				if r.X != was.Runs[ri].X {
					t.Errorf("line %d is not justified and yet its run %d moved", li, ri)
				}
			}
			if kind != "" && len(ln.Runs) > 1 && slack > 1 {
				exempt[kind]++
			}
			continue
		}
		justified++
		if kind != "" || len(ln.Runs) < 2 {
			t.Errorf("line %d was justified though it is %q (%d runs)", li, kind, len(ln.Runs))
		}
		if ln.Runs[0].X != was.Runs[0].X {
			t.Errorf("line %d: justifying moved its first run from %v to %v", li, was.Runs[0].X, ln.Runs[0].X)
		}
		if ln.ParaFirst && ln.Runs[0].X == justifyIndent {
			indented++
		}
		extra := slack / float32(len(ln.Runs)-1)
		for ri, r := range ln.Runs {
			if moved := r.X - was.Runs[ri].X; math.Abs(float64(moved-float32(ri)*extra)) > 0.01 {
				t.Errorf("line %d run %d (%q) moved %v, want %v: the slack is not shared evenly", li, ri, r.Text, moved, float32(ri)*extra)
			}
		}
		if last := ln.Runs[len(ln.Runs)-1]; math.Abs(float64(last.X+last.W-width)) > 0.01 {
			t.Errorf("line %d: justified to %v, not the measure %v", li, last.X+last.W, width)
		}
		if slack < 1 {
			flushBefore++
		}
	}
	if justified == 0 {
		t.Fatal("no line was justified — the fixture does not exercise it")
	}
	// The control: had the ragged lines already ended on the measure, the
	// check above could not tell a justified line from a ragged one.
	if flushBefore == justified {
		t.Fatal("every justified line was already flush when ragged; the fixture proves nothing")
	}
	if indented == 0 {
		t.Error("no indented first line was justified, so the indent was never tested")
	}
	for _, kind := range []string{"a paragraph's last line", "a line before a poem break", "a row of a poetic verse", "prose in a paragraph that opens on a poem line"} {
		if exempt[kind] == 0 {
			t.Errorf("the fixture has no %s that would otherwise be justified", kind)
		}
	}
}

// A layout built without asking — the golden file, the geometry tests — is the
// ragged one, whatever the spec says.
func TestALayoutIsRaggedUnlessItAsks(t *testing.T) {
	st := justifyState()
	p := testLayoutParams
	p.Width = 400
	for _, ln := range layoutChapter(st, st.Bible.GetChapter("Romans", 8), p, fixedMeasure).Lines {
		if ln.Justified {
			t.Fatal("a layout that did not ask for justification justified a line")
		}
	}
}

// The pane justifies when the switch is on, draws a justified line one word to
// an object at the layout's X — a merged string would draw its words one space
// apart and leave the washes and the selection stretched over text that is
// not — and keeps merging every other line. Everything that reads positions
// follows: a word's first and last rune sit where the word is drawn, a pointer
// in a widened gap goes to the nearer word (so a drag past a word ends on it
// and a double-click beside it selects it, not the next one), and a wash runs
// from the first word of its stretch to the end of the last.
func TestTheCanvasPaneJustifiesAndEverythingFollows(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	prev := readingJustifyOverride
	t.Cleanup(func() { readingJustifyOverride = prev })
	// On whatever readingJustifyProse says, so turning the spec off leaves
	// this test proving the code that is still there.
	readingJustifyOverride = func() (bool, bool) { return true, true }

	for _, width := range []float32{420, 1100} {
		st := justifyState()
		st.setHL(hlSearch, "Romans", 8, 1, 2)
		p := newTestPane(t, st, width)
		justified, merged, gaps := 0, 0, 0
		for li, ln := range p.lay.Lines {
			segs := p.lineSegs[li]
			if !ln.Justified {
				if len(segs) < len(ln.Runs) {
					merged++
				}
				continue
			}
			justified++
			if len(segs) != len(ln.Runs) {
				t.Fatalf("width %v line %d: %d drawn segments for %d words on a justified line", width, li, len(segs), len(ln.Runs))
			}
			y := ln.Y + ln.H/2
			for si, seg := range segs {
				r := ln.Runs[si]
				if seg.X != r.X || seg.Text != r.Text {
					t.Fatalf("width %v line %d: segment %d drawn at %v (%q), its word is at %v (%q)", width, li, si, seg.X, seg.Text, r.X, r.Text)
				}
				if r.Kind == runVerseGap {
					continue
				}
				n := len([]rune(r.Text))
				right := p.insetX() + r.X + p.segWidth(seg, r.Text)
				if x := p.xForOffset(li, r.Offset); math.Abs(float64(x-(p.insetX()+r.X))) > 0.01 {
					t.Errorf("width %v line %d: %q starts at %v for the selection and %v on the page", width, li, r.Text, x, p.insetX()+r.X)
				}
				if x := p.xForOffset(li, r.Offset+n); math.Abs(float64(x-right)) > 0.01 {
					t.Errorf("width %v line %d: %q ends at %v for the selection, not where it is drawn", width, li, r.Text, x)
				}
				for _, off := range []int{r.Offset, r.Offset + n} {
					if got := p.offsetAtPos(posForOffset(p, off)); got != off {
						t.Errorf("width %v line %d: offset %d, at a word's edge, round-trips to %d", width, li, off, got)
					}
				}
				// The gap to the next word, both sides of it.
				if si+1 == len(segs) || r.Kind != runWord || segs[si+1].Kind != runWord {
					continue
				}
				next := segs[si+1]
				if p.insetX()+next.X-right < 4 {
					continue
				}
				gaps++
				if got := p.offsetAtPos(fyne.NewPos(right+1, y)); got != r.Offset+n {
					t.Errorf("width %v line %d: a pointer just past %q lands on offset %d, not the word's end %d", width, li, r.Text, got, r.Offset+n)
				}
				if got := p.offsetAtPos(fyne.NewPos(p.insetX()+next.X-1, y)); got != next.FirstOffset {
					t.Errorf("width %v line %d: a pointer just before %q lands on offset %d, not its start %d", width, li, next.Text, got, next.FirstOffset)
				}
				// The gap splits at its middle: each half belongs to its word.
				mid := (right + p.insetX() + next.X) / 2
				if a, b := p.offsetAtPos(fyne.NewPos(mid-1, y)), p.offsetAtPos(fyne.NewPos(mid+1, y)); a != r.Offset+n || b != next.FirstOffset {
					t.Errorf("width %v line %d: either side of the middle of the gap after %q lands on %d and %d, want %d and %d", width, li, r.Text, a, b, r.Offset+n, next.FirstOffset)
				}
				p.DoubleTapped(&fyne.PointEvent{Position: fyne.NewPos(right+1, y)})
				if got := string([]rune(p.lay.Text)[p.selStart:p.selEnd]); got != r.Text {
					t.Errorf("width %v line %d: a double-click just past %q selected %q", width, li, r.Text, got)
				}
				p.clearSelection()
			}
			for off := ln.StartOffset; off <= ln.EndOffset; off++ {
				if got := p.offsetAtPos(posForOffset(p, off)); got < off-1 || got > off+1 {
					t.Fatalf("width %v line %d: offset %d round-trips to %d", width, li, off, got)
				}
			}
		}
		if justified == 0 {
			t.Fatalf("width %v: the pane justified no line", width)
		}
		if merged == 0 {
			t.Fatalf("width %v: no ragged line merged its words; merging must stay where the gaps are natural", width)
		}
		if gaps == 0 {
			t.Fatalf("width %v: no widened gap between two words was tried", width)
		}
		// The washes spread with the words: each stretch on a justified line
		// runs from its first run's X to its last run's end, and a line lit
		// from end to end is lit from its first word to the measure.
		washed, whole := 0, 0
		for _, sp := range tintSpansForLayout(p.lay) {
			ln := p.lay.Lines[sp.Line]
			if !ln.Justified {
				continue
			}
			washed++
			starts, ends := false, false
			for _, r := range ln.Runs {
				if r.Tint != sp.Tint {
					continue
				}
				starts = starts || math.Abs(float64(r.X-sp.LineX0)) < 0.01
				ends = ends || math.Abs(float64(r.X+r.W-sp.LineX1)) < 0.01
			}
			if !starts || !ends {
				t.Errorf("width %v line %d: a wash from %v to %v does not start and end on its words", width, sp.Line, sp.LineX0, sp.LineX1)
			}
			first, last := ln.Runs[0], ln.Runs[len(ln.Runs)-1]
			if sp.LineX0 == first.X && math.Abs(float64(sp.LineX1-(last.X+last.W))) < 0.01 {
				whole++
			}
		}
		if washed == 0 || whole == 0 {
			t.Fatalf("width %v: %d washes on justified lines, %d of them whole lines; the wash was not tested", width, washed, whole)
		}
	}
}

// Across a justified line the pointer only ever moves forward through the
// text: sweeping it left to right never lands on an earlier offset or one off
// the line — past an omitted verse's mark too, which is drawn but has no text
// (the Berean's Matthew 17 omits verse 21).
func TestAPointerAcrossAJustifiedLineMovesForward(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	prev := readingJustifyOverride
	t.Cleanup(func() { readingJustifyOverride = prev })
	readingJustifyOverride = func() (bool, bool) { return true, true }
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)

	gapped, _ := gappedState()
	for _, c := range []struct {
		name string
		st   *AppState
	}{{"Romans 8", justifyState()}, {"Matthew 17", gapped}} {
		p := newTestPane(t, c.st, 420)
		swept, ghosts := 0, 0
		for li, ln := range p.lay.Lines {
			if !ln.Justified {
				continue
			}
			swept++
			for _, r := range ln.Runs {
				if r.Kind == runVerseGap {
					ghosts++
				}
			}
			y, last := ln.Y+ln.H/2, ln.StartOffset
			for x := p.insetX() - 2; x < p.insetX()+p.lay.Lines[li].Runs[len(ln.Runs)-1].X+60; x++ {
				got := p.offsetAtPos(fyne.NewPos(x, y))
				if got < last || got < ln.StartOffset || got > ln.EndOffset {
					t.Fatalf("%s line %d: at x %v the pointer lands on offset %d, after %d (the line holds %d to %d)", c.name, li, x, got, last, ln.StartOffset, ln.EndOffset)
				}
				last = got
			}
		}
		if swept == 0 {
			t.Fatalf("%s: no justified line to sweep", c.name)
		}
		if c.name == "Matthew 17" && ghosts == 0 {
			t.Fatal("Matthew 17: no omitted verse's mark on a justified line, so the mark was never swept")
		}
	}
}

// The switch turns it all off: answered false, the pane lays out and draws the
// ragged page — the same lines, no wider gaps, and the words merged again. On
// the book page, the indented first line of a justified paragraph starts where
// the ragged one does.
func TestTheJustifySwitchGivesBackTheRaggedPage(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	prev := readingJustifyOverride
	t.Cleanup(func() { readingJustifyOverride = prev })

	for _, width := range []float32{420, 1100} {
		readingJustifyOverride = func() (bool, bool) { return true, true }
		on := newTestPane(t, justifyState(), width)
		readingJustifyOverride = func() (bool, bool) { return false, true }
		off := newTestPane(t, justifyState(), width)

		if len(on.lay.Lines) != len(off.lay.Lines) {
			t.Fatalf("width %v: the switch changed the line count: %d against %d", width, len(on.lay.Lines), len(off.lay.Lines))
		}
		// The control: on, something was justified, so off has something to undo.
		undone, indented := 0, 0
		for li, ln := range off.lay.Lines {
			if ln.Justified {
				t.Fatalf("width %v line %d justified with the switch off", width, li)
			}
			onLn := on.lay.Lines[li]
			if !onLn.Justified {
				continue
			}
			undone++
			if onLn.Runs[0].X != ln.Runs[0].X {
				t.Errorf("width %v line %d: the first word starts at %v justified and %v ragged", width, li, onLn.Runs[0].X, ln.Runs[0].X)
			}
			if ln.ParaFirst && ln.Runs[0].X > 0 {
				indented++
			}
			for ri := 1; ri < len(ln.Runs); ri++ {
				gapOff := ln.Runs[ri].X - (ln.Runs[ri-1].X + ln.Runs[ri-1].W)
				gapOn := onLn.Runs[ri].X - (onLn.Runs[ri-1].X + onLn.Runs[ri-1].W)
				if gapOff > gapOn {
					t.Fatalf("width %v line %d: a gap is wider off than on", width, li)
				}
			}
			if len(off.lineSegs[li]) >= len(on.lineSegs[li]) {
				t.Errorf("width %v line %d: the switch off left its words drawn one each (%d segments against %d)", width, li, len(off.lineSegs[li]), len(on.lineSegs[li]))
			}
		}
		if undone == 0 {
			t.Fatalf("width %v: the switch on justified nothing, so off proves nothing", width)
		}
		if width == 1100 && indented == 0 {
			t.Error("the book page justified no indented first line, so the indent was never compared")
		}
	}
}
