package bibletext

import (
	"strings"
	"testing"
)

// S11 of docs/SCRIPTURE_WORKLIST.md. A search for "Absalom" did not find Psalm
// 3, because the index was built from verse text alone — even though the psalm
// says so in its own title. Titles are indexed now, keyed to the chapter at
// verse 0 and carrying no text of their own.

func psalmSearchBible() *BibleData {
	return &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {
			3: {
				{BookName: "Psalms", Book: "Psalms", Chapter: 3, Verse: 1,
					Text: "Yahweh, how my adversaries have increased!", Search: "yahweh, how my adversaries have increased!"},
			},
			4: {
				{BookName: "Psalms", Book: "Psalms", Chapter: 4, Verse: 1,
					Text: "Answer me when I call.", Search: "answer me when i call."},
			},
		}},
		Superscriptions: map[string]map[int]Superscription{"Psalms": {
			3: {Text: "A Psalm by David, when he fled from Absalom his son."},
			4: {Text: "For the Chief Musician. On stringed instruments. A Psalm by David."},
		}},
	}
}

func TestSearchingATitleFindsItsPsalm(t *testing.T) {
	bd := psalmSearchBible()
	got, _ := bd.SearchSmartLimited("Absalom", 50)
	if len(got) == 0 {
		t.Fatal(`"Absalom" found nothing; the title is not indexed`)
	}
	hit := got[0]
	if hit.Chapter != 3 {
		t.Errorf("the hit is Psalm %d, want 3", hit.Chapter)
	}
	if hit.Verse != 0 {
		t.Errorf("a title hit is keyed to verse %d, want 0 — the chapter key", hit.Verse)
	}
	// The load-bearing property: the title must NOT ride on the Verse, or it
	// could be shared or sent to an assistant as though it were a verse.
	if hit.Text != "" {
		t.Errorf("the title rode on the result's Text: %q", hit.Text)
	}
}

// CONTROL: an ordinary verse search must be unaffected, and must not start
// returning verse-0 rows for words that are only in verses.
func TestSearchingAVerseIsUnchanged(t *testing.T) {
	bd := psalmSearchBible()
	got, _ := bd.SearchSmartLimited("adversaries", 50)
	if len(got) != 1 {
		t.Fatalf("%d results for a word only in a verse, want 1: %+v", len(got), got)
	}
	if got[0].Verse != 1 || !strings.Contains(got[0].Text, "adversaries") {
		t.Errorf("the verse hit is wrong: %+v", got[0])
	}
}

// A word in BOTH a title and a verse returns both, and they are distinguishable.
func TestAWordInATitleAndAVerseReturnsBoth(t *testing.T) {
	bd := psalmSearchBible()
	bd.Verses["Psalms"][3] = append(bd.Verses["Psalms"][3], Verse{
		BookName: "Psalms", Book: "Psalms", Chapter: 3, Verse: 2,
		Text: "David sang of Absalom.", Search: "david sang of absalom.",
	})
	got, _ := bd.SearchSmartLimited("Absalom", 50)
	titles, verses := 0, 0
	for _, r := range got {
		if r.Verse == 0 {
			titles++
		} else {
			verses++
		}
	}
	if titles != 1 || verses != 1 {
		t.Errorf("got %d title hits and %d verse hits, want 1 and 1: %+v", titles, verses, got)
	}
}

// Headings are deliberately OUT of scope. The Berean carries 3,091 section
// headings against 116 titles and the World English carries none at all, so
// indexing them would give the reader a search that changes shape with the
// version picker — and the heading vocabulary is the verse vocabulary, so the
// rows would compete with the verses they name for the result cap.
func TestSectionHeadingsAreNotIndexed(t *testing.T) {
	bd := psalmSearchBible()
	bd.Headings = map[string]map[int][]Heading{"Psalms": {
		3: {{Text: "A Cry Of Trust In Adversity", Style: "heading", BeforeVerse: 1}},
	}}
	got, _ := bd.SearchSmartLimited("Trust", 50)
	for _, r := range got {
		t.Errorf("a section heading was indexed: %+v", r)
	}
}

// A chapter with no title contributes nothing, and an edition with no titles at
// all must behave exactly as before.
func TestAChapterWithNoTitleAddsNoResult(t *testing.T) {
	bd := psalmSearchBible()
	delete(bd.Superscriptions, "Psalms")
	got, _ := bd.SearchSmartLimited("Absalom", 50)
	if len(got) != 0 {
		t.Errorf("with no titles, %d results: %+v", len(got), got)
	}
	if verses, _ := bd.SearchSmartLimited("adversaries", 50); len(verses) != 1 {
		t.Errorf("verse search broke when titles were absent: %+v", verses)
	}
}
