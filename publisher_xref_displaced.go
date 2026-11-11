package bibletext

// Two stretches of the NKJV feed whose notes stand one caller late —
// docs/SCRIPTURE_WORKLIST.md, S20.
//
// A cross-reference note has a caller, the place in the verse where it
// stands, and a content, its citations. In two stretches of the feed fetched
// on 6 November 2026 every content stands on the caller after its own:
//
//   - from the first caller of Ruth 3:12 to the last of 2 Samuel 24:25, 1,640
//     callers;
//   - from the first caller of Psalm 119:133 to the caller of Psalm 150:2,
//     323 callers.
//
// The feed lacks a caller the edition has on Ruth 3:11, and a second one on
// Psalm 119:132; from there each content sits one caller on, across verses,
// chapters and books (the last note of Ruth 4:22 is delivered on 1 Samuel
// 1:1), to the end of 2 Samuel and of the Psalter, where the last caller's
// own content is not in the feed at all. The callers themselves are where
// the edition has them. So on those 1,278 verses the first citations shown
// belong to the verse before, and each word's letter would name the next
// word's citations.
//
// Found on 11 November 2026 three ways that agree: against a second
// publication of the same apparatus, entry by entry (196 of 196 compared
// inside the stretches one caller late, 325 of 330 outside aligned); by the
// notes themselves, since a note is far more often answered by a note on the
// verse it cites when it stands where the edition set it (about half of all
// notes are, 16% in the first stretch as delivered and 34% moved back, 10%
// and 41% in the second); and by reading: the note the feed sets on the
// first word of 1 Samuel 2:1's prayer cites two verses about worship, the
// last word of the verse before.
//
// displacedContent undoes it where the panel reads a note: inside a stretch
// a caller is given the content delivered on the next caller. Two loose ends
// remain in each stretch, and are the feed's: the first content has no
// caller, so it is shown with its verse and keyed to no word (Unkeyed); and
// the last caller has no content, so it shows nothing. The stored notes are
// not changed.
//
// It is done only while the feed is as it was found (displacedStretch.live):
// the content delivered at each stretch's first and last caller is the one
// found there, and the verse the first belongs to still lacks its caller. A
// feed that is put right no longer matches, and is then read as delivered.

import "strings"

// displacedStretch is one run of callers whose contents stand one caller late.
type displacedStretch struct {
	First, Last noteCaller
	// Home is the verse the content delivered at First belongs to, whose
	// caller the feed lacks, and HomeNotes the cross-reference notes that
	// verse has in the feed as found.
	Home      noteCaller
	HomeNotes int
	// FirstText and LastText are the contents delivered at First and Last:
	// the mark of the feed as found.
	FirstText, LastText string
}

// displacedStretches are the two stretches of the NKJV feed.
var displacedStretches = []displacedStretch{
	{
		First: noteCaller{"Ruth", 3, 12, 0}, Last: noteCaller{"2 Samuel", 24, 25, 1},
		Home: noteCaller{Book: "Ruth", Chapter: 3, Verse: 11}, HomeNotes: 0,
		FirstText: "Prov. 12:4; 31:10–31", LastText: "2 Sam. 21:14",
	},
	{
		First: noteCaller{"Psalms", 119, 133, 0}, Last: noteCaller{"Psalms", 150, 2, 0},
		Home: noteCaller{Book: "Psalms", Chapter: 119, Verse: 132}, HomeNotes: 1,
		FirstText: "Ps. 51:1; (2 Thess. 1:6)", LastText: "Ps. 145:5, 6",
	},
}

// correctDisplacedNotes turns the correction off for a test that reads the
// feed as delivered.
var correctDisplacedNotes = true

// crossRefNotesOf are the cross-reference notes among fns that say anything.
func crossRefNotesOf(fns []Footnote) []Footnote {
	var out []Footnote
	for _, fn := range fns {
		if fn.Kind == footnoteKindCrossref && strings.TrimSpace(fn.Text) != "" {
			out = append(out, fn)
		}
	}
	return out
}

// callerNotes are the cross-reference notes of a caller's verse or title.
func callerNotes(bd *BibleData, book string, chapter, verse int) []Footnote {
	if verse == 0 {
		return crossRefNotesOf(bd.SuperscriptionFor(book, chapter).Footnotes)
	}
	if v := bd.GetVerse(book, chapter, verse); v != nil {
		return crossRefNotesOf(v.Footnotes)
	}
	return nil
}

// bookIndex is a book's place in the loaded text's order, -1 for none.
func bookIndex(bd *BibleData, book string) int {
	for i, b := range bd.Books {
		if b == book {
			return i
		}
	}
	return -1
}

