package bibletext

// The fallback page's selections, from the page Go builds to the answer Go
// gives. When a chapter's HTML will not import, the Mac and the iPhone and
// iPad show the page reading_apple_fallback.go builds. A selection there was
// handed over with no span, because the page had no verse numbers the panes
// could find, and every verb searched the whole chapter for the words. The
// page now carries runs that the panes set in the numeral's face, so their
// verse lookups answer as on an imported page.
//
// The host cannot run the panes, so fallbackPane models the part of them a
// selection crosses, in the page's UTF-16 units: the clamp at verse 1's
// number (btIOSContentStart, hbScriptureSelection), the verse at a character
// (btIOSVerseAtIndex, btMacVerseSpanForRange) and the lead
// (docs/SELECTION_OFFSET.md), handed over as nativeSelSpan reads them. The
// faces it takes for granted, that a numeral run is what the walks find, are
// held at the source by reading_native_fallback_contract_test.go. The same
// model over the page with its runs taken away is the page as it was, and
// each test shows that page giving the wrong answer the runs correct.

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"
)

// fallbackPane is the selection side of either Apple pane over a fallback
// page.
type fallbackPane struct {
	units []uint16
	nums  []appleFallbackRun // the verse numbers the walks find, in page order
	verse []int              // and the verse each one numbers
}

func newFallbackPane(p appleFallbackPage) fallbackPane {
	fp := fallbackPane{units: pageUnits(p)}
	for _, r := range p.runs {
		if r.kind != appleFallbackNumeral {
			continue
		}
		// integerValue: a run that does not read as a positive number is
		// passed over, as both panes pass it over.
		v, err := strconv.Atoi(unitText(fp.units, r.start, r.length))
		if err != nil || v <= 0 {
			continue
		}
		fp.nums = append(fp.nums, r)
		fp.verse = append(fp.verse, v)
	}
	return fp
}

// contentStart is verse 1's number, or 0 when the page has none.
func (fp fallbackPane) contentStart() int {
	if len(fp.nums) == 0 {
		return 0
	}
	return fp.nums[0].start
}

// verseAt is the verse whose number is the last at or before unit ci, and
// where that number starts; 0 above the first.
func (fp fallbackPane) verseAt(ci int) (verse, loc int) {
	for i, r := range fp.nums {
		if r.start > ci {
			break
		}
		verse, loc = fp.verse[i], r.start
	}
	return verse, loc
}

// dispatch is what the pane hands Go for a selection of units
// [loc, loc+n): its text and span, or ok false where the pane offers the
// system's verbs alone (a selection wholly above verse 1's number).
func (fp fallbackPane) dispatch(loc, n int) (text string, span selSpan, ok bool) {
	if n <= 0 || loc < 0 || loc+n > len(fp.units) {
		return "", selSpan{}, false
	}
	cs := fp.contentStart()
	if loc+n <= cs {
		return "", selSpan{}, false
	}
	if loc < cs {
		n -= cs - loc
		loc = cs
	}
	text = unitText(fp.units, loc, n)
	lo, loLoc := fp.verseAt(loc)
	hi, _ := fp.verseAt(loc + n - 1)
	if lo > 0 && loLoc <= loc {
		return text, selSpanWithLead(lo, hi, unitText(fp.units, loLoc, loc-loLoc)), true
	}
	return text, selSpanFromNative(lo, hi), true
}

// withoutRuns is the page as it was shown before it carried its runs.
func withoutRuns(p appleFallbackPage) appleFallbackPage { return appleFallbackPage{text: p.text} }

// pageVerse is where a verse stands on the page, found by the page's text
// alone, so that where a test selects does not depend on the runs it tests:
// the unit offset of the verse's number, and of the end of its text.
type pageVerse struct{ verse, number, end int }

