package bibletext

// THE SHAPE OF THE BAND — what the native panes must paint, checked against what
// buildChapterHTML actually puts inside `.hl`.
//
// The Apple panes paint a wash twice by two different routes. On a rebuild the
// HTML importer reads `background-color` off the `.hl` spans; on a wash-only
// change the live mutation computes a CHARACTER RANGE from verse numbers and
// paints that (btIOSChapterWashRange / btMacChapterWashRange). Those two have to
// cover exactly the same characters, or a verse the mutation repainted comes back
// a different SHAPE from its neighbours — permanently, because nothing rebuilds
// after a repaint.
//
// They did not. A verse's character range runs from its own number to the NEXT
// verse's number, so for every verse but a run's last it swallowed whatever sits
// between the two — and buildChapterHTML deliberately leaves three of those
// things bare: the poetic line separator (`</span><br><sup class="v hl">`), the
// paragraph boundary (`</p><p>`), and the reporter layout's first-line indent
// (`&#8195;&#8194;`). TextKit paints a background on a break character all the way
// out to the right margin, so a multi-verse band grew a full-width gold tail on
// every interior verse and a gold block on the next paragraph's indent. Only the
// join SPACE between two verses under the same wash belongs inside the band, and
// that one buildChapterHTML puts inside the span on purpose (tint.go's JoinSpace
// — written bare it punched a notch through the band at every mid-line join).
//
// So this test runs the native rule — in Go, over a model of what the importer
// produces — and asserts the character set it paints is EQUAL to the set the HTML
// marks up. It covers a multi-verse mark in prose, in poetry, and across a
// paragraph boundary in both the phone and the reporter layouts.
//
// It is a model of the importer, not the importer (that is what the captured
// pixels are for). What it pins exactly is the RULE, which is the half that was
// wrong: every case below also asserts that the un-trimmed rule — the one this
// replaced — would have painted MORE than the HTML does, so the test cannot pass
// vacuously.

import (
	"strconv"
	"strings"
	"testing"
	"unicode"

	"fyne.io/fyne/v2/test"
)

// --- A model of the imported attributed string --------------------------------

// importedChapter is what buildChapterHTML's output becomes on the other side of
// the HTML importer, reduced to the two things the wash rule needs: the plain
// characters, and which of them the `.hl` markup covers.
type importedChapter struct {
	text  []rune
	hl    []bool // parallel to text
	verse []verseAt
}

// verseAt is a verse number run: the character location the native side finds by
// font size (btIOSBuildVerseIndex / btMacLocForVerse), which is where the digits
// begin.
type verseAt struct {
	verse int
	loc   int
}

var htmlEntities = strings.NewReplacer(
	"&nbsp;", " ",
	"&#8195;", " ", // em space — the reporter indent's first half
	"&#8194;", " ", // en space — its second
	"&amp;", "&",
	"&lt;", "<",
	"&gt;", ">",
	"&quot;", `"`,
)

// importChapterHTML turns buildChapterHTML's output into the model above.
//
// Only the tags buildChapterHTML emits are understood, deliberately: an unknown
// tag is a fixture that has outgrown this parser, and failing loudly is better
// than silently mis-modelling the string the real rule runs against.
func importChapterHTML(t *testing.T, html string) importedChapter {
	t.Helper()
	body := html
	if i := strings.Index(body, "<body>"); i >= 0 {
		body = body[i+len("<body>"):]
	}
	body = strings.TrimSuffix(body, "</body></html>")

	var doc importedChapter
	var stack []bool // one entry per open element that could carry .hl
	inSup, supStart := false, 0
	paraSeen := false

	emit := func(s string) {
		on := false
		for _, v := range stack {
			on = on || v
		}
		for _, r := range htmlEntities.Replace(s) {
			doc.text = append(doc.text, r)
			doc.hl = append(doc.hl, on)
		}
	}

	// emitBare writes characters the importer never washes, whatever span they
	// are nested in.
	emitBare := func(str string) {
		for _, r := range htmlEntities.Replace(str) {
			doc.text = append(doc.text, r)
			doc.hl = append(doc.hl, false)
		}
	}

	for len(body) > 0 {
		i := strings.IndexByte(body, '<')
		if i < 0 {
			emit(body)
			break
		}
		if i > 0 {
			emit(body[:i])
		}
		j := strings.IndexByte(body[i:], '>')
		if j < 0 {
			t.Fatalf("unterminated tag in %q", body[i:])
		}
		tag := body[i : i+j+1]
		body = body[i+j+1:]

		switch {
		case strings.HasPrefix(tag, "<p"):
			// The importer makes each <p> a paragraph; paragraphs are separated
			// by a newline, which is exactly the character the old rule swallowed.
			if paraSeen {
				stack = append(stack, false)
				emit("\n")
				stack = stack[:len(stack)-1]
			}
			paraSeen = true
		case tag == "</p>":
			// nothing: the separator is written by the next <p>
		case tag == "<br>":
			// A BREAK IS BARE, even nested inside the .hl span. Measured against
			// the real AppKit importer rather than read off the markup:
			//
			//     <span class="hl">washed one<br>washed two</span>
			//     idx 16  BREAK(U+2028)  background=NONE
			//     idx 33  BREAK(U+000A)  background=NONE
			//
			// This line previously modelled it as INSIDE the band, taking its
			// answer from what buildChapterHTML emits. What the markup contains
			// and what the importer paints are different questions and only the
			// second decides pixels — so the model certified the very defect it
			// was written to catch, and every poetic verse the live mutation
			// repainted came back with a full-width tail.
			emitBare("\n")
		case strings.HasPrefix(tag, "<sup"):
			stack = append(stack, strings.Contains(tag, "hl"))
			inSup, supStart = true, len(doc.text)
		case tag == "</sup>":
			if inSup {
				num := 0
				for _, r := range doc.text[supStart:] {
					if r < '0' || r > '9' {
						num = 0
						break
					}
					num = num*10 + int(r-'0')
				}
				if num > 0 {
					doc.verse = append(doc.verse, verseAt{verse: num, loc: supStart})
				}
				inSup = false
			}
			stack = stack[:len(stack)-1]
		case strings.HasPrefix(tag, "<span"):
			stack = append(stack, strings.Contains(tag, `class="hl`) ||
				strings.Contains(tag, ` hl"`) || strings.Contains(tag, ` hl `))
		case tag == "</span>":
			stack = stack[:len(stack)-1]
		default:
			t.Fatalf("importChapterHTML: unhandled tag %q", tag)
		}
	}
	return doc
}

