package bibletext

import (
	"fmt"
	"reflect"
	"strings"
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
		t.Fatal("John Chapter 1 has no verses")
	}
	// Every populated verse must be self-consistent — the right book/chapter
	// stamped on it and real text — since navigation and search trust these.
	for i, v := range johnVerses {
		if v.BookName != "John" || v.Chapter != 1 {
			t.Fatalf("John 1 verse %d stamped %s %d", i, v.BookName, v.Chapter)
		}
		if v.Verse != i+1 {
			t.Fatalf("John 1 verses out of order: index %d holds verse %d", i, v.Verse)
		}
		if strings.TrimSpace(v.Text) == "" {
			t.Fatalf("John 1:%d has empty text", v.Verse)
		}
	}
}

// TestGetVerse tests retrieving a specific verse
func TestGetVerse(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	verse := bd.GetVerse("John", 1, 1)
	if verse == nil {
		t.Fatal("GetVerse returned nil for John 1:1")
	}
	// The RIGHT verse, not just any verse.
	if verse.BookName != "John" || verse.Chapter != 1 || verse.Verse != 1 {
		t.Fatalf("GetVerse(John,1,1) returned %s %d:%d", verse.BookName, verse.Chapter, verse.Verse)
	}
	if !strings.Contains(verse.Text, "beginning") {
		t.Fatalf("John 1:1 text looks wrong: %q", verse.Text)
	}
	if bd.GetVerse("John", 1, 9999) != nil {
		t.Fatal("GetVerse must return nil for a verse that does not exist")
	}
	if bd.GetVerse("Nobook", 1, 1) != nil {
		t.Fatal("GetVerse must return nil for an unknown book")
	}
}

// TestGetChapter tests retrieving all verses in a chapter
func TestGetChapter(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	verses := bd.GetChapter("John", 1)
	if len(verses) == 0 {
		t.Fatal("GetChapter returned no verses for John 1")
	}
	for _, v := range verses {
		if v.BookName != "John" || v.Chapter != 1 {
			t.Fatalf("GetChapter(John,1) leaked %s %d:%d", v.BookName, v.Chapter, v.Verse)
		}
	}
	if got := bd.GetChapter("John", 9999); len(got) != 0 {
		t.Fatalf("GetChapter must be empty for a missing chapter, got %d verses", len(got))
	}
}

// TestGetChaptersForBook tests chapter count
func TestGetChaptersForBook(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	johnChapters := bd.GetChaptersForBook("John")
	if johnChapters == 0 {
		t.Fatal("John has 0 chapters")
	}
	// The count must agree with the actual chapter list (chapters can be
	// sparse in sample data, so count ≠ highest number).
	nums := bd.GetChapterNumbersForBook("John")
	if johnChapters != len(nums) {
		t.Fatalf("GetChaptersForBook(John) = %d, but chapters present are %v", johnChapters, nums)
	}
	if bd.GetChaptersForBook("Nobook") != 0 {
		t.Fatal("GetChaptersForBook must be 0 for an unknown book")
	}
}

// TestSearch tests verse search functionality
func TestSearch(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	results := bd.Search("God")
	if len(results) == 0 {
		t.Fatal("Search for 'God' returned no results")
	}
	// Hits must actually contain the term (case-insensitively) — a regression
	// returning arbitrary verses would otherwise pass.
	for _, v := range results {
		if !strings.Contains(strings.ToLower(v.Text), "god") {
			t.Fatalf("Search(\"God\") returned a non-matching verse %s %d:%d: %q",
				v.BookName, v.Chapter, v.Verse, v.Text)
		}
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

// TestNewLoadingState locks the pre-load state contract: the UI renders the
// loading screen off loadPhase, and the annotation store must be usable before
// the Bible lands.
func TestNewLoadingState(t *testing.T) {
	state := NewLoadingState()
	if state.loadPhase != loadPending {
		t.Fatalf("loadPhase = %v, want loadPending", state.loadPhase)
	}
	if state.Annotations == nil {
		t.Fatal("a loading state must carry a ready annotation store")
	}
}

// TestChapterNavigationBounds drives the REAL navigation code (moveChapter) at
// both ends of a book. (The old TestChapterNavigation reimplemented the
// clamping inline in the test body, so it could never fail.)
func TestChapterNavigationBounds(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	state := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 1}

	if moveChapter(state, -1) {
		t.Fatal("moveChapter must refuse to go below the first chapter")
	}
	if state.CurrentChapter != 1 {
		t.Fatalf("failed backward move must not change the chapter, now %d", state.CurrentChapter)
	}

	nums := bd.GetChapterNumbersForBook("John")
	last := nums[len(nums)-1]
	state.CurrentChapter = last
	if moveChapter(state, 1) {
		t.Fatal("moveChapter must refuse to go past the last chapter")
	}
	if state.CurrentChapter != last {
		t.Fatalf("failed forward move must not change the chapter, now %d", state.CurrentChapter)
	}
}
