//go:build !next

package bibletext

// THE GREEK ESTHER IN THE SHIPPING BUILD. WEB Catholic prints the Greek
// Esther, and the shipping versification table (versification_data.go)
// records the whole book as incommensurable with the Hebrew Esther the other
// editions print: no reference, note, highlight or cross-reference crosses
// between them, and its three gaps are not called omissions.
//
// The next major release maps it verse for verse (greek_esther_next_test.go,
// docs/NEXT.md). These tests pin what the shipping build does until then, and
// are deleted on the day that release ships.

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// greekAdditionUnplacedSentence is the line under a note on one of the Greek
// Esther's additions read in the WEB, and greekAdditionUnplacedSeen the start
// of it a banner shows, as seenText reads it. In the shipping build no verse
// of the book can be placed: the numbering does not correspond.
const (
	greekAdditionUnplacedSentence = "The numbering here does not correspond to the note's."
	greekAdditionUnplacedSeen     = "the numbering here does not"
)

// No verse of the Greek Esther maps into the Hebrew Esther or back.
func TestTheGreekEstherCannotBeMapped(t *testing.T) {
	for _, tc := range []struct {
		name           string
		from, to, book string
		chapter, verse int
	}{
		{"Esther cannot be mapped into WEBC", "web", "webc", "Esther", 4, 1},
		{"nor back out of it", "webc", "web", "Esther", 1, 1},
		{"nor from the NKJV", "nkjv", "webc", "Esther", 10, 3},
	} {
		if ch, v, res := MapVerse(tc.from, tc.to, tc.book, tc.chapter, tc.verse); res != verseMapIncommensurable || ch != 0 || v != 0 {
			t.Errorf("%s: %s->%s Esther %d:%d = %d:%d (%s), want 0:0 (incommensurable)",
				tc.name, tc.from, tc.to, tc.chapter, tc.verse, ch, v, res)
		}
	}
	if VerseExistsIn("webc", "Esther", 4, 1) {
		t.Error("VerseExistsIn offers a Greek Esther verse; there is no correspondence to offer")
	}
	if IncommensurableBook("webc", "Esther") == "" {
		t.Error("IncommensurableBook gives no reason for the Greek Esther")
	}
}

// Every chapter of the Greek Esther differs from the NKJV's, and the kind is
// incommensurable, so the web page says the book does not correspond.
func TestTheGreekEstherNumbersEveryChapterIncommensurably(t *testing.T) {
	for _, tc := range []struct{ chapter, span int }{{1, 22}, {4, 17}, {9, 32}} {
		if ChapterNumberingAgrees("nkjv", "webc", "Esther", tc.chapter, tc.span) {
			t.Errorf("nkjv->webc Esther %d agrees; the Greek Esther is incommensurable", tc.chapter)
		}
		if got := ChapterNumberingDifference("nkjv", "webc", "Esther", tc.chapter, tc.span); got != NumberingIncommensurable {
			t.Errorf("nkjv->webc Esther %d: got %q, want %q", tc.chapter, got, NumberingIncommensurable)
		}
	}
}

// Greek Esther's numbering corresponds to nothing, so its gaps are not
// omissions and must never be marked as such.
func TestGreekEsthersGapsAreNotCalledOmissions(t *testing.T) {
	for _, v := range []int{6} {
		if omitsVerse("webc", "Esther", 4, v) {
			t.Errorf("Esther 4:%d is recorded as omitted; Greek Esther is incommensurable "+
				"and a gap in it is not an omission", v)
		}
	}
	if got := omittedVersesIn("webc", "Esther", 9); got != nil {
		t.Errorf("Greek Esther chapter 9 reports omissions %v", got)
	}
}