// --- The native rule, in Go ---------------------------------------------------

func (d importedChapter) locOf(verse int) int {
	for _, v := range d.verse {
		if v.verse == verse {
			return v.loc
		}
	}
	return -1
}

// endOf is where verse's own span stops: the next verse number, or the end of the
// text. The twin of btIOSReadAlongRange / btMacReadAlongRange.
func (d importedChapter) endOf(verse int) int {
	for i, v := range d.verse {
		if v.verse == verse {
			if i+1 < len(d.verse) {
				return d.verse[i+1].loc
			}
			return len(d.text)
		}
	}
	return -1
}

func isBreakRune(r rune) bool { return r == '\n' || r == '\r' || r == ' ' || r == ' ' }

// unwashBreaks is btIOSUnwashBreaks / btMacUnwashBreaks: a BREAK CHARACTER is
// never washed, wherever it sits.
//
// This replaced a narrower model — "the trailing whitespace before the next
// verse number, but only when it contains a break" — which was written from the
// HTML markup, where an intra-verse <br> really is nested inside the .hl span.
// What the markup CONTAINS and what the importer PAINTS are different
// questions. Measured:
//
//	<span class="hl">washed one<br>washed two</span>
//	idx 16  BREAK(U+2028)  background=NONE
//	idx 33  BREAK(U+000A)  background=NONE
//
// The narrow rule left every poetic verse the wrong shape on both panes, on a
// single-verse mark, and this model agreed with it — so the test certified the
// defect instead of catching it.
func unwashBreaks(text []rune, mask []bool, start, end int) {
	for i := start; i < end && i < len(mask); i++ {
		if !isLineBreakRune(text[i]) {
			continue
		}
		// The WHOLE whitespace run around the break, because the reporter
		// layout's first-line indent (EM SPACE + EN SPACE) is emitted by the
		// <p> that follows the break and is outside the span too. A bare join
		// space with no break in its run is INSIDE the band and stays.
		lo, hi := i, i
		for lo > start && unicode.IsSpace(text[lo-1]) {
			lo--
		}
		for hi < end && hi < len(text) && unicode.IsSpace(text[hi]) {
			hi++
		}
		for j := lo; j < hi; j++ {
			mask[j] = false
		}
		i = hi
	}
}

func isLineBreakRune(r rune) bool {
	return r == '\n' || r == '\r' || r == '\u2028' || r == '\u2029'
}

// washMask runs the native rule for a run of verses lo..hi and returns the
// characters it paints. trimBreaks=false reproduces the rule this replaced — the
// plain intersection of the run's span with each verse's span — so a test can
// show the difference is real.
func (d importedChapter) washMask(lo, hi int, trimBreaks bool) []bool {
	mask := make([]bool, len(d.text))
	start, end := d.locOf(lo), d.endOf(hi)
	if start < 0 || end < 0 {
		return mask
	}
	// btIOSRunWashRange: the run's span, trimmed of trailing whitespace.
	for end > start && unicode.IsSpace(d.text[end-1]) {
		end--
	}
	for v := lo; v <= hi; v++ {
		vs, ve := d.locOf(v), d.endOf(v)
		if vs < 0 {
			continue
		}
		if vs < start {
			vs = start
		}
		if ve > end {
			ve = end
		}
		if vs >= ve {
			continue
		}
		for i := vs; i < ve; i++ {
			mask[i] = true
		}
	}
	// ...then every break character taken back out, which is the whole rule.
	if trimBreaks {
		unwashBreaks(d.text, mask, start, end)
	}
	return mask
}

