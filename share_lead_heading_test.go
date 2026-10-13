package bibletext

// A SELECTION RUN ON INTO A HEADING, with its lead. The Apple and Android
// panes draw a publisher's heading between two verses, on a line of its own
// above the next verse's number, and the chapter prose the share pipeline
// reads does not hold it. A drag from the end of one verse into the heading
// below carries the heading's first words, and where those words also open
// the next verse, the selection read as the prose ran on into that verse:
// "lood.\nThe", begun inside the last word of a verse above a heading that
// opens "The", was cited as both verses. Every pane draws a verse's number
// before its words, so a selection that lacks the number there holds the
// heading, and the walk from the lead reads it as one (walkSelectionFrom).

import (
	"os"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// headingPageVerse is one verse of a drawnHeadingPage: where its number, its
// words and its end stand on the page, and where its words start in the
// chapter prose.
type headingPageVerse struct{ verse, num, body, end, cstart int }

// drawnHeadingPage is a chapter as the Apple and Android panes draw it: each
// verse's number in superscript digits joined to its words by a no-break
// space, the headings above a verse each on a line of its own, and every
// fifth verse opening an indented paragraph.
func drawnHeadingPage(corpus string, spans []verseSpan, heads []Heading) (string, []headingPageVerse) {
	before := map[int][]string{}
	for _, h := range heads {
		if t := collapseSpaces(h.DrawnText()); t != "" && h.BeforeVerse > 0 {
			before[h.BeforeVerse] = append(before[h.BeforeVerse], t)
		}
	}
	var b strings.Builder
	var pvs []headingPageVerse
	prev := 0
	for i, sp := range spans {
		if b.Len() > 0 {
			var hs []string
			for v := prev + 1; v <= sp.verse; v++ {
				hs = append(hs, before[v]...)
			}
			switch {
			case len(hs) > 0:
				b.WriteString("\n" + strings.Join(hs, "\n") + "\n")
			case i%5 == 0:
				b.WriteString("\n ")
			default:
				b.WriteByte(' ')
			}
		}
		num := b.Len()
		b.WriteString(superscriptNumber(sp.verse))
		b.WriteRune(' ')
		body := b.Len()
		b.WriteString(corpus[sp.start:sp.end])
		pvs = append(pvs, headingPageVerse{sp.verse, num, body, b.Len(), sp.start})
		prev = sp.verse
	}
	return b.String(), pvs
}

// headingPageVerseAt is the index of the verse whose number is the last at
// or before page offset off, as the panes resolve a selection's verses.
func headingPageVerseAt(pvs []headingPageVerse, off int) int {
	k := -1
	for i, p := range pvs {
		if p.num <= off {
			k = i
		}
	}
	return k
}

// headingPageSpan is the span and the lead a pane drawing the page sends
// for the selection page[a:b].
func headingPageSpan(page string, pvs []headingPageVerse, a, b int) selSpan {
	ka, kb := headingPageVerseAt(pvs, a), headingPageVerseAt(pvs, b-1)
	return selSpanWithLead(pvs[ka].verse, pvs[kb].verse, page[pvs[ka].num:a])
}

// headingPageStretch is the stretch of the chapter prose that the selection
// page[a:b] covers: from the first character of verse words it holds to the
// end of the last.
func headingPageStretch(pvs []headingPageVerse, a, b int) (start, end int) {
	start, end = -1, 0
	for _, p := range pvs {
		if start < 0 {
			switch {
			case a < p.body:
				start = p.cstart
			case a <= p.end:
				start = p.cstart + a - p.body
			}
		}
		if b < p.body {
			break
		}
		if b <= p.end {
			end = p.cstart + b - p.body
			break
		}
		end = p.cstart + p.end - p.body
	}
	return start, end
}

type headingFixtureChapter struct {
	st     *AppState
	book   string
	ch     int
	corpus string
	spans  []verseSpan
	page   string
	pvs    []headingPageVerse
}

func headingFixture(book string, ch int, texts map[int]string, heads ...Heading) headingFixtureChapter {
	st := leadChapter(book, ch, texts)
	st.Bible.Headings = map[string]map[int][]Heading{book: {ch: heads}}
	corpus, spans := chapterProseIn(st, book, ch)
	page, pvs := drawnHeadingPage(corpus, spans, heads)
	return headingFixtureChapter{st, book, ch, corpus, spans, page, pvs}
}

// drag is the page's text from the first character of from to the last of
// to, searched from the page offset after, with the span and lead the pane
// sends for it.
func (f headingFixtureChapter) drag(t *testing.T, after int, from, to string) (raw string, span selSpan, a int) {
	t.Helper()
	a = strings.Index(f.page[after:], from)
	if a < 0 {
		t.Fatalf("fixture: %q is not on the page after %d", from, after)
	}
	a += after
	b := strings.Index(f.page[a:], to)
	if b < 0 {
		t.Fatalf("fixture: %q is not on the page after %q", to, from)
	}
	b += a + len(to)
	return f.page[a:b], headingPageSpan(f.page, f.pvs, a, b), a
}

// A drag from the end of a verse into the heading below it, where the
// heading opens with the words the next verse opens with. The heading is
// drawn above the next verse's number, so the drag touched no word of that
// verse: it is cited by the verse above, as it is without the lead, and as
// its own link and its AI passage name it; and it is placed at its own
// place in the verse above.
func TestADragRunOnIntoAHeadingIsCitedByTheVerseItTouched(t *testing.T) {
	f := headingFixture("Genesis", 10, map[int]string{
		1: "These are the generations of the sons, born to them after the flood.",
		2: "The sons of the north were Gomer and Magog.",
	}, Heading{Text: "The Table of Nations", BeforeVerse: 2})
	v1 := spanOf(t, f.spans, 1)
	flood := v1.start + strings.Index(f.corpus[v1.start:v1.end], "flood.")
	for _, c := range []struct {
		name, from, to  string
		wantAt, wantLen int
	}{
		{"begun inside the word", "lood.", "The", flood + 1, len("lood.")},
		{"begun inside the word, to the heading's end", "lood.", "Nations", flood + 1, len("lood.")},
		{"the whole word", "flood.", "The", flood, len("flood.")},
	} {
		raw, span, _ := f.drag(t, 0, c.from, c.to)
		if span.lo != 1 || span.hi != 1 || !span.hasLead {
			t.Fatalf("fixture: %q spans %d–%d (lead %v), want 1–1 with its lead", raw, span.lo, span.hi, span.hasLead)
		}
		_, cite, at, baseLen := prepareShareQuoteIn(f.st, f.book, f.ch, raw, span)
		lo, hi := linkVersesForSelectionIn(f.st, f.book, f.ch, raw, span)
		p := aiPassageFor(f.st, raw, span)
		if cite != "Genesis 10:1" || lo != 1 || hi != 1 || p.Lo != 1 || p.Hi != 1 {
			t.Errorf("%s, %q with its lead: cited %q, linked %d–%d, AI passage %d–%d; want Genesis 10:1 for each", c.name, raw, cite, lo, hi, p.Lo, p.Hi)
		}
		if at != c.wantAt || baseLen != c.wantLen {
			t.Errorf("%s, %q with its lead: placed at %d (%d bytes), want its own place %d (%d bytes)", c.name, raw, at, baseLen, c.wantAt, c.wantLen)
		}
		if _, cite, _, _ := prepareShareQuoteIn(f.st, f.book, f.ch, raw, selSpanFromNative(1, 1)); cite != "Genesis 10:1" {
			t.Errorf("control: %q without a lead cited %q, want Genesis 10:1", raw, cite)
		}
	}
}

// A piece of a word that the locate declines is completed at its own place
// with the lead, and cited as it is without the lead: by the span, which is
// what its link names. A drag from inside a verse's last word to the next
// verse's number, through a heading or not, spans both verses.
func TestAPieceOfAWordIsCitedAsItsLinkCitesIt(t *testing.T) {
	texts := map[int]string{
		1: "These are the generations of the sons, born to them after the flood.",
		2: "The sons of the north were Gomer and Magog.",
	}
	for _, f := range []headingFixtureChapter{
		headingFixture("Genesis", 10, texts),
		headingFixture("Genesis", 10, texts, Heading{Text: "The Table of Nations", BeforeVerse: 2}),
	} {
		v1 := spanOf(t, f.spans, 1)
		lood := v1.start + strings.Index(f.corpus[v1.start:v1.end], "lood.")
		raw, span, _ := f.drag(t, 0, "lood.", superscriptNumber(2))
		if span.lo != 1 || span.hi != 2 {
			t.Fatalf("fixture: %q spans %d–%d, want 1–2", raw, span.lo, span.hi)
		}
		_, cite, at, _ := prepareShareQuoteIn(f.st, f.book, f.ch, raw, span)
		lo, hi := linkVersesForSelectionIn(f.st, f.book, f.ch, raw, span)
		_, without, _, _ := prepareShareQuoteIn(f.st, f.book, f.ch, raw, selSpanFromNative(1, 2))
		if cite != verseRangeCitation(f.book, f.ch, lo, hi) || cite != without {
			t.Errorf("%q with its lead cited %q; its link names %d–%d, and without the lead it is cited %q", raw, cite, lo, hi, without)
		}
		if at != lood {
			t.Errorf("%q with its lead placed at %d, want its own place %d", raw, at, lood)
		}
	}
}

// A later copy of the words that end a verse, dragged on into the heading
// below, where the earlier copy is followed by the heading's first word.
// With the lead it is shared from its own place; without one the earlier
// copy is taken and the heading's word is quoted as the verse's, as before.
// And a drag through a heading that repeats the next verse's opening words,
// to that verse's number: the number's verse gave no word, so the quotation
// is the verse above's.
func TestALaterCopyRunOnIntoAHeadingIsSharedFromItsOwnPlace(t *testing.T) {
	f := headingFixture("Exodus", 37, map[int]string{
		9:  "They spread their wings over the mercy seat. The cherubim faced the mercy seat.",
		10: "He made the table of acacia wood.",
	}, Heading{Text: "The Table", BeforeVerse: 10})
	v9 := spanOf(t, f.spans, 9)
	second := v9.start + strings.LastIndex(f.corpus[v9.start:v9.end], "mercy seat.")
	raw, span, _ := f.drag(t, f.pvs[0].body+strings.Index(f.page[f.pvs[0].body:], "faced"), "mercy", "\nThe")
	if quote, cite := shareQuoteIn(f.st, f.book, f.ch, raw, span); quote != "“[M]ercy seat.”" || cite != "Exodus 37:9" {
		t.Errorf("the second %q with its lead shared (%q, %q), want (“[M]ercy seat.”, Exodus 37:9)", raw, quote, cite)
	}
	if _, _, _, at, ok := normalizeShareSelectionProse(f.st, f.book, f.ch, f.corpus, f.spans, raw, span); !ok || at != second {
		t.Errorf("the second %q with its lead placed at %d (ok %v), want %d", raw, at, ok, second)
	}
	if quote, _ := shareQuoteIn(f.st, f.book, f.ch, raw, selSpanFromNative(9, 9)); quote != "“[M]ercy seat. The . . . .”" {
		t.Errorf("control: %q without a lead shared %q, want the first copy's “[M]ercy seat. The . . . .”", raw, quote)
	}

	f = headingFixture("1 Chronicles", 23, map[int]string{
		6: "He divided them into divisions by the sons of Levi: Gershon, Kohath, and Merari.",
		7: "The Gershonites were Ladan and Shimei.",
	}, Heading{Text: "The Gershonites", BeforeVerse: 7})
	for _, to := range []string{"The Gershonites", superscriptNumber(7)} {
		raw, span, _ := f.drag(t, 0, "Merari.", "\n"+to)
		text, cite, _, _ := prepareShareQuoteIn(f.st, f.book, f.ch, raw, span)
		lo, hi := linkVersesForSelectionIn(f.st, f.book, f.ch, raw, span)
		if text != "Merari." || cite != "1 Chronicles 23:6" || lo != 6 || hi != 6 {
			t.Errorf("%q with its lead: (%q, %q), linked %d–%d; want (\"Merari.\", \"1 Chronicles 23:6\"), 6–6", raw, text, cite, lo, hi)
		}
	}
	raw, _, _ = f.drag(t, 0, "Merari.", "\n"+superscriptNumber(7))
	if text, cite, _, _ := prepareShareQuoteIn(f.st, f.book, f.ch, raw, selSpanFromNative(6, 7)); text != "Merari. The Gershonites" || cite != "1 Chronicles 23:6–7" {
		t.Errorf("control: %q without a lead: (%q, %q), want the heading read as verse 7's words, (\"Merari. The Gershonites\", \"1 Chronicles 23:6–7\")", raw, text, cite)
	}
}

// inventHeadings gives every fourth verse of bd a heading of one of the
// shapes that meet the words around it:
//
//   - where the last word of the verse above also stands earlier in that
//     verse, followed by a word of letters, the heading opens with that
//     word, so a drag from the later copy into the heading also reads as
//     the earlier copy and the words after it;
//   - otherwise, in turn: the first two words of the verse below, the
//     first word of the verse below and words of its own, or words of its
//     own alone.
//
// Every fifth heading has a second one above it. It returns how many
// verses have headings.
func inventHeadings(bd *BibleData) int {
	bd.Headings = map[string]map[int][]Heading{}
	n := 0
	for _, book := range bd.Books {
		bd.Headings[book] = map[int][]Heading{}
		for _, ch := range bd.GetChapterNumbersForBook(book) {
			st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: ch}
			corpus, spans := chapterProseIn(st, book, ch)
			for k := 3; k < len(spans); k += 4 {
				above := strings.Fields(corpus[spans[k-1].start:spans[k-1].end])
				below := strings.Fields(corpus[spans[k].start:spans[k].end])
				text := ""
				last := above[len(above)-1]
				for i := 0; i+2 < len(above); i++ {
					if above[i] == last && strings.IndexFunc(above[i+1], func(r rune) bool { return !unicode.IsLetter(r) }) < 0 {
						text = above[i+1] + " in Galilee"
						break
					}
				}
				if text == "" {
					switch {
					case n%3 == 0 && len(below) >= 2:
						text = below[0] + " " + below[1]
					case n%3 == 1:
						text = below[0] + " of the Kingdom"
					default:
						text = "A Saying About Rest"
					}
				}
				var hs []Heading
				if n%5 == 4 {
					hs = append(hs, Heading{Text: "The Second Part", BeforeVerse: spans[k].verse})
				}
				hs = append(hs, Heading{Text: text, BeforeVerse: spans[k].verse})
				bd.Headings[book][ch] = append(bd.Headings[book][ch], hs...)
				n++
			}
		}
	}
	return n
}