// versesByText finds each verse of st's chapter on page p: its number, a
// space and the start of its text, each after the one before.
func versesByText(t *testing.T, st *AppState, p appleFallbackPage) []pageVerse {
	t.Helper()
	units := pageUnits(p)
	var out []pageVerse
	at := 0
	for _, v := range st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter) {
		first := []rune(strings.SplitN(v.Text, "\n", 2)[0])
		key := utf16.Encode([]rune(strconv.Itoa(v.Verse) + " " + string(first[:min(len(first), 12)])))
		i := at
		for ; i+len(key) <= len(units); i++ {
			if string(utf16.Decode(units[i:i+len(key)])) == string(utf16.Decode(key)) {
				break
			}
		}
		if i+len(key) > len(units) {
			t.Fatalf("verse %d is not on the page", v.Verse)
		}
		if len(out) > 0 {
			out[len(out)-1].end = i
		}
		out = append(out, pageVerse{verse: v.Verse, number: i, end: len(units)})
		at = i + len(key)
	}
	return out
}

// line is where line `line` (from 0) of verse pv's text stands, in units,
// without the line break that ends it, a poem line's line separator or a
// paragraph's line feed; ok false past its last line.
func (pv pageVerse) line(units []uint16, line int) (loc, n int, ok bool) {
	at := pv.number + len(strconv.Itoa(pv.verse)) + 1 // past the number and its space
	for k := 0; at < pv.end; k++ {
		stop := at
		for stop < pv.end && units[stop] != '\n' && units[stop] != 0x2028 {
			stop++
		}
		if k == line {
			return at, stop - at, stop > at
		}
		at = stop + 1
	}
	return 0, 0, false
}

// fallbackVerseLine is line `line` of verse v on p, found by text.
func fallbackVerseLine(t *testing.T, st *AppState, p appleFallbackPage, v, line int) (loc, n int) {
	t.Helper()
	for _, pv := range versesByText(t, st, p) {
		if pv.verse == v {
			if loc, n, ok := pv.line(pageUnits(p), line); ok {
				return loc, n
			}
		}
	}
	t.Fatalf("verse %d has no line %d on the page", v, line)
	return 0, 0
}

// The World English Bible's Psalm 136, verses 1 to 11, a poem line to a
// line: the refrain closes every verse, the first copy with a semicolon is
// verse 3's.
func psalm136State() *AppState {
	lines := [][2]string{
		{"Give thanks to the LORD, for he is good,", "for his loving kindness endures forever."},
		{"Give thanks to the God of gods,", "for his loving kindness endures forever."},
		{"Give thanks to the Lord of lords,", "for his loving kindness endures forever;"},
		{"to him who alone does great wonders,", "for his loving kindness endures forever;"},
		{"to him who by understanding made the heavens,", "for his loving kindness endures forever;"},
		{"to him who spread out the earth above the waters,", "for his loving kindness endures forever;"},
		{"to him who made the great lights,", "for his loving kindness endures forever;"},
		{"the sun to rule by day,", "for his loving kindness endures forever;"},
		{"the moon and stars to rule by night,", "for his loving kindness endures forever;"},
		{"to him who struck down the Egyptian firstborn,", "for his loving kindness endures forever;"},
		{"and brought out Israel from among them,", "for his loving kindness endures forever;"},
	}
	var vs []Verse
	for i, l := range lines {
		vs = append(vs, Verse{BookName: "Psalms", Book: "Psalms", Chapter: 136, Verse: i + 1, Text: l[0] + "\n" + l[1]})
	}
	bd := &BibleData{Books: []string{"Psalms"}, Verses: map[string]map[int][]Verse{"Psalms": {136: vs}}}
	return &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 136}
}

// The World English Bible's Genesis 1:1–5: "light" is in verses 3, 4 and 5.
func genesis1State() *AppState {
	texts := []string{
		"In the beginning, God created the heavens and the earth.",
		"The earth was formless and empty. Darkness was on the surface of the deep and God’s Spirit was hovering over the surface of the waters.",
		"God said, “Let there be light,” and there was light.",
		"God saw the light, and saw that it was good. God divided the light from the darkness.",
		"God called the light “day”, and the darkness he called “night”. There was evening and there was morning, the first day.",
	}
	var vs []Verse
	for i, s := range texts {
		vs = append(vs, Verse{BookName: "Genesis", Book: "Genesis", Chapter: 1, Verse: i + 1, Text: s})
	}
	bd := &BibleData{Books: []string{"Genesis"}, Verses: map[string]map[int][]Verse{"Genesis": {1: vs}}}
	return &AppState{Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1}
}

