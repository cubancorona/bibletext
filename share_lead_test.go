package bibletext

// THE SELECTION'S LEAD. A pane hands over the selection's text, its verses,
// and its lead: its own text of verse lo from the first character of that
// verse's number to the selection's first character (selSpan.lead,
// docs/SELECTION_OFFSET.md). Without the lead the share pipeline places a
// selection at the first copy of its words in its verses, and the sentence
// completion, the original terminal and the poem-line restore read that
// copy's surroundings: the second "Abraham" of Genesis 22:11 is shared as
// "“Abraham . . . !”" where its own place gives "“Abraham!”". These tests hold
// each placement to the reader's own place with the lead, and hold the answer
// without one to what it was before the lead existed.

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// quoteAtPlace is what the share pipeline gives sel placed at corpus offset
// pos: its own stages that read the surroundings, fed pos.
func quoteAtPlace(st *AppState, book string, ch int, corpus, sel string, pos int) string {
	completed := completeTrailingSentenceProse(corpus, sel, pos)
	term := originalSentenceTerminalIn(st, book, ch, completed, pos, len(sel))
	return formatBibleQuote(restoreShareLineBreaksIn(st, book, ch, completed, pos, len(sel)), term)
}

// leadAt is the lead a pane drawing verse sp as "N body" sends for a
// selection that begins at corpus offset pos inside it.
func leadAt(corpus string, sp verseSpan, pos int) string {
	return strconv.Itoa(sp.verse) + " " + corpus[sp.start:pos]
}

func leadChapter(book string, ch int, texts map[int]string) *AppState {
	var vs []Verse
	for n := 1; n <= 200; n++ {
		if t, ok := texts[n]; ok {
			vs = append(vs, Verse{BookName: book, Book: book, Chapter: ch, Verse: n, Text: t})
		}
	}
	return &AppState{Bible: &BibleData{
		Books:  []string{book},
		Verses: map[string]map[int][]Verse{book: {ch: vs}},
	}, CurrentBook: book, CurrentChapter: ch}
}

func spanOf(t *testing.T, spans []verseSpan, verse int) verseSpan {
	t.Helper()
	for _, sp := range spans {
		if sp.verse == verse {
			return sp
		}
	}
	t.Fatalf("no verse %d", verse)
	return verseSpan{}
}

// The lead is kept only while the span starts at the verse it was measured
// from: a selection begun above verse 1 reports lo 0 and is clamped to verse
// 1, and an inverted pair is swapped, and either way the lead describes
// another verse.
func TestALeadIsKeptOnlyForTheVerseItWasMeasuredFrom(t *testing.T) {
	for _, c := range []struct {
		name         string
		lo, hi       int
		wantLo       int
		wantHasLead  bool
		wantZeroSpan bool
	}{
		{"its own verse", 3, 5, 3, true, false},
		{"clamped from above verse 1", 0, 5, 1, false, false},
		{"swapped", 5, 3, 3, false, false},
		{"no span", 0, 0, 0, false, true},
	} {
		s := selSpanWithLead(c.lo, c.hi, "3 Then")
		if c.wantZeroSpan {
			if s != (selSpan{}) {
				t.Errorf("%s: got %+v, want the zero span", c.name, s)
			}
			continue
		}
		if s.lo != c.wantLo || s.hasLead != c.wantHasLead {
			t.Errorf("%s: got lo %d hasLead %v, want lo %d hasLead %v", c.name, s.lo, s.hasLead, c.wantLo, c.wantHasLead)
		}
	}
	if selSpanFromNative(3, 3).hasLead {
		t.Error("selSpanFromNative must carry no lead")
	}
}

