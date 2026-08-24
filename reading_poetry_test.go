package bibletext

// Poetry rendering in the reading pane: authored poem lines (the "\n" breaks
// the decoder derives from helloao {text,poem:N} clauses) must display as real
// line breaks on every surface, and a verse boundary inside a poem is a line
// boundary too (poeticJoin — the same rule the share pipeline restores with).
// The two HTML dialects are pinned side by side because they must make
// identical poetry decisions or platforms diverge.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// Real BSB Psalm 23:1–2 as the decoder now produces it (see bsb_test.go's
// real captures): two poem lines per verse.
func psalm23State() *AppState {
	v1 := "The LORD is my shepherd;\nI shall not want."
	v2 := "He makes me lie down in green pastures;\nHe leads me beside quiet waters."
	bd := &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {23: {
			{BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 1, Text: v1},
			{BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 2, Text: v2},
		}}},
	}
	return &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 23}
}

func TestBuildChapterHTMLPoetry(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	html := buildChapterHTML(st, st.Bible.GetChapter("Psalms", 23))

	// Poem lines inside a verse become explicit <br> (a literal "\n" is just
	// HTML whitespace), and the poetic verse boundary breaks before the next
	// verse's number.
	if !strings.Contains(html, "The LORD is my shepherd;<br>I shall not want.") {
		t.Errorf("poem line must be a <br>:\n%s", html)
	}
	if !strings.Contains(html, `I shall not want.<br><sup class="v">2</sup>`) {
		t.Errorf("poetic verse join must be a <br>:\n%s", html)
	}
	// Poetic paragraphs are ragged-right (justification would stretch short
	// poem lines full-width).
	if !strings.Contains(html, `<p class="pm">`) {
		t.Errorf("poetic paragraph must carry the pm class:\n%s", html)
	}
	if !strings.Contains(html, `p.pm { text-align: left; }`) {
		t.Errorf("the pm style must be defined:\n%s", html)
	}
}

func TestBuildChapterHTMLProseUnchanged(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := acts4ShareState() // prose fixture (no "\n" anywhere)
	html := buildChapterHTML(st, st.Bible.GetChapter("Acts", 4))

	if strings.Contains(html, "<br>") || strings.Contains(html, `class="pm"`) {
		t.Errorf("prose chapters must be untouched by the poetry path:\n%s", html)
	}
	if !strings.Contains(html, ` <sup class="v">20</sup>`) {
		t.Errorf("prose verse join stays a plain space:\n%s", html)
	}
}

func TestBuildChapterHTMLMixedProsePoetryJoin(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	// A prose introduction followed by a song (the Exodus 15:1 shape): the
	// join into the poetry breaks even though the previous verse is prose.
	bd := &BibleData{
		Books: []string{"Exodus"},
		Verses: map[string]map[int][]Verse{"Exodus": {15: {
			{BookName: "Exodus", Book: "Exodus", Chapter: 15, Verse: 1,
				Text: "Then Moses and the Israelites sang this song to the LORD:"},
			{BookName: "Exodus", Book: "Exodus", Chapter: 15, Verse: 2,
				Text: "The LORD is my strength and my song,\nand He has become my salvation."},
		}}},
	}
	st := &AppState{Bible: bd, CurrentBook: "Exodus", CurrentChapter: 15}
	html := buildChapterHTML(st, bd.GetChapter("Exodus", 15))
	if !strings.Contains(html, `LORD:<br><sup class="v">2</sup>`) {
		t.Errorf("prose→poetry join must break:\n%s", html)
	}
}

func TestBuildChapterHTMLAndroidPoetry(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	html := buildChapterHTMLAndroid(st, st.Bible.GetChapter("Psalms", 23))

	if !strings.Contains(html, "The LORD is my shepherd;<br>I shall not want.") {
		t.Errorf("Android poem line must be a <br>:\n%s", html)
	}
	if !strings.Contains(html, "I shall not want.<br><sup>") {
		t.Errorf("Android poetic verse join must be a <br>:\n%s", html)
	}

	prose := acts4ShareState()
	if h := buildChapterHTMLAndroid(prose, prose.Bible.GetChapter("Acts", 4)); strings.Contains(h, "<br>") {
		t.Errorf("Android prose chapters must carry no <br>:\n%s", h)
	}
}

