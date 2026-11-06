package bibletext

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The census of a whole canon's cross-reference apparatus, measured as the
// resolver reads it (publisher_xrefs.go). The feed's notes are citations and
// three words — title, LXX and DSS — each standing where the resolver expects
// it: ":title" after a whole-chapter id, ", LXX" and ", DSS" after a citation.
// Anything else would reach a reader as a caption nothing explains, so the
// live canon test asserts the set on every fresh download, and the render
// harness over the cached canon. The patterns are the ones the 6 November
// 2026 census of the raw feed used.

var (
	censusBookish    = `(?:[1-3]\s?)?[A-Z][a-z]+\.?`
	censusNumber     = `\d+(?::\d+[a-c]?)?`
	censusCitation   = regexp.MustCompile(`(?:` + censusBookish + `\s+)?` + censusNumber + `(?:\s*[–—-]\s*` + censusNumber + `)?`)
	censusSeparators = regexp.MustCompile(`[;,()\[\]\s.–—\-:]+`)
	censusWord       = regexp.MustCompile(`[A-Za-zÀ-ɏ][A-Za-zÀ-ɏ'’.]*`)
)

// noteCensus is what a canon's apparatus holds beyond plain citations.
type noteCensus struct {
	Notes          int            // cross-reference notes: on verses, on Psalm titles, on headings
	Words          map[string]int // the words besides citations, with their counts
	Stray          []string       // a word not where the resolver reads it, by place
	WholeChapters  int            // tagged ids naming whole chapters
	TitleCitations int            // of them, followed by ":title"
	Bases          int            // citations followed by a basis
	InParentheses  int            // tagged citations inside a parenthesised group
	Cards, Lines   int            // what split gives over every chapter's notes, selected whole
	Unbalanced     []string       // remainders whose parentheses do not balance, by place
}

// noteApparatusCensus measures bd's cross-reference apparatus.
func noteApparatusCensus(bd *BibleData) noteCensus {
	c := noteCensus{Words: map[string]int{}}
	measure := func(where string, fn Footnote) {
		if fn.Kind != footnoteKindCrossref {
			return
		}
		c.Notes++
		text := fn.Text
		runes := []rune(text)
		leftover := censusSeparators.ReplaceAllString(censusCitation.ReplaceAllString(text, " "), " ")
		for _, w := range censusWord.FindAllString(leftover, -1) {
			c.Words[w]++
		}
		// Where the resolver reads the three words: the runes a title suffix
		// or a basis run covers.
		covered := make([]bool, len(runes))
		for _, r := range fn.Refs {
			if r.Start < 0 || r.End > len(runes) || r.Start >= r.End {
				continue
			}
			if parenDepthAt(runes, r.Start) > 0 {
				c.InParentheses++
			}
			cut := r.End
			if len(strings.Split(strings.Split(r.ID, "-")[0], ".")) == 2 {
				c.WholeChapters++
				if hasRunesAt(runes, r.End, []rune(titleSuffix)) {
					c.TitleCitations++
					cut = r.End + len([]rune(titleSuffix))
					for i := r.End; i < cut; i++ {
						covered[i] = true
					}
				}
			}
			if basis, end := basisAfter(runes, cut); basis != "" {
				c.Bases++
				for i := cut; i < end; i++ {
					covered[i] = true
				}
			}
		}
		for _, m := range censusWord.FindAllStringIndex(text, -1) {
			w := text[m[0]:m[1]]
			if w != "title" && w != "LXX" && w != "DSS" {
				continue
			}
			at := len([]rune(text[:m[0]]))
			if !covered[at] {
				c.Stray = append(c.Stray, where+": "+w)
			}
		}
	}
	for book, chapters := range bd.Verses {
		for ch, vs := range chapters {
			for _, v := range vs {
				for _, fn := range v.Footnotes {
					measure(fmt.Sprintf("%s %d:%d", book, ch, v.Verse), fn)
				}
			}
			for _, fn := range bd.SuperscriptionFor(book, ch).Footnotes {
				measure(fmt.Sprintf("%s %d:title", book, ch), fn)
			}
			for _, h := range bd.Headings[book][ch] {
				for _, fn := range h.Footnotes {
					measure(fmt.Sprintf("%s %d:heading", book, ch), fn)
				}
			}
			// What the panel would make of the chapter selected whole: every
			// note's cards and lines, with balanced parentheses in each line.
			for _, n := range publisherCrossRefsFor(bd, book, ch, vs) {
				cards, remainder := n.split()
				c.Cards += len(cards)
				if remainder != "" {
					c.Lines++
					if strings.Count(remainder, "(") != strings.Count(remainder, ")") {
						c.Unbalanced = append(c.Unbalanced, fmt.Sprintf("%s %d, note on %d: %q", book, ch, n.Verse, remainder))
					}
				}
			}
		}
	}
	sort.Strings(c.Stray)
	sort.Strings(c.Unbalanced)
	return c
}

