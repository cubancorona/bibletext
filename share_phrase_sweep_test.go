package bibletext

// A DRAG FROM WORD TO WORD SHARES THE WORDS IT TOOK. The share code places a
// selection once and never searches for it again (normalizeShareSelection),
// so the one search has to find a copy a drag from word to word could have
// made. It took the first copy of any kind, and where that stood inside a
// longer word, "the wicked" in "the wickedness", the end repair cut the
// reader's last word off; with no span the verse of that copy was cited too.

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode"
)

// phraseWords is a phrase without the marks at either end, which the share
// code may drop (an orphan full stop at the front, a clause mark at the end)
// or keep (a closing question mark) without changing its words.
func phraseWords(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// sweepPhrases selects every phrase of two, three and five words inside each
// step-th verse of bd, once with the span a pane reports for a drag inside
// one verse and once with none, as the plain-text fallback the Mac and the
// iPhone and iPad show hands a selection over. Each must be placed, and
// share its own words; with the span it must cite its verse. With no span
// the citation is that of the first copy that splits no word, which for
// wording the chapter repeats need not be the reader's (BACKLOG.md, "A
// Psalm superscription can be shared as scripture"). It returns how many
// selections it made and how many were wrong, logging the first few.
func sweepPhrases(t *testing.T, bd *BibleData, books []string, step int, label string) (selections, wrong int) {
	t.Helper()
	shown := 0
	for _, book := range books {
		for _, ch := range bd.GetChapterNumbersForBook(book) {
			st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: ch}
			corpus, spans := chapterProse(st)
			for vi := 0; vi < len(spans); vi += step {
				sp := spans[vi]
				words := strings.Fields(corpus[sp.start:sp.end])
				for _, n := range []int{2, 3, 5} {
					for i := 0; i+n <= len(words); i++ {
						sel := strings.Join(words[i:i+n], " ")
						for _, span := range []selSpan{{sp.verse, sp.verse}, {}} {
							selections++
							text, lo, hi, _, ok := normalizeShareSelectionProse(st, book, ch, corpus, spans, sel, span)
							bad := ""
							switch {
							case !ok:
								bad = "not placed"
							case phraseWords(text) != phraseWords(sel):
								bad = fmt.Sprintf("shared %q", text)
							case span.valid() && (lo != sp.verse || hi != sp.verse):
								bad = fmt.Sprintf("cited %d–%d", lo, hi)
							}
							if bad == "" {
								continue
							}
							wrong++
							if shown < 5 {
								shown++
								t.Logf("%s: %s %d:%d %q span %d–%d: %s", label, book, ch, sp.verse, sel, span.lo, span.hi, bad)
							}
						}
					}
				}
			}
		}
	}
	return selections, wrong
}

// Every phrase of two, three and five words in every verse of the embedded
// Gospels, 214,115 of them, each with the span and without. Placed at the
// first copy of any kind, 59 with the span and 1,247 without were wrong: with
// the span Matthew 15:14's "blind guide" shared "blind", the verse having
// "blind guides" first, and with none Matthew 3:13's "to the" shared "to".
// None is now.
func TestEveryPhraseInAVerseSharesItsOwnWords(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	step := 1
	if testing.Short() {
		step = 7
	}
	selections, wrong := sweepPhrases(t, bd, bd.Books, step, "Gospels")
	t.Logf("%d selections, %d wrong", selections, wrong)
	if wrong != 0 {
		t.Errorf("%d of %d phrases shared other words than the ones selected, or cited another verse with the span", wrong, selections)
	}
}

// The shape itself, on an invented verse that holds the phrase first inside
// a longer word: the drag's own words are shared, from the copy that splits
// no word, with the span and without. The control is the same verse without
// the longer word first, which the first copy of any kind answered too.
func TestAPhraseFirstFoundInsideALongerWordSharesItsOwnWords(t *testing.T) {
	for _, c := range []struct {
		name, text string
	}{
		{"the longer word first", "The proud boast of the wickedness they plan, but the wicked will fall."},
		{"control: no longer word", "The proud boast of their plans, but the wicked will fall."},
	} {
		bd := &BibleData{
			Books: []string{"Psalms"},
			Verses: map[string]map[int][]Verse{"Psalms": {7: {
				{BookName: "Psalms", Book: "Psalms", Chapter: 7, Verse: 1, Text: "Hear me, and answer my cry."},
				{BookName: "Psalms", Book: "Psalms", Chapter: 7, Verse: 2, Text: c.text},
			}}},
		}
		st := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 7}
		for _, span := range []selSpan{selSpanFromNative(2, 2), {}} {
			quote, cite, _, _ := prepareShareQuote(st, "the wicked", span)
			if quote != "the wicked" || cite != "Psalms 7:2" {
				t.Errorf("%s, span %d–%d: shared (%q, %q), want (\"the wicked\", \"Psalms 7:2\")", c.name, span.lo, span.hi, quote, cite)
			}
		}
	}
}