// A note on the Hebrew Esther has nowhere to land in the Greek Esther, and the
// reason is the numbering.
func TestANoteOnTheGreekEstherIsIncommensurable(t *testing.T) {
	bible := anchorTestBible()
	for _, n := range []StoredNote{
		{Kind: noteKindReceived, VersionID: "web", Book: "Esther", Chapter: 4, VerseLo: 1, Text: "x"},
		{Kind: noteKindReceived, VersionID: "web", Book: "Esther", Chapter: 4, VerseLo: 6, Text: "x"},
	} {
		if got := resolveNoteAnchor(n, "webc", bible); got.Kind != unplacedIncommensurable || len(got.Here) != 0 {
			t.Errorf("web Esther 4:%d read in webc: got %v %+v, want unplaced-incommensurable", n.VerseLo, got.Kind, got)
		}
	}
	// And the other way: a note on one of its additions, read in the WEB.
	n := StoredNote{Kind: noteKindReceived, VersionID: "webc", Book: "Esther", Chapter: 4, VerseLo: 20, Text: "x"}
	if got := resolveNoteAnchor(n, "web", bible); got.Kind != unplacedIncommensurable {
		t.Errorf("webc Esther 4:20 read in web: got %v, want unplaced-incommensurable", got.Kind)
	}
}

// ...but only where the passage genuinely corresponds. Greek Esther is a
// different book from Esther, not a renumbering, so a note on one says nothing
// about the other — MapVerse calls that incommensurable and the note must stay
// where it is rather than being planted on unrelated text.
func TestANoteDoesNotFollowIntoTheGreekEsther(t *testing.T) {
	p := newNotePrefs()
	addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Esther", Chapter: 4, VerseLo: 1,
		Text: "fixture translation message alpha"})

	if _, ok := noteForChapter(p, "webc", "Esther", 4, nil); ok {
		t.Error("a note crossed into Greek Esther, where its verse numbers mean something else")
	}
}

// A highlight on the Greek Esther clears when the reader switches to the WEB,
// at a verse number both have as much as at one of its additions.
func TestAHighlightOnTheGreekEstherClears(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	if _, _, res := MapVerse("webc", "web", "Esther", 1, 1); res != verseMapIncommensurable {
		t.Fatalf("precondition: webc Esther should be INCOMMENSURABLE with web, got %v", res)
	}
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	st := &AppState{
		Bible: bd, CurrentBook: "Esther", CurrentChapter: 1,
		CurrentVersion: "webc", loadPhase: loadReady,
		loadedVersions: map[string]*BibleData{"webc": bd},
	}
	goToVerseRange(st, "Esther", 1, 1, 1)
	web := NewBibleData()
	web.PopulateWithSampleVerses()
	applyVersionSwitchForTest(t, st, "web", web)
	if sp, ok := st.markSpan(); ok {
		t.Errorf("an incommensurable mark must clear, still lights %d:%d", sp.Chapter, sp.Lo)
	}

	for _, lo := range []int{17, 20} {
		st := markedInGreekEsther(t, lo)
		applyVersionSwitchForTest(t, st, "web", estherFourBible(17, 0))
		if sp, ok := st.markSpan(); ok {
			t.Errorf("a mark on the Greek Esther's 4:%d must clear, still lights %d:%d", lo, sp.Chapter, sp.Lo)
		}
	}
}

// The sentence a reader of the shipping build sees under a note the Greek
// Esther's numbering keeps from being placed.
func TestTheIncommensurableSentenceIsTheShippingOne(t *testing.T) {
	const want = "The numbering here does not correspond to the note's."
	if got := placementCopy(unplacedIncommensurable); got != want {
		t.Errorf("the incommensurable sentence is %q, want %q", got, want)
	}
}

// A book whose numbering does not correspond at all cannot be checked for
// completeness, and the Greek Esther is one.
func TestExpectedVerseNumbersRefuseTheGreekEsther(t *testing.T) {
	var one []Verse
	for v := 1; v <= 22; v++ {
		one = append(one, Verse{BookName: "Esther", Chapter: 1, Verse: v, Text: "x"})
	}
	ref := &BibleData{Books: []string{"Esther"}, Verses: map[string]map[int][]Verse{"Esther": {1: one}}}
	if _, err := ExpectedVerseNumbers("webc", ref); err == nil {
		t.Error("WEBC's Greek Esther was given expected verse numbers")
	}
}
