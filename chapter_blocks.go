package bibletext

// A CHAPTER AS IT IS SET ON THE PAGE, decided once for every surface.
//
// The publishers put section headings in their text — "The Beatitudes", "Jesus
// Feeds the Five Thousand" — and every edition the app reads carries them. They
// have been captured for some time and nothing drew them, because a heading is
// not a fact about a verse: it sits BETWEEN verses, and each of the four
// renderers builds its own idea of what a chapter looks like.
//
// So the shape is decided here instead, in the order the page has: heading,
// paragraph, heading, paragraph. A surface walks the blocks and only has to
// know how to set two things, which is a smaller thing to know than how a
// chapter is arranged.
//
// It is deliberately NOT folded into groupVersesIntoParagraphs. That function
// answers a different question — which verses share a paragraph — and the note
// machinery asks it for geometry that has nothing to do with headings.

// chapterBlock is one piece of a chapter: either a heading the publisher
// placed, or a paragraph of verses. Never both.
type chapterBlock struct {
	// Heading is set for a heading block, nil for a paragraph.
	Heading *Heading
	// Verses is the paragraph's verses, nil for a heading.
	Verses []Verse
}

// IsHeading reports whether this block is a heading rather than a paragraph.
func (b chapterBlock) IsHeading() bool { return b.Heading != nil }

// chapterBlocksFor returns the chapter in the order it is set: the publisher's
// headings among the paragraphs its own paragraph marks made.
//
// A heading always OPENS a paragraph, which is what print does and what makes
// it read as a heading rather than as an interruption. Where the publisher's
// heading names a verse partway through one of its own paragraphs — the two
// come from the same markup, so it is rare — the paragraph is split at that
// verse rather than the heading being moved: the heading's placement is the
// publisher's statement about the text, and the paragraph break is the softer
// of the two claims.
func chapterBlocksFor(bd *BibleData, book string, chapter int, verses []Verse) []chapterBlock {
	paras := groupVersesIntoParagraphs(verses)
	heads := headingsFor(bd, book, chapter)
	if len(heads) == 0 {
		out := make([]chapterBlock, 0, len(paras))
		for _, p := range paras {
			out = append(out, chapterBlock{Verses: p})
		}
		return out
	}

	// Which heading stands above which verse. A verse can carry at most one:
	// where a publisher stacks a major heading and a section heading on the
	// same verse, both are kept in order and both are emitted.
	above := make(map[int][]*Heading, len(heads))
	for i := range heads {
		v := heads[i].BeforeVerse
		if v <= 0 || heads[i].Text == "" {
			continue
		}
		above[v] = append(above[v], &heads[i])
	}

	out := make([]chapterBlock, 0, len(paras)+len(heads))
	for _, para := range paras {
		start := 0
		for i, v := range para {
			hs := above[v.Verse]
			if len(hs) == 0 {
				continue
			}
			// A heading inside the paragraph ends the part before it.
			if i > start {
				out = append(out, chapterBlock{Verses: para[start:i]})
				start = i
			}
			for _, h := range hs {
				out = append(out, chapterBlock{Heading: h})
			}
		}
		if start < len(para) {
			out = append(out, chapterBlock{Verses: para[start:]})
		}
	}
	return out
}

// headingsFor returns the publisher's headings for one chapter, or nil.
func headingsFor(bd *BibleData, book string, chapter int) []Heading {
	if bd == nil || bd.Headings == nil {
		return nil
	}
	return bd.Headings[book][chapter]
}
