package bibletext

// A drag begun inside a publisher's heading and run on through the number of
// the verse beneath it, into that verse's first words. The panes report it
// with a span from the verse above the heading to the verse beneath (the
// last number before the heading is the verse above's), and the lead they
// send reads its way through the heading and fails its check, so the
// selection's words decide. The heading's piece is in no verse. These must
// share the verse's words the drag reached, cited to that verse, and never
// the heading's words as Scripture.
//
// Every fixture is invented, shaped on what the BSB's headings do: one whose
// words open the verse beneath it, one whose first word is the second word
// of a later verse, one whose first word the verse beneath opens with too, a
// speaker's label that also stands inside an earlier verse, a heading that
// holds its own verse's number as a word, a stack of a psalm's title and a
// heading, and a verse that opens with an ellipsis.

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func headingRepairState() *AppState {
	v := func(n int, text string) Verse {
		return Verse{BookName: "Psalms", Book: "Psalms", Chapter: 140, Verse: n, Text: text}
	}
	bd := &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {140: {
			v(1, "The Keeper is near to all who call on him."),
			v(2, "He hears the cry of the weary and the poor."),
			v(3, "And Seed went out to the field at dawn."),
			v(4, "The field was sowed, and the rain came down. The Friends\nWe will rejoice and sing."),
			v(5, "He waited for the harvest, and Seed waited with him."),
			v(6, "We will make you a crown of the first grain."),
			v(7, "Do not move the old boundary stone."),
			v(8, "... and the grain was gathered into the barn."),
			v(9, "The barn was full, and He waited for all who came."),
		}}},
		Headings: map[string]map[int][]Heading{"Psalms": {140: {
			{Text: "The Keeper Is Near", Style: "s", BeforeVerse: 1},
			{Text: "Seed and Harvest", Style: "s", BeforeVerse: 2},
			{Text: "The Sons of Seed", Style: "s", BeforeVerse: 3},
			{Text: "The Seed Is Sowed", Style: "s", BeforeVerse: 5},
			{Text: "The Friends", Style: "s", BeforeVerse: 6},
			{Text: "Saying 7", Style: "s", BeforeVerse: 7},
			{Text: "He Gathers", Style: "s", BeforeVerse: 8},
			{Text: "The Barn", Style: "s", BeforeVerse: 9},
		}}},
		Superscriptions: map[string]map[int]Superscription{"Psalms": {140: {Text: "A song of the field."}}},
	}
	return &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 140}
}

func TestADragFromAHeadingIntoItsVerseSharesTheVersesWords(t *testing.T) {
	st := headingRepairState()
	for _, c := range []struct {
		name, raw   string
		lo, hi      int
		quote, cite string
	}{
		// The heading's words open the verse beneath it: the end trim took
		// the verse's words for the front of the heading the front trim had
		// just taken off, and the heading was shared as Scripture.
		{"the heading's words open its verse", "Keeper Is Near\n1 The Keeper", 1, 1,
			"“The Keeper . . . .”", "Psalms 140:1"},
		{"the heading whole", "The Keeper Is Near\n1 The", 1, 1, "“The . . . .”", "Psalms 140:1"},
		// A heading higher in the chapter opens with the verse's second
		// word, and the end trim took it: "And" alone was shared.
		{"an earlier heading opens with the verse's last word", "Sons of Seed\n3 And Seed", 2, 3,
			"“And Seed . . . .”", "Psalms 140:3"},
		// A heading lower in the chapter opens with the verse's first word:
		// the end trim took it, and the number alone was left.
		{"a later heading opens with the verse's word", "Sowed\n5 He", 4, 5, "“He . . . .”", "Psalms 140:5"},
		// Read as "d He waited", the drag's words stand together in verse 9,
		// outside the span, and were quoted from there.
		{"a one-letter piece", "d\n5 He waited", 4, 5, "“He waited . . . .”", "Psalms 140:5"},
		// A speaker's label stands inside verse 4 as well, before the same
		// words: the search took that copy.
		{"a label that also stands in an earlier verse", "The Friends\n6 We will", 5, 6,
			"“We will . . . .”", "Psalms 140:6"},
		{"a piece of the label", "riends\n6 We will", 5, 6, "“We will . . . .”", "Psalms 140:6"},
		// The heading holds its verse's number as a word: the first "7" is
		// the heading's.
		{"a heading that holds its number", "ying 7\n7 Do not", 6, 7, "“Do not . . . .”", "Psalms 140:7"},
		// A psalm's title over a heading over verse 1.
		{"a title and a heading", "of the field.\nThe Keeper Is Near\n1 The Keeper", 1, 1,
			"“The Keeper . . . .”", "Psalms 140:1"},
	} {
		t.Run(c.name, func(t *testing.T) {
			quote, cite := shareQuoteIn(st, "Psalms", 140, c.raw, selSpanFromNative(c.lo, c.hi))
			if quote != c.quote || cite != c.cite {
				t.Errorf("%q, spanned %d to %d, shared (%q, %q); want (%q, %q)", c.raw, c.lo, c.hi, quote, cite, c.quote, c.cite)
			}
		})
	}
}