func maskDiff(d importedChapter, got, want []bool) string {
	var b strings.Builder
	for i := range want {
		if got[i] == want[i] {
			continue
		}
		verb := "painted but bare in the HTML"
		if want[i] {
			verb = "left bare but inside .hl"
		}
		b.WriteString("\n  index " + strconv.Itoa(i) + " " + washRuneName(d.text[i]) + ": " + verb)
	}
	return b.String()
}

func washRuneName(r rune) string {
	switch r {
	case '\n':
		return `"\n"`
	case ' ':
		return `"NBSP"`
	case ' ':
		return `"EM SPACE"`
	case ' ':
		return `"EN SPACE"`
	case ' ':
		return `" "`
	}
	return `"` + string(r) + `"`
}

// --- The cases ----------------------------------------------------------------

// longProseChapter's verses are long enough that groupVersesIntoParagraphs breaks
// between 3 and 4 (its threshold is 320 characters ending on a sentence), so a
// mark over 2..5 crosses a paragraph boundary — and in the reporter layout, the
// em+en space indent that opens the new paragraph.
func longProseChapter() []Verse {
	long := strings.Repeat("The word of the LORD stands, and the promise given to the fathers is kept. ", 2) + "Amen."
	vs := make([]Verse, 0, 6)
	for i := 1; i <= 6; i++ {
		vs = append(vs, Verse{BookName: "Romans", Chapter: 8, Verse: i, Text: long})
	}
	return vs
}

func TestChapterWashCoversExactlyTheMarkedUpCharacters(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	// bare says whether this layout puts anything between two marked verses
	// OUTSIDE the band — which is the whole subject. Prose inside one paragraph
	// does not (its joins are tint.go's JoinSpace, inside the span on purpose);
	// poetry does (the line separator), a paragraph boundary does, and in the
	// reporter layout that boundary also carries the em+en space indent. Spelled
	// out per case so the ones that prove something are distinguishable from the
	// ones that only guard against over-trimming.
	cases := []struct {
		name     string
		book     string
		chapter  int
		verses   []Verse
		lo, hi   int
		reporter bool
		bare     bool
	}{
		{"prose/1-4", "Romans", 8, proseChapter(), 1, 4, false, false},
		{"prose/1-4 reporter", "Romans", 8, proseChapter(), 1, 4, true, false},
		{"poetry/2-3", "Psalms", 23, poeticChapter(), 2, 3, false, true},
		{"poetry/2-3 reporter", "Psalms", 23, poeticChapter(), 2, 3, true, true},
		{"paragraphs/2-5", "Romans", 8, longProseChapter(), 2, 5, false, true},
		{"paragraphs/2-5 reporter", "Romans", 8, longProseChapter(), 2, 5, true, true},
		{"single/3", "Romans", 8, proseChapter(), 3, 3, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := tintState(tc.book, tc.chapter, "web", tc.verses)
			st.setHL(hlSearch, tc.book, tc.chapter, tc.lo, tc.hi)
			var html string
			withReporterLayout(tc.reporter, func() { html = buildChapterHTML(st, tc.verses) })
			doc := importChapterHTML(t, html)

			// The model must agree with Go's own flattening of the tint, or the
			// case is testing a run the native side would never be handed.
			runs := nativeTintRuns(st, tc.verses)
			if len(runs) != 1 || runs[0].Lo != tc.lo || runs[0].Hi != tc.hi {
				t.Fatalf("nativeTintRuns gave %+v, want one run %d..%d", runs, tc.lo, tc.hi)
			}

			got := doc.washMask(tc.lo, tc.hi, true)
			if d := maskDiff(doc, got, doc.hl); d != "" {
				t.Errorf("the wash range and the .hl markup disagree:%s", d)
			}

			// And where the layout HAS a bare interior, show that the rule this
			// replaced painted it — so the case above is not passing vacuously.
			// (The single-verse mark is the one that always passed, which is
			// exactly why the defect shipped: it was the only case tried.)
			loose := doc.washMask(tc.lo, tc.hi, false)
			extra := 0
			for i := range loose {
				if loose[i] && !doc.hl[i] {
					extra++
				}
			}
			if (extra > 0) != tc.bare {
				t.Errorf("untrimmed rule painted %d characters the HTML leaves bare; want bare interior = %v",
					extra, tc.bare)
			}
		})
	}
}
