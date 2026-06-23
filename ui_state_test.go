package bibletext

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
		{Book: "John", Chapter: 3, Verse: 12, Delta: 4, Frac: 0.5}, // carries a saved scroll anchor
		{Book: "John", Chapter: 5},                                 // duplicate
	}
	got := groupVisitsByBook(visits)
	// Most-recently-read first, de-duped, book kept first — and each chapter keeps its
	// full visit (incl. the scroll anchor) so a history tap can restore the position.
	want := []bookChapters{
		{Book: "John", Chapters: []ChapterVisit{
			{Book: "John", Chapter: 5},
			{Book: "John", Chapter: 1},
			{Book: "John", Chapter: 3, Verse: 12, Delta: 4, Frac: 0.5},
		}},
		{Book: "Genesis", Chapters: []ChapterVisit{{Book: "Genesis", Chapter: 1}}},
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

func TestNavigateToVisitRestoresAnchor(t *testing.T) {
	state := sampleState()
	// The reader had left John 3 scrolled to verse 12; tapping it should re-arm
	// that position rather than landing at the top.
	navigateToVisit(state, ChapterVisit{Book: "John", Chapter: 3, Verse: 12, Delta: 4, Frac: 0.5})

	if state.CurrentBook != "John" || state.CurrentChapter != 3 {
		t.Fatalf("expected navigation to John 3, got %s %d", state.CurrentBook, state.CurrentChapter)
	}
	if state.restore == nil {
		t.Fatal("expected a restore anchor to be armed")
	}
	if state.restore.Book != "John" || state.restore.Chapter != 3 || state.restore.Verse != 12 {
		t.Fatalf("unexpected restore anchor: %+v", state.restore)
	}
}

func TestNavigateToVisitWithoutAnchorGoesToTop(t *testing.T) {
	state := sampleState()
	state.restore = &restoreAnchor{Book: "John", Chapter: 9, Verse: 5} // stale, must be cleared
	navigateToVisit(state, ChapterVisit{Book: "John", Chapter: 1})
	if state.restore != nil {
		t.Fatalf("a visit with no anchor should land at the top (no restore), got %+v", state.restore)
	}
}

func TestAddRecentChapterClearsPendingRestore(t *testing.T) {
	state := &AppState{restore: &restoreAnchor{Book: "John", Chapter: 3, Verse: 5}}
	addRecentChapter(state, "Genesis", 1)
	if state.restore != nil {
		t.Fatalf("plain navigation should clear the restore target, got %+v", state.restore)
	}
}

func TestUpdateCurrentVisitAnchorStampsHead(t *testing.T) {
	state := &AppState{
		CurrentBook:    "John",
		CurrentChapter: 3,
		RecentChapters: []ChapterVisit{{Book: "John", Chapter: 3}, {Book: "Genesis", Chapter: 1}},
	}
	updateCurrentVisitAnchor(state, 12, 4, 0.25)
	if h := state.RecentChapters[0]; h.Verse != 12 || h.Delta != 4 || h.Frac != 0.25 {
		t.Fatalf("expected the current entry's anchor stamped, got %+v", h)
	}
	// A capture that arrives after navigation (head no longer the captured chapter)
	// must not overwrite the wrong entry.
	stale := &AppState{CurrentBook: "Mark", CurrentChapter: 1, RecentChapters: state.RecentChapters}
	updateCurrentVisitAnchor(stale, 99, 0, 0)
	if state.RecentChapters[0].Verse != 12 {
		t.Fatal("anchor must not be written when the head book/chapter mismatch")
	}
}

func TestGoToReferenceResolvesVerseAndChapter(t *testing.T) {
	state := sampleState()

	if !goToReference(state, "John 3:16") {
		t.Fatal("expected John 3:16 to resolve")
	}
	if state.CurrentBook != "John" || state.CurrentChapter != 3 {
		t.Fatalf("expected John 3, got %s %d", state.CurrentBook, state.CurrentChapter)
	}
	if !state.HasHighlightedVerse || state.HighlightedVerse != 16 {
		t.Fatalf("expected verse 16 highlighted, got verse=%d hv=%v", state.HighlightedVerse, state.HasHighlightedVerse)
	}

	if !goToReference(state, "Psalms 23") {
		t.Fatal("expected Psalms 23 to resolve")
	}
	if state.CurrentBook != "Psalms" || state.CurrentChapter != 23 {
		t.Fatalf("expected Psalms 23, got %s %d", state.CurrentBook, state.CurrentChapter)
	}
	if state.HasHighlightedVerse {
		t.Fatal("a chapter-only reference should not highlight a verse")
	}

	if goToReference(state, "this is not a reference") {
		t.Fatal("expected non-reference text to fail to resolve")
	}
}

func TestParseVerseBox(t *testing.T) {
	type span struct{ start, end int }
	ok := map[string]span{
		"16":    {16, 16},
		"  16 ": {16, 16},
		"16-18": {16, 18}, // hyphen range
		"16–18": {16, 18}, // en-dash range
		"16:18": {16, 18}, // colon range
		"1":     {1, 1},
		"16-":   {16, 16}, // missing end → single
		"18-16": {18, 18}, // reversed end → single
	}
	for in, want := range ok {
		if s, e, valid := parseVerseBox(in); !valid || s != want.start || e != want.end {
			t.Errorf("parseVerseBox(%q) = (%d,%d,%v), want (%d,%d,true)", in, s, e, valid, want.start, want.end)
		}
	}
	for _, in := range []string{"", "  ", "abc", "0", "-5", "v16"} {
		if s, _, valid := parseVerseBox(in); valid {
			t.Errorf("parseVerseBox(%q) start=%d ok=true, want ok=false", in, s)
		}
	}
}

func TestGoToChapterWithVerse(t *testing.T) {
	state := sampleState()

	// Empty box -> chapter top, no highlight.
	goToChapterWithVerse(state, "John", 3, "")
	if state.CurrentBook != "John" || state.CurrentChapter != 3 {
		t.Fatalf("expected John 3, got %s %d", state.CurrentBook, state.CurrentChapter)
	}
	if state.HasHighlightedVerse {
		t.Fatal("empty verse box should not highlight a verse")
	}

	// A valid verse -> highlight it.
	if state.Bible.GetVerse("John", 3, 16) != nil {
		goToChapterWithVerse(state, "John", 3, "16")
		if !state.HasHighlightedVerse || state.HighlightedVerse != 16 {
			t.Fatalf("expected verse 16 highlighted, got hv=%v v=%d", state.HasHighlightedVerse, state.HighlightedVerse)
		}
		// A range -> highlight the whole span 16..18.
		goToChapterWithVerse(state, "John", 3, "16-18")
		if state.HighlightedVerse != 16 || state.HighlightedVerseEnd != 18 {
			t.Fatalf("expected range 16-18, got start=%d end=%d", state.HighlightedVerse, state.HighlightedVerseEnd)
		}
		hl := func(n int) bool { return isVerseHighlighted(state, Verse{BookName: "John", Chapter: 3, Verse: n}) }
		if !hl(16) || !hl(17) || !hl(18) {
			t.Fatal("expected verses 16, 17 and 18 all highlighted for range 16-18")
		}
		if hl(15) || hl(19) {
			t.Fatal("verses just outside the range must not be highlighted")
		}
	}

	// An out-of-range verse -> falls back to chapter top (no highlight).
	goToChapterWithVerse(state, "John", 3, "9999")
	if state.HasHighlightedVerse {
		t.Fatal("out-of-range verse should fall back to chapter top, not highlight")
	}
}

func TestAlphabeticalBooksSortsNumberedByName(t *testing.T) {
	in := []string{"John", "1 John", "3 John", "2 John", "Acts", "1 Corinthians", "2 Corinthians", "Jonah"}
	got := alphabeticalBooks(in)
	want := []string{"Acts", "1 Corinthians", "2 Corinthians", "John", "1 John", "2 John", "3 John", "Jonah"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("alphabeticalBooks:\n got %v\nwant %v", got, want)
	}
	// Input must not be mutated.
	if in[0] != "John" {
		t.Fatalf("alphabeticalBooks mutated its input: %v", in)
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