// Each shape of lead resolves to where the selection begins, in the corpus's
// coordinates, through the drawn forms the panes send (superscript digits,
// the no-break join, a line break), and a lead that is not verse lo's text as
// the corpus has it resolves to nothing.
func TestALeadResolvesToTheSelectionsPlace(t *testing.T) {
	st := leadChapter("John", 3, map[int]string{
		15: "that whoever believes in him should not perish, but have eternal life.",
		16: "For God so loved the world, that he gave his one and only Son.",
		17: "For God didn’t send his Son into the world to judge the world.",
	})
	corpus, spans := chapterProse(st)
	v16, v17 := spanOf(t, spans, 16), spanOf(t, spans, 17)
	loved := v16.start + strings.Index(corpus[v16.start:], "loved")
	for _, c := range []struct {
		name         string
		span         selSpan
		at           int
		tail         string
		onNumber, ok bool
	}{
		{"empty: on the number", selSpanWithLead(16, 16, ""), v16.start, "", true, true},
		{"the number: just after it", selSpanWithLead(16, 16, "16"), v16.start, "", false, true},
		{"the number and its space", selSpanWithLead(16, 16, "16 "), v16.start, "", false, true},
		{"inside the number", selSpanWithLead(16, 16, "1"), v16.start, "6", false, true},
		{"before a word", selSpanWithLead(16, 16, "16 For God so "), loved, "", false, true},
		{"before the space", selSpanWithLead(16, 16, "16 For God so"), loved, "", false, true},
		{"as the page draws it", selSpanWithLead(16, 16, "¹⁶ For God\nso "), loved, "", false, true},
		{"inside a word", selSpanWithLead(16, 16, "16 For God so lo"), loved + 2, "", false, true},
		{"the whole verse", selSpanWithLead(16, 17, "16 For God so loved the world, that he gave his one and only Son.\n"), v17.start, "", true, true},
		{"a heading inside it", selSpanWithLead(16, 16, "16 For God\nThe Love of God\nso "), -1, "", false, false},
		{"other words", selSpanWithLead(16, 16, "16 For God so hated "), -1, "", false, false},
		{"another verse's number", selSpanWithLead(16, 16, "17 For God "), -1, "", false, false},
		{"a verse the chapter lacks", selSpanWithLead(18, 18, "18 He "), -1, "", false, false},
		{"no lead", selSpanFromNative(16, 16), -1, "", false, false},
	} {
		at, tail, onNumber, ok := c.span.leadStart(corpus, spans)
		if !ok && !c.ok {
			continue
		}
		if at != c.at || tail != c.tail || onNumber != c.onNumber || ok != c.ok {
			t.Errorf("%s: got (at %d, tail %q, onNumber %v, ok %v), want (%d, %q, %v, %v)",
				c.name, at, tail, onNumber, ok, c.at, c.tail, c.onNumber, c.ok)
		}
	}
}

