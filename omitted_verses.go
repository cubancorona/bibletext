package bibletext

// The verses a translation omits — docs/SCRIPTURE_WORKLIST.md, S19.
//
// A translation that omits a verse leaves a hole in its own numbering: Luke
// 17:35 is followed by 17:37, and unless the chapter's notes happen to explain
// it the reader is given no reason. Explaining it means knowing it IS a hole,
// and that cannot be learned at runtime from the numbering — a merged verse is
// keyed by its first number, and an interrupted fetch leaves gaps that are
// decode faults rather than translators' decisions. So the holes are resolved
// offline and shipped as a table (omitted_verses_data.go), the same shape the
// red-letter and paragraph tables use.
//
// The table is DATA THE RUNTIME ONLY READS. Nothing here writes into Verse.Text
// or anything derived from it, so nothing here can reach search, sharing,
// copying, a link, the website or speech.

// omittedVersesIn returns the verse numbers this edition omits from one
// chapter, in ascending order, or nil.
//
// The edition is keyed by version ID, so an edition with no table — the
// licensed one — simply gets nothing rather than a guess.
func omittedVersesIn(versionID, book string, chapter int) []int {
	byBook, ok := omittedVerses[versionID]
	if !ok {
		return nil
	}
	return byBook[book][chapter]
}

// omitsVerse reports whether this edition omits one specific verse.
func omitsVerse(versionID, book string, chapter, verse int) bool {
	for _, v := range omittedVersesIn(versionID, book, chapter) {
		if v == verse {
			return true
		}
	}
	return false
}
