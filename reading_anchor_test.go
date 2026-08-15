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

// TestChapterTextHighlightBand pins the geometry behind the Win/Linux visible
// highlight: the band must cover the highlighted verse's wrapped lines (start
// at its first line, extend through its last), and a multi-verse range must
// extend the band to the range's end verse.
func TestChapterTextHighlightBand(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := sampleState()
	verses := state.Bible.GetChapter("John", 1)
	if len(verses) < 3 {
		t.Fatalf("sample chapter too small: %d verses", len(verses))
	}

	// No highlight → no band.
	c := newChapterText(state, verses)
	c.rewrap(300)
	if _, _, ok := c.highlightBand(); ok {
		t.Error("no highlighted verse, but highlightBand reported one")
	}

	// Single verse highlighted (the middle one, so y > 0).
	mid := verses[len(verses)/2]
	state.setHL(hlSearch, mid.BookName, mid.Chapter, mid.Verse, 0)
	c = newChapterText(state, verses)
	c.rewrap(300) // narrow → the verse spans multiple wrapped lines
	y, h, ok := c.highlightBand()
	if !ok {
		t.Fatal("highlighted verse but no band")
	}
	if y <= 0 {
		t.Errorf("mid-chapter verse must start below the top, y=%v", y)
	}
	lineH := c.MinSize().Height / float32(c.totalLines)
	if h < lineH-0.5 {
		t.Errorf("band height %v is thinner than one line (%v)", h, lineH)
	}
	if c.highlightEndLine < c.highlightLine {
		t.Errorf("end line %d before start line %d", c.highlightEndLine, c.highlightLine)
	}

	// A range (verse..verse+1) must produce a band at least as tall.
	state.setHL(hlSearch, mid.BookName, mid.Chapter, mid.Verse, mid.Verse+1)
	c2 := newChapterText(state, verses)
	c2.rewrap(300)
	_, h2, ok2 := c2.highlightBand()
	if !ok2 || h2 < h-0.5 {
		t.Errorf("range band (h=%v) must be at least the single-verse band (h=%v)", h2, h)
	}
	state.clearMark()
}