// A selection of wording its verse repeats, at each of its copies, with the
// lead: placed at that copy, and shared as that copy's place gives it. The
// control is the same selection without the lead, placed at the first copy
// as before, whose answer is unchanged.
func TestALaterCopyIsSharedFromItsOwnPlaceWithItsLead(t *testing.T) {
	st := leadChapter("Psalms", 148, map[int]string{
		1: "He said peace to them, and they said peace.",
		2: "Praise him, you heavens!\nPraise him, you waters.",
		3: "He cried, holy, holy, holy is the LORD of hosts!",
	})
	corpus, spans := chapterProse(st)
	for _, c := range []struct {
		verse int
		sel   string
	}{{1, "said peace"}, {2, "Praise him, you"}, {3, "holy"}} {
		sp := spanOf(t, spans, c.verse)
		var copies []int
		for from := sp.start; ; {
			i := wordAlignedIndex(corpus[from:sp.end], c.sel)
			if i < 0 {
				break
			}
			copies = append(copies, from+i)
			from += i + 1
		}
		if len(copies) < 2 {
			t.Fatalf("fixture: %q must stand twice in verse %d", c.sel, c.verse)
		}
		for n, pos := range copies {
			span := selSpanWithLead(c.verse, c.verse, leadAt(corpus, sp, pos))
			_, _, _, at, ok := normalizeShareSelectionProse(st, "Psalms", 148, corpus, spans, c.sel, span)
			got, _ := shareQuoteIn(st, "Psalms", 148, c.sel, span)
			want := quoteAtPlace(st, "Psalms", 148, corpus, c.sel, pos)
			if !ok || at != pos || got != want {
				t.Errorf("%q, copy %d of verse %d, with its lead: placed at %d (ok %v) and shared %q; its place is %d and gives %q",
					c.sel, n+1, c.verse, at, ok, got, pos, want)
			}
			plain := selSpanFromNative(c.verse, c.verse)
			if _, _, _, at, _ := normalizeShareSelectionProse(st, "Psalms", 148, corpus, spans, c.sel, plain); at != copies[0] {
				t.Errorf("control: %q in verse %d without a lead placed at %d, want the first copy %d", c.sel, c.verse, at, copies[0])
			}
		}
	}
	// The two answers, spelled out for one of them.
	sp := spanOf(t, spans, 1)
	second := sp.start + strings.LastIndex(corpus[sp.start:sp.end], "said peace")
	if got, _ := shareQuoteIn(st, "Psalms", 148, "said peace", selSpanWithLead(1, 1, leadAt(corpus, sp, second))); got != "“[S]aid peace.”" {
		t.Errorf("the second \"said peace\" with its lead shared %q, want “[S]aid peace.”", got)
	}
	if got, _ := shareQuoteIn(st, "Psalms", 148, "said peace", selSpanFromNative(1, 1)); got != "“[S]aid peace . . . .”" {
		t.Errorf("control: \"said peace\" without a lead shared %q, want the first copy's “[S]aid peace . . . .”", got)
	}
}

// Genesis 22:11 in the WEB: "Abraham, Abraham!" The second one, double-
// clicked, with its lead.
func TestTheSecondAbrahamIsSharedFromItsOwnPlace(t *testing.T) {
	st := leadChapter("Genesis", 22, map[int]string{
		10: "Abraham stretched out his hand, and took the knife to kill his son.",
		11: "The LORD’s angel called to him out of the sky, and said, “Abraham, Abraham!” He said, “Here I am.”",
	})
	corpus, spans := chapterProse(st)
	sp := spanOf(t, spans, 11)
	second := sp.start + strings.LastIndex(corpus[sp.start:sp.end], "Abraham")
	if got, cite := shareQuoteIn(st, "Genesis", 22, "Abraham", selSpanWithLead(11, 11, "11 The LORD’s angel called to him out of the sky, and said, “Abraham, ")); got != "“Abraham!”" || cite != "Genesis 22:11" {
		t.Errorf("the second Abraham with its lead shared (%q, %q), want (“Abraham!”, Genesis 22:11); its place is %d", got, cite, second)
	}
	if got, _ := shareQuoteIn(st, "Genesis", 22, "Abraham", selSpanFromNative(11, 11)); got != "“Abraham . . . !”" {
		t.Errorf("control: without a lead shared %q, want the first copy's “Abraham . . . !”", got)
	}
}

