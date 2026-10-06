package bibletext

// The share pipeline's LATER stages must read the selected copy too. The span
// fix taught the citation which copy of repeated wording was selected; these
// pin the same discipline through the three stages that used to re-find the
// text first-match (sweep findings share.go:332/:565/:846): the trailing-
// sentence completion, the poem/paragraph line-break restore, and the original
// sentence terminal. Each test's fixture puts the SAME words in two places
// with different surroundings, selects the SECOND copy by position, and
// demands the second copy's surroundings — then pins the zero-span path's
// old first-match answer as the recorded reason the offset is threaded.

import (
	"strings"
	"testing"
)

// twoCopyState: the refrain's first copy sits mid-sentence (words follow it),
// the second copy ends its sentence — and in verse 3 the copies differ in
// TERMINAL too (a question), for the terminal test.
func twoCopyState() *AppState {
	bd := &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {136: {
			{BookName: "Psalms", Book: "Psalms", Chapter: 136, Verse: 1,
				Text: "Sing to the LORD with thanksgiving and with joy."},
			{BookName: "Psalms", Book: "Psalms", Chapter: 136, Verse: 2,
				Text: "Sing to the LORD with thanksgiving."},
			{BookName: "Psalms", Book: "Psalms", Chapter: 136, Verse: 3,
				Text: "Will you not sing to him? Sing to the LORD with thanksgiving?"},
		}}},
	}
	return &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 136}
}

// The completion: the selection is every word of verse 2's sentence, missing
// only its period. Located at verse 2 the completion appends it; located at
// verse 1's copy (where "and with joy." follows) a word intervenes and the
// completion must refuse — which is exactly what the zero-span path does,
// pinned below as the old defect.
func TestCompleteTrailingSentenceReadsTheSelectedCopy(t *testing.T) {
	st := twoCopyState()
	sel := "Sing to the LORD with thanksgiving"

	text, cite, _, _ := prepareShareQuote(st, sel, selSpanFromNative(2, 2))
	if text != sel+"." {
		t.Errorf("positional: completion = %q, want the period appended (verse 2's copy is sentence-final)", text)
	}
	if cite != "Psalms 136:2" {
		t.Errorf("positional: cite = %q, want Psalms 136:2", cite)
	}

	// The zero span locates verse 1's copy, where "and with joy" follows — no
	// completion, and the citation names verse 1: the recorded old behavior.
	text, cite, _, _ = prepareShareQuote(st, sel, selSpan{})
	if text != sel {
		t.Errorf("zero span: completion = %q, want the selection unchanged (a word follows the first copy)", text)
	}
	if cite != "Psalms 136:1" {
		t.Errorf("zero span: cite = %q — the misattribution the span exists to fix", cite)
	}
}

// The terminal: verse 3's copy of the words ends its sentence with a question
// mark; verse 2's identical words end with a period. The four-dot slot must
// carry the SELECTED copy's terminal.
func TestOriginalSentenceTerminalReadsTheSelectedCopy(t *testing.T) {
	st := twoCopyState()
	sel := "Sing to the LORD with"

	_, _, at, baseLen := prepareShareQuote(st, sel, selSpanFromNative(3, 3))
	if at < 0 {
		t.Fatal("precondition: the span locate should pin the selection in verse 3")
	}
	if got := originalSentenceTerminal(st, sel, at, baseLen); got != '?' {
		t.Errorf("positional terminal = %q, want '?' (verse 3's sentence)", got)
	}
	// Unlocated, the scan runs from verse 1's copy: a period. The old answer.
	if got := originalSentenceTerminal(st, sel, -1, 0); got != '.' {
		t.Errorf("zero span terminal = %q, want the first copy's '.'", got)
	}
}

// The line-break restore: the same words appear inside one poem line in verse 1
// and CROSSING an authored line break in verse 2. The share of verse 2's copy
// must break where verse 2 breaks; the located offset is what knows that.
func TestRestoreShareLineBreaksReadsTheSelectedCopy(t *testing.T) {
	bd := &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {136: {
			{BookName: "Psalms", Book: "Psalms", Chapter: 136, Verse: 1,
				Text: "praise him sun and moon in the heights above."},
			{BookName: "Psalms", Book: "Psalms", Chapter: 136, Verse: 2,
				Text: "praise him sun\nand moon in the heights beneath."},
		}}},
	}
	st := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 136}
	sel := "praise him sun and moon"

	text, _, at, baseLen := prepareShareQuote(st, sel, selSpanFromNative(2, 2))
	restored := restoreShareLineBreaks(st, text, at, baseLen)
	if !strings.Contains(restored, "sun\nand") {
		t.Errorf("positional restore = %q, want verse 2's authored break after %q", restored, "sun")
	}

	// The zero span locates verse 1's copy — one unbroken line: no break
	// restored. The recorded old behavior for the same drag.
	text, _, at, baseLen = prepareShareQuote(st, sel, selSpan{})
	if restored := restoreShareLineBreaks(st, text, at, baseLen); strings.Contains(restored, "\n") {
		t.Errorf("zero span restore = %q injected a break the first copy does not have", restored)
	}
}