// A heading's piece run on to the number of the verse beneath and to no
// word of it, the ellipsis a verse can open with only: the drag's words are
// the heading's, answered as a heading selected on its own is, the verse
// beneath and no quotation.
func TestAHeadingPieceRunOnToAVerseWithNoWordsQuotesNothing(t *testing.T) {
	st := headingRepairState()
	for _, raw := range []string{"Gathers\n8 ...", "e Gathers\n8"} {
		quote, cite, _, _ := prepareShareQuoteIn(st, "Psalms", 140, raw, selSpanFromNative(7, 8))
		if quote != "" || cite != "Psalms 140:8" {
			t.Errorf("%q, spanned 7 to 8, shared (%q, %q); want (\"\", \"Psalms 140:8\")", raw, quote, cite)
		}
	}
}

// The repair must leave the drags it is not for as they were: one that
// begins in a verse and runs into the heading below it, one that begins in a
// heading and runs through a verse into the next heading, and one that
// begins in a verse whose own last word is the heading's last letter.
func TestTheHeadingRepairKeepsTheOtherDrags(t *testing.T) {
	st := headingRepairState()
	for _, c := range []struct {
		raw         string
		lo, hi      int
		quote, cite string
	}{
		{"to all who call on him.\nSeed and Har", 1, 1, "“[T]o all who call on him.”", "Psalms 140:1"},
		{"Is Sowed\n5 He waited for the harvest, and Seed waited with him.\nThe Fri", 4, 5,
			"“He waited for the harvest, and Seed waited with him.”", "Psalms 140:5"},
		{"the weary and the poor.\nThe Sons of Seed\n3 And", 2, 3,
			"“[T]he weary and the poor. And . . . .”", "Psalms 140:2–3"},
	} {
		quote, cite := shareQuoteIn(st, "Psalms", 140, c.raw, selSpanFromNative(c.lo, c.hi))
		if quote != c.quote || cite != c.cite {
			t.Errorf("%q, spanned %d to %d, shared (%q, %q); want (%q, %q)", c.raw, c.lo, c.hi, quote, cite, c.quote, c.cite)
		}
	}
}

