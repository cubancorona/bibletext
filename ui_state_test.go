package main

import (
	"reflect"
	"testing"
)

func sampleState() *AppState {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	bd.PrepareSearchIndex()
	return &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 1}
}

func TestSelectBookResetsToFirstAvailableChapter(t *testing.T) {
	state := &AppState{
		Bible: &BibleData{
			Books: []string{"Genesis"},
			Verses: map[string]map[int][]Verse{
				"Genesis": {
					3: {{BookName: "Genesis", Chapter: 3, Verse: 1, Text: "a"}},
					7: {{BookName: "Genesis", Chapter: 7, Verse: 1, Text: "b"}},
				},
			},
		},
		CurrentChapter:           99,
		IsSearching:              true,
		CanReturnToSearchResults: true,
		HasHighlightedVerse:      true,
	}

	selectBook(state, "Genesis", true)

	if state.CurrentChapter != 3 {
		t.Fatalf("expected first available chapter 3, got %d", state.CurrentChapter)
	}
	if state.IsSearching {
		t.Fatal("expected IsSearching=false after selecting book")
	}
	if state.CanReturnToSearchResults {
		t.Fatal("expected CanReturnToSearchResults=false after selecting book")
	}
	if state.HasHighlightedVerse {
		t.Fatal("expected highlight to be cleared")
	}
}

func TestMoveChapterHandlesSparseChapterList(t *testing.T) {
	state := &AppState{
		Bible: &BibleData{
			Books: []string{"Genesis"},
			Verses: map[string]map[int][]Verse{
				"Genesis": {
					1: {{BookName: "Genesis", Chapter: 1, Verse: 1, Text: "a"}},
					3: {{BookName: "Genesis", Chapter: 3, Verse: 1, Text: "b"}},
					5: {{BookName: "Genesis", Chapter: 5, Verse: 1, Text: "c"}},
				},
			},
		},
		CurrentBook:    "Genesis",
		CurrentChapter: 3,
	}

	if ok := moveChapter(state, 1); !ok {
		t.Fatal("expected move forward to succeed")
	}
	if state.CurrentChapter != 5 {
		t.Fatalf("expected chapter 5, got %d", state.CurrentChapter)
	}
	if ok := moveChapter(state, 1); ok {
		t.Fatal("expected move forward at end to fail")
	}
	if ok := moveChapter(state, -1); !ok {
		t.Fatal("expected move backward to succeed")
	}
	if state.CurrentChapter != 3 {
		t.Fatalf("expected chapter 3, got %d", state.CurrentChapter)
	}
}

func TestExecuteSearchLifecycle(t *testing.T) {
	state := sampleState()

	executeSearch(state, "god")
	if !state.IsSearching {
		t.Fatal("expected search mode enabled")
	}
	if state.ActiveSearchQuery != "god" {
		t.Fatalf("expected active query to be 'god', got %q", state.ActiveSearchQuery)
	}
	if len(state.SearchResults) == 0 {
		t.Fatal("expected non-empty search results")
	}

	executeSearch(state, "")
	if state.IsSearching {
		t.Fatal("expected search mode disabled for empty query")
	}
	if state.ActiveSearchQuery != "" {
		t.Fatalf("expected cleared active query, got %q", state.ActiveSearchQuery)
	}
	if len(state.SearchResults) != 0 {
		t.Fatalf("expected cleared search results, got %d", len(state.SearchResults))
	}
}

func TestExecuteSearchJumpsToExactVerseReference(t *testing.T) {
	state := sampleState()

	executeSearch(state, "John 3:16")

	if state.IsSearching {
		t.Fatal("an exact verse reference should open the verse, not the results list")
	}
	if state.CurrentBook != "John" || state.CurrentChapter != 3 {
		t.Fatalf("expected navigation to John 3, got %s %d", state.CurrentBook, state.CurrentChapter)
	}
	if !state.HasHighlightedVerse || state.HighlightedVerse != 16 {
		t.Fatalf("expected verse 16 highlighted, got %+v", state)
	}
}

func TestSearchResultsOnlyListsWithoutNavigating(t *testing.T) {
	state := sampleState()

	// Live (as-you-type) search must never navigate away, even for an exact
	// verse reference — that only happens on Enter (executeSearch).
	searchResultsOnly(state, "John 3:16")

	if !state.IsSearching {
		t.Fatal("expected live search to stay in results mode")
	}
	if state.HasHighlightedVerse {
		t.Fatal("live search must not navigate to / highlight a verse")
	}
	if len(state.SearchResults) == 0 {
		t.Fatal("expected the referenced verse to appear as a result")
	}
}

func TestExecuteSearchChapterReferenceShowsResults(t *testing.T) {
	state := sampleState()

	executeSearch(state, "Psalms 23")

	if !state.IsSearching {
		t.Fatal("a chapter reference should list the chapter's verses as results")
	}
	if len(state.SearchResults) == 0 {
		t.Fatal("expected verses for Psalms 23")
	}
	for _, v := range state.SearchResults {
		if v.BookName != "Psalms" || v.Chapter != 23 {
			t.Fatalf("unexpected verse in Psalms 23 results: %s %d:%d", v.BookName, v.Chapter, v.Verse)
		}
	}
}

