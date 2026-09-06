package bibletext

// THE WEB'S PARAGRAPHS COME FROM ITS PUBLISHER, NOT ITS FEED.
//
// Paragraphing is the translators' reading of a passage, and the app draws it
// wherever an edition supplies it (Verse.ParaStart). The BSB's feed and the
// NKJV's carry that structure and their decoders read it directly. The World
// English Bible's runtime feed does not: it sends 742 break nodes for a whole
// Bible and none at all in Genesis 1, John 3 or Romans 8, while the edition's
// own USFM carries thousands and breaks Genesis 1 at the days of creation.
// About 92% of the paragraphing is lost in the supply, so it is recovered
// from the publisher's files into a generated table of verse references
// (paragraph_web_data.go, scripts/gen-web-paragraphs.py) — the same answer,
// for the same reason, as the red-letter span tables.
//
// The table is a set of references, so it carries no text and cannot drift
// against the wording the way an offset table can. A reference for a verse
// this edition does not have is simply not found.

// applyPublisherParagraphs marks every verse the table names as opening a
// paragraph. It ADDS to whatever the feed already marked rather than
// replacing it: a break the feed did carry is the publisher's too.
func applyPublisherParagraphs(bd *BibleData, table map[string][]int) {
	if bd == nil || len(table) == 0 {
		return
	}
	for key, verses := range table {
		usfm, chapter, ok := splitParagraphKey(key)
		if !ok {
			continue
		}
		book := apiBibleBookName(usfm)
		if book == "" {
			continue
		}
		inChapter := bd.Verses[book][chapter]
		if len(inChapter) == 0 {
			continue
		}
		for _, want := range verses {
			// The paragraph opens at that verse, or — where the edition omits
			// it, as it omits Acts 8:37 — at the next verse it does have. The
			// publisher put a break there; dropping it because the verse
			// beneath it is absent would silently close the paragraph up.
			for i := range inChapter {
				if inChapter[i].Verse >= want {
					inChapter[i].ParaStart = true
					break
				}
			}
		}
	}
}

// splitParagraphKey parses a table key ("GEN 1") into its USFM id and chapter.
func splitParagraphKey(key string) (usfm string, chapter int, ok bool) {
	i := len(key) - 1
	for i >= 0 && key[i] >= '0' && key[i] <= '9' {
		i--
	}
	if i < 1 || i == len(key)-1 || key[i] != ' ' {
		return "", 0, false
	}
	return key[:i], leadingInt(key[i+1:]), true
}
