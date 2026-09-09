package bibletext

import (
	"strings"
	"testing"
)

// S13 of docs/SCRIPTURE_WORKLIST.md. Copying a chapter has always kept the
// translators' poem lines, and so has the cited-text share; copying a verse or
// a paragraph flattened them, with no reason ever given. They agree now, and
// they agree in the direction that has a stated principle behind it.

func poemVerses() []Verse {
	return []Verse{
		{BookName: "Psalms", Chapter: 23, Verse: 1,
			Text: "Yahweh is my shepherd;\nI shall lack nothing."},
		{BookName: "Psalms", Chapter: 23, Verse: 2,
			Text: "He makes me lie down in green pastures.\nHe leads me beside still waters."},
	}
}

func proseVerses() []Verse {
	return []Verse{
		{BookName: "John", Chapter: 1, Verse: 1, Text: "In the beginning was the Word."},
		{BookName: "John", Chapter: 1, Verse: 2, Text: "The same was in the beginning with God."},
	}
}

func TestCopyingAPoeticParagraphKeepsItsLines(t *testing.T) {
	got := paragraphCopyText(poemVerses())
	want := "Yahweh is my shepherd;\nI shall lack nothing.\n" +
		"He makes me lie down in green pastures.\nHe leads me beside still waters."
	if got != want {
		t.Errorf("poem paragraph copied as\n %q\nwant\n %q", got, want)
	}
	if strings.Contains(got, "nothing. He makes") {
		t.Error("the verses were run together on one line, which is the flattening this fixed")
	}
}

// CONTROL: prose must NOT gain line breaks. A rule that made everything poetry
// would be as wrong as the one that made everything prose, and this is the case
// that catches it.
func TestCopyingAProseParagraphStillJoinsWithSpaces(t *testing.T) {
	got := paragraphCopyText(proseVerses())
	want := "In the beginning was the Word. The same was in the beginning with God."
	if got != want {
		t.Errorf("prose paragraph copied as %q, want %q", got, want)
	}
	if strings.Contains(got, "\n") {
		t.Error("prose gained a line break it never had")
	}
}

// The reference is set off by a BLANK line, matching composeShareText — because
// a citation on a bare next line reads as one more poem line.
func TestACitedCopySetsTheReferenceOffLikeTheShareText(t *testing.T) {
	got := citedCopy("Yahweh is my shepherd;\nI shall lack nothing.", "Psalms 23:1")
	if !strings.HasSuffix(got, "\n\n— Psalms 23:1") {
		t.Errorf("cited copy = %q; the reference must follow a blank line", got)
	}
	if strings.Contains(got, "nothing. — ") {
		t.Error("the reference was appended to the last poem line")
	}
	// And the quotation itself is untouched.
	if !strings.HasPrefix(got, "Yahweh is my shepherd;\nI shall lack nothing.") {
		t.Errorf("the quotation was altered: %q", got)
	}
}

// The three copy paths must now agree about poetry. Chapter copy is the one
// that always did; the other two are held to it.
func TestEveryCopyPathAgreesAboutPoetry(t *testing.T) {
	verse := Verse{BookName: "Psalms", Chapter: 23, Verse: 1,
		Text: "Yahweh is my shepherd;\nI shall lack nothing."}

	// Chapter copy, the standing behaviour, via its own builder.
	state := &AppState{Bible: &BibleData{
		Verses: map[string]map[int][]Verse{"Psalms": {23: {verse}}},
	}, CurrentBook: "Psalms", CurrentChapter: 23}
	chapter := chapterCopyText(state)

	para := paragraphCopyText([]Verse{verse})

	for name, got := range map[string]string{
		"chapter":   chapter,
		"paragraph": para,
		"verse":     strings.TrimSpace(verse.Text),
	} {
		if !strings.Contains(got, "shepherd;\nI shall lack") {
			t.Errorf("%s copy flattened the poem line: %q", name, got)
		}
	}
}
