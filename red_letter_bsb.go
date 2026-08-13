package bibletext

import (
	"os"
	"strconv"
	"strings"
)

// The BSB's words of Christ, as SPANS rather than whole verses.
//
// The BSB ships no words-of-Jesus markup: its published USFM (ebible.org,
// engbsb) contains zero \wj markers, so unlike the WEB and the NKJV there is no
// publisher's judgement to copy. Whole-verse reddening from the WEB's marks put
// narration — and in about 79 verses another speaker's words — in red: "We are
// able" (James and John), "No one, Lord" (the woman), "Caesar's" (the
// Pharisees). These spans are derived instead; see scripts/gen-bsb-redletter.py
// for how, and docs/TEXTUAL-DATA.md for why each rule is shaped as it is.

// bsbRedLetterSpansOn is the INTERNAL switch for this whole feature. Off, the
// app behaves exactly as it did before the spans existed: bsbRedLetterSpansFor
// answers "no data", and every caller falls back to reddening the whole verse
// from the WEB's verse-level marks.
//
// Deliberately not a user setting. The spans are our editorial judgement rather
// than the Berean translators' — the BSB publishes no words-of-Jesus markup at
// all — so this is a decision the project takes, not one to put in front of a
// reader as a preference. It is one line to flip if we decide the BSB should
// show no derived red letters, or to promote to a setting later.
const bsbRedLetterSpansOn = true

// bsbRedLetterSpansEnabled resolves the switch, allowing an environment override
// so the two behaviours can be compared on a device or in the simulator without
// two builds. The constant is what ships; the variable only ever narrows or
// widens it for whoever set it deliberately.
//
//	BIBLETEXT_BSB_RED_LETTER=0   force the old whole-verse behaviour
//	BIBLETEXT_BSB_RED_LETTER=1   force the spans
func bsbRedLetterSpansEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BIBLETEXT_BSB_RED_LETTER"))) {
	case "0", "off", "false", "no":
		return false
	case "1", "on", "true", "yes":
		return true
	}
	return bsbRedLetterSpansOn
}

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
	if !bsbRedLetterSpansEnabled() {
		return nil, false
	}
	spans, ok := bsbRedLetterSpans[verseKeyFor(book, chapter, verse)]
	if !ok {
		return nil, false
	}
	if len([]rune(text)) != bsbRedLetterRuneLen(book, chapter, verse) {
		return nil, false
	}
	return spans, true
}

// redLetterSpansFor is the one lookup every pane goes through. It answers only
// for editions we hold span data for — the BSB, derived because it publishes
// none, and the NKJV, whose own words-of-Jesus spans API.Bible serves. The WEB
// and WEB Catholic have spans in their USFM but no table here yet, so they still
// get the whole-verse answer.
func redLetterSpansFor(versionID, book string, chapter, verse int, text string) ([]redLetterSpan, bool) {
	switch versionID {
	case "bsb":
		return bsbRedLetterSpansFor(book, chapter, verse, text)
	case "nkjv":
		return nkjvRedLetterSpansFor(book, chapter, verse, text)
	}
	return nil, false
}

// nkjvRedLetterSpansFor mirrors the BSB accessor, including the rune-length
// guard: offsets computed against different text would colour arbitrary words.
// It honours the same switch, so one flag turns the whole feature off.
func nkjvRedLetterSpansFor(book string, chapter, verse int, text string) ([]redLetterSpan, bool) {
	if !bsbRedLetterSpansEnabled() {
		return nil, false
	}
	key := verseKeyFor(book, chapter, verse)
	spans, ok := nkjvRedLetterSpans[key]
	if !ok || len([]rune(text)) != nkjvRedLetterRunes[key] {
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
