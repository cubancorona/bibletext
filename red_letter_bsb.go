package bibletext

import "strconv"

// The BSB's words of Christ, as SPANS rather than whole verses.
//
// The BSB ships no words-of-Jesus markup: its published USFM (ebible.org,
// engbsb) contains zero \wj markers, so unlike the WEB and the NKJV there is no
// publisher's judgement to copy. Whole-verse reddening from the WEB's marks put
// narration — and in about 79 verses another speaker's words — in red: "We are
// able" (James and John), "No one, Lord" (the woman), "Caesar's" (the
// Pharisees). These spans are derived instead; see scripts/gen-bsb-redletter.py
// for how, and docs/TEXTUAL-DATA.md for why each rule is shaped as it is.

// redLetterSpan is a half-open rune range [Start, End) within a verse's text.
type redLetterSpan struct {
	Start int
	End   int
}

// bsbRedLetterSpansFor returns the ranges of a BSB verse that are Christ's
// words, and whether the answer is usable at all.
//
// The offsets were computed against a particular rendering of the verse, so they
// are only meaningful for text of the same length. A mismatch means the supplier
// changed the text under us: the honest response is to report "no span data" and
// let the caller fall back to reddening the whole verse, which is what the app
// did before these spans existed. Painting stale offsets would colour arbitrary
// words — worse than the coarse behaviour it replaced.
func bsbRedLetterSpansFor(book string, chapter, verse int, text string) ([]redLetterSpan, bool) {
	spans, ok := bsbRedLetterSpans[verseKeyFor(book, chapter, verse)]
	if !ok {
		return nil, false
	}
	if len([]rune(text)) != bsbRedLetterRuneLen(book, chapter, verse) {
		return nil, false
	}
	return spans, true
}

func bsbRedLetterRuneLen(book string, chapter, verse int) int {
	return bsbRedLetterRunes[verseKeyFor(book, chapter, verse)]
}

// verseKeyFor is the "Book Chapter:Verse" key the generated tables use.
func verseKeyFor(book string, chapter, verse int) string {
	return book + " " + strconv.Itoa(chapter) + ":" + strconv.Itoa(verse)
}