// fallbackAnswer is every verb's answer for one selection: the share's
// quotation and citation, the verses a link or a note is kept against, the
// verses the cross-references are drawn from, and the AI request's passage.
type fallbackAnswer struct{ quote, cite, link, xrefs, ai string }

func fallbackAnswerFor(st *AppState, text string, span selSpan) fallbackAnswer {
	var a fallbackAnswer
	a.quote, a.cite, _, _ = prepareShareQuote(st, text, span)
	lo, hi := linkVersesForSelectionIn(st, st.CurrentBook, st.CurrentChapter, text, span)
	a.link = fmt.Sprintf("%d-%d", lo, hi)
	var vs []string
	for _, v := range selectionVersesIn(st, st.CurrentBook, st.CurrentChapter, text, span) {
		vs = append(vs, strconv.Itoa(v.Verse))
	}
	a.xrefs = strings.Join(vs, ",")
	a.ai = aiPassageFor(st, text, span).Ref
	return a
}

// Psalm 136:10's refrain, selected on the fallback page, is cited 136:10 by
// every verb, as on an imported page. The page without its runs, which is
// how it was shown, sent no span, and every verb cited the first copy of the
// line with a semicolon, verse 3's.
func TestOnTheFallbackPageARefrainIsCitedFromTheVerseItWasSelectedIn(t *testing.T) {
	st := psalm136State()
	p := fallbackPageFor(st)
	loc, n := fallbackVerseLine(t, st, p, 10, 1)
	for _, tc := range []struct {
		page appleFallbackPage
		want fallbackAnswer
	}{
		{p, fallbackAnswer{cite: "Psalms 136:10", link: "10-10", xrefs: "10", ai: "Psalms 136:10"}},
		{withoutRuns(p), fallbackAnswer{cite: "Psalms 136:3", link: "3-3", xrefs: "3", ai: "Psalms 136:3"}},
	} {
		fp := newFallbackPane(tc.page)
		text, span, ok := fp.dispatch(loc, n)
		if !ok || text != "for his loving kindness endures forever;" {
			t.Fatalf("runs=%d: the selection is %q (offered %v)", len(tc.page.runs), text, ok)
		}
		got := fallbackAnswerFor(st, text, span)
		if !strings.Contains(got.quote, "for his loving kindness endures forever") {
			t.Errorf("runs=%d: the line is quoted as %q", len(tc.page.runs), got.quote)
		}
		got.quote = ""
		if got != tc.want {
			t.Errorf("runs=%d: span %+v answers %+v, want %+v", len(tc.page.runs), span, got, tc.want)
		}
	}
}

// "light" double-clicked in Genesis 1:5 on the fallback page is cited 1:5.
// Without the runs it was cited 1:3, the first verse with the word.
func TestOnTheFallbackPageAWordIsCitedFromTheVerseItWasSelectedIn(t *testing.T) {
	st := genesis1State()
	p := fallbackPageFor(st)
	loc, n := fallbackVerseLine(t, st, p, 5, 0)
	line := unitText(pageUnits(p), loc, n)
	at := loc + len(utf16.Encode([]rune(line[:strings.Index(line, "light")])))
	for _, tc := range []struct {
		page appleFallbackPage
		cite string
	}{{p, "Genesis 1:5"}, {withoutRuns(p), "Genesis 1:3"}} {
		fp := newFallbackPane(tc.page)
		text, span, ok := fp.dispatch(at, len("light"))
		if !ok || text != "light" {
			t.Fatalf("runs=%d: the selection is %q", len(tc.page.runs), text)
		}
		if _, cite, _, _ := prepareShareQuote(st, text, span); cite != tc.cite {
			t.Errorf("runs=%d: \"light\" in verse 5 with span %+v is cited %s, want %s", len(tc.page.runs), span, cite, tc.cite)
		}
	}
}