type headingDragResult struct {
	drags, outsideWith, outsideWithout, disagree int
}

// headingDrags is every drag over the headings of every step-th chapter of
// bd: begun on one of the last three words of the verse above a heading, and
// ended in the heading or on the number or the first two words of the verse
// below, of up to five words; each whole, begun one character late and
// ended one character early. With its lead each is placed, when it is
// placed, inside the stretch of verse words it covers, and cited as its
// link cites it. Those placed outside that stretch without the lead are
// counted.
func headingDrags(t *testing.T, bd *BibleData, step int, label string) headingDragResult {
	t.Helper()
	var r headingDragResult
	shown := 0
	show := func(format string, args ...any) {
		if shown < 8 {
			shown++
			t.Logf(label+": "+format, args...)
		}
	}
	isSep := func(c byte) bool { return c == ' ' || c == '\n' }
	n := 0
	for _, book := range bd.Books {
		for _, ch := range bd.GetChapterNumbersForBook(book) {
			heads := bd.Headings[book][ch]
			if len(heads) == 0 {
				continue
			}
			if n++; (n-1)%step != 0 {
				continue
			}
			st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: ch}
			corpus, spans := chapterProseIn(st, book, ch)
			page, pvs := drawnHeadingPage(corpus, spans, heads)
			for k := 1; k < len(pvs); k++ {
				from, to := pvs[k-1].end, pvs[k].num
				if !strings.Contains(page[from:to], "\n") || strings.TrimSpace(page[from:to]) == "" {
					continue // no heading between these verses
				}
				var toks [][2]int
				for i := pvs[k-1].body; i < pvs[k].end; i++ {
					if !isSep(page[i]) && (i == pvs[k-1].body || isSep(page[i-1])) {
						j := i
						for j < len(page) && !isSep(page[j]) {
							j++
						}
						toks = append(toks, [2]int{i, j})
					}
				}
				firstAfter, lastIdx, belowSeen := -1, -1, 0
				for i, tk := range toks {
					if firstAfter < 0 && tk[0] >= pvs[k-1].end {
						firstAfter = i
					}
					if tk[0] >= pvs[k].num {
						if belowSeen++; belowSeen == 3 {
							break
						}
					}
					lastIdx = i
				}
				if firstAfter < 0 {
					continue
				}
				for si := max(0, firstAfter-3); si < firstAfter; si++ {
					for ei := firstAfter; ei <= lastIdx && ei-si < 5; ei++ {
						a, b := toks[si][0], toks[ei][1]
						cuts := [][2]int{{a, b}}
						if _, size := utf8.DecodeRuneInString(page[a:]); a+size < toks[si][1] {
							cuts = append(cuts, [2]int{a + size, b})
						}
						if _, size := utf8.DecodeLastRuneInString(page[:b]); b-size > toks[ei][0] {
							cuts = append(cuts, [2]int{a, b - size})
						}
						for _, c := range cuts {
							raw := page[c[0]:c[1]]
							ca, cb := headingPageStretch(pvs, c[0], c[1])
							span := headingPageSpan(page, pvs, c[0], c[1])
							r.drags++
							outside := func(at, n int) bool { return at >= 0 && (at < ca || at+n > cb) }
							text, cite, at, bl := prepareShareQuoteIn(st, book, ch, raw, span)
							if outside(at, bl) {
								r.outsideWith++
								show("%s %d %q with its lead: (%q, %q, at %d, %d bytes), the drag covers [%d, %d)", book, ch, raw, text, cite, at, bl, ca, cb)
							}
							if lo, hi := linkVersesForSelectionIn(st, book, ch, raw, span); lo > 0 && cite != verseRangeCitation(book, ch, lo, hi) {
								r.disagree++
								show("%s %d %q with its lead cited %q, linked %d–%d", book, ch, raw, cite, lo, hi)
							}
							if _, _, at, bl := prepareShareQuoteIn(st, book, ch, raw, selSpanFromNative(span.lo, span.hi)); outside(at, bl) {
								r.outsideWithout++
							}
						}
					}
				}
			}
		}
	}
	t.Logf("%s: %d drags into a heading; with the lead %d placed outside the drag and %d cited otherwise than linked; without it %d placed outside",
		label, r.drags, r.outsideWith, r.disagree, r.outsideWithout)
	return r
}

