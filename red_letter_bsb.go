package bibletext

import (
	"os"
	"strconv"
	"strings"
)

// Span-level words of Christ, and the lookup every reading pane goes through.
// Each mapped edition uses only its own publisher's words-of-Jesus markup.

// bsbRedLetterSpansOn is an internal diagnostics switch for BSB span rendering.
// Off, publisher-marked BSB verses remain red but fall back to whole-verse red.
// It does not affect any other edition.
const bsbRedLetterSpansOn = true

// bsbRedLetterSpansEnabled resolves the switch, allowing an environment override
// so the two behaviours can be compared on a device or in the simulator without
// two builds. The constant is what ships; the variable only ever narrows or
// widens it for whoever set it deliberately.
//
//	BIBLETEXT_BSB_RED_LETTER=0   force BSB whole-verse rendering
//	BIBLETEXT_BSB_RED_LETTER=1   force BSB publisher spans
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
// The offsets are accepted only for the exact runtime verse revision from which
// they were generated. A mismatch reports no usable spans; redLetterRuns then
// falls back to whole-verse red from this edition's own marked-verse set.
func bsbRedLetterSpansFor(book string, chapter, verse int, text string) ([]redLetterSpan, bool) {
	if !bsbRedLetterSpansEnabled() {
		return nil, false
	}
	return tableSpansFor(bsbRedLetterSpans, bsbRedLetterRunes, bsbRedLetterHashes,
		book, chapter, verse, text)
}

// redLetterSpansFor is the one lookup every pane goes through. It answers only
// for editions whose publisher span data we hold and only when the supplied
// verse matches the exact text revision used to generate the offsets.
func redLetterSpansFor(versionID, book string, chapter, verse int, text string) ([]redLetterSpan, bool) {
	switch versionID {
	case "bsb":
		return bsbRedLetterSpansFor(book, chapter, verse, text)
	case "nkjv":
		return nkjvRedLetterSpansFor(book, chapter, verse, text)
	case "web":
		return tableSpansFor(webRedLetterSpans, webRedLetterRunes, webRedLetterHashes,
			book, chapter, verse, text)
	case "webc":
		return tableSpansFor(webcRedLetterSpans, webcRedLetterRunes, webcRedLetterHashes,
			book, chapter, verse, text)
	}
	return nil, false
}

// redLetterVerseMarked reports the edition's own verse-level judgement. Table
// presence is enough even when a content mismatch makes the offsets unusable.
// WEB and WEBC retain generated source-marked sets independently of their span
// metadata so a future runtime text change can still fall back safely. An
// edition without its own table deliberately returns false: red-letter data is
// editorial and must never be borrowed from another translation.
func redLetterVerseMarked(versionID, book string, chapter, verse int) bool {
	key := verseKeyFor(book, chapter, verse)
	switch versionID {
	case "bsb":
		_, ok := bsbRedLetterSpans[key]
		return ok
	case "nkjv":
		_, ok := nkjvRedLetterSpans[key]
		return ok
	case "web":
		_, ok := webRedLetterMarked[key]
		return ok
	case "webc":
		_, ok := webcRedLetterMarked[key]
		return ok
	default:
		return false
	}
}

// tableSpansFor is the lookup every span table shares: find the verse, then
// refuse unless both its rune length and content fingerprint match.
func tableSpansFor(spans map[string][]redLetterSpan, runes map[string]int, hashes map[string]uint64,
	book string, chapter, verse int, text string) ([]redLetterSpan, bool) {
	key := verseKeyFor(book, chapter, verse)
	got, ok := spans[key]
	if !ok || len([]rune(text)) != runes[key] || redLetterTextHash(text) != hashes[key] {
		return nil, false
	}
	return got, true
}

// nkjvRedLetterSpansFor applies the shared exact-text guard to the NKJV table.
func nkjvRedLetterSpansFor(book string, chapter, verse int, text string) ([]redLetterSpan, bool) {
	return tableSpansFor(nkjvRedLetterSpans, nkjvRedLetterRunes, nkjvRedLetterHashes,
		book, chapter, verse, text)
}

func redLetterTextHash(text string) uint64 {
	const (
		offset64 = uint64(14695981039346656037)
		prime64  = uint64(1099511628211)
	)
	hash := offset64
	for index := 0; index < len(text); index++ {
		hash ^= uint64(text[index])
		hash *= prime64
	}
	return hash
}

// verseKeyFor is the "Book Chapter:Verse" key the generated tables use.
func verseKeyFor(book string, chapter, verse int) string {
	return book + " " + strconv.Itoa(chapter) + ":" + strconv.Itoa(verse)
}
