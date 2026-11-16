package bibletext

// The public-domain / CC0 editions' own cross-references — docs/SCRIPTURE_WORKLIST.md, S20.
//
// World English Bible (WEB), World English Bible Catholic (WEBC) and the Berean
// Standard Bible (BSB) all publish cross-references in their official USFM:
// 340 in-verse \x notes across 322 WEB verses, 343 across 325 WEBC verses (the
// same set plus Tobit 2:6, Wisdom 14:27 and Sirach 6:30), and 1,325 section-head
// \r parallel-passage lines in BSB. bible.helloao.org strips all of them from
// the runtime JSON feed (save one leaked "(Joshua–Malachi)" heading node at
// BSB Hebrews 11:30).
//
// Rather than bumping cacheEpoch to re-download identical feed JSON,
// applyPublisherCrossRefs attaches the generated tables from
// publisher_xrefs_public_data.go (built offline by scripts/gen-publisher-xrefs.py)
// on decode, on cache load, and on the embedded WEB Gospels seed, idempotently.

import (
	"strconv"
	"strings"
)

// publisherVerseNote is one cross-reference note recovered from an edition's
// published USFM for a verse ("Book Chapter:Verse").
type publisherVerseNote struct {
	Anchor int       // rune offset in the runtime verse text
	AtEnd  bool      // true when no verse text follows \x* in the verse
	Text   string    // the citation text as printed in the edition
	Refs   []NoteRef // resolved citations within Text, in print order
}

// publisherVerseEntry pairs a verse's recovered cross-reference notes with the
// exact-text guard for its mid-verse offsets (Runes and Hash, via
// redLetterTextHash), matching the red-letter span tables' guard. If a future
// runtime feed revises a verse's wording, an AtEnd note still lands at the end
// of the new verse text, while a mid-verse note sets Anchor = -1 (Unkeyed) so
// its citations still show with the verse rather than placing a letter on the
// wrong word.
type publisherVerseEntry struct {
	Runes int
	Hash  uint64
	Notes []publisherVerseNote
}

// publisherHeadingNote is one parallel-passage reference line (\r) recovered
// from an edition's published USFM for a section heading before a verse.
type publisherHeadingNote struct {
	BeforeVerse  int       // Heading.BeforeVerse in the chapter
	HeadingCount int       // number of section headings at BeforeVerse in the source snapshot
	HeadingHash  uint64    // redLetterTextHash(Heading.Text)
	Text         string    // the reference line without its outer wrapper parentheses
	Refs         []NoteRef // resolved citations within Text; nil for words-only (e.g. "Joshua–Malachi")
}

// applyPublisherCrossRefs attaches an edition's recovered publisher
// cross-references to bd idempotently. No-op for nil data or other editions.
func applyPublisherCrossRefs(versionID string, bd *BibleData) {
	if bd == nil {
		return
	}
	switch versionID {
	case "web":
		applyPublisherVerseCrossRefs(bd, webPublisherCrossRefs)
	case "webc":
		applyPublisherVerseCrossRefs(bd, webcPublisherCrossRefs)
	case "bsb":
		applyPublisherHeadingCrossRefs(bd, bsbPublisherCrossRefs)
	}
}

func applyPublisherVerseCrossRefs(bd *BibleData, table map[string]publisherVerseEntry) {
	if len(bd.Verses) == 0 {
		return
	}
	for _, chapters := range bd.Verses {
		for _, verses := range chapters {
			for i := range verses {
				verses[i].Footnotes = withoutPublisherTableNotes(verses[i].Footnotes)
			}
		}
	}
	for key, entry := range table {
		book, ch, verseNum, ok := splitPublisherVerseKey(key)
		if !ok {
			continue
		}
		chapters := bd.Verses[book]
		if chapters == nil {
			continue
		}
		verses := chapters[ch]
		idx := -1
		for i := range verses {
			if verses[i].Verse == verseNum {
				idx = i
				break
			}
		}
		if idx < 0 || hasCrossRefFootnote(verses[idx].Footnotes) {
			continue
		}
		runes := []rune(verses[idx].Text)
		exact := len(runes) == entry.Runes && redLetterTextHash(verses[idx].Text) == entry.Hash
		for _, n := range entry.Notes {
			anchor := -1
			if exact {
				anchor = n.Anchor
			} else if n.AtEnd {
				anchor = len(runes)
			}
			var refs []NoteRef
			if len(n.Refs) > 0 {
				refs = append([]NoteRef(nil), n.Refs...)
			}
			verses[idx].Footnotes = append(verses[idx].Footnotes, Footnote{
				Anchor:         anchor,
				Text:           n.Text,
				Kind:           footnoteKindCrossref,
				Caller:         "+",
				Refs:           refs,
				PublisherTable: true,
			})
		}
	}
}