// A drag begun on a later digit of verse lo's number: the lead is the digits
// before it, so the rest of the number at the front of the selection is the
// page's apparatus and comes off. Without the lead the digit stays, in the
// share and in the AI panel's preview: the fallback strips a verse number
// only as a whole token followed by its verse's body (cleanQuoteTextIn), and
// the "3" left of 13 is neither. That strip takes "14" off whole; the
// 12-rune probe it replaced took verse 4's opening, which 14 shares, for
// verse 4's marker inside "14", and the quotation carried "3 ‘You shall not
// murder. 1‘You shall not commit".
func TestADragBegunInsideAVerseNumberSharesNoDigit(t *testing.T) {
	st := leadChapter("Exodus", 20, map[int]string{
		3:  "“You shall have no other gods before me.",
		4:  "“You shall not make for yourselves an idol, nor any image of anything that is in the heavens above, or that is in the earth beneath, or that is in the water under the earth:",
		12: "“Honor your father and your mother, that your days may be long in the land which the LORD your God gives you.",
		13: "“You shall not murder.",
		14: "“You shall not commit adultery.",
	})
	raw := "3 “You shall not murder.\n14 “You shall not commit adultery."
	span := selSpanWithLead(13, 14, "1")
	// What selecting the two verses whole shares.
	want, wantCite, _ := shareQuoteForPassage(st, "Exodus", 20, 13, 14)
	quote, cite := shareQuoteIn(st, "Exodus", 20, raw, span)
	if quote != want || cite != wantCite || strings.ContainsAny(quote, "0123456789") {
		t.Errorf("begun on the 3 of 13, with the lead: shared (%q, %q), want (%q, %q)", quote, cite, want, wantCite)
	}
	if p := aiPassageFor(st, raw, span); p.PreviewText != "“You shall not murder. “You shall not commit adultery." || p.Lo != 13 || p.Hi != 14 {
		t.Errorf("the AI passage with the lead previews %q for %d–%d", p.PreviewText, p.Lo, p.Hi)
	}
	// The control: without the lead the stray digit is still there, which is
	// the defect the lead removes, and 14 comes off whole, by the fallback's
	// strip.
	if quote, _ := shareQuoteIn(st, "Exodus", 20, raw, selSpanFromNative(13, 14)); !strings.Contains(quote, "3 ") || strings.Contains(quote, "1‘You") {
		t.Errorf("control: without the lead shared %q; the stray 3 is expected there, and 14 taken off whole", quote)
	}
	if p := aiPassageFor(st, raw, selSpanFromNative(13, 14)); !strings.Contains(p.PreviewText, "3 “You") || strings.Contains(p.PreviewText, "1“You") {
		t.Errorf("control: without the lead the AI passage previews %q; the stray 3 is expected there, and 14 taken off whole", p.PreviewText)
	}

	// The rest of a number selected on its own: the verse it labels, no
	// quotation. Without the lead "9" read as verse 9's number.
	st = leadChapter("Luke", 1, map[int]string{
		9:  "according to the custom of the priest’s office, his lot was to enter into the temple.",
		29: "But when she saw him, she was greatly troubled at the saying.",
	})
	if quote, cite := shareQuoteIn(st, "Luke", 1, "9", selSpanWithLead(29, 29, "2")); quote != "" || cite != "Luke 1:29" {
		t.Errorf("the 9 of 29 alone, with the lead: shared (%q, %q), want (\"\", \"Luke 1:29\")", quote, cite)
	}
}

// A figure in a verse's words followed by the words that open the verse with
// that number. The strip by text takes the figure for that verse's number;
// with the lead the walk reads it as text, because it does not stand at that
// verse's start.
func TestAFigureInAVerseIsNotTakenForAVerseNumber(t *testing.T) {
	st := leadChapter("Revelation", 21, map[int]string{
		3:  "The city had 12 gates of pearl, and the wall was great.",
		12: "gates of pearl were in the wall, and names were written on them.",
	})
	corpus, spans := chapterProse(st)
	sp := spanOf(t, spans, 3)
	pos := sp.start + strings.Index(corpus[sp.start:], "had 12")
	text, lo, hi, at, ok := normalizeShareSelectionProse(st, "Revelation", 21, corpus, spans, "had 12 gates of pearl", selSpanWithLead(3, 3, leadAt(corpus, sp, pos)))
	if !ok || text != "had 12 gates of pearl" || lo != 3 || hi != 3 || at != pos {
		t.Errorf("with the lead: (%q, %d–%d, at %d, ok %v), want (\"had 12 gates of pearl\", 3–3, at %d)", text, lo, hi, at, ok, pos)
	}
	if text, _, _, _, _ := normalizeShareSelectionProse(st, "Revelation", 21, corpus, spans, "had 12 gates of pearl", selSpanFromNative(3, 3)); text == "had 12 gates of pearl" {
		t.Error("control: without the lead the strip is expected to take 12 for verse 12's number")
	}
}

