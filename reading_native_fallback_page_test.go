package bibletext

// The fallback page, as Go builds it (reading_apple_fallback.go) and checked
// here on the host, where the panes' Objective-C cannot run. When a chapter's
// HTML will not import, the Mac and the iPhone and iPad set this page in the
// faces its runs name and install it as an imported page is
// (btIOSFallbackPage, btMacFallbackPage); reading_native_fallback_contract_test.go
// holds the panes to that. These tests hold the page to the chapter's text,
// in the paragraphs and line breaks the importer gives it, and its runs to
// where the verse numbers, the title and the headings stand, in the UTF-16
// units the panes count in.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"fyne.io/fyne/v2/test"
)

// fallbackPageFor is the page the panes would show for st's chapter.
func fallbackPageFor(st *AppState) appleFallbackPage {
	return appleFallbackPageOf(buildChapterHTML(st, st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter)))
}

// pageUnits is the page as NSString holds it.
func pageUnits(p appleFallbackPage) []uint16 { return utf16.Encode([]rune(p.text)) }

// unitText is the text of units [start, start+length), read as the panes
// read a run; a range outside the page reads as a marker no page holds.
func unitText(units []uint16, start, length int) string {
	if start < 0 || length < 0 || start+length > len(units) {
		return "\x00out of range"
	}
	return string(utf16.Decode(units[start : start+length]))
}

// fallbackVerseNumbers checks that each of the first n verse numbers stands
// on the page as a word of its own, after a space, a line feed or a line
// separator, and says which does not.
func fallbackVerseNumbers(page string, n int) error {
	pos := 0
	for v := 1; v <= n; v++ {
		num := strconv.Itoa(v) + " "
		i := strings.Index(page[pos:], num)
		if i < 0 {
			return fmt.Errorf("verse %d's number is not on the page as a word", v)
		}
		i += pos
		if before := page[:i]; before != "" && !strings.HasSuffix(before, " ") && !strings.HasSuffix(before, "\n") &&
			!strings.HasSuffix(before, appleFallbackLineSeparator) {
			return fmt.Errorf("verse %d's number is joined to what stands before it: %q", v, page[max(0, i-20):i+len(num)])
		}
		pos = i + len(num)
	}
	return nil
}

// flatLines is s with its line feeds and line separators read as spaces,
// as a verse's text stands in the chapter's prose.
func flatLines(s string) string {
	return strings.NewReplacer("\n", " ", appleFallbackLineSeparator, " ").Replace(s)
}

// The page is the chapter's text and nothing else: the title, a heading,
// and the verses each stand apart, a line feed after each paragraph, so
// verse 1's number arrives as a word of its own, and no rule of the
// stylesheet or entity of the markup is left in it, while an entity the
// text itself spells out stands as spelled. The controls show the check sees
// each of those go wrong.
func TestTheFallbackPageIsTheChaptersTextAndNothingElse(t *testing.T) {
	const spelled = `Its marks are spelled &quot;, &lt;b&gt;, &nbsp; and &amp;.`
	st := psalmTitleState(true)
	st.Bible.Verses["Psalms"][3][2].Text = `But you, Yahweh, are a "shield" around me, my glory & my <help>. ` + spelled
	_, spans := chapterProse(st)
	title := collapseSpaces(st.Bible.SuperscriptionFor("Psalms", 3).DrawnText())
	check := func(page string) error {
		want := title + "\n" + psalmHeading + "\n1 "
		if !strings.HasPrefix(page, want) {
			return fmt.Errorf("the page opens %q, want %q", page[:min(len(page), len(want)+20)], want)
		}
		if !strings.Contains(page, `my glory & my <help>.`) || !strings.Contains(page, `"shield"`) || strings.ContainsAny(page, "{}") {
			return fmt.Errorf("the page carries markup or the stylesheet: %q", page)
		}
		if !strings.Contains(page, spelled) {
			return fmt.Errorf("the text's own spelling of an entity is not on the page as spelled: %q", page)
		}
		return fallbackVerseNumbers(page, len(spans))
	}
	page := fallbackPageFor(st).text
	if err := check(page); err != nil {
		t.Error(err)
	}
	readTwice := strings.NewReplacer("&quot;", `"`, "&lt;", "<", "&gt;", ">", "&nbsp;", " ", "&amp;", "&").Replace(spelled)
	for _, bad := range []struct{ what, page string }{
		{"the title run into the heading", strings.Replace(page, title+"\n", title, 1)},
		{"the stylesheet left on it", "p { margin: 0 }\n" + page},
		{"a quotation mark left as its entity", strings.Replace(page, `"shield"`, `&quot;shield&quot;`, 1)},
		{"the text's own entities read twice", strings.Replace(page, spelled, readTwice, 1)},
		{"verse 2's number joined to verse 1", strings.Replace(page, appleFallbackLineSeparator+"2 ", "2 ", 1)},
	} {
		if bad.page == page || check(bad.page) == nil {
			t.Fatalf("the check passes the page with %s; it proves nothing", bad.what)
		}
	}
}

