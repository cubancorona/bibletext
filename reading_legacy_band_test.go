package bibletext

import "testing"

// The legacy pane's band opens on the span's OWN Lo verse, even when that verse
// is not in the chapter being drawn.
//
// Not a hypothetical: WEB and BSB omit Mark 9:44, 9:46, 11:26, Matthew 17:21,
// John 5:4 and Acts 8:37, and a mark can arrive from a stored note's span or a
// link fragment with nothing checking the verse exists in this translation. The
// S3 tint refactor briefly rewrote this as "the first tinted verse we reach",
// which agrees with the old rule ONLY when Lo is present -- so it silently
// changed behaviour on exactly the chapters that are missing verses.
func TestLegacyBandOpensOnTheSpansOwnLoVerse(t *testing.T) {
	// Verse 2 does not exist here, and the mark starts at it.
	vs := []Verse{
		{BookName: "Mark", Book: "Mark", Chapter: 9, Verse: 1, Text: "Alpha."},
		{BookName: "Mark", Book: "Mark", Chapter: 9, Verse: 3, Text: "Gamma."},
		{BookName: "Mark", Book: "Mark", Chapter: 9, Verse: 4, Text: "Delta."},
	}
	bd := &BibleData{Books: []string{"Mark"},
		Verses: map[string]map[int][]Verse{"Mark": {9: vs}}}
	st := &AppState{Bible: bd, CurrentBook: "Mark", CurrentChapter: 9}
	st.setHL(hlSearch, "Mark", 9, 2, 4)

	c := newChapterText(st, vs)
	c.rewrap(300)

	if c.highlightLine >= 0 {
		t.Errorf("band opened at line %d for a span whose Lo verse (2) is absent; "+
			"the shipped rule draws no band there.\n"+
			"If this is now a deliberate improvement, change it in its own commit "+
			"with its own evidence -- not inside a refactor that claims to be invisible.",
			c.highlightLine)
	}

	// ...and it still opens normally when Lo IS present.
	st.setHL(hlSearch, "Mark", 9, 3, 4)
	c2 := newChapterText(st, vs)
	c2.rewrap(300)
	if c2.highlightLine < 0 {
		t.Error("no band for a span whose Lo verse is present")
	}
}