func TestChapterTextPoetryRewrap(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	c := newChapterText(st, st.Bible.GetChapter("Psalms", 23))
	c.rewrap(10000) // wide: every break in the text is a poem break, not a wrap

	want := superscriptNumber(1) + " The LORD is my shepherd;\n" +
		"I shall not want.\n" +
		superscriptNumber(2) + " He makes me lie down in green pastures;\n" +
		"He leads me beside quiet waters."
	if got := c.Entry.Text; got != want {
		t.Errorf("poetry lines in the Fyne pane:\n got %q\nwant %q", got, want)
	}

	// The scroll-anchor line index must account for the hard breaks: verse 2
	// starts on the third line (index 2), not mid-line.
	if len(c.verseLines) != 2 {
		t.Fatalf("verseLines = %v", c.verseLines)
	}
	if c.verseLines[0].line != 0 || c.verseLines[1].line != 2 {
		t.Errorf("verse start lines = %v, want lines 0 and 2", c.verseLines)
	}
	if last := c.verseLines[1].line; last >= c.totalLines {
		t.Errorf("verse line %d out of range (totalLines %d)", last, c.totalLines)
	}
}

func TestVerseTokensPoemSentinels(t *testing.T) {
	got := verseTokens(Verse{Verse: 3, Text: "For those who\nsow in tears"})
	want := []string{superscriptNumber(3) + " For", "those", "who", "\n", "sow", "in", "tears"}
	if len(got) != len(want) {
		t.Fatalf("tokens = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPoeticJoin(t *testing.T) {
	poem, prose := "a\nb", "a b"
	for _, tc := range []struct {
		prev, cur string
		want      bool
	}{
		{poem, poem, true},
		{prose, poem, true},
		{poem, prose, true},
		{prose, prose, false},
	} {
		if got := poeticJoin(tc.prev, tc.cur); got != tc.want {
			t.Errorf("poeticJoin(%q, %q) = %v, want %v", tc.prev, tc.cur, got, tc.want)
		}
	}
}

// --- Highlight geometry, narrow wrapping, copy
// fidelity, highlight spans across poem breaks, mixed chapters, and the iPad
// reporter indent rule (via the reporterLayout seam). ---

func TestChapterTextPoetryHighlightBand(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	st.setHL(hlSearch, "Psalms", 23, 2, 0)
	c := newChapterText(st, st.Bible.GetChapter("Psalms", 23))
	c.rewrap(10000)

	// Verse 2 spans rows 2-3 (two poem lines); the band must cover both.
	if c.highlightLine != 2 {
		t.Errorf("highlightLine = %d, want 2", c.highlightLine)
	}
	if c.highlightEndLine != 3 {
		t.Errorf("highlightEndLine = %d, want 3 (the verse's second poem line)", c.highlightEndLine)
	}
}

func TestChapterTextPoetryNarrowWrapAndCopy(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	c := newChapterText(st, st.Bible.GetChapter("Psalms", 23))
	c.rewrap(10000)
	authored := c.Entry.Text // every break is an authored poem break

	// Narrow: soft wraps now interleave with the authored breaks. No content
	// may be lost, verse anchors stay ordered, and copySelection must give
	// back EXACTLY the authored form — poem lines kept, width wraps flattened.
	c.rewrap(140)
	if got, want := strings.Join(strings.Fields(c.Entry.Text), " "), strings.Join(strings.Fields(authored), " "); got != want {
		t.Fatalf("narrow rewrap lost content:\n got %q\nwant %q", got, want)
	}
	if c.verseLines[1].line <= c.verseLines[0].line {
		t.Errorf("verse 2 must start on a later line: %v", c.verseLines)
	}
	c.TypedShortcut(&fyne.ShortcutSelectAll{})
	if got := c.copySelection(); got != authored {
		t.Errorf("copySelection must restore the authored form:\n got %q\nwant %q", got, authored)
	}

	// Wide: the copy equals the display (all breaks authored).
	c.rewrap(10000)
	c.TypedShortcut(&fyne.ShortcutSelectAll{})
	if got := c.copySelection(); got != authored {
		t.Errorf("wide copySelection:\n got %q\nwant %q", got, authored)
	}
}

func TestBuildChapterHTMLPoetryHighlightSpan(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	st.setHL(hlSearch, "Psalms", 23, 2, 0)

	html := buildChapterHTML(st, st.Bible.GetChapter("Psalms", 23))
	want := `<span class="hl">He makes me lie down in green pastures;<br>He leads me beside quiet waters.</span>`
	if !strings.Contains(html, want) {
		t.Errorf("highlight span must carry the poem break inside it:\n%s", html)
	}

	ahtml := buildChapterHTMLAndroid(st, st.Bible.GetChapter("Psalms", 23))
	if !strings.Contains(ahtml, "He makes me lie down in green pastures;<br>He leads me beside quiet waters.</span>") {
		t.Errorf("Android highlight span must carry the poem break inside it:\n%s", ahtml)
	}
}

// A highlight marks a verse; it must not re-typeset it. Bold Georgia sets ~17%
// wider than the regular face, so a weight change re-wrapped the paragraph and
// the text jumped when the highlight cleared. Both mobile dialects are pinned
// here because they are the two that used to bold.
func TestHighlightDoesNotChangeWeight(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	st.setHL(hlSearch, "Psalms", 23, 2, 0)

	// Only the .hl rule: sup.v legitimately carries a weight for the verse
	// number, and that never changes as a highlight comes and goes.
	html := buildChapterHTML(st, st.Bible.GetChapter("Psalms", 23))
	i := strings.Index(html, ".hl {")
	if i < 0 {
		t.Fatalf("no .hl rule in the chapter HTML:\n%s", html)
	}
	rule := html[i:]
	if end := strings.Index(rule, "}"); end >= 0 {
		rule = rule[:end]
	}
	if strings.Contains(rule, "font-weight") {
		t.Errorf("the .hl rule must not change weight — it re-typesets the verse:\n%s", rule)
	}

	ahtml := buildChapterHTMLAndroid(st, st.Bible.GetChapter("Psalms", 23))
	i = strings.Index(ahtml, "He makes me lie down")
	if i < 0 {
		t.Fatalf("highlighted verse missing from Android HTML:\n%s", ahtml)
	}
	// The verse NUMBER legitimately keeps its <b>; the verse TEXT must not.
	span := ahtml[strings.LastIndex(ahtml[:i], "<span"):]
	if end := strings.Index(span, "</span>"); end >= 0 {
		span = span[:end]
	}
	if strings.Contains(span, "<b>") {
		t.Errorf("Android highlight must not bold the verse text:\n%s", span)
	}
}

func TestBuildChapterHTMLMixedParagraphs(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	// Pin PHONE typesetting explicitly. This test asserts the airy phone
	// paragraph grammar, and the platform default no longer implies it — the
	// host is darwin, and macOS now reads as the reporter page.
	orig := reporterLayout
	reporterLayout = func() bool { return false }
	defer func() { reporterLayout = orig }()

	// Enough prose to close the first paragraph (>=320 chars ending on a
	// sentence), then poetry: only the poetry paragraph is ragged-right.
	prose := "In the beginning God created the heavens and the earth. Now the earth was formless and void, and darkness was over the surface of the deep. And the Spirit of God was hovering over the surface of the waters. And God said, Let there be light, and there was light. And God saw that the light was good, and He separated the light from the darkness."
	bd := &BibleData{
		Books: []string{"Genesis"},
		Verses: map[string]map[int][]Verse{"Genesis": {1: {
			{BookName: "Genesis", Book: "Genesis", Chapter: 1, Verse: 1, Text: prose},
			{BookName: "Genesis", Book: "Genesis", Chapter: 1, Verse: 27,
				Text: "So God created man in His own image;\nin the image of God He created him;\nmale and female He created them."},
		}}},
	}
	st := &AppState{Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1}
	html := buildChapterHTML(st, bd.GetChapter("Genesis", 1))

	if !strings.Contains(html, `<p><sup class="v">1</sup>`) {
		t.Errorf("prose paragraph must stay plain:\n%s", html)
	}
	if !strings.Contains(html, `<p class="pm"><sup class="v">27</sup>`) {
		t.Errorf("poetry paragraph must be pm:\n%s", html)
	}
}

func TestBuildChapterHTMLReporterIndent(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	orig := reporterLayout
	reporterLayout = func() bool { return true }
	defer func() { reporterLayout = orig }()

	const indent = "  " // the em+en pair, as characters post-entity

	// All-poetic paragraph: no first-line indent (print poetry is unindented).
	st := psalm23State()
	html := buildChapterHTML(st, st.Bible.GetChapter("Psalms", 23))
	if strings.Contains(html, `<p class="pm">&#8195;&#8194;`) || strings.Contains(html, `<p class="pm">`+indent) {
		t.Errorf("a paragraph opening on a poem line must not be indented:\n%s", html)
	}

	// Mixed paragraph OPENING with prose: the indent is reporter mode's only
	// paragraph-boundary marker and must survive.
	bd := &BibleData{
		Books: []string{"Exodus"},
		Verses: map[string]map[int][]Verse{"Exodus": {15: {
			{BookName: "Exodus", Book: "Exodus", Chapter: 15, Verse: 1,
				Text: "Then Moses and the Israelites sang this song to the LORD:"},
			{BookName: "Exodus", Book: "Exodus", Chapter: 15, Verse: 2,
				Text: "The LORD is my strength and my song,\nand He has become my salvation."},
		}}},
	}
	st2 := &AppState{Bible: bd, CurrentBook: "Exodus", CurrentChapter: 15}
	html2 := buildChapterHTML(st2, bd.GetChapter("Exodus", 15))
	if !strings.Contains(html2, `<p class="pm">&#8195;&#8194;`) {
		t.Errorf("a mixed paragraph opening with prose must keep its indent:\n%s", html2)
	}
}
