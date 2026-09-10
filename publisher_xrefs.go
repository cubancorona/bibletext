package bibletext

// The publisher's own cross references — docs/SCRIPTURE_WORKLIST.md, S20.
//
// The NKJV feed carries the edition's cross-reference apparatus as notes, and
// the app has captured every one since its first epoch without showing any:
// 32,473 across the canon, each a citation string exactly as the print
// edition's centre column sets it ("Rom. 5:8; Eph. 2:4; (1 John 4:9, 10)"),
// with the targets the source tagged kept as NoteRef spans (apiBibleNote).
// This file resolves those into rows for the cross-references panel — ONE ROW
// PER NOTE, verbatim:
//
//   - the text is the publisher's, never re-cut into one row per target. The
//     parenthesised "compare" citations straddle tagged and untagged spans, a
//     continuation such as "19:39" is unreadable on its own, and a verse's
//     several notes are several entries in print;
//   - a tagged span becomes a tap target only when the passage it names is in
//     the loaded text; an untagged citation stays words;
//   - nothing is sorted, capped, deduplicated or mapped through the
//     versification tables. The targets are in the edition's own numbering,
//     which is the numbering on screen, so the set the reader sees is the set
//     the publisher printed. The Treasury of Scripture Knowledge rows from
//     crossRefsForSelection are ranked, capped and mapped, and are credited to
//     their own source; the two never share a list.
//
// Whether the rows DISPLAY is BibleVersion.PublisherCrossRefs, true for the
// NKJV only in the file behind the nkjvxrefs build tag: displaying the
// publisher's apparatus is the subject of a licensing enquiry that has been
// sent and not yet answered, and a store build must not be able to do it
// before the answer comes. The resolver itself is plain code over data every
// build already holds.

import (
	"strconv"
	"strings"
)

// publisherCrossRef is one note of the publisher's apparatus, verbatim.
type publisherCrossRef struct {
	Verse   int               // the verse the note belongs to; 0 for the Psalm's title
	Text    string            // the citation text exactly as delivered
	Targets []publisherTarget // the tappable spans, in text order; none = words only
}

// publisherTarget is one tagged citation resolved against the loaded text.
type publisherTarget struct {
	Start, End int      // the citation's rune span within Text
	Ref        crossRef // the passage, in the on-screen edition's own numbering
	Current    bool     // the passage contains the note's own verse: shown, not followed
}

// parseUSFMRefID reads a source target id — "JHN.7.50", or the range
// "MAT.3.1-MAT.3.12" — into a crossRef in the app's book names. Anything
// malformed is rejected. A range whose end is in another book, or before its
// start, keeps only the start, so the row points at scripture the reader can
// reach rather than at nothing.
func parseUSFMRefID(id string) (crossRef, bool) {
	parts := strings.Split(strings.TrimSpace(id), "-")
	if len(parts) > 2 {
		return crossRef{}, false
	}
	book, ch, v, ok := parseUSFMRef(parts[0])
	if !ok {
		return crossRef{}, false
	}
	out := crossRef{Book: book, Chapter: ch, Verse: v}
	if len(parts) == 2 {
		if eb, ec, ev, ok := parseUSFMRef(parts[1]); ok && eb == book && (ec > ch || (ec == ch && ev > v)) {
			out.EndCh, out.EndV = ec, ev
		}
	}
	return out, true
}

// parseUSFMRef reads one "BOOK.chapter.verse" id.
func parseUSFMRef(s string) (book string, ch, v int, ok bool) {
	f := strings.Split(strings.TrimSpace(s), ".")
	if len(f) != 3 {
		return "", 0, 0, false
	}
	book = apiBibleBookName(strings.ToUpper(f[0]))
	if book == "" {
		return "", 0, 0, false
	}
	ch, err1 := strconv.Atoi(f[1])
	v, err2 := strconv.Atoi(f[2])
	if err1 != nil || err2 != nil || ch < 1 || v < 1 {
		return "", 0, 0, false
	}
	return book, ch, v, true
}

// contains reports whether the passage c covers verse (ch, v) of its own book.
func (c crossRef) contains(ch, v int) bool {
	endCh, endV := c.EndCh, c.EndV
	if endV == 0 {
		return ch == c.Chapter && v == c.Verse
	}
	if endCh == 0 {
		endCh = c.Chapter
	}
	afterStart := ch > c.Chapter || (ch == c.Chapter && v >= c.Verse)
	beforeEnd := ch < endCh || (ch == endCh && v <= endV)
	return afterStart && beforeEnd
}

// publisherCrossRefsFor resolves the publisher's notes for the selected
// verses of one chapter: one row per note, in verse order and then in the
// order the notes stand in the verse, with the Psalm's title notes first
// (keyed 0) when the selection reaches the chapter's first verse — the title
// precedes verse 1 on the page, so its references precede verse 1's.
func publisherCrossRefsFor(bd *BibleData, book string, chapter int, selected []Verse) []publisherCrossRef {
	if bd == nil || len(selected) == 0 {
		return nil
	}
	var rows []publisherCrossRef
	add := func(verse int, fn Footnote) {
		if fn.Kind != footnoteKindCrossref {
			return
		}
		text := strings.TrimSpace(fn.Text)
		if text == "" {
			return
		}
		row := publisherCrossRef{Verse: verse, Text: text}
		runes := len([]rune(fn.Text))
		for _, r := range fn.Refs {
			if r.Start < 0 || r.End > runes || r.Start >= r.End {
				continue // a span that does not fit the text it claims to index
			}
			ref, ok := parseUSFMRefID(r.ID)
			if !ok || bd.GetVerse(ref.Book, ref.Chapter, ref.Verse) == nil {
				continue // words only: the passage is not in the loaded text
			}
			row.Targets = append(row.Targets, publisherTarget{
				Start: r.Start, End: r.End, Ref: ref,
				Current: verse > 0 && ref.Book == book && ref.contains(chapter, verse),
			})
		}
		rows = append(rows, row)
	}

	if ch := bd.GetChapter(book, chapter); len(ch) > 0 {
		first := ch[0].Verse
		for _, v := range selected {
			if v.Verse == first {
				for _, fn := range bd.SuperscriptionFor(book, chapter).Footnotes {
					add(0, fn)
				}
				break
			}
		}
	}
	for _, v := range selected {
		for _, fn := range v.Footnotes {
			add(v.Verse, fn)
		}
	}
	return rows
}