// What the search costs. A drag begun or ended inside a word arrives as
// letters, and the text alone cannot tell "leavened bread shall", begun two
// letters into "Unleavened", from a phrase whose first copy stands inside a
// longer word: the region's first copy that splits no word is taken. In a
// verse that has the same letters as whole words further on, the drag is
// quoted from there: the words the reader's letters spell, not the words the
// drag took whole, and with the span the citation is the same verse. Where
// the verse has no such copy, the reader's copy is taken and cut back to its
// whole words, as before. This pins the trade-off, not a preference: telling
// the two apart needs the selection's place in its verse, which the panes do
// not hand over.
func TestADragBegunOrEndedInsideAWordIsQuotedFromAWholeCopyInItsVerse(t *testing.T) {
	for _, c := range []struct {
		name, text, sel     string
		wantQuote, wantCite string
	}{
		{"begun inside a word, the letters whole later",
			"Unleavened bread shall be eaten for seven days, and no leavened bread shall be seen with you.",
			"leavened bread shall", "leavened bread shall", "Exodus 13:7"},
		{"control: begun inside a word, no whole copy",
			"Unleavened bread shall be eaten for seven days.",
			"leavened bread shall", "bread shall", "Exodus 13:7"},
		{"ended inside a word, the letters whole later",
			"He will command them that they keep the way, that the promise may come.",
			"that the", "that the", "Exodus 13:7"},
		{"control: ended inside a word, no whole copy",
			"He will command them that they keep the way.",
			"that the", "that", "Exodus 13:7"},
	} {
		st := &AppState{Bible: &BibleData{
			Books: []string{"Exodus"},
			Verses: map[string]map[int][]Verse{"Exodus": {13: {
				{BookName: "Exodus", Book: "Exodus", Chapter: 13, Verse: 6, Text: "Seven days you shall eat."},
				{BookName: "Exodus", Book: "Exodus", Chapter: 13, Verse: 7, Text: c.text},
			}}},
		}, CurrentBook: "Exodus", CurrentChapter: 13}
		quote, cite, _, _ := prepareShareQuote(st, c.sel, selSpanFromNative(7, 7))
		if quote != c.wantQuote || cite != c.wantCite {
			t.Errorf("%s: %q with the span shared (%q, %q), want (%q, %q)", c.name, c.sel, quote, cite, c.wantQuote, c.wantCite)
		}
	}
}