// The coordinate invariant every positional stage rests on: chapterProse and
// chapterShareStructure flatten to the SAME string, so an offset located in one
// indexes the other. If these ever drift, the offset threading breaks silently
// — this is the loud version.
// The two corpora must read character for character alike, because one is
// searched for text taken out of the other: restoreShareLineBreaks locates the
// quote (built from chapterProse) inside chapterShareStructure to know where
// the authored breaks belong, and a miss returns the text unbroken rather than
// failing.
//
// psalm23SmallCapsState is in the list deliberately. The three fixtures below
// it carry no SmallCaps span, and neither does any verse of the shipped WEB
// gospels — so when chapterProse moved to the outbound form and
// chapterShareStructure did not, this test kept passing while every NKJV psalm
// quietly lost its line breaks. A coupling test is only worth the fixture that
// can break it.
func TestChapterProseAndShareStructureAgree(t *testing.T) {
	for _, st := range []*AppState{twoCopyState(), refrainChapterState(), sampleState(), psalm23SmallCapsState()} {
		prose, _ := chapterProse(st)
		flat, _ := chapterShareStructure(st)
		if prose != flat {
			t.Fatalf("chapterProse and chapterShareStructure diverge for %s %d:\n %q\nvs %q",
				st.CurrentBook, st.CurrentChapter, prose, flat)
		}
	}
}

// stripVerseMarkersProse reads each verse's body out of the chapter the
// pipeline has already built instead of drawing every verse again, so a span's
// slice of chapterProse must be exactly the body the strip compares a token's
// aftermath with — collapseSpaces(verseSharedText(v)) — verse for verse, in
// chapter order, with the verses whose body is empty left out.
func TestChapterProseSpansAreTheVerseBodies(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	states := []*AppState{twoCopyState(), refrainChapterState(), sampleState(), psalm23SmallCapsState()}
	for _, book := range bd.Books {
		for _, ch := range bd.GetChapterNumbersForBook(book) {
			states = append(states, &AppState{Bible: bd, CurrentBook: book, CurrentChapter: ch})
		}
	}
	for _, st := range states {
		corpus, spans := chapterProse(st)
		i := 0
		for _, v := range st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter) {
			body := collapseSpaces(verseSharedText(v))
			if body == "" {
				continue
			}
			if i >= len(spans) {
				t.Fatalf("%s %d: verse %d has a body but no span", st.CurrentBook, st.CurrentChapter, v.Verse)
			}
			if sp := spans[i]; sp.verse != v.Verse || corpus[sp.start:sp.end] != body {
				t.Fatalf("%s %d: span %d is verse %d %q, want verse %d %q",
					st.CurrentBook, st.CurrentChapter, i, sp.verse, corpus[sp.start:sp.end], v.Verse, body)
			}
			i++
		}
		if i != len(spans) {
			t.Fatalf("%s %d: %d spans for %d verse bodies", st.CurrentBook, st.CurrentChapter, len(spans), i)
		}
	}
}

// The verbs agree on the dangling-number drag: a selection swept just past the
// next verse's NUMBER spans (2,3) natively, but no word of verse 3 is quoted —
// the share cites verse 2, and the crossref resolver must name the same verse,
// not pull verse 3's references.
func TestCrossrefAndShareAgreeOnDanglingNumber(t *testing.T) {
	st := refrainChapterState()
	sel := refrain + " 3"
	span := selSpanFromNative(2, 3)

	if _, cite, _, _ := prepareShareQuote(st, sel, span); cite != "Psalms 136:2" {
		t.Fatalf("share cite = %q, want Psalms 136:2 (the dangling number introduces nothing)", cite)
	}
	vs := selectionVerses(st, sel, span)
	if len(vs) != 1 || vs[0].Verse != 2 {
		t.Errorf("selectionVerses = %v, want exactly verse 2 — the crossref panel would cite a verse the share refuses to name", vs)
	}

	// A selection that is ONLY the number still resolves to that verse: the
	// normalize declines it, and the span verbatim is the honest reading.
	if vs := selectionVerses(st, "3", selSpanFromNative(3, 3)); len(vs) != 1 || vs[0].Verse != 3 {
		t.Errorf("number-only selection = %v, want verse 3 (span verbatim when the normalize declines)", vs)
	}
}

// A single partial word: the normalize declines it (no whole word to quote) but
// the span still knows the verse — the LINK must carry it, or the message pairs
// a verse-level citation with a chapter-level URL.
func TestShareLinkKeepsSpanVersesWhenNormalizeDeclines(t *testing.T) {
	st := refrainChapterState()
	if _, _, _, _, ok := normalizeShareSelection(st, "lov", selSpanFromNative(2, 2)); ok {
		t.Fatal("precondition: a single partial word should decline to normalize")
	}
	lo, hi := linkVersesForSelection(st, "lov", selSpanFromNative(2, 2))
	if lo != 2 || hi != 2 {
		t.Errorf("link verses = %d-%d, want 2-2 from the span", lo, hi)
	}
	// And without a span it stays an honest chapter link, as ever.
	if lo, hi := linkVersesForSelection(st, "lov", selSpan{}); lo != 0 || hi != 0 {
		t.Errorf("zero span link verses = %d-%d, want 0-0", lo, hi)
	}
}