// leadWord is one word of a verse: the byte range of its letters and digits,
// without the marks at either end, which a double-click or a drag from letter
// to letter takes.
type leadWord struct{ start, end int }

func leadWords(s string) []leadWord {
	var out []leadWord
	for i := 0; i < len(s); {
		j := strings.IndexByte(s[i:], ' ')
		if j < 0 {
			j = len(s) - i
		}
		tok := s[i : i+j]
		a := strings.IndexFunc(tok, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) })
		b := strings.LastIndexFunc(tok, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) })
		if a >= 0 {
			_, size := utf8.DecodeRuneInString(tok[b:])
			out = append(out, leadWord{i + a, i + b + size})
		}
		i += j + 1
	}
	return out
}

// leadSweep is every selection of one, two and three words made letter to
// letter in every step-th verse of bd. With its lead each must be placed at
// its own offset with its own words. Those whose verse holds the same words
// earlier, the later copies, must share what their own place gives with the
// lead, and those that share something else without it are counted.
//
// It also takes every piece of a word of four or more letters begun or ended
// one letter inside it (or both), a fragment the locate declines and the
// fallback shares as it came. With its lead each must be completed at its
// own place: the fallback's text, place and length are checked, from which
// the terminal and the line-break restore follow. full also compares the
// whole shared quotation with the lead and without it, counting the
// fragments that share something else without; it is the slow half, run over
// the local caches.
type leadSweepResult struct {
	selections, laterCopies, laterWrongWithout, wrongWith int
	fragments, fragmentsWrongWithout, fragmentsWrongWith  int
}

func leadSweep(t *testing.T, bd *BibleData, step int, full bool, label string) leadSweepResult {
	t.Helper()
	var r leadSweepResult
	shown := 0
	show := func(format string, args ...any) {
		if shown < 8 {
			shown++
			t.Logf(label+": "+format, args...)
		}
	}
	for _, book := range bd.Books {
		for _, ch := range bd.GetChapterNumbersForBook(book) {
			st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: ch}
			corpus, spans := chapterProseIn(st, book, ch)
			for vi := 0; vi < len(spans); vi += step {
				sp := spans[vi]
				region := corpus[sp.start:sp.end]
				words := leadWords(region)
				ref := fmt.Sprintf("%s %d:%d", book, ch, sp.verse)
				for n := 1; n <= 3; n++ {
					for i := 0; i+n <= len(words); i++ {
						a, b := words[i].start, words[i+n-1].end
						sel := region[a:b]
						pos := sp.start + a
						r.selections++
						span := selSpanWithLead(sp.verse, sp.verse, leadAt(corpus, sp, pos))
						text, lo, hi, at, ok := normalizeShareSelectionProse(st, book, ch, corpus, spans, sel, span)
						if !ok || at != pos || text != sel || lo != sp.verse || hi != sp.verse {
							r.wrongWith++
							show("%s %q with its lead: (%q, %d–%d, at %d, ok %v), its place is %d", ref, sel, text, lo, hi, at, ok, pos)
							continue
						}
						if first := wordAlignedIndex(region, sel); first < 0 || first == a {
							continue
						}
						r.laterCopies++
						want := quoteAtPlace(st, book, ch, corpus, sel, pos)
						if got, _ := shareQuoteIn(st, book, ch, sel, span); got != want {
							r.wrongWith++
							show("%s %q with its lead shared %q, its place gives %q", ref, sel, got, want)
						}
						if got, _ := shareQuoteIn(st, book, ch, sel, selSpanFromNative(sp.verse, sp.verse)); got != want {
							r.laterWrongWithout++
						}
					}
				}
				for _, w := range words {
					if w.end-w.start < 4 {
						continue
					}
					for _, cut := range [][2]int{{1, 0}, {0, 1}, {1, 1}} {
						a, b := w.start+cut[0], w.end-cut[1]
						if a >= b || !utf8RuneStart(region, a) || !utf8RuneStart(region, b) {
							continue
						}
						frag := region[a:b]
						if strings.IndexFunc(frag, func(r rune) bool { return !isWordRune(r) }) >= 0 {
							continue // a piece of one word, letters and digits only
						}
						pos := sp.start + a
						r.fragments++
						span := selSpanWithLead(sp.verse, sp.verse, leadAt(corpus, sp, pos))
						text, cite, at, baseLen := prepareShareQuoteIn(st, book, ch, frag, span)
						if at != pos || baseLen != len(frag) || text != completeTrailingSentenceProse(corpus, frag, pos) || cite != verseRangeCitation(book, ch, sp.verse, sp.verse) {
							r.fragmentsWrongWith++
							show("%s fragment %q with its lead: (%q, %q, at %d, %d), its place is %d", ref, frag, text, cite, at, baseLen, pos)
							continue
						}
						if !full {
							continue
						}
						want := quoteAtPlace(st, book, ch, corpus, frag, pos)
						if got, _ := shareQuoteIn(st, book, ch, frag, span); got != want {
							r.fragmentsWrongWith++
							show("%s fragment %q with its lead shared %q, its place gives %q", ref, frag, got, want)
						}
						if got, _ := shareQuoteIn(st, book, ch, frag, selSpanFromNative(sp.verse, sp.verse)); got != want {
							r.fragmentsWrongWithout++
						}
					}
				}
			}
		}
	}
	t.Logf("%s: %d selections, %d later copies (%d shared otherwise without the lead), %d fragments (%d shared otherwise without the lead, counted only in full); with the lead %d and %d wrong",
		label, r.selections, r.laterCopies, r.laterWrongWithout, r.fragments, r.fragmentsWrongWithout, r.wrongWith, r.fragmentsWrongWith)
	return r
}

