package bibletext

// Tests for the styled chapter layout engine (the Windows/Linux pane rewrite).
// The load-bearing property is PARITY: layoutChapter's flat text model and
// verse geometry must match chapterText.rewrap exactly, because everything
// downstream — selection normalization, share, copy, the scroll anchor — was
// built and hardened against that shape.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

// fixedMeasure is a deterministic ruler: every rune is 10 units wide at body
// size, 6 at verse-number size — so wraps land at predictable points.
func fixedMeasure(text string, kind runKind) float32 {
	w := float32(10)
	if kind == runVerseNum {
		w = 6
	}
	return w * float32(len([]rune(text)))
}

var testLayoutParams = styledLayoutParams{
	Width:      10000, // wide: no soft wraps unless a test narrows it
	LineHeight: 20,
	ParaGap:    10,
	SpaceW:     10,
}

func layoutText(lay *chapterLayout) string { return lay.Text }

// fyneMeasure is the production ruler at a fixed size, for parity tests
// against rewrap (which uses fyne.MeasureText with the same style).
func fyneMeasure(size float32) styledMeasure {
	return func(text string, kind runKind) float32 {
		return fyne.MeasureText(text, size, fyne.TextStyle{}).Width
	}
}

// TestStyledLayoutTextModelParity: for both a prose and a poetic chapter, at a
// narrow width forcing many wraps, the flat text model must equal rewrap's
// Entry text RUNE FOR RUNE when both use the same measure. This is the
// contract that keeps selection/share/copy behaviour identical.
func TestStyledLayoutTextModelParity(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	for _, tc := range []struct {
		name  string
		state *AppState
		book  string
		ch    int
	}{
		{"prose (Acts 4)", acts4ShareState(), "Acts", 4},
		{"poetry (Psalm 23)", psalm23State(), "Psalms", 23},
		{"mixed (Exodus 15)", exodus15State(), "Exodus", 15},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verses := tc.state.Bible.GetChapter(tc.book, tc.ch)

			// The rewrap side: chapterText at a narrow width.
			c := newChapterText(tc.state, verses)
			c.rewrap(300)

			// The styled side: same available width and the same ruler.
			// rewrap's avail is width - 4*InnerPadding; textSize as set by
			// newChapterText.
			size := c.textSize
			avail := float32(300) - 4*theme.InnerPadding()
			lay := layoutChapter(tc.state, verses, styledLayoutParams{
				Width:      avail,
				LineHeight: 20,
				ParaGap:    10,
				SpaceW:     fyne.MeasureText(" ", size, fyne.TextStyle{}).Width,
			}, func(text string, kind runKind) float32 {
				return fyne.MeasureText(text, size, fyne.TextStyle{}).Width
			})

			if got, want := layoutText(lay), c.Entry.Text; got != want {
				t.Errorf("text model diverges from rewrap:\n got %q\nwant %q", got, want)
			}

			// Verse geometry parity: same verse→first-line mapping.
			if len(lay.VerseLines) != len(c.verseLines) {
				t.Fatalf("verseLines count = %d, want %d", len(lay.VerseLines), len(c.verseLines))
			}
			for i := range c.verseLines {
				if lay.VerseLines[i] != c.verseLines[i] {
					t.Errorf("verseLines[%d] = %+v, want %+v", i, lay.VerseLines[i], c.verseLines[i])
				}
			}
		})
	}
}

func exodus15State() *AppState {
	bd := &BibleData{
		Books: []string{"Exodus"},
		Verses: map[string]map[int][]Verse{"Exodus": {15: {
			{BookName: "Exodus", Book: "Exodus", Chapter: 15, Verse: 1,
				Text: "Then Moses and the Israelites sang this song to the LORD:"},
			{BookName: "Exodus", Book: "Exodus", Chapter: 15, Verse: 2,
				Text: "The LORD is my strength and my song,\nand He has become my salvation."},
		}}},
	}
	return &AppState{Bible: bd, CurrentBook: "Exodus", CurrentChapter: 15}
}

// TestStyledLayoutRuns pins the run-level styling the whole project exists
// for: verse numbers as their own small runs, red-letter attribution on word
// runs only, and poem lines as separate styled lines.
func TestStyledLayoutRuns(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	lay := layoutChapter(st, st.Bible.GetChapter("Psalms", 23), testLayoutParams, fixedMeasure)

	// Wide layout of Ps 23:1-2 = 4 authored lines.
	if len(lay.Lines) != 4 {
		t.Fatalf("lines = %d, want 4 (authored poem lines)", len(lay.Lines))
	}
	l0 := lay.Lines[0]
	if l0.Runs[0].Kind != runVerseNum || l0.Runs[0].Text != superscriptNumber(1) {
		t.Errorf("line 0 must open with the verse-number run, got %+v", l0.Runs[0])
	}
	if l0.Runs[1].Kind != runWord || l0.Runs[1].Verse != 1 {
		t.Errorf("number's glued word run wrong: %+v", l0.Runs[1])
	}
	for _, ln := range lay.Lines[:3] {
		if !ln.PoemBreakAfter {
			t.Errorf("authored poem line must be marked PoemBreakAfter: %+v", ln.Runs[0])
		}
	}
	if lay.Lines[3].PoemBreakAfter {
		t.Error("the chapter's final line has no authored break after it")
	}

	// Geometry: runs advance left to right with the configured space.
	if l0.Runs[1].X <= l0.Runs[0].X {
		t.Errorf("runs must advance: %+v", l0.Runs)
	}
	// Verse 2 starts on line 2 (same as the shipping pane's geometry).
	if len(lay.VerseLines) != 2 || lay.VerseLines[1].line != 2 {
		t.Errorf("verseLines = %+v, want verse 2 on line 2", lay.VerseLines)
	}
}