// notAfter reports whether c stands at or before o in the feed's order.
func (c noteCaller) notAfter(bd *BibleData, o noteCaller) bool {
	if a, b := bookIndex(bd, c.Book), bookIndex(bd, o.Book); a != b {
		return a < b
	}
	if c.Chapter != o.Chapter {
		return c.Chapter < o.Chapter
	}
	if c.Verse != o.Verse {
		return c.Verse < o.Verse
	}
	return c.Index <= o.Index
}

// live reports whether the loaded text is displaced over this stretch as the
// feed was found to be.
func (s displacedStretch) live(bd *BibleData) bool {
	first := callerNotes(bd, s.First.Book, s.First.Chapter, s.First.Verse)
	last := callerNotes(bd, s.Last.Book, s.Last.Chapter, s.Last.Verse)
	home := callerNotes(bd, s.Home.Book, s.Home.Chapter, s.Home.Verse)
	return len(first) > s.First.Index && strings.TrimSpace(first[s.First.Index].Text) == s.FirstText &&
		len(last) == s.Last.Index+1 && strings.TrimSpace(last[s.Last.Index].Text) == s.LastText &&
		len(home) == s.HomeNotes
}

// displacedStretchOf is the live stretch a caller stands in.
func displacedStretchOf(bd *BibleData, c noteCaller) (displacedStretch, bool) {
	if !correctDisplacedNotes || bd == nil {
		return displacedStretch{}, false
	}
	for _, s := range displacedStretches {
		if s.First.notAfter(bd, c) && c.notAfter(bd, s.Last) && bookIndex(bd, c.Book) >= 0 && s.live(bd) {
			return s, true
		}
	}
	return displacedStretch{}, false
}

// callerAfter is the next caller in the feed's order, and its note: the next
// note of the same verse or title, else the first of the next verse that has
// one, the title before the verses of a chapter, on through the chapters and
// the books.
func callerAfter(bd *BibleData, c noteCaller) (noteCaller, Footnote, bool) {
	if notes := callerNotes(bd, c.Book, c.Chapter, c.Verse); c.Index+1 < len(notes) {
		c.Index++
		return c, notes[c.Index], true
	}
	first := func(book string, chapter, afterVerse int, title bool) (noteCaller, Footnote, bool) {
		if title {
			if notes := callerNotes(bd, book, chapter, 0); len(notes) > 0 {
				return noteCaller{book, chapter, 0, 0}, notes[0], true
			}
		}
		for _, v := range bd.GetChapter(book, chapter) {
			if v.Verse <= afterVerse {
				continue
			}
			if notes := crossRefNotesOf(v.Footnotes); len(notes) > 0 {
				return noteCaller{book, chapter, v.Verse, 0}, notes[0], true
			}
		}
		return noteCaller{}, Footnote{}, false
	}
	if next, fn, ok := first(c.Book, c.Chapter, c.Verse, false); ok {
		return next, fn, true
	}
	for bi := bookIndex(bd, c.Book); bi >= 0 && bi < len(bd.Books); bi++ {
		book := bd.Books[bi]
		for _, ch := range bd.GetChapterNumbersForBook(book) {
			if book == c.Book && ch <= c.Chapter {
				continue
			}
			if next, fn, ok := first(book, ch, 0, true); ok {
				return next, fn, true
			}
		}
	}
	return noteCaller{}, Footnote{}, false
}

// displacedContent is the note a caller shows: its own as delivered, or,
// inside a live stretch, the one delivered on the next caller. shown is
// false for a stretch's last caller, whose content the feed does not have.
func displacedContent(bd *BibleData, c noteCaller, delivered Footnote) (fn Footnote, shown bool) {
	s, ok := displacedStretchOf(bd, c)
	if !ok {
		return delivered, true
	}
	if c == s.Last {
		return Footnote{}, false
	}
	_, next, ok := callerAfter(bd, c)
	if !ok {
		return Footnote{}, false
	}
	// The caller keeps its place; the content is the next one's.
	delivered.Text, delivered.Refs = next.Text, next.Refs
	return delivered, true
}

// displacedHomeNotes are the contents that belong to a verse whose caller
// the feed lacks: the first content of a live stretch, for the stretch's
// Home verse.
func displacedHomeNotes(bd *BibleData, book string, chapter, verse int) []Footnote {
	if !correctDisplacedNotes || bd == nil {
		return nil
	}
	var out []Footnote
	for _, s := range displacedStretches {
		if s.Home.Book != book || s.Home.Chapter != chapter || s.Home.Verse != verse || !s.live(bd) {
			continue
		}
		if notes := callerNotes(bd, s.First.Book, s.First.Chapter, s.First.Verse); len(notes) > s.First.Index {
			out = append(out, notes[s.First.Index])
		}
	}
	return out
}