func utf8RuneStart(s string, i int) bool {
	return i >= len(s) || utf8.RuneStart(s[i])
}

// The sweep over the embedded Gospels: 229,224 selections, 13,853 of them
// later copies, and 123,177 fragments. With the lead none is wrong. Without
// it 1,605 of the later copies share another copy's completion, terminal or
// line breaks; the count is pinned, so a change to the path a selection with
// no lead takes shows here.
func TestEverySelectionInTheGospelsIsPlacedByItsLead(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	step := 1
	if testing.Short() {
		step = 7
	}
	r := leadSweep(t, bd, step, false, "Gospels")
	if r.wrongWith != 0 || r.fragmentsWrongWith != 0 {
		t.Errorf("with the lead, %d selections and %d fragments were not shared from their own place", r.wrongWith, r.fragmentsWrongWith)
	}
	if step == 1 && (r.selections != 229224 || r.laterCopies != 13853 || r.fragments != 123177 || r.laterWrongWithout != 1605) {
		t.Errorf("swept %d selections, %d later copies (%d shared otherwise without the lead), %d fragments; want 229,224, 13,853 (1,605), 123,177",
			r.selections, r.laterCopies, r.laterWrongWithout, r.fragments)
	}
}

// The same sweep over the WEB, BSB and WEBC caches the app keeps, every verse,
// with the whole quotation compared for the fragments too. LOCAL-ONLY and
// opt-in by BIBLETEXT_EXHAUSTIVE=1, as the phrase sweep over them is; about
// six minutes a translation. With the lead none is wrong. Without it the
// later copies and the pieces of a word that share another place's answer
// are pinned: the WEB's 14,209 of 185,053 and 12,925 of 1,160,304, the
// BSB's 11,785 of 145,335 and 11,218 of 1,125,726, the WEBC's 15,100 of
// 204,817 and 13,764 of 1,325,427. The pieces of a word are what the
// fallback leaves without a lead, once it places a piece in its span's
// verses (placeInSpan): the repeats inside them. Completed from the first
// copy of their letters in the chapter, as before that, 118,136, 106,972
// and 127,888 shared another place's answer.
func TestEverySelectionIsPlacedByItsLeadOverLocalCaches_LocalOnly(t *testing.T) {
	if os.Getenv("BIBLETEXT_EXHAUSTIVE") == "" {
		t.Skip("local-only and opt-in: every one-to-three-word selection and one-word fragment of three translation caches; run with BIBLETEXT_EXHAUSTIVE=1")
	}
	without := map[string][2]int{"web": {14209, 12925}, "bsb": {11785, 11218}, "webc": {15100, 13764}}
	for _, c := range localPublicDomainCaches(t) {
		r := leadSweep(t, c.bd, 1, true, c.id)
		if r.wrongWith != 0 || r.fragmentsWrongWith != 0 {
			t.Errorf("%s: with the lead, %d selections and %d fragments were not shared from their own place", c.id, r.wrongWith, r.fragmentsWrongWith)
		}
		if w := without[c.id]; r.laterWrongWithout != w[0] || r.fragmentsWrongWithout != w[1] {
			t.Errorf("%s: without the lead %d later copies and %d fragments shared another place's answer; pinned %d and %d",
				c.id, r.laterWrongWithout, r.fragmentsWrongWithout, w[0], w[1])
		}
	}
}