// A drag that opens on the question mark ending Matthew 25:38 and runs into
// 25:39 is honestly spanned 38–39. The mark is rightly dropped from the
// quotation, and what is left is verse 39's opening, "When did we see you" —
// which is verse 38's opening too. Searched for again from the start of the
// span's region after the mark came off, the words were found in verse 38 and
// cited there. The control is the same words dragged from verse 39's number.
func TestADragOpenedOnThePreviousVersesMarkCitesTheVerseDragged(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	st := &AppState{Bible: bd, CurrentBook: "Matthew", CurrentChapter: 25}
	const words = "When did we see you"

	text, cite, _, _ := prepareShareQuote(st, "39 "+words, selSpanFromNative(39, 39))
	if text != words || cite != "Matthew 25:39" {
		t.Fatalf("control: dragged from verse 39's number, shared (%q, %q)", text, cite)
	}
	for _, sel := range []string{"? 39 " + words, "? 39 " + words + " si"} {
		text, cite, at, _ := prepareShareQuote(st, sel, selSpanFromNative(38, 39))
		if text != words || cite != "Matthew 25:39" {
			t.Errorf("%q, span 38–39: shared (%q, %q), want (%q, Matthew 25:39)", sel, text, cite, words)
		}
		corpus, spans := chapterProse(st)
		if v39 := spans[38]; v39.verse != 39 || at != v39.start {
			t.Errorf("%q: placed at %d, want verse 39's start %d (%q)", sel, at, v39.start, corpus[v39.start:v39.start+len(words)])
		}
	}
}

// The same slide in the first search, before any repair: John 8:23 says
// "this world. I" twice, and a drag from its last words across verse 24's
// number shares "this world. I" too. Found first in the span's region, the
// words were placed at the copy inside verse 23 and cited 8:23 alone, though
// the reader dragged into verse 24. The verse number in the selection says
// where the words are, and the citation takes both verses.
func TestADragAcrossAVerseNumberIsPlacedWhereTheNumberSays(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	st := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 8}

	// The control: the words of verse 23 alone, with no number, stay in 23.
	if _, cite, _, _ := prepareShareQuote(st, "of this world.", selSpanFromNative(23, 23)); cite != "John 8:23" {
		t.Fatalf("control: verse 23's own words cited %q", cite)
	}
	text, cite, at, _ := prepareShareQuote(st, "this world. 24 I", selSpanFromNative(23, 24))
	if text != "this world. I" || cite != "John 8:23–24" {
		t.Errorf("shared (%q, %q), want (%q, John 8:23–24)", text, cite, "this world. I")
	}
	_, spans := chapterProse(st)
	if want := spans[23].start - len("this world. "); at != want {
		t.Errorf("placed at %d, want %d: the words that run into verse 24's number", at, want)
	}
}

// A verse number is only the page's word for where a verse begins when the
// span agrees with it. A drag begun on the second digit of Mark 7:29's
// number arrives as "9 He said to", spanned 29–29, and verse 9 opens with
// the same words, so the marker strip takes the "9 " as verse 9's number.
// Placed where that number says, the words were cited 7:9, a verse the
// reader never touched; the span puts them in 29, where they are. The same
// in Matthew 4, where 4:1 and 4:11 both open "Then". The controls are the
// same words dragged from the whole number.
func TestADragBegunOnAVerseNumbersLaterDigitStaysInItsSpan(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	for _, c := range []struct {
		book            string
		chapter, verse  int
		whole, fromLast string
		want            string
	}{
		{"Mark", 7, 29, "29 He said to", "9 He said to", "He said to"},
		{"Matthew", 4, 11, "11 Then", "1 Then", "Then"},
	} {
		st := &AppState{Bible: bd, CurrentBook: c.book, CurrentChapter: c.chapter}
		wantCite := verseRangeCitation(c.book, c.chapter, c.verse, c.verse)
		_, spans := chapterProse(st)
		start := -1
		for _, sp := range spans {
			if sp.verse == c.verse {
				start = sp.start
			}
		}
		span := selSpanFromNative(c.verse, c.verse)
		if text, cite, at, _ := prepareShareQuote(st, c.whole, span); text != c.want || cite != wantCite || at != start {
			t.Fatalf("control %q: shared (%q, %q) at %d, want (%q, %s) at %d", c.whole, text, cite, at, c.want, wantCite, start)
		}
		text, cite, at, _ := prepareShareQuote(st, c.fromLast, span)
		if text != c.want || cite != wantCite || at != start {
			t.Errorf("%q, span %d–%d: shared (%q, %q) at %d, want (%q, %s) at %d",
				c.fromLast, c.verse, c.verse, text, cite, at, c.want, wantCite, start)
		}
	}
}