// A drag begun inside the last word of verse 8 and ended on verse 9's number
// arrives as "in Christ 9". The number comes off, and with it verse 9 leaves
// the region the selection is looked for in: its only part in the drag was
// its number. Otherwise the region's preference for a copy that splits no
// word took verse 9's own "in Christ" over the reader's, the end of "gain
// Christ", and cited verse 9, none of whose words were taken. The reader's
// copy is cut back to its whole word and cited as verse 8. The control is the
// same drag begun on a word boundary, which quotes both words from verse 8.
func TestADragEndedOnTheNextVersesNumberIsNotPlacedInThatVerse(t *testing.T) {
	st := &AppState{Bible: &BibleData{
		Books: []string{"Philippians"},
		Verses: map[string]map[int][]Verse{"Philippians": {3: {
			{BookName: "Philippians", Book: "Philippians", Chapter: 3, Verse: 8, Text: "I count all things as loss, that I may gain Christ"},
			{BookName: "Philippians", Book: "Philippians", Chapter: 3, Verse: 9, Text: "and be found in Christ, not having a righteousness of my own."},
		}}},
	}, CurrentBook: "Philippians", CurrentChapter: 3}
	for _, c := range []struct{ sel, wantQuote, wantCite string }{
		{"in Christ 9", "Christ", "Philippians 3:8"},
		{"in Christ\n9", "Christ", "Philippians 3:8"},
		{"gain Christ 9", "gain Christ", "Philippians 3:8"},
	} {
		quote, cite, _, _ := prepareShareQuote(st, c.sel, selSpanFromNative(8, 9))
		if quote != c.wantQuote || cite != c.wantCite {
			t.Errorf("%q, span 8–9, shared (%q, %q), want (%q, %q)", c.sel, quote, cite, c.wantQuote, c.wantCite)
		}
	}

	// Only words the verses before the number hold make it leave the region.
	// A drag from a heading's last word to the number below holds no end of
	// a verse; the word is found in the region as before, in the verse below,
	// not at the chapter's first copy, which the drag never touched. What is
	// quoted for it is the headings' question, so only the citation is held.
	heading := &AppState{Bible: &BibleData{
		Books: []string{"Genesis"},
		Verses: map[string]map[int][]Verse{"Genesis": {13: {
			{BookName: "Genesis", Book: "Genesis", Chapter: 13, Verse: 12, Text: "Abram lived in the land of Canaan."},
			{BookName: "Genesis", Book: "Genesis", Chapter: 13, Verse: 13, Text: "Now the men of the city were wicked."},
			{BookName: "Genesis", Book: "Genesis", Chapter: 13, Verse: 14, Text: "After Lot had gone, the LORD said to Abram, “Lift up your eyes.”"},
		}}},
		Headings: map[string]map[int][]Heading{"Genesis": {13: {
			{Text: "The Promise to Abram", Style: "s", BeforeVerse: 14},
		}}},
	}, CurrentBook: "Genesis", CurrentChapter: 13}
	for _, sel := range []string{"Abram\n14", "Abram\n14 "} {
		if _, cite, _, _ := prepareShareQuote(heading, sel, selSpanFromNative(13, 14)); cite != "Genesis 13:14" {
			t.Errorf("%q, span 13–14, cited %q, want \"Genesis 13:14\"", sel, cite)
		}
	}
}

// With a span, a selection its verses do not hold is not a phrase of those
// verses, and is found in the chapter at its first copy of any kind, as it
// was before the region preferred a copy that splits no word, or, where it
// runs on into the heading below them, placed by that heading
// (verseEndBeforeHeading). Taking the first whole copy in the chapter moved
// each of these to a verse the drag never touched:
//
//   - a drag from the end of verse 13 into the first word of the heading
//     below, whose copy in the corpus runs into verse 14's "Then": it was
//     cited 32–33, where verse 32 ends as 13 does and verse 33 opens "The".
//     It is now placed at the end of verse 13, the heading's word taken off;
//   - verse 20's number selected on its own, which an earlier verse holds as
//     a word after "200": it was cited verse 14;
//   - the first word of a heading selected on its own, with the span the
//     panes report for it, the verse above: it was cited verse 33, the first
//     to hold "The" as a word.
//
// Each is checked by its citation only; what is quoted for the last two is
// older than the search and is the headings' and the numbers' own question.
func TestASelectionItsVersesDoNotHoldIsNotMovedToAWholeCopyElsewhere(t *testing.T) {
	verses := []Verse{
		{Verse: 12, Text: "Then the earth yielded grain, and God saw that it was good."},
		{Verse: 13, Text: "And all Judah stood before the LORD."},
		{Verse: 14, Text: "Then the Spirit came upon him, 200 female goats and 20 male goats."},
		{Verse: 20, Text: "And they rose early in the morning and went out."},
		{Verse: 32, Text: "He did what was right in the eyes of the LORD."},
		{Verse: 33, Text: "The high places, however, were not removed."},
	}
	for i := range verses {
		verses[i].BookName, verses[i].Book, verses[i].Chapter = "2 Chronicles", "2 Chronicles", 20
	}
	st := &AppState{Bible: &BibleData{
		Books:  []string{"2 Chronicles"},
		Verses: map[string]map[int][]Verse{"2 Chronicles": {20: verses}},
		Headings: map[string]map[int][]Heading{"2 Chronicles": {20: {
			{Text: "The Spirit Comes", Style: "s", BeforeVerse: 14},
		}}},
	}, CurrentBook: "2 Chronicles", CurrentChapter: 20}
	for _, c := range []struct {
		sel      string
		lo, hi   int
		wantCite string
	}{
		{"the LORD.\nThe", 13, 13, "2 Chronicles 20:13"},
		{"20", 20, 20, "2 Chronicles 20:20"},
		{"The", 13, 13, "2 Chronicles 20:13"},
	} {
		if _, cite, _, _ := prepareShareQuote(st, c.sel, selSpanFromNative(c.lo, c.hi)); cite != c.wantCite {
			t.Errorf("%q, span %d–%d, cited %q, want %q", c.sel, c.lo, c.hi, cite, c.wantCite)
		}
	}
	// The control: with no span the chapter is searched for a copy that
	// splits no word first, and verse 32's "the LORD. The" is taken, as any
	// selection with no span is placed at the first such copy.
	if _, cite, _, _ := prepareShareQuote(st, "the LORD.\nThe", selSpan{}); cite != "2 Chronicles 20:32–33" {
		t.Errorf("control: \"the LORD.\\nThe\" with no span cited %q, want \"2 Chronicles 20:32–33\"", cite)
	}
}