// problems is what in the census the resolver cannot rely on: a word besides
// title, LXX and DSS, one of the three where it is not read, a remainder a
// reader would see with a parenthesis short of its pair, or a split that made
// nothing of a whole canon.
func (c noteCensus) problems() []string {
	var out []string
	if c.Notes == 0 {
		return []string{"the census saw no cross-reference notes"}
	}
	var words []string
	for w := range c.Words {
		words = append(words, w)
	}
	sort.Strings(words)
	if got := strings.Join(words, " "); got != "DSS LXX title" {
		out = append(out, fmt.Sprintf("the words besides citations are %q (%v), want exactly DSS, LXX and title: the resolver would show any other as a caption", got, c.Words))
	}
	if c.Words["title"] != c.TitleCitations || c.Words["LXX"]+c.Words["DSS"] == 0 || c.Bases == 0 {
		out = append(out, fmt.Sprintf("title ×%d against %d title citations; LXX ×%d, DSS ×%d against %d bases", c.Words["title"], c.TitleCitations, c.Words["LXX"], c.Words["DSS"], c.Bases))
	}
	if len(c.Stray) != 0 {
		out = append(out, fmt.Sprintf("%d words stand where the resolver does not read them: %q", len(c.Stray), c.Stray))
	}
	if len(c.Unbalanced) != 0 {
		out = append(out, fmt.Sprintf("%d remainders have unbalanced parentheses, the first %q", len(c.Unbalanced), c.Unbalanced[0]))
	}
	if c.Cards == 0 || c.Lines == 0 {
		out = append(out, fmt.Sprintf("%d cards and %d lines over the canon; the split made nothing", c.Cards, c.Lines))
	}
	return out
}

// assertNoteCensus fails the test for each of the census's problems.
func assertNoteCensus(t *testing.T, c noteCensus) {
	t.Helper()
	for _, p := range c.problems() {
		t.Error(p)
	}
}

// The census on a canon shaped like the feed: the fixture's three words are
// counted, each in its place, and a word out of place is reported.
func TestNoteApparatusCensusReadsTheFixture(t *testing.T) {
	bd := publisherFixture()
	c := noteApparatusCensus(bd)
	if c.Notes != 8 {
		t.Errorf("notes = %d, want the fixture's 8 (seven on verses, one on a title)", c.Notes)
	}
	if p := c.problems(); len(p) != 0 {
		t.Errorf("the fixture's census has problems: %q", p)
	}
	if c.Words["title"] != 1 || c.Words["LXX"] != 1 || c.Words["DSS"] != 1 || len(c.Words) != 3 {
		t.Errorf("words = %v", c.Words)
	}
	if c.WholeChapters != 2 || c.TitleCitations != 1 || c.Bases != 1 || c.InParentheses != 1 {
		t.Errorf("census = %+v", c)
	}
	if len(c.Stray) != 0 || len(c.Unbalanced) != 0 {
		t.Errorf("stray %q, unbalanced %q", c.Stray, c.Unbalanced)
	}
	// Cards over every chapter selected whole: John 3 (7:50), Psalm 3's
	// title note resolves to nothing (2 Samuel is not loaded), 1 Samuel 19
	// (the title), Genesis 7 (Leviticus 11), Hebrews 1 (two). Lines: John
	// 3's four, Psalm 3's title note, 1 Samuel 19's Judges.
	if c.Cards != 5 || c.Lines != 6 {
		t.Errorf("cards %d, lines %d; want 5 and 6", c.Cards, c.Lines)
	}
	// CONTROL: a word the resolver does not read is reported, and a title
	// suffix after a verse id is a stray.
	v := &bd.Verses["John"][3][0]
	v.Footnotes = append(v.Footnotes, Footnote{Text: "Rom. 5:8, Vulgate; Ps. 3:title", Kind: footnoteKindCrossref, Caller: "-",
		Refs: []NoteRef{{"ROM.5.8", 0, 8}, {"PSA.3.1", 19, 24}}})
	c = noteApparatusCensus(bd)
	if c.Words["Vulgate"] != 1 || len(c.Stray) != 1 || !strings.HasSuffix(c.Stray[0], "John 3:1: title") {
		t.Errorf("a stray word is not reported: words %v, stray %q", c.Words, c.Stray)
	}
	// And the census of such a canon has problems to report.
	if p := c.problems(); len(p) < 2 || !strings.Contains(p[0], "Vulgate") {
		t.Errorf("a canon with a fourth word passed the census: %q", p)
	}
}