func TestOpenSearchResultSetsHighlightAndReturnContext(t *testing.T) {
	state := &AppState{
		Bible: &BibleData{
			Books: []string{"John"},
			Verses: map[string]map[int][]Verse{
				"John": {
					3: {{BookName: "John", Chapter: 3, Verse: 16, Text: "For God so loved the world"}},
				},
			},
		},
		IsSearching: true,
	}

	openSearchResult(state, Verse{BookName: "John", Chapter: 3, Verse: 16, Text: "For God so loved the world"})

	if state.CurrentBook != "John" || state.CurrentChapter != 3 {
		t.Fatalf("expected navigation to John 3, got %s %d", state.CurrentBook, state.CurrentChapter)
	}
	if !state.HasHighlightedVerse {
		t.Fatal("expected highlighted verse set")
	}
	if state.IsSearching {
		t.Fatal("expected reading mode after opening search result")
	}
	if !state.CanReturnToSearchResults {
		t.Fatal("expected back-to-results context enabled")
	}
}

func TestAddRecentChapterDedupesAndCaps(t *testing.T) {
	state := &AppState{}
	addRecentChapter(state, "John", 1)
	addRecentChapter(state, "John", 1)
	addRecentChapter(state, "Genesis", 1)
	addRecentChapter(state, "Psalms", 23)
	addRecentChapter(state, "Romans", 8)
	addRecentChapter(state, "Hebrews", 11)
	addRecentChapter(state, "Matthew", 5)
	addRecentChapter(state, "Revelation", 21)
	addRecentChapter(state, "Acts", 2)

	if len(state.RecentChapters) != maxRecent {
		t.Fatalf("expected %d recent chapters, got %d", maxRecent, len(state.RecentChapters))
	}
	first := state.RecentChapters[0]
	if first.Book != "Acts" || first.Chapter != 2 {
		t.Fatalf("expected most recent Acts 2, got %s %d", first.Book, first.Chapter)
	}
	countJohn1 := 0
	for _, v := range state.RecentChapters {
		if v.Book == "John" && v.Chapter == 1 {
			countJohn1++
		}
	}
	if countJohn1 > 1 {
		t.Fatalf("expected no duplicates for John 1, found %d", countJohn1)
	}
}

func TestRecentJumpTargetsExcludesCurrentAndOrders(t *testing.T) {
	state := &AppState{
		RecentChapters: []ChapterVisit{
			{Book: "Acts", Chapter: 2},     // current chapter (index 0)
			{Book: "Hebrews", Chapter: 11}, // most recent jump target
			{Book: "Romans", Chapter: 8},
			{Book: "Psalms", Chapter: 23},
		},
	}

	got := recentJumpTargets(state, 6)
	want := []ChapterVisit{
		{Book: "Hebrews", Chapter: 11},
		{Book: "Romans", Chapter: 8},
		{Book: "Psalms", Chapter: 23},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recent jump targets mismatch: got %v want %v", got, want)
	}

	if limited := recentJumpTargets(state, 2); len(limited) != 2 {
		t.Fatalf("expected jump targets capped at 2, got %d", len(limited))
	}

	single := &AppState{RecentChapters: []ChapterVisit{{Book: "John", Chapter: 1}}}
	if targets := recentJumpTargets(single, 6); targets != nil {
		t.Fatalf("expected no jump targets when only the current chapter exists, got %v", targets)
	}
}

func TestGroupVisitsByBookConsolidates(t *testing.T) {
	visits := []ChapterVisit{
		{Book: "John", Chapter: 5},
		{Book: "Genesis", Chapter: 1},
		{Book: "John", Chapter: 1},
		{Book: "John", Chapter: 3},
		{Book: "John", Chapter: 5}, // duplicate
	}
	got := groupVisitsByBook(visits)
	want := []bookChapters{
		{Book: "John", Chapters: []int{1, 3, 5}}, // sorted + de-duped, book kept first (most recent)
		{Book: "Genesis", Chapters: []int{1}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("groupVisitsByBook mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func TestClearHistoryKeepsCurrentChapter(t *testing.T) {
	state := &AppState{
		RecentChapters: []ChapterVisit{
			{Book: "John", Chapter: 3},
			{Book: "Genesis", Chapter: 1},
			{Book: "Psalms", Chapter: 23},
		},
	}
	clearHistory(state)
	if len(state.RecentChapters) != 1 {
		t.Fatalf("expected only the current chapter to remain, got %d", len(state.RecentChapters))
	}
	if state.RecentChapters[0] != (ChapterVisit{Book: "John", Chapter: 3}) {
		t.Fatalf("expected current chapter preserved, got %+v", state.RecentChapters[0])
	}
}

func TestFilterBooks(t *testing.T) {
	books := []string{"Genesis", "Exodus", "John", "1 John", "2 John"}

	results := filterBooks(books, "john")
	if len(results) != 3 {
		t.Fatalf("expected 3 John matches, got %d (%v)", len(results), results)
	}

	all := filterBooks(books, " ")
	if len(all) != len(books) {
		t.Fatalf("expected all books for blank filter, got %d", len(all))
	}

	none := filterBooks(books, "zzz")
	if len(none) != 0 {
		t.Fatalf("expected no matches, got %d", len(none))
	}
}
