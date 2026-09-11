package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// A chapter with a publisher's heading between its first and second paragraphs,
// as the Berean Standard Bible sets Matthew 5.
func headedChapterState() *AppState {
	bd := NewBibleData()
	bd.Books = []string{"Matthew"}
	bd.Verses["Matthew"] = map[int][]Verse{5: {
		{BookName: "Matthew", Chapter: 5, Verse: 1, Text: "When Jesus saw the crowds, he went up on the mountain.", ParaStart: true},
		{BookName: "Matthew", Chapter: 5, Verse: 2, Text: "He opened his mouth and taught them, saying,"},
		{BookName: "Matthew", Chapter: 5, Verse: 3, Text: "Blessed are the poor in spirit.", ParaStart: true},
		{BookName: "Matthew", Chapter: 5, Verse: 4, Text: "Blessed are those who mourn.", ParaStart: true},
	}}
	bd.Headings = map[string]map[int][]Heading{"Matthew": {5: {{Text: "The Beatitudes", Style: "heading", BeforeVerse: 3}}}}
	return &AppState{Bible: bd, CurrentBook: "Matthew", CurrentChapter: 5}
}

// THE LEAD ABOVE A HEADING RIDES ON THE PARAGRAPH BEFORE IT, on the Apple
// panes. Their sweeps zero paragraphSpacingBefore on every paragraph, so a
// margin-top on the heading never reaches the page: headings stood with .35em
// below and nothing above. Mutations: the pre-sec class no longer written on
// the paragraph before a heading; the lead moved back onto the heading's
// margin-top.
func TestApplePanesLeadAHeadingFromTheParagraphBefore(t *testing.T) {
	st := headedChapterState()
	html := buildChapterHTML(st, st.Bible.GetChapter("Matthew", 5))

	if n := strings.Count(html, "pre-sec"); n != 2 { // one class attribute, one CSS rule
		t.Errorf("pre-sec appears %d times; want the one paragraph before the heading and the one rule", n)
	}
	lead, head := strings.Index(html, `<p class="pre-sec">`), strings.Index(html, `<p class="sec">`)
	if lead < 0 || head < 0 || lead > head {
		t.Errorf("the paragraph before the heading does not carry the lead class (lead at %d, heading at %d)", lead, head)
	}
	if !strings.Contains(html, "p.pre-sec {") || !strings.Contains(html, "margin-bottom: 1.1em;") {
		t.Error("the stylesheet does not give p.pre-sec the 1.1em bottom margin")
	}
	sec := html[strings.Index(html, "p.sec {"):]
	sec = sec[:strings.Index(sec, "}")]
	if !strings.Contains(sec, "margin: 0 0 0.35em 0;") {
		t.Errorf("the heading's own margins are %q; the lead above must not be a margin-top the sweep zeroes", sec)
	}
	// A psalm whose title stands before its first heading: the title carries
	// the lead, the way a paragraph would.
	ps := headedChapterState()
	ps.Bible.Books = []string{"Psalms"}
	ps.Bible.Verses = map[string]map[int][]Verse{"Psalms": {23: {
		{BookName: "Psalms", Chapter: 23, Verse: 1, Text: "The LORD is my shepherd.", ParaStart: true},
		{BookName: "Psalms", Chapter: 23, Verse: 2, Text: "He makes me lie down."},
	}}}
	ps.Bible.Headings = map[string]map[int][]Heading{"Psalms": {23: {{Text: "The LORD the Shepherd", Style: "s", BeforeVerse: 1}}}}
	ps.Bible.Superscriptions = map[string]map[int]Superscription{"Psalms": {23: {Text: "A Psalm of David."}}}
	ps.CurrentBook, ps.CurrentChapter = "Psalms", 23
	if h := buildChapterHTML(ps, ps.Bible.GetChapter("Psalms", 23)); !strings.Contains(h, `<p class="pst pre-sec">`) {
		t.Error("a psalm title standing before the first heading does not carry the heading's lead")
	}
	// The control: a chapter without a heading writes no lead class at all, so
	// the count above measures the heading and not a stray class.
	st.Bible.Headings = nil
	if plain := buildChapterHTML(st, st.Bible.GetChapter("Matthew", 5)); strings.Contains(plain, `class="pre-sec"`) {
		t.Error("a chapter without headings still marks a paragraph pre-sec")
	}
}

// THE FYNE PANE GIVES A HEADING ITS AIR IN EMS OF THE BODY: 1.1em above and
// .35em below, the native panes' numbers, whatever the paragraph gap is — the
// reporter page's gap is zero, and headings there stood flush on both sides.
// Mutations: the lead taken from ParaGap again; the paragraph after a heading
// adding its own gap on top of the heading's tail.
func TestStyledLayoutLeadsAHeadingInEmsOfTheBody(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	st := headedChapterState()
	p := testLayoutParams
	p.TextSize = 20
	p.ParaGap = 11 // deliberately unlike .35em, so a gap that adds it is visible
	verses := st.Bible.GetChapter("Matthew", 5)
	lay := layoutChapter(st, verses, p, fixedMeasure)

	hi := -1
	for i, ln := range lay.Lines {
		if ln.Heading != "" {
			hi = i
			break
		}
	}
	if hi < 1 || hi+1 >= len(lay.Lines) {
		t.Fatalf("the heading line is at %d of %d; the fixture needs a paragraph on each side", hi, len(lay.Lines))
	}
	prev, head, next := lay.Lines[hi-1], lay.Lines[hi], lay.Lines[hi+1]
	near := func(a, b float32) bool { return a-b < 0.01 && b-a < 0.01 }
	if above := head.Y - (prev.Y + prev.H); !near(above, 1.1*p.TextSize) {
		t.Errorf("the heading stands %.2fpt below the text; want 1.1em = %.2f", above, 1.1*p.TextSize)
	}
	if below := next.Y - (head.Y + head.H); !near(below, 0.35*p.TextSize) {
		t.Errorf("the next paragraph starts %.2fpt below the heading; want .35em = %.2f (the paragraph gap must not be added)", below, 0.35*p.TextSize)
	}
	// The control: without the heading the two paragraphs are the paragraph
	// gap apart, which proves the fixture's paragraphs are two and the gap is
	// live.
	st.Bible.Headings = nil
	plain := layoutChapter(st, verses, p, fixedMeasure)
	if len(plain.Lines) != len(lay.Lines)-1 {
		t.Fatalf("without the heading the layout has %d lines, with it %d", len(plain.Lines), len(lay.Lines))
	}
	if gap := plain.Lines[hi].Y - (plain.Lines[hi-1].Y + plain.Lines[hi-1].H); !near(gap, p.ParaGap) {
		t.Errorf("without a heading the paragraphs are %.2fpt apart, not the paragraph gap %.2f", gap, p.ParaGap)
	}
}