// The runs say where the title, the heading and each verse number stand, in
// UTF-16 units: the panes set those runs in the title's, the heading's and
// the numeral's faces, and find the verses by them. The fixture puts a
// character outside the Basic Multilingual Plane in verse 1 and curly quotes
// in verse 2, so a run counted in bytes or in runes lands elsewhere, which
// the control shows the check can tell.
func TestTheFallbackPageRecordsWhereEachVerseNumberTitleAndHeadingStand(t *testing.T) {
	st := psalmTitleState(true)
	st.Bible.Verses["Psalms"][3][0].Text = "Yahweh, how my adversaries have increased! \U0001D11E\nMany are those who rise up against me."
	p := fallbackPageFor(st)
	corpus, spans := chapterProse(st)
	title := st.Bible.SuperscriptionFor("Psalms", 3).DrawnText()

	check := func(p appleFallbackPage) error {
		units := pageUnits(p)
		if len(p.runs) != 2+len(spans) {
			return fmt.Errorf("%d runs, want the title, the heading and %d numbers: %+v", len(p.runs), len(spans), p.runs)
		}
		if r := p.runs[0]; r.kind != appleFallbackTitle || unitText(units, r.start, r.length) != title {
			return fmt.Errorf("the first run is %+v %q, want the title %q", r, unitText(units, r.start, r.length), title)
		}
		if r := p.runs[1]; r.kind != appleFallbackHeading || unitText(units, r.start, r.length) != psalmHeading {
			return fmt.Errorf("the second run is %+v %q, want the heading %q", r, unitText(units, r.start, r.length), psalmHeading)
		}
		for i, sp := range spans {
			r := p.runs[2+i]
			if r.kind != appleFallbackNumeral || unitText(units, r.start, r.length) != strconv.Itoa(sp.verse) {
				return fmt.Errorf("run %d is %+v %q, want verse %d's number", 2+i, r, unitText(units, r.start, r.length), sp.verse)
			}
			// The verse's text follows its number and a space, as far as
			// the next number, its poem lines broken as the importer
			// breaks them.
			end := len(units)
			if 3+i < len(p.runs) {
				end = p.runs[3+i].start
			}
			body := unitText(units, r.start+r.length, end-r.start-r.length)
			if !strings.HasPrefix(body, " ") || flatLines(strings.TrimSpace(body)) != corpus[sp.start:sp.end] {
				return fmt.Errorf("verse %d's number is followed by %q, not its text", sp.verse, body)
			}
		}
		return nil
	}
	if err := check(p); err != nil {
		t.Fatal(err)
	}
	// CONTROL: the same runs counted in bytes put verses 2 and 3 elsewhere.
	inBytes := appleFallbackPage{text: p.text, runs: append([]appleFallbackRun(nil), p.runs...)}
	moved := 0
	for i, r := range inBytes.runs {
		prefix := string(utf16.Decode(pageUnits(p)[:r.start]))
		if len(prefix) != r.start {
			inBytes.runs[i].start = len(prefix)
			moved++
		}
	}
	if moved == 0 || check(inBytes) == nil {
		t.Fatalf("runs counted in bytes pass the check (%d moved); it proves nothing", moved)
	}
}

// The page ends where the translators' notes begin: the panes find the end
// of Scripture on an imported page by the notes' size, which this page does
// not set, so a note on it would be taken for Scripture by every verb. The
// control is the same HTML with its notes' opening renamed, which the walk
// then reads through.
func TestTheFallbackPageEndsBeforeTheTranslatorsNotes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)
	verses := footnoteFixtureVerses()
	st := footnoteFixtureState(verses)
	html := buildChapterHTML(st, verses)
	const note = "Or God loved the world in this way."
	if !strings.Contains(html, appleFallbackCut) || !strings.Contains(html, note) {
		t.Fatalf("the fixture's HTML carries no notes section to cut:\n%s", html)
	}
	p := appleFallbackPageOf(html)
	if strings.Contains(p.text, note) || !strings.HasSuffix(p.text, "Whoever believes in him is not condemned.\n") {
		t.Errorf("the page does not end with the last verse: %q", p.text)
	}
	if n := len(p.runs); n != 3 {
		t.Errorf("%d runs, want the three verse numbers: %+v", n, p.runs)
	}
	renamed := appleFallbackPageOf(strings.Replace(html, appleFallbackCut, `<p class="fnsep-renamed">`, 1))
	if !strings.Contains(renamed.text, note) {
		t.Fatal("the notes are not on the page even with their opening renamed; the check proves nothing")
	}
}

