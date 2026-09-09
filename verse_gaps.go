package bibletext

// Where an omitted verse's gap is marked — docs/SCRIPTURE_WORKLIST.md, S19.
//
// Every reading surface asks this ONE question and draws the answer in its own
// dialect, the way chapter_blocks.go settles the order of headings once for all
// of them. Deciding it per surface is how three panes would come to disagree
// about which holes exist.
//
// WHICH holes exist comes from the offline table (omitted_verses.go), never
// from a jump in the numbering: a merged verse is keyed by its first number and
// an interrupted fetch leaves gaps that are decode faults, and marking either
// would have the app assert an omission the publisher never made.
//
// WHETHER to mark is the footnotes toggle. The apparatus is where a reader
// already goes for "why is this verse not here", so the mark extends a
// decision the reader has taken rather than adding a control; with the toggle
// off the page is byte-identical to today's.

// gapsBefore files each omitted verse of the chapter under the rendered verse
// it precedes, so a builder walking the verses in order can write the marks
// just before that verse's number. Only INTERIOR holes are filed — a number
// below the first rendered verse or above the last is the chapter being
// shorter, not a hole, and the table never records those anyway.
//
// Returns nil when there is nothing to mark, which is every chapter of the
// licensed edition and almost every chapter of the others.
func gapsBefore(versionID, book string, chapter int, verses []Verse) map[int][]int {
	omitted := omittedVersesIn(versionID, book, chapter)
	if len(omitted) == 0 || len(verses) == 0 {
		return nil
	}
	lowest := verses[0].Verse
	for _, v := range verses {
		if v.Verse < lowest {
			lowest = v.Verse
		}
	}
	var out map[int][]int
	for _, n := range omitted {
		if n <= lowest {
			continue // not interior: nothing rendered stands before it
		}
		// The first rendered verse above the hole. Verses arrive in order, so
		// this is the one whose number the mark goes in front of.
		for _, v := range verses {
			if v.Verse > n {
				if out == nil {
					out = make(map[int][]int, 2)
				}
				out[v.Verse] = append(out[v.Verse], n)
				break
			}
		}
	}
	return out
}

// verseGapMark is the mark's text: the omitted number in square brackets.
// Brackets rather than a bare number, so the iOS and macOS verse indexes —
// which read a small run's integerValue — read zero and skip it, and so a reader
// cannot mistake it for a verse that is there.
func verseGapMark(n int) string {
	return "[" + itoa(n) + "]"
}