// TestStyledLayoutRedLetter: words of Christ are red on WORD runs only, never
// on the verse-number run, and only for the marked verses.
func TestStyledLayoutRedLetter(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setRedLetterEnabled(true)

	// John 3:16 is in the words-of-Christ table; 3:1 (narration) is not.
	bd := &BibleData{
		Books: []string{"John"},
		Verses: map[string]map[int][]Verse{"John": {3: {
			{BookName: "John", Book: "John", Chapter: 3, Verse: 1,
				Text: "Now there was a man of the Pharisees named Nicodemus."},
			{BookName: "John", Book: "John", Chapter: 3, Verse: 16,
				Text: "For God so loved the world."},
		}}},
	}
	st := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3}
	lay := layoutChapter(st, bd.GetChapter("John", 3), testLayoutParams, fixedMeasure)

	sawRed, sawPlain := false, false
	for _, ln := range lay.Lines {
		for _, r := range ln.Runs {
			switch {
			case r.Kind == runVerseNum && r.RedLetter:
				t.Errorf("verse-number run must never be red: %+v", r)
			case r.Kind == runWord && r.Verse == 16 && !r.RedLetter:
				t.Errorf("John 3:16 word run must be red: %+v", r)
			case r.Kind == runWord && r.Verse == 1 && r.RedLetter:
				t.Errorf("narration must not be red: %+v", r)
			}
			if r.RedLetter {
				sawRed = true
			} else if r.Kind == runWord {
				sawPlain = true
			}
		}
	}
	if !sawRed || !sawPlain {
		t.Fatalf("fixture must produce both red and plain runs (red=%v plain=%v)", sawRed, sawPlain)
	}
}

// TestStyledLayoutOffsets: every run's Offset must index its own text inside
// the flat model — the property selection ranges depend on.
func TestStyledLayoutOffsets(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	for _, st := range []*AppState{acts4ShareState(), psalm23State(), exodus15State()} {
		verses := st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter)
		lay := layoutChapter(st, verses, styledLayoutParams{
			Width: 300, LineHeight: 20, ParaGap: 10, SpaceW: 10,
		}, fixedMeasure)
		runes := []rune(lay.Text)
		for li, ln := range lay.Lines {
			for ri, r := range ln.Runs {
				want := []rune(r.Text)
				if r.Offset+len(want) > len(runes) {
					t.Fatalf("line %d run %d offset out of range", li, ri)
				}
				if got := string(runes[r.Offset : r.Offset+len(want)]); got != r.Text {
					t.Fatalf("line %d run %d: model[%d:] = %q, want %q", li, ri, r.Offset, got, r.Text)
				}
			}
		}
	}
}

// TestStyledLayoutHighlightBand: the highlighted verse range maps to the
// correct line span, including a multi-line poetic verse.
func TestStyledLayoutHighlightBand(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	st.setHL(hlSearch, "Psalms", 23, 2, 0)
	lay := layoutChapter(st, st.Bible.GetChapter("Psalms", 23), testLayoutParams, fixedMeasure)

	if lay.HighlightStart != 2 || lay.HighlightEnd != 3 {
		t.Errorf("highlight band = [%d,%d], want [2,3]", lay.HighlightStart, lay.HighlightEnd)
	}
	// No highlight → both -1.
	st2 := psalm23State()
	lay2 := layoutChapter(st2, st2.Bible.GetChapter("Psalms", 23), testLayoutParams, fixedMeasure)
	if lay2.HighlightStart != -1 || lay2.HighlightEnd != -1 {
		t.Errorf("no-highlight band = [%d,%d], want [-1,-1]", lay2.HighlightStart, lay2.HighlightEnd)
	}
}

// TestStyledLayoutNarrowWrap: a narrow width forces soft wraps; no content is
// lost, no line exceeds the width, and soft wraps are never marked authored.
func TestStyledLayoutNarrowWrap(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := acts4ShareState()
	verses := st.Bible.GetChapter("Acts", 4)
	p := styledLayoutParams{Width: 200, LineHeight: 20, ParaGap: 10, SpaceW: 10}
	lay := layoutChapter(st, verses, p, fixedMeasure)

	if len(lay.Lines) < 5 {
		t.Fatalf("narrow layout should wrap into many lines, got %d", len(lay.Lines))
	}
	var words []string
	for _, ln := range lay.Lines {
		lineW := float32(0)
		for ri, r := range ln.Runs {
			if ri > 0 {
				lineW += p.SpaceW
			}
			lineW += r.W
			words = append(words, r.Text)
		}
		// A single over-wide unit may exceed the width (same as rewrap); a
		// MULTI-unit line may not.
		if len(ln.Runs) > 2 && lineW > p.Width+0.01 {
			t.Errorf("line exceeds width: %.1f > %.1f (%q…)", lineW, p.Width, ln.Runs[0].Text)
		}
		if ln.PoemBreakAfter {
			t.Errorf("prose chapter must have no authored breaks: %+v", ln.Runs[0])
		}
	}
	joined := strings.Join(strings.Fields(strings.Join(words, " ")), " ")
	var src []string
	for _, v := range verses {
		src = append(src, superscriptNumber(v.Verse))
		src = append(src, strings.Fields(v.Text)...)
	}
	if want := strings.Join(src, " "); joined != want {
		t.Errorf("content lost or reordered:\n got %q\nwant %q", joined, want)
	}
}
