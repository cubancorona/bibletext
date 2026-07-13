package bibletext

// Tests for chapterText's verse→line index (verseLines) and the geometry
// helpers behind the Fyne within-chapter scroll anchor. Untagged: the index is
// built on every platform, so this runs on the host too, not just Linux/Windows.

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestChapterTextVerseAnchors(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := sampleState()
	verses := state.Bible.GetChapter("John", 1)
	if len(verses) < 3 {
		t.Fatalf("sample chapter too small for the test: %d verses", len(verses))
	}

	c := newChapterText(state, verses)
	c.rewrap(300) // narrow, so verses span multiple wrapped lines

	if len(c.verseLines) != len(verses) {
		t.Fatalf("verseLines must index every verse: got %d, want %d", len(c.verseLines), len(verses))
	}
	for i := 1; i < len(c.verseLines); i++ {
		if c.verseLines[i].line < c.verseLines[i-1].line {
			t.Fatalf("verse start lines must be non-decreasing: %v then %v", c.verseLines[i-1], c.verseLines[i])
		}
	}
	if last := c.verseLines[len(c.verseLines)-1].line; last >= c.totalLines {
		t.Fatalf("verse line %d out of range (totalLines %d)", last, c.totalLines)
	}

	// Round trip: the verse at a verse's own top Y must start on that same line
	// (several short verses can share a wrapped line, so compare lines, not
	// verse numbers).
	for _, vl := range c.verseLines {
		y, ok := c.yForVerse(vl.verse)
		if !ok {
			t.Fatalf("yForVerse(%d) not found", vl.verse)
		}
		got, delta := c.verseAtY(y + 0.5)
		gy, ok := c.yForVerse(got)
		if !ok || gy != y {
			t.Fatalf("verseAtY(yForVerse(%d)+0.5) = verse %d at y %v, want a verse at y %v", vl.verse, got, gy, y)
		}
		if delta < 0 || delta > 1 {
			t.Fatalf("delta just past a verse top should be ~0.5, got %v", delta)
		}
	}

	// An offset well past a verse's top reports that verse plus the distance.
	lastVL := c.verseLines[len(c.verseLines)-1]
	y, _ := c.yForVerse(lastVL.verse)
	if v, delta := c.verseAtY(y + 7); v != lastVL.verse || delta < 6.5 || delta > 7.5 {
		t.Fatalf("verseAtY(last verse + 7px) = (%d, %v), want (%d, ~7)", v, delta, lastVL.verse)
	}

	// Rewrap resets the index (no duplicate accumulation).
	c.rewrap(500)
	if len(c.verseLines) != len(verses) {
		t.Fatalf("rewrap must rebuild verseLines, got %d entries for %d verses", len(c.verseLines), len(verses))
	}

	// Top of chapter: no anchor.
	if v, d := c.verseAtY(0); v != 0 || d != 0 {
		t.Fatalf("verseAtY(0) must report the chapter top, got (%d, %v)", v, d)
	}
}