// midWordDrags is every drag of two and three words in every step-th verse of
// bd begun one letter into its first word or ended one letter short of its
// last, where that word has three letters or more: the drags whose letters
// can also stand whole later in the verse, which the text alone placed at
// that later copy. With its lead each must be placed at its own place and
// cut back there to its whole words: begun inside a word, from the token
// after it; ended inside one, to the end of the token before it, without the
// clause mark the end repair drops. It returns how many drags it made, how
// many were placed otherwise with the lead, and how many without it.
func midWordDrags(t *testing.T, bd *BibleData, step int, label string) (drags, wrongWith, elsewhereWithout int) {
	t.Helper()
	shown := 0
	for _, book := range bd.Books {
		for _, ch := range bd.GetChapterNumbersForBook(book) {
			st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: ch}
			corpus, spans := chapterProseIn(st, book, ch)
			for vi := 0; vi < len(spans); vi += step {
				sp := spans[vi]
				region := corpus[sp.start:sp.end]
				words := leadWords(region)
				for n := 2; n <= 3; n++ {
					for i := 0; i+n <= len(words); i++ {
						first, last := words[i], words[i+n-1]
						type drag struct {
							a, b, wantAt, wantEnd int
						}
						var ds []drag
						if r := []rune(region[first.start:first.end]); len(r) >= 3 {
							_, size := utf8.DecodeRuneInString(region[first.start:])
							// Begun inside the first word: kept from the next token.
							next := strings.IndexByte(region[first.start:], ' ')
							ds = append(ds, drag{first.start + size, last.end, first.start + next + 1, last.end})
						}
						if r := []rune(region[last.start:last.end]); len(r) >= 3 {
							_, size := utf8.DecodeLastRuneInString(region[:last.end])
							// Ended inside the last word: kept to the token before it.
							prev := strings.TrimRight(region[:strings.LastIndexByte(region[:last.start], ' ')], ",;:— ")
							ds = append(ds, drag{first.start, last.end - size, first.start, len(prev)})
						}
						for _, d := range ds {
							if d.wantEnd <= d.wantAt {
								continue
							}
							// Between two letters, so the cut is inside a word of
							// letters: one at an apostrophe ("I’m" less its "I")
							// asks the repairs a question of their own.
							if r, _ := utf8.DecodeRuneInString(region[d.a:]); !isWordRune(r) {
								continue
							}
							if r, _ := utf8.DecodeLastRuneInString(region[:d.b]); !isWordRune(r) {
								continue
							}
							drags++
							sel := region[d.a:d.b]
							want := region[d.wantAt:d.wantEnd]
							span := selSpanWithLead(sp.verse, sp.verse, leadAt(corpus, sp, sp.start+d.a))
							text, lo, hi, at, ok := normalizeShareSelectionProse(st, book, ch, corpus, spans, sel, span)
							if !ok || text != want || at != sp.start+d.wantAt || lo != sp.verse || hi != sp.verse {
								wrongWith++
								if shown < 5 {
									shown++
									t.Logf("%s: %s %d:%d %q with its lead: (%q, %d–%d, at %d, ok %v), want %q at %d",
										label, book, ch, sp.verse, sel, text, lo, hi, at, ok, want, sp.start+d.wantAt)
								}
								continue
							}
							if _, _, _, at, ok := normalizeShareSelectionProse(st, book, ch, corpus, spans, sel, selSpanFromNative(sp.verse, sp.verse)); !ok || at != sp.start+d.wantAt {
								elsewhereWithout++
							}
						}
					}
				}
			}
		}
	}
	t.Logf("%s: %d drags begun or ended inside a word; with the lead %d placed otherwise, without it %d", label, drags, wrongWith, elsewhereWithout)
	return drags, wrongWith, elsewhereWithout
}