// The same sweep over the public-domain translations' caches the app keeps,
// every verse of every book. LOCAL-ONLY and opt-in by BIBLETEXT_EXHAUSTIVE=1,
// as TestPassageShareEqualsSelectionRouteOverLocalCaches_LocalOnly is: the
// caches are downloaded data a CI checkout does not have. Two phrases of the
// BSB, each with the span and without, are held apart: each ends on a dash
// as its verse does, and the verse has the same words earlier with a word
// straight after the dash ("herself—or" in Numbers 5:14, "truth—and" in 2
// John 1). That copy splits no word and is taken, and the end repair reads
// the letter after the dash as the rest of a word and cuts the phrase to its
// first word, as it did before the search changed. With the search as it
// was, the WEB had 12,161 wrong, the BSB 9,339 and the WEBC 13,855.
func TestEveryPhraseInAVerseSharesItsOwnWordsOverLocalCaches_LocalOnly(t *testing.T) {
	if os.Getenv("BIBLETEXT_EXHAUSTIVE") == "" {
		t.Skip("local-only and opt-in: about twelve million selections over three translation caches, half a minute; " +
			"run with BIBLETEXT_EXHAUSTIVE=1")
	}
	known := map[string]int{"bsb": 4}
	for _, c := range localPublicDomainCaches(t) {
		selections, wrong := sweepPhrases(t, c.bd, c.bd.Books, 1, c.id)
		t.Logf("%s: %d selections, %d wrong", c.id, selections, wrong)
		if wrong != known[c.id] {
			t.Errorf("%s: %d selections shared other words than the ones selected, or cited another verse with the span; want %d", c.id, wrong, known[c.id])
		}
	}
}

// localPublicDomainCaches loads the WEB, BSB and WEBC caches the app keeps,
// found as TestPassageShareEqualsSelectionRouteOverLocalCaches_LocalOnly
// finds them: the current epoch's file, else the newest superseded one. A
// translation with no cache is skipped; one whose cache will not load is an
// error. With none present the test is skipped.
func localPublicDomainCaches(t *testing.T) []struct {
	id string
	bd *BibleData
} {
	t.Helper()
	var out []struct {
		id string
		bd *BibleData
	}
	for _, id := range []string{"web", "bsb", "webc"} {
		v, ok := versionByID(id)
		if !ok {
			t.Fatalf("version %q is not in the catalogue", id)
		}
		var bd *BibleData
		for _, own := range append([]string{cachePathForVersion(id)}, supersededCachePaths(v)...) {
			path := realCachePath(own)
			if _, err := os.Stat(path); err != nil {
				continue
			}
			loaded, err := loadBibleFromCache(path)
			if err != nil {
				t.Errorf("%s: a cache is present but will not load: %v", id, err)
				continue
			}
			bd = loaded
			break
		}
		if bd == nil {
			t.Logf("%s: no cache present, skipped", id)
			continue
		}
		out = append(out, struct {
			id string
			bd *BibleData
		}{id, bd})
	}
	if len(out) == 0 {
		t.Skip("local-only: no translation cache present")
	}
	return out
}