// The Gospels with invented headings over every fourth verse: 23,567 drags
// into a heading. With the lead none is placed outside the stretch it
// covers and each is cited as its link cites it. Without it 75 are
// placed outside, at an earlier copy of their words or with the heading's
// words read as the verse's; the count is pinned, so the path without a
// lead is held unchanged.
func TestEveryDragIntoAHeadingIsPlacedInsideItselfWithItsLead(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if n := inventHeadings(bd); n == 0 {
		t.Fatal("fixture: no headings invented")
	}
	step := 1
	if testing.Short() {
		step = 7
	}
	r := headingDrags(t, bd, step, "Gospels")
	if r.drags == 0 || r.outsideWith != 0 || r.disagree != 0 {
		t.Errorf("with the lead, of %d drags into a heading %d were placed outside the drag and %d cited otherwise than linked", r.drags, r.outsideWith, r.disagree)
	}
	if r.outsideWithout == 0 {
		t.Errorf("control: without the lead none of %d drags into a heading was placed outside the drag; some are expected there", r.drags)
	}
	if step == 1 && (r.drags != 23567 || r.outsideWithout != 75) {
		t.Errorf("%d drags, %d placed outside without the lead; want 23,567 and 75", r.drags, r.outsideWithout)
	}
}

// The same drags over the publishers' own headings in the WEB, BSB and WEBC
// caches: 503, 50,680 and 557 of them. LOCAL-ONLY and opt-in by
// BIBLETEXT_EXHAUSTIVE=1. With the lead none is placed outside the drag or
// cited otherwise than linked. Without it the BSB's 14 are placed outside,
// pinned.
func TestEveryDragIntoAHeadingIsPlacedInsideItselfOverLocalCaches_LocalOnly(t *testing.T) {
	if os.Getenv("BIBLETEXT_EXHAUSTIVE") == "" {
		t.Skip("local-only and opt-in: every drag into a heading of three translation caches; run with BIBLETEXT_EXHAUSTIVE=1")
	}
	without := map[string]int{"web": 0, "bsb": 14, "webc": 0}
	for _, c := range localPublicDomainCaches(t) {
		r := headingDrags(t, c.bd, 1, c.id)
		if r.drags == 0 || r.outsideWith != 0 || r.disagree != 0 {
			t.Errorf("%s: with the lead, of %d drags into a heading %d were placed outside the drag and %d cited otherwise than linked", c.id, r.drags, r.outsideWith, r.disagree)
		}
		if want, ok := without[c.id]; ok && r.outsideWithout != want {
			t.Errorf("%s: without the lead %d drags into a heading were placed outside; pinned %d", c.id, r.outsideWithout, want)
		}
	}
}