// Every word of both chapters, double-clicked on the fallback page, is cited
// from the verse it stands in. Without the runs a share of the same words
// cited another verse for many of them.
func TestOnTheFallbackPageEveryWordIsCitedFromItsOwnVerse(t *testing.T) {
	for _, st := range []*AppState{psalm136State(), genesis1State()} {
		p := fallbackPageFor(st)
		units := pageUnits(p)
		verses := versesByText(t, st, p) // where each word really stands
		wrong := func(page appleFallbackPage) (words, wrong int, example string) {
			fp := newFallbackPane(page)
			for _, pv := range verses {
				v := pv.verse
				for line := 0; ; line++ {
					loc, n, ok := pv.line(units, line)
					if !ok {
						break
					}
					text := unitText(units, loc, n)
					off := 0
					for _, w := range strings.Fields(text) {
						j := strings.Index(text[off:], w) + off
						at := loc + len(utf16.Encode([]rune(text[:j])))
						off = j + len(w)
						sel, span, offered := fp.dispatch(at, len(utf16.Encode([]rune(w))))
						if !offered {
							continue
						}
						words++
						want := verseRangeCitation(st.CurrentBook, st.CurrentChapter, v, v)
						if _, cite, _, _ := prepareShareQuote(st, sel, span); cite != want {
							wrong++
							if example == "" {
								example = fmt.Sprintf("%q in verse %d cited %s", sel, v, cite)
							}
						}
					}
				}
			}
			return words, wrong, example
		}
		words, bad, ex := wrong(p)
		if words == 0 || bad != 0 {
			t.Errorf("%s %d: %d of %d words cited from another verse with the runs (%s)", st.CurrentBook, st.CurrentChapter, bad, words, ex)
		}
		words, bad, ex = wrong(withoutRuns(p))
		t.Logf("%s %d without the runs: %d of %d words cited from another verse (%s)", st.CurrentBook, st.CurrentChapter, bad, words, ex)
		if bad == 0 {
			t.Errorf("%s %d: no word is cited from another verse without the runs; the sweep proves nothing", st.CurrentBook, st.CurrentChapter)
		}
	}
}

// A psalm's title on the fallback page is cut off as on an imported page:
// a selection inside it is offered the system's verbs alone, and a drag
// from it into the psalm is cut at verse 1's number, so the share quotes the
// psalm's words and cites verse 1. Without the runs the page offered the app
// verbs on the title and handed the title's words over with no span.
func TestOnTheFallbackPageATitleIsCutAtVerseOne(t *testing.T) {
	st := psalmTitleState(true)
	p := fallbackPageFor(st)
	title := st.Bible.SuperscriptionFor("Psalms", 3).DrawnText()
	if !strings.HasPrefix(p.text, title+"\n") {
		t.Fatalf("the page does not open with its title: %q", p.text[:min(len(p.text), 80)])
	}
	titleUnits := len(utf16.Encode([]rune(title)))
	after, before := newFallbackPane(p), newFallbackPane(withoutRuns(p))
	if _, _, ok := after.dispatch(0, titleUnits); ok {
		t.Error("the title selected whole is offered the app's verbs")
	}
	if _, span, ok := before.dispatch(0, titleUnits); !ok || span.valid() {
		t.Fatalf("control: without the runs the title is not handed over with no span (offered %v, span %+v)", ok, span)
	}
	loc, n := fallbackVerseLine(t, st, p, 1, 0)
	end := loc + len(utf16.Encode([]rune("Yahweh, how my adversaries have increased!")))
	if unitText(after.units, loc, end-loc) != "Yahweh, how my adversaries have increased!" || n < end-loc {
		t.Fatalf("verse 1's first line is not where the test looks: %q", unitText(after.units, loc, n))
	}
	text, span, ok := after.dispatch(0, end)
	if !ok || !strings.HasPrefix(text, "1 Yahweh") || span.lo != 1 || span.hi != 1 || !span.hasLead || span.lead != "" {
		t.Fatalf("a drag from the title into verse 1 is handed over as %q, span %+v", text, span)
	}
	quote, cite, _, _ := prepareShareQuote(st, text, span)
	if cite != "Psalms 3:1" || strings.Contains(quote, "Absalom") || strings.Contains(quote, psalmHeading) || !strings.Contains(quote, "Yahweh, how my adversaries have increased!") {
		t.Errorf("a drag from the title into verse 1 is shared as (%q, %q)", quote, cite)
	}
}