func applyPublisherHeadingCrossRefs(bd *BibleData, table map[string][]publisherHeadingNote) {
	if len(bd.Headings) == 0 {
		return
	}
	for _, chapters := range bd.Headings {
		for _, headings := range chapters {
			for i := range headings {
				headings[i].Footnotes = withoutPublisherTableNotes(headings[i].Footnotes)
			}
		}
	}
	for key, list := range table {
		book, ch, ok := splitPublisherChapterKey(key)
		if !ok {
			continue
		}
		origHeads := bd.Headings[book][ch]
		if len(origHeads) == 0 {
			continue
		}
		// Clean up any leaked parenthesised \r line that helloao emitted as a
		// duplicate section heading beside the real heading at the same verse
		// (BSB Hebrews 11:30 "(Joshua–Malachi)" after "The Faith of Many").
		chHeads := stripLeakedRefHeadings(origHeads, list)
		replacedSlice := len(chHeads) != len(origHeads)

		for _, hn := range list {
			var atVerse []int
			for i := range chHeads {
				if chHeads[i].BeforeVerse == hn.BeforeVerse && chHeads[i].Style == "heading" {
					atVerse = append(atVerse, i)
				}
			}
			if len(atVerse) == 0 {
				continue
			}
			target := -1
			matches := 0
			for _, i := range atVerse {
				if redLetterTextHash(chHeads[i].Text) == hn.HeadingHash {
					target = i
					matches++
				}
			}
			if matches != 1 {
				target = -1
				// A heading that was and remains alone can retain its note
				// after a wording change.
				// With multiple candidates, an ordinal cannot identify which
				// heading survived a rename or reorder.
				if len(atVerse) == 1 && hn.HeadingCount == 1 {
					target = atVerse[0]
				}
			}
			if target < 0 || hasCrossRefFootnoteText(chHeads[target].Footnotes, hn.Text) {
				continue
			}
			var refs []NoteRef
			if len(hn.Refs) > 0 {
				refs = append([]NoteRef(nil), hn.Refs...)
			}
			chHeads[target].Footnotes = append(chHeads[target].Footnotes, Footnote{
				Anchor:         0,
				Text:           hn.Text,
				Kind:           footnoteKindCrossref,
				Caller:         "-",
				Refs:           refs,
				PublisherTable: true,
			})
		}
		if replacedSlice {
			bd.Headings[book][ch] = chHeads
		}
	}
}

// withoutPublisherTableNotes keeps feed notes in order and removes only
// recovered references. Cache-loaded data can then take additions, revisions
// and deletions from the current tables without accumulating stale notes.
func withoutPublisherTableNotes(notes []Footnote) []Footnote {
	for i, note := range notes {
		if note.Kind != footnoteKindCrossref || !note.PublisherTable {
			continue
		}
		kept := append([]Footnote(nil), notes[:i]...)
		for _, rest := range notes[i+1:] {
			if rest.Kind != footnoteKindCrossref || !rest.PublisherTable {
				kept = append(kept, rest)
			}
		}
		return kept
	}
	return notes
}

func stripLeakedRefHeadings(heads []Heading, notes []publisherHeadingNote) []Heading {
	leaked := map[[2]string]bool{}
	countAtVerse := map[int]int{}
	for _, h := range heads {
		if h.Style == "heading" {
			countAtVerse[h.BeforeVerse]++
		}
	}
	for _, hn := range notes {
		if countAtVerse[hn.BeforeVerse] > 1 {
			leaked[[2]string{strconv.Itoa(hn.BeforeVerse), "(" + hn.Text + ")"}] = true
		}
	}
	if len(leaked) == 0 {
		return heads
	}
	changed := false
	for _, h := range heads {
		if h.Style == "heading" && countAtVerse[h.BeforeVerse] > 1 && leaked[[2]string{strconv.Itoa(h.BeforeVerse), h.Text}] {
			changed = true
			break
		}
	}
	if !changed {
		return heads
	}
	kept := make([]Heading, 0, len(heads))
	for _, h := range heads {
		if h.Style == "heading" && countAtVerse[h.BeforeVerse] > 1 && leaked[[2]string{strconv.Itoa(h.BeforeVerse), h.Text}] {
			countAtVerse[h.BeforeVerse]--
			continue
		}
		kept = append(kept, h)
	}
	return kept
}

func hasCrossRefFootnote(fns []Footnote) bool {
	for _, fn := range fns {
		if fn.Kind == footnoteKindCrossref {
			return true
		}
	}
	return false
}

func hasCrossRefFootnoteText(fns []Footnote, text string) bool {
	for _, fn := range fns {
		if fn.Kind == footnoteKindCrossref && fn.Text == text {
			return true
		}
	}
	return false
}

func splitPublisherVerseKey(key string) (book string, ch, verse int, ok bool) {
	sp := strings.LastIndexByte(key, ' ')
	if sp <= 0 {
		return "", 0, 0, false
	}
	colon := strings.IndexByte(key[sp+1:], ':')
	if colon <= 0 {
		return "", 0, 0, false
	}
	c, err1 := strconv.Atoi(key[sp+1 : sp+1+colon])
	v, err2 := strconv.Atoi(key[sp+1+colon+1:])
	if err1 != nil || err2 != nil || c < 1 || v < 1 {
		return "", 0, 0, false
	}
	return key[:sp], c, v, true
}

func splitPublisherChapterKey(key string) (book string, ch int, ok bool) {
	sp := strings.LastIndexByte(key, ' ')
	if sp <= 0 {
		return "", 0, false
	}
	c, err := strconv.Atoi(key[sp+1:])
	if err != nil || c < 1 {
		return "", 0, false
	}
	return key[:sp], c, true
}