// nsParagraphOrdinal is how many paragraphs end before unit at of units, as
// NSString ends them (paragraphRangeForRange:): at a line feed, a carriage
// return, the two together, and U+2029 (PARAGRAPH SEPARATOR). A line
// separator, U+2028, breaks a line inside a paragraph.
func nsParagraphOrdinal(units []uint16, at int) int {
	n := 0
	for i := 0; i < at && i < len(units); i++ {
		switch units[i] {
		case '\n':
			if i == 0 || units[i-1] != '\r' {
				n++
			}
		case '\r', 0x2029:
			n++
		}
	}
	return n
}

// Each verse number stands in the paragraph its <p> makes, as on an
// imported page: the importer ends a paragraph at each </p> and nowhere
// else, and writes a <br> as U+2028, which NSString does not end a paragraph
// at. Both panes reserve a note's band, place its sticker, card and pills,
// and test which notes share a paragraph, by the paragraph that holds the
// anchor verse's number, so a page that ends a paragraph at each poem line
// opens a note between two lines of a stanza. Psalm 136 is one <p> of
// verses of two poem lines each, Psalm 3 has a title and a heading above
// its verses, and Genesis 1 is prose. The control is the same page with its
// line separators made line feeds, as the page was first built.
func TestTheFallbackPageEndsAParagraphWhereTheImportedPageDoes(t *testing.T) {
	numberSup := regexp.MustCompile(`<sup class="v(?: [^"]*)?">`)
	controlled := false
	for _, st := range []*AppState{psalm136State(), psalmTitleState(true), genesis1State()} {
		name := fmt.Sprintf("%s %d", st.CurrentBook, st.CurrentChapter)
		html := buildChapterHTML(st, st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter))
		if i := strings.Index(html, appleFallbackCut); i >= 0 {
			html = html[:i]
		}
		var want []int // the paragraph each verse number's <p> is, from 0
		for _, m := range numberSup.FindAllStringIndex(html, -1) {
			want = append(want, strings.Count(html[:m[0]], "</p>"))
		}
		wantEnds := strings.Count(html, "</p>")
		if len(want) != len(st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter)) {
			t.Fatalf("%s: %d verse numbers found in the HTML, want one per verse", name, len(want))
		}
		paragraphs := func(p appleFallbackPage) (got []int, ends int) {
			units := pageUnits(p)
			for _, r := range p.runs {
				if r.kind == appleFallbackNumeral {
					got = append(got, nsParagraphOrdinal(units, r.start))
				}
			}
			return got, nsParagraphOrdinal(units, len(units))
		}
		p := fallbackPageFor(st)
		if got, ends := paragraphs(p); fmt.Sprint(got) != fmt.Sprint(want) || ends != wantEnds {
			t.Errorf("%s: the verse numbers stand in paragraphs %v of the page's %d; the imported page's are %v of %d, one to a </p>",
				name, got, ends, want, wantEnds)
		}
		if !strings.Contains(html, "<br>") {
			continue // prose: no poem line to break
		}
		controlled = true
		asFeeds := appleFallbackPage{text: strings.ReplaceAll(p.text, appleFallbackLineSeparator, "\n"), runs: p.runs}
		if got, ends := paragraphs(asFeeds); fmt.Sprint(got) == fmt.Sprint(want) && ends == wantEnds {
			t.Fatalf("%s: the page with a line feed at each poem line passes; the check proves nothing", name)
		}
	}
	if !controlled {
		t.Fatal("no fixture has a poem line, so the control never ran")
	}
}

