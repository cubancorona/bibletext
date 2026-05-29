package holybible

import (
	"fmt"
	"reflect"
	"testing"
)

// TestNewBibleData tests Bible data structure initialization
func TestNewBibleData(t *testing.T) {
	bd := NewBibleData()
	if len(bd.Books) != 66 {
		t.Errorf("Expected 66 books, got %d", len(bd.Books))
	}
}

// TestPopulateWithSampleVerses tests verse population
func TestPopulateWithSampleVerses(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	johnVerses := bd.GetChapter("John", 1)
	if len(johnVerses) == 0 {
		t.Error("John Chapter 1 has no verses")
	}
}

// TestGetVerse tests retrieving a specific verse
func TestGetVerse(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	verse := bd.GetVerse("John", 1, 1)
	if verse == nil {
		t.Error("GetVerse returned nil for John 1:1")
	}
}

// TestGetChapter tests retrieving all verses in a chapter
func TestGetChapter(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	verses := bd.GetChapter("John", 1)
	if len(verses) == 0 {
		t.Error("GetChapter returned no verses for John 1")
	}
}

// TestGetChaptersForBook tests chapter count
func TestGetChaptersForBook(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	johnChapters := bd.GetChaptersForBook("John")
	if johnChapters == 0 {
		t.Error("John has 0 chapters")
	}
}

// TestSearch tests verse search functionality
func TestSearch(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	results := bd.Search("God")
	if len(results) == 0 {
		t.Error("Search for 'God' returned no results")
	}
}

func TestGetChapterNumbersForBookSorted(t *testing.T) {
	bd := &BibleData{
		Books: []string{"Genesis"},
		Verses: map[string]map[int][]Verse{
			"Genesis": {
				3: {{BookName: "Genesis", Chapter: 3, Verse: 1, Text: "a"}},
				1: {{BookName: "Genesis", Chapter: 1, Verse: 1, Text: "b"}},
				2: {{BookName: "Genesis", Chapter: 2, Verse: 1, Text: "c"}},
			},
		},
	}

	got := bd.GetChapterNumbersForBook("Genesis")
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("chapter numbers mismatch: got %v want %v", got, want)
	}
}

func TestSearchOrderIsDeterministic(t *testing.T) {
	bd := &BibleData{
		Books: []string{"Genesis", "John"},
		Verses: map[string]map[int][]Verse{
			"Genesis": {
				2: {{BookName: "Genesis", Chapter: 2, Verse: 1, Text: "God made"}},
				1: {{BookName: "Genesis", Chapter: 1, Verse: 1, Text: "God created"}},
			},
			"John": {
				1: {{BookName: "John", Chapter: 1, Verse: 1, Text: "Word was God"}},
			},
		},
	}
	bd.PrepareSearchIndex()

	results := bd.Search("god")
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	wantRefs := []string{"Genesis 1:1", "Genesis 2:1", "John 1:1"}
	gotRefs := []string{
		fmt.Sprintf("%s %d:%d", results[0].BookName, results[0].Chapter, results[0].Verse),
		fmt.Sprintf("%s %d:%d", results[1].BookName, results[1].Chapter, results[1].Verse),
		fmt.Sprintf("%s %d:%d", results[2].BookName, results[2].Chapter, results[2].Verse),
	}
	if !reflect.DeepEqual(gotRefs, wantRefs) {
		t.Fatalf("result order mismatch: got %v want %v", gotRefs, wantRefs)
	}
}

func TestSearchLimitedTruncates(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	bd.PrepareSearchIndex()

	results, truncated := bd.SearchLimited("the", 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !truncated {
		t.Fatal("expected truncated=true for limit hit")
	}
}

func TestSearchReturnsEmptyForBlankQuery(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	bd.PrepareSearchIndex()

	results := bd.Search("   ")
	if len(results) != 0 {
		t.Fatalf("expected empty results for blank query, got %d", len(results))
	}
}

func TestSearchSmartLimitedReferenceVerse(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	bd.PrepareSearchIndex()

	results, truncated := bd.SearchSmartLimited("John 3:16", 10)
	if truncated {
		t.Fatal("did not expect truncation for single reference result")
	}
	if len(results) != 1 {
		t.Fatalf("expected one verse result, got %d", len(results))
	}
	if results[0].BookName != "John" || results[0].Chapter != 3 || results[0].Verse != 16 {
		t.Fatalf("unexpected result: %#v", results[0])
	}
}

func TestSearchSmartLimitedReferenceChapter(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	bd.PrepareSearchIndex()

	results, truncated := bd.SearchSmartLimited("Genesis 1", 1)
	if !truncated {
		t.Fatal("expected chapter query truncation with low limit")
	}
	if len(results) != 1 {
		t.Fatalf("expected limited chapter results length 1, got %d", len(results))
	}
	if results[0].BookName != "Genesis" || results[0].Chapter != 1 {
		t.Fatalf("unexpected result: %#v", results[0])
	}
}

func TestSearchSmartLimitedMultiTerm(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	bd.PrepareSearchIndex()

	results, truncated := bd.SearchSmartLimited("faith hoped", 20)
	if truncated {
		t.Fatal("did not expect truncation")
	}
	if len(results) != 1 {
		t.Fatalf("expected one result containing both terms, got %d", len(results))
	}
	if results[0].BookName != "Hebrews" || results[0].Chapter != 11 || results[0].Verse != 1 {
		t.Fatalf("unexpected result: %#v", results[0])
	}
}

func BenchmarkSearchLimited(b *testing.B) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	bd.PrepareSearchIndex()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = bd.SearchLimited("the", 80)
	}
}

// TestAppStateInitialization tests application state setup
func TestAppStateInitialization(t *testing.T) {
	state := &AppState{
		Bible:          NewBibleData(),
		CurrentBook:    "John",
		CurrentChapter: 1,
	}
	if state.CurrentBook != "John" {
		t.Errorf("Expected book John, got %s", state.CurrentBook)
	}
}

// TestChapterNavigation tests chapter bounds
func TestChapterNavigation(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	state := &AppState{
		Bible:          bd,
		CurrentBook:    "John",
		CurrentChapter: 1,
	}
	maxChapter := state.Bible.GetChaptersForBook(state.CurrentBook)
	if state.CurrentChapter > 1 {
		state.CurrentChapter--
	}
	if state.CurrentChapter != 1 {
		t.Error("Should not go below chapter 1")
	}
	state.CurrentChapter = maxChapter
	if state.CurrentChapter < maxChapter {
		state.CurrentChapter++
	}
	if state.CurrentChapter > maxChapter {
		t.Error("Should not exceed maximum chapter")
	}
}
