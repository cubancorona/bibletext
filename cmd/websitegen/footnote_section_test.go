package main

import (
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

// S21 of docs/SCRIPTURE_WORKLIST.md. The site rendered no apparatus at all, so
// the translators' notes, the explanations of an omitted verse and a psalm
// title's notes were absent from every page. They are rendered now, always
// visible: the site has no settings, so there is no toggle to honour and
// nothing to remember a choice in.

func notedChapter() (*bibletext.BibleData, []bibletext.Verse) {
	verses := []bibletext.Verse{
		{BookName: "Luke", Book: "Luke", Chapter: 17, Verse: 35,
			Text:      "There will be two grinding grain together.",
			Footnotes: []bibletext.Footnote{{Text: "Some manuscripts add: and one is taken."}}},
		{BookName: "Luke", Book: "Luke", Chapter: 17, Verse: 37,
			Text: "They answered, Where, Lord?"},
	}
	bd := &bibletext.BibleData{
		Verses: map[string]map[int][]bibletext.Verse{"Luke": {17: verses}},
		OrphanFootnotes: map[string]map[int][]bibletext.OrphanFootnote{
			"Luke": {17: {{Verse: 36, Text: "Some manuscripts add verse 36."}}},
		},
	}
	return bd, verses
}

func TestAChapterPageCarriesItsNotes(t *testing.T) {
	bd, verses := notedChapter()
	got := chapterNotes(bd, "Luke", 17, verses)

	for _, want := range []string{
		"Some manuscripts add: and one is taken.",
		"Some manuscripts add verse 36.",
		`<aside class="notes">`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the notes section is missing %q:\n%s", want, got)
		}
	}
	// The omitted verse's note sorts between its neighbours, not at the end.
	if strings.Index(got, ">35<") > strings.Index(got, ">36<") {
		t.Errorf("the notes are not in verse order:\n%s", got)
	}
}

// The apparatus sits OUTSIDE the article: the article is the scripture, and
// nothing here may reach a verse.
func TestTheNotesSectionSitsOutsideTheScripture(t *testing.T) {
	bd, verses := notedChapter()
	body := chapterBody(bd, "web", "Luke", 17, verses)
	if strings.Contains(body, "Some manuscripts") {
		t.Errorf("a note reached the chapter body:\n%s", body)
	}
	if strings.Contains(body, "notes") {
		t.Errorf("the notes section was written inside the article:\n%s", body)
	}
}

// CONTROL: a chapter with no notes writes nothing at all — no empty aside, no
// stray rule across the foot of the page.
func TestAChapterWithNoNotesWritesNothing(t *testing.T) {
	verses := []bibletext.Verse{
		{BookName: "John", Book: "John", Chapter: 1, Verse: 1, Text: "In the beginning was the Word."},
	}
	bd := &bibletext.BibleData{
		Verses: map[string]map[int][]bibletext.Verse{"John": {1: verses}},
	}
	if got := chapterNotes(bd, "John", 1, verses); got != "" {
		t.Errorf("a chapter with no notes produced %q", got)
	}
	if got := chapterNotes(nil, "John", 1, verses); got != "" {
		t.Errorf("a nil BibleData produced %q", got)
	}
}

// A psalm title's notes are keyed "Title" and sort before verse 1, because the
// title precedes verse 1 on the page.
func TestATitlesNotesAreKeyedTitleAndComeFirst(t *testing.T) {
	verses := []bibletext.Verse{
		{BookName: "Psalms", Book: "Psalms", Chapter: 3, Verse: 1, Text: "Yahweh, how my adversaries have increased!",
			Footnotes: []bibletext.Footnote{{Text: "A note on verse one."}}},
	}
	bd := &bibletext.BibleData{
		Verses: map[string]map[int][]bibletext.Verse{"Psalms": {3: verses}},
		Superscriptions: map[string]map[int]bibletext.Superscription{
			"Psalms": {3: {
				Text:      "A Psalm by David.",
				Footnotes: []bibletext.Footnote{{Text: "A note on the title."}},
			}},
		},
	}
	got := chapterNotes(bd, "Psalms", 3, verses)
	if !strings.Contains(got, ">Title<") {
		t.Errorf("a title note is not keyed Title:\n%s", got)
	}
	if strings.Index(got, "A note on the title.") > strings.Index(got, "A note on verse one.") {
		t.Errorf("the title's note does not come first:\n%s", got)
	}
}

// Cross references stay out, and the exclusion is the app's rather than one
// re-derived here. The NKJV's entire apparatus is cross references, so this is
// what keeps a licensed publisher's editorial work off the site — belt and
// braces, since a licensed chapter has no page here at all.
func TestCrossReferencesAreNotRendered(t *testing.T) {
	verses := []bibletext.Verse{
		{BookName: "John", Book: "John", Chapter: 3, Verse: 16, Text: "For God so loved the world.",
			Footnotes: []bibletext.Footnote{
				{Text: "A wording note."},
				{Text: "John 7:50; 19:39", Kind: "crossref"},
			}},
	}
	bd := &bibletext.BibleData{
		Verses: map[string]map[int][]bibletext.Verse{"John": {3: verses}},
	}
	got := chapterNotes(bd, "John", 3, verses)
	if !strings.Contains(got, "A wording note.") {
		t.Fatalf("the ordinary note is missing, so this test proves nothing:\n%s", got)
	}
	if strings.Contains(got, "19:39") {
		t.Errorf("a cross reference was rendered:\n%s", got)
	}
}

func TestNotesAreEscaped(t *testing.T) {
	verses := []bibletext.Verse{
		{BookName: "John", Book: "John", Chapter: 1, Verse: 1, Text: "In the beginning.",
			Footnotes: []bibletext.Footnote{{Text: `<script>alert(1)</script> & "so"`}}},
	}
	bd := &bibletext.BibleData{Verses: map[string]map[int][]bibletext.Verse{"John": {1: verses}}}
	got := chapterNotes(bd, "John", 1, verses)
	if strings.Contains(got, "<script>") {
		t.Errorf("a note was not escaped:\n%s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("a note is not escaped as expected:\n%s", got)
	}
}