// fallbackPsalmTitlePage is psalmTitleState's chapter as the fallback page
// lays it out, the title, the heading and each paragraph on lines of their
// own, with where each verse's number and text stand, found and checked
// against the chapter's prose. Its line separators are read as line feeds:
// the share pipeline takes both for white space alike
// (TestShareSelectionPoetryDisplayShape), and a line feed is one byte, as
// the prose's space is, so a page offset inside a verse's text maps to the
// prose by the sweep's arithmetic (sweepPsalmTitleDrags).
func fallbackPsalmTitlePage(t *testing.T, st *AppState, withHeading bool) psalmTitlePage {
	t.Helper()
	page := strings.ReplaceAll(fallbackPageFor(st).text, appleFallbackLineSeparator, "\n")
	title := collapseSpaces(st.Bible.SuperscriptionFor(st.CurrentBook, st.CurrentChapter).DrawnText())
	if !strings.HasPrefix(page, title+"\n") {
		t.Fatalf("the page does not open with the title on a line of its own: %q", page[:min(len(page), 120)])
	}
	pos := len(title) + 1
	if withHeading {
		if !strings.HasPrefix(page[pos:], psalmHeading+"\n") {
			t.Fatalf("the heading is not on a line of its own under the title: %q", page[pos:min(len(page), pos+80)])
		}
		pos += len(psalmHeading) + 1
	}
	corpus, spans := chapterProse(st)
	p := psalmTitlePage{page: page, titleEnd: len(title), number: make([]int, len(spans)), text: make([]int, len(spans))}
	for i, sp := range spans {
		num := strconv.Itoa(sp.verse) + " "
		if !strings.HasPrefix(page[pos:], num) {
			t.Fatalf("verse %d's number is not where its text begins: %q", sp.verse, page[pos:min(len(page), pos+40)])
		}
		p.number[i], p.text[i] = pos, pos+len(num)
		end := p.text[i] + sp.end - sp.start
		if end > len(page) || strings.ReplaceAll(page[p.text[i]:end], "\n", " ") != corpus[sp.start:sp.end] {
			t.Fatalf("the page does not hold verse %d's text after its number", sp.verse)
		}
		pos = end
		if pos < len(page) && (page[pos] == ' ' || page[pos] == '\n') {
			pos++
		}
	}
	return p
}

// The share's rules for a selection with no span, on a psalm's title laid
// out as the fallback page lays it out: a drag from the title into the psalm is
// answered as TestPsalmTitleDragSweep answers it with a span, the psalm's
// words alone, cited by their verses, and the title never quoted. The
// fallback page used to hand such drags over with no span; it now cuts them
// at verse 1's number, as an imported page does
// (TestOnTheFallbackPageATitleIsCutAtVerseOne), and the rules stay for any
// selection that still arrives without one.
func TestADragFromATitleWithNoSpanSharesThePsalmsWordsAlone(t *testing.T) {
	for _, withHeading := range []bool{false, true} {
		st := psalmTitleState(withHeading)
		p := fallbackPsalmTitlePage(t, st, withHeading)
		name := fmt.Sprintf("heading=%v", withHeading)
		drags, copies := sweepPsalmTitleDrags(t, st, p, func(int) selSpan { return selSpan{} }, name)
		t.Logf("%s: %d drags, %d of them inside the title and quoted from the psalm", name, drags, copies)
	}
}

// What the walk reads of the markup beyond the chapter's own: a "<" with no
// ">" after it, and an empty "<>", stay as text, as a tag-stripping
// expression leaves them; a closing tag and a tag with attributes are read
// by name; and an unknown entity stays as spelled.
func TestTheFallbackWalkReadsTagsByNameAndLeavesStrayMarksAsText(t *testing.T) {
	for _, tc := range []struct{ html, text string }{
		{`<p>a <> b</p>`, "a <> b\n"},
		{`<p>a &copy; b</p>`, "a &copy; b\n"},
		{`<p>tail <`, "tail <"},
		{`<p class="sec">Head</p><p><sup class="v hl">7</sup><span class="hl"> </span>x<br/>y</p>`, "Head\n7 x\u2028y\n"},
	} {
		if got := appleFallbackPageOf(tc.html).text; got != tc.text {
			t.Errorf("%q: the page is %q, want %q", tc.html, got, tc.text)
		}
	}
	p := appleFallbackPageOf(`<p class="sec">Head</p><p><sup class="v hl">7</sup><span class="hl"> </span>x</p>`)
	want := []appleFallbackRun{{appleFallbackHeading, 0, 4}, {appleFallbackNumeral, 5, 1}}
	if fmt.Sprint(p.runs) != fmt.Sprint(want) {
		t.Errorf("the runs of a washed verse under a heading are %+v, want %+v", p.runs, want)
	}
	// A gap mark is drawn at the numeral's size on an imported page, but it is
	// not a number: [21] reads as 0, so no run is recorded for it.
	if p := appleFallbackPageOf(`<p><span class="vg">[21]</span> <sup class="v">22</sup> x</p>`); len(p.runs) != 1 || p.runs[0].start != 5 {
		t.Errorf("a gap mark before a number gives the runs %+v, want the number's alone at 5", p.runs)
	}
}