// The drags begun or ended inside a word over the embedded Gospels, 237,232
// of them: with the lead each is cut back to its whole words at its own
// place, the shape of "leavened bread shall" at scale. Without it 4,050 are
// placed at another copy of their letters, or not placed at all and left to
// the fallback; the count is pinned.
func TestEveryDragBegunOrEndedInsideAWordIsCutBackAtItsOwnPlace(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	step := 1
	if testing.Short() {
		step = 7
	}
	drags, wrongWith, elsewhereWithout := midWordDrags(t, bd, step, "Gospels")
	if drags == 0 || wrongWith != 0 {
		t.Errorf("%d of %d drags begun or ended inside a word were not cut back at their own place with the lead", wrongWith, drags)
	}
	if step == 1 && (drags != 237232 || elsewhereWithout != 4050) {
		t.Errorf("%d drags, %d placed otherwise without the lead; want 237,232 and 4,050", drags, elsewhereWithout)
	}
	if elsewhereWithout == 0 {
		t.Errorf("control: without the lead every one of %d drags was placed at its own place; some are expected elsewhere", drags)
	}
}

// The same drags over the WEB, BSB and WEBC caches, every verse, half a
// minute. LOCAL-ONLY and opt-in by BIBLETEXT_EXHAUSTIVE=1. With the lead none
// is placed otherwise. Without it the WEB's 61,234 of 2,249,198, the BSB's
// 38,786 of 2,163,292 and the WEBC's 65,616 of 2,560,919 are placed at
// another copy of their letters or not placed; pinned.
func TestEveryDragBegunOrEndedInsideAWordIsCutBackAtItsOwnPlaceOverLocalCaches_LocalOnly(t *testing.T) {
	if os.Getenv("BIBLETEXT_EXHAUSTIVE") == "" {
		t.Skip("local-only and opt-in: every drag of two and three words begun or ended inside a word of three translation caches; run with BIBLETEXT_EXHAUSTIVE=1")
	}
	without := map[string]int{"web": 61234, "bsb": 38786, "webc": 65616}
	for _, c := range localPublicDomainCaches(t) {
		_, wrongWith, elsewhereWithout := midWordDrags(t, c.bd, 1, c.id)
		if wrongWith != 0 {
			t.Errorf("%s: with the lead, %d drags begun or ended inside a word were not cut back at their own place", c.id, wrongWith)
		}
		if want, ok := without[c.id]; ok && elsewhereWithout != want {
			t.Errorf("%s: without the lead %d drags were placed otherwise; pinned %d", c.id, elsewhereWithout, want)
		}
	}
}