// A drag from the end of a verse across a STACK of headings, two the page
// draws one above the other over the next verse, into that verse's first
// words. Each heading was removed whole only where the number of the verse
// beneath stood straight after it, which the upper heading of a stack never
// has: the lower came off, the upper stayed, and the selection could not be
// placed, so the fallback quoted both headings as Scripture. The stack comes
// off whole, as cutOpeningMatter takes one a drag begins inside. The BSB has
// the shape under "Thirty Sayings of the Wise" and "Saying 1" over Proverbs
// 22:17, and under Hebrews 11:30's two; the wording here is invented.
func TestADragAcrossAStackOfHeadingsSharesTheVersesOnEitherSide(t *testing.T) {
	v := func(n int, text string) Verse {
		return Verse{BookName: "Proverbs", Book: "Proverbs", Chapter: 22, Verse: n, Text: text}
	}
	st := &AppState{Bible: &BibleData{
		Books: []string{"Proverbs"},
		Verses: map[string]map[int][]Verse{"Proverbs": {22: {
			v(16, "Whoever hoards the grain comes only to want."),
			v(17, "Turn your ear and hear the sayings of the wise."),
			v(18, "For it is pleasant to keep them within you."),
		}}},
		Headings: map[string]map[int][]Heading{"Proverbs": {22: {
			{Text: "Sayings of the Wise", Style: "s", BeforeVerse: 17},
			{Text: "Saying 1", Style: "s", BeforeVerse: 17},
		}}},
	}, CurrentBook: "Proverbs", CurrentChapter: 22}
	const quote, cite = "“[T]o want. Turn your . . . .”", "Proverbs 22:16–17"
	for _, c := range []struct {
		name, raw string
		span      selSpan
	}{
		{"the stack, spanned", "to want.\nSayings of the Wise\nSaying 1\n17 Turn your", selSpanFromNative(16, 17)},
		{"the stack, with no span", "to want.\nSayings of the Wise\nSaying 1\n17 Turn your", selSpan{}},
		{"the stack, with the lead", "to want.\nSayings of the Wise\nSaying 1\n17 Turn your",
			selSpanWithLead(16, 17, "16 Whoever hoards the grain comes only ")},
		// The control: the lower heading alone was always removed.
		{"the lower heading alone", "to want.\nSaying 1\n17 Turn your", selSpanFromNative(16, 17)},
	} {
		if q, ci := shareQuoteIn(st, "Proverbs", 22, c.raw, c.span); q != quote || ci != cite {
			t.Errorf("%s: %q shared (%q, %q); want (%q, %q)", c.name, c.raw, q, ci, quote, cite)
		}
	}
}

// Every drag of the public-domain translations' caches from the last one,
// two or three words of a verse across the whole of the matter the page
// draws below it, a heading or a stack of them, into the next verse's first
// one or two words, with the span the panes report and no lead, and with no
// span. Each must share the words on either side, and with the span cite
// both verses. LOCAL-ONLY and opt-in by BIBLETEXT_EXHAUSTIVE=1. Before a
// stack came off whole, all 24 of the BSB's 22,728 such drags that cross a
// stack, over Proverbs 22:17 and Hebrews 11:30, with the span and without,
// were not placed, and were quoted with the headings; the rest, and the
// WEB's and the WEB Catholic's, which have no stack there, were right.
func TestEveryDragAcrossTheHeadingsBetweenTwoVersesOverLocalCaches_LocalOnly(t *testing.T) {
	if os.Getenv("BIBLETEXT_EXHAUSTIVE") == "" {
		t.Skip("local-only and opt-in: every drag across the headings between two verses over three translation caches; run with BIBLETEXT_EXHAUSTIVE=1")
	}
	for _, c := range localPublicDomainCaches(t) {
		drags, wrong, stacked := 0, 0, 0
		for _, book := range c.bd.Books {
			for _, ch := range c.bd.GetChapterNumbersForBook(book) {
				hs := c.bd.Headings[book][ch]
				if len(hs) == 0 {
					continue
				}
				st := &AppState{Bible: c.bd, CurrentBook: book, CurrentChapter: ch}
				corpus, spans := chapterProseIn(st, book, ch)
				over := map[int][]string{}
				for _, h := range hs {
					if text := collapseSpaces(h.DrawnText()); text != "" && h.BeforeVerse > 0 {
						over[h.BeforeVerse] = append(over[h.BeforeVerse], text)
					}
				}
				for k := 1; k < len(spans); k++ {
					above, below := spans[k-1], spans[k]
					if len(over[below.verse]) == 0 {
						continue
					}
					heading := strings.Join(over[below.verse], "\n")
					aw := strings.Fields(corpus[above.start:above.end])
					bw := strings.Fields(corpus[below.start:below.end])
					for a := 1; a <= 3 && a <= len(aw); a++ {
						for m := 1; m <= 2 && m <= len(bw); m++ {
							tail, head := strings.Join(aw[len(aw)-a:], " "), strings.Join(bw[:m], " ")
							raw := tail + "\n" + heading + "\n" + strconv.Itoa(below.verse) + " " + head
							want := phraseWords(tail + " " + head)
							for _, span := range []selSpan{selSpanFromNative(above.verse, below.verse), {}} {
								drags++
								if len(over[below.verse]) > 1 {
									stacked++
								}
								text, lo, hi, _, ok := normalizeShareSelectionProse(st, book, ch, corpus, spans, raw, span)
								if ok && phraseWords(text) == want && (!span.valid() || lo == above.verse && hi == below.verse) {
									continue
								}
								if wrong++; wrong <= 5 {
									t.Errorf("%s: %s %d %q spanned %d to %d gave %q at %d-%d (ok %v)",
										c.id, book, ch, raw, span.lo, span.hi, text, lo, hi, ok)
								}
							}
						}
					}
				}
			}
		}
		t.Logf("%s: %d drags across the headings between two verses (%d across a stack), %d wrong", c.id, drags, stacked, wrong)
		if wrong != 0 {
			t.Errorf("%s: %d of %d drags across the headings between two verses shared other words or cited other verses", c.id, wrong, drags)
		}
	}
}

