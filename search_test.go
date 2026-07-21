package bibletext

import (
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func segmentText(segs []widget.RichTextSegment) string {
	var b strings.Builder
	for _, s := range segs {
		if ts, ok := s.(*widget.TextSegment); ok {
			b.WriteString(ts.Text)
		}
	}
	return b.String()
}

func boldText(segs []widget.RichTextSegment) string {
	var b strings.Builder
	for _, s := range segs {
		if ts, ok := s.(*widget.TextSegment); ok && ts.Style.TextStyle.Bold {
			b.WriteString(ts.Text)
		}
	}
	return b.String()
}

func TestMatchRangesMergesOverlapsCaseInsensitive(t *testing.T) {
	ranges := matchRanges("Faith and faithful faith", []string{"faith"})
	if len(ranges) != 3 {
		t.Fatalf("expected 3 matches of 'faith', got %d (%v)", len(ranges), ranges)
	}

	// Overlapping terms should merge into one span.
	merged := matchRanges("steadfastness", []string{"stead", "steadfast"})
	if len(merged) != 1 {
		t.Fatalf("expected overlapping terms to merge, got %v", merged)
	}
	if merged[0].start != 0 || merged[0].end != len("steadfast") {
		t.Fatalf("unexpected merged span: %+v", merged[0])
	}

	if got := matchRanges("nothing here", []string{"a"}); got != nil {
		t.Fatalf("single-character terms should be ignored, got %v", got)
	}
}

func TestTermHighlightSegmentsPreserveTextAndEmphasis(t *testing.T) {
	text := "For God so loved the world"
	segs := termHighlightSegments(text, []string{"god", "world"}, colorNameVerseText, colorNameHighlightHi)

	if got := segmentText(segs); got != text {
		t.Fatalf("highlight segments lost text: got %q want %q", got, text)
	}
	if got := boldText(segs); got != "Godworld" {
		t.Fatalf("expected matched terms emphasised, got bold=%q", got)
	}
}

func TestTermHighlightSegmentsNoMatch(t *testing.T) {
	segs := termHighlightSegments("plain text", []string{"zebra"}, colorNameVerseText, colorNameHighlightHi)
	if len(segs) != 1 {
		t.Fatalf("expected a single segment when nothing matches, got %d", len(segs))
	}
	if boldText(segs) != "" {
		t.Fatal("expected no emphasis when nothing matches")
	}
}

func TestResolveBookName(t *testing.T) {
	books := NewBibleData().Books

	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"John", "John", true},
		{"john", "John", true},
		{"jn", "John", true},
		{"ps", "Psalms", true},
		{"psalm", "Psalms", true},
		{"1 cor", "1 Corinthians", true},
		{"philipp", "Philippians", true}, // unique prefix
		{"song of songs", "Song of Solomon", true},
		{"zebra", "", false},
		{"j", "", false}, // ambiguous prefix (John, Jude, James, Joshua...)
	}
	for _, c := range cases {
		got, ok := resolveBookName(books, c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("resolveBookName(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

// collectResultCards gathers every searchResultCard under o in render order.
func collectResultCards(o fyne.CanvasObject) []*searchResultCard {
	var cards []*searchResultCard
	walkTree(o, func(n fyne.CanvasObject) {
		if c, ok := n.(*searchResultCard); ok {
			cards = append(cards, c)
		}
	})
	return cards
}

// findRichText returns the first RichText widget under o (a result card's verse body).
func findRichText(o fyne.CanvasObject) *widget.RichText {
	var rt *widget.RichText
	walkTree(o, func(n fyne.CanvasObject) {
		if r, ok := n.(*widget.RichText); ok && rt == nil {
			rt = r
		}
	})
	return rt
}

func TestBuildSearchResultsViewKeywordStates(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	// A real query renders the count line plus one tappable card per match, in
	// result order, with the matched term emphasised inside the verse text.
	state := sampleState()
	runSearch(state, "god")
	if len(state.SearchResults) == 0 {
		t.Fatal("sample data must produce results for 'god'")
	}
	view := buildSearchResultsView(state)
	wantCount := fmt.Sprintf("%d matches", len(state.SearchResults))
	if !treeHasText(view, wantCount) {
		t.Fatalf("results view missing count line %q; texts: %v", wantCount, treeTexts(view))
	}
	cards := collectResultCards(view)
	if len(cards) != len(state.SearchResults) {
		t.Fatalf("expected %d result cards, got %d", len(state.SearchResults), len(cards))
	}
	for i, card := range cards {
		want := state.SearchResults[i]
		if card.verse.BookName != want.BookName || card.verse.Chapter != want.Chapter || card.verse.Verse != want.Verse {
			t.Fatalf("card %d bound to %s %d:%d, want %s %d:%d", i,
				card.verse.BookName, card.verse.Chapter, card.verse.Verse,
				want.BookName, want.Chapter, want.Verse)
		}
		ref := fmt.Sprintf("%s %d:%d", want.BookName, want.Chapter, want.Verse)
		if !treeHasText(card.content, ref) {
			t.Errorf("card %d missing its reference heading %q", i, ref)
		}
	}

	// The card body carries the full verse text with the match emphasised.
	rt := findRichText(cards[0].content)
	if rt == nil {
		t.Fatal("result card has no RichText verse body")
	}
	if got, want := segmentText(rt.Segments), strings.TrimSpace(state.SearchResults[0].Text); got != want {
		t.Errorf("card text = %q, want the verse text %q", got, want)
	}
	if bold := boldText(rt.Segments); !strings.Contains(strings.ToLower(bold), "god") {
		t.Errorf("expected 'god' emphasised in the first result, got bold=%q", bold)
	}

	// Tapping a card opens that verse: reading mode, highlight, back-to-results.
	tapped := cards[len(cards)-1].verse
	test.Tap(cards[len(cards)-1])
	if state.IsSearching {
		t.Fatal("tapping a result must leave search mode")
	}
	if state.CurrentBook != tapped.BookName || state.CurrentChapter != tapped.Chapter {
		t.Fatalf("tap navigated to %s %d, want %s %d",
			state.CurrentBook, state.CurrentChapter, tapped.BookName, tapped.Chapter)
	}
	if !state.HasHighlightedVerse || state.HighlightedVerse != tapped.Verse {
		t.Fatalf("expected verse %d highlighted after the tap", tapped.Verse)
	}
	if !state.CanReturnToSearchResults {
		t.Fatal("expected the back-to-results context after opening a result")
	}

	// No matches: the honest empty line, and no cards.
	state = sampleState()
	runSearch(state, "zzzznothing")
	view = buildSearchResultsView(state)
	if !treeHasText(view, "No verses matched your search.") {
		t.Errorf("no-match view missing its message; texts: %v", treeTexts(view))
	}
	if got := collectResultCards(view); len(got) != 0 {
		t.Errorf("no-match view must render no cards, got %d", len(got))
	}

	// Truncated results swap the count for the "first N" line.
	state = sampleState()
	runSearch(state, "god")
	state.SearchTruncated = true
	view = buildSearchResultsView(state)
	wantTrunc := fmt.Sprintf("Showing the first %d matches — refine your search to narrow it down.", len(state.SearchResults))
	if !treeHasText(view, wantTrunc) {
		t.Errorf("truncated view missing %q; texts: %v", wantTrunc, treeTexts(view))
	}

	// Under two runes: the calm search prompt, not an empty results frame.
	state = sampleState()
	runSearch(state, "g")
	view = buildSearchResultsView(state)
	if !treeHasText(view, "Search the Bible") {
		t.Errorf("short-query view must show the search prompt; texts: %v", treeTexts(view))
	}
}

func TestBuildSearchResultsViewAIStates(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	t.Setenv("GEMINI_API_KEY", "") // no ambient dev key — the no-key branch must be reachable

	aiState := func(withKey bool) *AppState {
		state := sampleState()
		state.aiKeys = newKeyStoreWith(newFakePrefs())
		if withKey {
			state.aiKeys.setAPIKey(state.aiKeys.activeProvider(), "test-key")
		}
		state.aiSearchActive = true
		return state
	}

	// Loading wins over every other AI state: the calm in-progress line.
	state := aiState(true)
	state.aiSearchLoading = true
	if view := buildSearchResultsView(state); !treeHasText(view, "Searching with AI…") {
		t.Errorf("loading state missing its message; texts: %v", treeTexts(view))
	}

	// No key: the setup invitation with its settings button — never a raw error.
	state = aiState(false)
	view := buildSearchResultsView(state)
	if !treeHasText(view, "Find needs your own AI key") {
		t.Errorf("no-key state missing its title; texts: %v", treeTexts(view))
	}
	if findTreeButton(view, "Set up AI") == nil {
		t.Error("no-key state missing the Set up AI button")
	}

	// Error: the reader-facing message plus Try again wired to retryAISearch.
	state = aiState(true)
	state.aiSearchErr = fmt.Errorf("connection reset")
	retried := false
	state.retryAISearch = func() { retried = true }
	view = buildSearchResultsView(state)
	if want := friendlyAIError(state.aiSearchErr); !treeHasText(view, want) {
		t.Errorf("error state missing %q; texts: %v", want, treeTexts(view))
	}
	retry := findTreeButton(view, "Try again")
	if retry == nil {
		t.Fatal("error state missing the Try again button")
	}
	test.Tap(retry)
	if !retried {
		t.Fatal("Try again must invoke state.retryAISearch")
	}

	// Results: one card per AI-found verse, the count line and the honesty note —
	// and no term emphasis (nothing was keyword-matched).
	state = aiState(true)
	state.aiSearchQuery = "what did God say"
	jn := state.Bible.GetVerse("John", 3, 16)
	ps := state.Bible.GetVerse("Psalms", 23, 1)
	if jn == nil || ps == nil {
		t.Fatal("sample data must include John 3:16 and Psalms 23:1")
	}
	state.aiSearchResults = []Verse{*jn, *ps}
	view = buildSearchResultsView(state)
	if !treeHasText(view, "2 passages found by AI") {
		t.Errorf("AI results missing the count line; texts: %v", treeTexts(view))
	}
	if !treeHasText(view, "AI-suggested passages — read each in context.") {
		t.Error("AI results missing the honesty note")
	}
	cards := collectResultCards(view)
	if len(cards) != 2 {
		t.Fatalf("expected 2 AI result cards, got %d", len(cards))
	}
	for i, card := range cards {
		want := state.aiSearchResults[i]
		if card.verse.BookName != want.BookName || card.verse.Chapter != want.Chapter || card.verse.Verse != want.Verse {
			t.Fatalf("AI card %d bound to %s %d:%d, want %s %d:%d", i,
				card.verse.BookName, card.verse.Chapter, card.verse.Verse,
				want.BookName, want.Chapter, want.Verse)
		}
	}
	if rt := findRichText(cards[0].content); rt == nil || boldText(rt.Segments) != "" {
		t.Error("AI result cards must not fake keyword emphasis")
	}

	// A request that found nothing: the honest empty-results line.
	state = aiState(true)
	state.aiSearchQuery = "anything"
	if view := buildSearchResultsView(state); !treeHasText(view, "AI didn’t find matching passages — try rephrasing.") {
		t.Errorf("empty AI results missing their message; texts: %v", treeTexts(view))
	}

	// No request yet: the Find prompt.
	state = aiState(true)
	if view := buildSearchResultsView(state); !treeHasText(view, "Find passages by meaning") {
		t.Errorf("AI prompt state missing its title; texts: %v", treeTexts(view))
	}
}

func TestSearchToggleUI(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	state := sampleState() // no keystore bound → AI features on by default

	var picked []bool
	toggle := buildSearchModeToggle(state, func(ai bool) { picked = append(picked, ai) })

	searchBtn := findTreeButton(toggle, "Search")
	findBtn := findTreeButton(toggle, "Find")
	if searchBtn == nil || findBtn == nil {
		t.Fatalf("toggle must hold Search and Find buttons; texts: %v", treeTexts(toggle))
	}
	filled := func(b *widget.Button) bool { return b.Importance == widget.HighImportance }

	// Keyword mode is the default: Search is the filled half, and construction
	// alone never fires the callback.
	if !filled(searchBtn) || filled(findBtn) {
		t.Fatalf("expected Search filled initially (search=%v find=%v)", searchBtn.Importance, findBtn.Importance)
	}
	if len(picked) != 0 {
		t.Fatalf("building the toggle must not fire onSelect, got %v", picked)
	}

	// Tapping Find reports AI mode and moves the fill.
	test.Tap(findBtn)
	if len(picked) != 1 || !picked[0] {
		t.Fatalf("tapping Find must report ai=true, got %v", picked)
	}
	if !filled(findBtn) || filled(searchBtn) {
		t.Fatalf("expected Find filled after tap (search=%v find=%v)", searchBtn.Importance, findBtn.Importance)
	}

	// Tapping Search flips back.
	test.Tap(searchBtn)
	if len(picked) != 2 || picked[1] {
		t.Fatalf("tapping Search must report ai=false, got %v", picked)
	}
	if !filled(searchBtn) || filled(findBtn) {
		t.Fatal("expected Search filled again after flipping back")
	}
}

func TestSearchToggleStartsInPersistedFindMode(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	state := sampleState()
	state.aiSearchMode = true // the session left off in Find mode

	toggle := buildSearchModeToggle(state, func(bool) {})
	searchBtn := findTreeButton(toggle, "Search")
	findBtn := findTreeButton(toggle, "Find")
	if searchBtn == nil || findBtn == nil {
		t.Fatal("toggle must hold Search and Find buttons")
	}
	if findBtn.Importance != widget.HighImportance || searchBtn.Importance == widget.HighImportance {
		t.Fatalf("persisted Find mode must start with Find filled (search=%v find=%v)",
			searchBtn.Importance, findBtn.Importance)
	}
}

func TestParseReferenceQueryWithAliases(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()

	book, chapter, verse, hasVerse, ok := bd.parseReferenceQuery("jn 3:16")
	if !ok || book != "John" || chapter != 3 || verse != 16 || !hasVerse {
		t.Fatalf("expected John 3:16, got %s %d:%d (hasVerse=%v ok=%v)", book, chapter, verse, hasVerse, ok)
	}

	book, chapter, _, hasVerse, ok = bd.parseReferenceQuery("Ps 23")
	if !ok || book != "Psalms" || chapter != 23 || hasVerse {
		t.Fatalf("expected Psalms 23 chapter ref, got %s %d (hasVerse=%v ok=%v)", book, chapter, hasVerse, ok)
	}
}
