package bibletext

// The translators' supplied words on the Windows and Linux pane. They are drawn
// in the italic cut, so the selection has to measure them in it: every position
// the selection reads inside a supplied segment — a caret, a selection's end, the
// rune under a pointer — is judged here against the ink the renderer put down.

import (
	"math"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// suppliedState is justifyState's chapter with words marked as supplied, the
// way an API.Bible edition such as the NKJV marks them: phrases of several
// words, which a ragged line draws as one merged segment, through prose that
// justifies at both page widths.
func suppliedState(t *testing.T) *AppState {
	t.Helper()
	st := justifyState()
	verses := st.Bible.Verses["Romans"][8]
	mark := func(verse int, phrases ...string) {
		v := &verses[verse-1]
		for _, ph := range phrases {
			i := strings.Index(v.Text, ph)
			if i < 0 {
				t.Fatalf("verse %d has no %q to mark", verse, ph)
			}
			start := len([]rune(v.Text[:i]))
			v.Supplied = append(v.Supplied, TextSpan{Start: start, End: start + len([]rune(ph))})
		}
	}
	mark(1, "there is now", "for those who are")
	mark(2, "the law of the Spirit", "set you free from")
	mark(3, "what the law was", "it was weakened by")
	mark(4, "the righteous standard", "who do not walk according")
	return st
}

// A supplied word's caret, its selection and the pointer over it are where its
// italic is drawn. For each supplied segment the renderer drew, and each word in
// it: the X the selection gives the word's start and end is the drawn object's
// left plus the width of the text before that point in the object's own face and
// size; a pointer just inside the word's first letter, or just past its last,
// comes back to that edge, so an offset goes to the ink and back unchanged; a
// double-click on its first letter selects that word; and a selection of the
// whole segment ends where its ink ends. On ragged lines, where a phrase is one
// merged segment and the difference between the cuts builds up word by word, and
// on justified lines, where each word is drawn alone.
func TestSuppliedWordsAreHitTestedWhereTheirItalicIsDrawn(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	prev := readingJustifyOverride
	t.Cleanup(func() { readingJustifyOverride = prev })
	drv := fyne.CurrentApp().Driver()

	for _, justify := range []bool{false, true} {
		readingJustifyOverride = func() (bool, bool) { return justify, true }
		for _, width := range []float32{420, 1100} {
			p := newTestPane(t, suppliedState(t), width)
			r := p.CreateRenderer().(*styledPaneRenderer)
			merged, alone := 0, 0
			for i, dr := range p.drawRuns {
				if !dr.Supplied {
					continue
				}
				ln := p.lay.Lines[dr.Line]
				segs := p.lineSegs[dr.Line]
				ink := r.texts[i]
				if ink.Text != dr.Text {
					t.Fatalf("justify %v width %v: drawn object %d spells %q, its segment %q", justify, width, i, ink.Text, dr.Text)
				}
				if name := ink.FontSource.Name(); name != "Junicode-Italic.ttf" {
					t.Fatalf("justify %v width %v: supplied %q drawn in %s, not the italic cut", justify, width, dr.Text, name)
				}
				drawn := func(s string) float32 {
					w, _ := drv.RenderedTextSize(s, ink.TextSize, ink.TextStyle, ink.FontSource)
					return w.Width
				}
				// Only a segment whose upright width differs from its italic one
				// can show the selection measuring in the wrong cut, and the
				// controls at the end ask for one on each kind of line.
				upright, _ := drv.RenderedTextSize(dr.Text, ink.TextSize, ink.TextStyle, p.font)
				differs := math.Abs(float64(upright.Width-drawn(dr.Text))) >= 1
				left := ink.Position().X
				if math.Abs(float64(left-(p.insetX()+dr.X))) > 0.01 {
					t.Fatalf("justify %v width %v: %q drawn at %v, its segment at %v", justify, width, dr.Text, left, p.insetX()+dr.X)
				}
				runes := []rune(dr.Text)
				last := dr.FirstOffset == segs[len(segs)-1].FirstOffset
				y := ln.Y + ln.H/2
				for s := 0; s < len(runes); {
					e := s
					for e < len(runes) && runes[e] != ' ' {
						e++
					}
					word := string(runes[s:e])
					xs, xe := left+drawn(string(runes[:s])), left+drawn(string(runes[:e]))
					if x := p.xForOffset(dr.Line, dr.FirstOffset+s); math.Abs(float64(x-xs)) > 0.01 {
						t.Errorf("justify %v width %v: supplied %q starts at %v for the selection and %v on the page", justify, width, word, x, xs)
					}
					if x := p.xForOffset(dr.Line, dr.FirstOffset+e); math.Abs(float64(x-xe)) > 0.01 {
						t.Errorf("justify %v width %v: supplied %q ends at %v for the selection and %v on the page", justify, width, word, x, xe)
					}
					if got := p.offsetAtPos(fyne.NewPos(xs+0.25, y)); got != dr.FirstOffset+s {
						t.Errorf("justify %v width %v: a pointer on the first letter of supplied %q lands on offset %d, not its start %d", justify, width, word, got, dr.FirstOffset+s)
					}
					// Past a word's end the pointer is on the space after it
					// when that space is drawn in this segment, on the nearer
					// word on a justified line, and on the line's end after its
					// last segment. Between two segments of a ragged line the
					// space belongs to the next one, so that edge is not a word
					// end for the pointer.
					if e < len(runes) || ln.Justified || last {
						if got := p.offsetAtPos(fyne.NewPos(xe+0.25, y)); got != dr.FirstOffset+e {
							t.Errorf("justify %v width %v: a pointer just past supplied %q lands on offset %d, not its end %d", justify, width, word, got, dr.FirstOffset+e)
						}
					}
					p.DoubleTapped(&fyne.PointEvent{Position: fyne.NewPos(xs+0.25, y)})
					if got := p.selectedRaw(); got != word {
						t.Errorf("justify %v width %v: a double-click on the first letter of supplied %q selected %q", justify, width, word, got)
					}
					p.clearSelection()
					s = e + 1
				}
				// The selection over the whole segment is its ink, end to end.
				p.setSelection(dr.FirstOffset, dr.FirstOffset+len(runes))
				spans := p.selectionSpans()
				if len(spans) != 1 || spans[0].Line != dr.Line {
					t.Fatalf("justify %v width %v: selecting supplied %q gave %+v", justify, width, dr.Text, spans)
				}
				if end := left + drawn(dr.Text); math.Abs(float64(spans[0].X0-left)) > 0.01 || math.Abs(float64(spans[0].X1-end)) > 0.01 {
					t.Errorf("justify %v width %v: the selection of supplied %q runs %v to %v, its ink %v to %v", justify, width, dr.Text, spans[0].X0, spans[0].X1, left, end)
				}
				p.clearSelection()
				if differs && ln.Justified {
					alone++
				} else if differs && strings.Contains(dr.Text, " ") {
					merged++
				}
			}
			// The controls: each kind of line was tried with a segment the
			// wrong cut would have measured differently.
			if !justify && merged == 0 {
				t.Fatalf("width %v: no ragged line drew a supplied phrase as one segment of a telling width", width)
			}
			if justify && alone == 0 {
				t.Fatalf("width %v: no justified line carried a supplied word of a telling width", width)
			}
		}
	}
}

// A prefix is measured in the face its whole segment is drawn in, which is not
// always the face the prefix alone would be given. A Hebrew word that opens on
// a Latin mark is drawn in the Hebrew face from its first rune, the mark
// included, so the caret after the mark stands where the mark's ink ends in
// that face.
func TestAPrefixIsMeasuredInTheFaceItsSegmentIsDrawnIn(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	bd := NewBibleData()
	bd.Verses = map[string]map[int][]Verse{"Genesis": {1: {
		{BookName: "Genesis", Chapter: 1, Verse: 1, ParaStart: true, Text: "A word (שָׁלוֹם) set among others."},
	}}}
	bd.Books = []string{"Genesis"}
	bd.PrepareSearchIndex()
	st := &AppState{Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1, CurrentVersion: "web"}
	p := newTestPane(t, st, 420)
	r := p.CreateRenderer().(*styledPaneRenderer)
	drv := fyne.CurrentApp().Driver()

	found := false
	for i, dr := range p.drawRuns {
		if !hasHebrew(dr.Text) {
			continue
		}
		found = true
		ink := r.texts[i]
		if dr.Text != "(שָׁלוֹם)" || ink.FontSource != hebrewReadingFont() {
			t.Fatalf("the Hebrew word is drawn as %q in %s", dr.Text, ink.FontSource.Name())
		}
		mark, _ := drv.RenderedTextSize("(", ink.TextSize, ink.TextStyle, ink.FontSource)
		// The control: the reading face gives the mark another width.
		if latin, _ := drv.RenderedTextSize("(", ink.TextSize, ink.TextStyle, p.font); math.Abs(float64(latin.Width-mark.Width)) < 1 {
			t.Fatalf("the mark is %v wide in either face; the fixture cannot tell them apart", mark.Width)
		}
		want := ink.Position().X + mark.Width
		if got := p.xForOffset(dr.Line, dr.FirstOffset+1); math.Abs(float64(got-want)) > 0.01 {
			t.Errorf("the caret after the mark stands at %v; the mark's ink ends at %v", got, want)
		}
	}
	if !found {
		t.Fatal("no Hebrew segment was drawn")
	}
}