// Every drag of the public-domain translations' caches from each character
// of a heading through the number of the verse beneath to the end of that
// verse's first, second or third word, with the panes' span. Each must share
// those words, cited to that verse. LOCAL-ONLY and opt-in by
// BIBLETEXT_EXHAUSTIVE=1, as the other sweeps over the caches are. Before
// the front of a selection decided what its end could lose, 15,081 of the
// BSB's 189,996 such drags under 682 headings shared the heading as
// Scripture or cut the verse short, 6,126 of them under a heading above a
// chapter's first verse, and 7 of the Catholic WEB's 1,824, under the note
// fused into the heading over Daniel 3:24; the WEB's 291 were all right.
func TestEveryDragFromAHeadingIntoItsVerseOverLocalCaches_LocalOnly(t *testing.T) {
	if os.Getenv("BIBLETEXT_EXHAUSTIVE") == "" {
		t.Skip("local-only and opt-in: every drag from a heading into its verse over three translation caches; run with BIBLETEXT_EXHAUSTIVE=1")
	}
	for _, c := range localPublicDomainCaches(t) {
		drags, wrong := 0, 0
		for _, book := range c.bd.Books {
			for _, ch := range c.bd.GetChapterNumbersForBook(book) {
				hs := c.bd.Headings[book][ch]
				if len(hs) == 0 {
					continue
				}
				st := &AppState{Bible: c.bd, CurrentBook: book, CurrentChapter: ch}
				corpus, spans := chapterProseIn(st, book, ch)
				over := map[int][]string{}
				for _, h := range hs {
					if text := collapseSpaces(h.DrawnText()); text != "" && h.BeforeVerse > 0 {
						over[h.BeforeVerse] = append(over[h.BeforeVerse], text)
					}
				}
				for k, below := range spans {
					if len(over[below.verse]) == 0 {
						continue
					}
					above := 0
					if k > 0 {
						above = spans[k-1].verse
					}
					heading := strings.Join(over[below.verse], "\n")
					words := strings.Fields(corpus[below.start:below.end])
					for from := 0; from < len(heading); from++ {
						if !utf8.RuneStart(heading[from]) || heading[from] == ' ' || heading[from] == '\n' {
							continue
						}
						for n := 1; n <= 3 && n <= len(words); n++ {
							want := strings.Join(words[:n], " ")
							raw := heading[from:] + "\n" + strconv.Itoa(below.verse) + " " + want
							drags++
							text, lo, hi, _, ok := normalizeShareSelectionProse(st, book, ch, corpus, spans, raw, selSpanFromNative(above, below.verse))
							if ok && phraseWords(text) == phraseWords(want) && lo == below.verse && hi == below.verse {
								continue
							}
							if wrong++; wrong <= 5 {
								quote, cite := shareQuoteIn(st, book, ch, raw, selSpanFromNative(above, below.verse))
								t.Logf("%s: %s %d %q spanned %d to %d shared (%q, %q)", c.id, book, ch, raw, above, below.verse, quote, cite)
							}
						}
					}
				}
			}
		}
		t.Logf("%s: %d drags from a heading into its verse, %d wrong", c.id, drags, wrong)
		if wrong != 0 {
			t.Errorf("%s: %d of %d drags from a heading into its verse shared other words or cited another verse", c.id, wrong, drags)
		}
	}
}
