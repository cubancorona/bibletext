package holybible

import (
	"sort"
	"strings"

	"fyne.io/fyne/v2"
)

// AppState holds everything the UI renders from, plus hooks the widgets install
// so state-mutating helpers can request a redraw without knowing about widgets.
type AppState struct {
	Bible *BibleData

	// Translation the reader is showing. CurrentVersion is a BibleVersion ID
	// (see versions.go); currentMode says whether Bible is real scripture or a
	// testing placeholder; loadedVersions caches already-loaded translations so
	// switching back is instant (the default/base version is always present).
	CurrentVersion string
	currentMode    dataMode
	loadedVersions map[string]*BibleData

	CurrentBook    string
	CurrentChapter int

	BookFilterQuery string

	SearchQuery              string
	ActiveSearchQuery        string
	SearchResults            []Verse
	SearchTruncated          bool
	IsSearching              bool
	CanReturnToSearchResults bool

	HighlightedBook     string
	HighlightedChapter  int
	HighlightedVerse    int
	HasHighlightedVerse bool

	RecentChapters []ChapterVisit

	// IsFullScreen is the mobile "distraction-free reading" toggle. When true,
	// CreateMainUI on iOS/Android renders only the reading area and a tiny exit
	// button — no app header, no chapter toolbar, no bottom tabs.
	IsFullScreen bool

	// CurrentTab is the selected mobile bottom-bar tab: 0 Read, 1 Books,
	// 2 Search. The mobile UI rebuilds the window on tab change (reliable
	// repaint) rather than swapping a content host in place.
	CurrentTab int

	// Annotations is the foundation for note/highlight + research features. It is
	// populated/persisted by future work; the reading view already renders verses
	// as selectable, individually-referenceable blocks.
	Annotations *AnnotationStore

	// Wiring installed by the UI. All are nil during unit tests, so every call
	// site must go through the do* helpers below.
	theme         *bibleTheme
	app           fyne.App
	window        fyne.Window
	showReading   func()       // rebuild only the right-hand reading/results pane
	syncSidebar   func()       // refresh the sidebar book list selection
	focusSearch   func()       // move keyboard focus into the search field
	setSearchText func(string) // set the search field's text (e.g. to clear it)
	// surfaceReading is called when a result is opened from search (or another
	// off-screen view) so the platform can bring the reading pane back into
	// focus. No-op on desktop (the reading pane is always visible alongside);
	// on mobile it switches the bottom tab bar to Read.
	surfaceReading func()
	// hideReadingOverlay / showReadingOverlay let shared code (e.g. the chapter
	// picker popup) temporarily hide the iOS native reading overlay (a
	// UITextView that floats above the Fyne canvas, so it would otherwise cover
	// any popup). Both are nil/no-op on desktop and Android.
	hideReadingOverlay func()
	showReadingOverlay func()

	// aiKeys holds the user's AI provider choice + keys (bring-your-own-key),
	// lazily created via keys(); nil-safe so unit tests work without a Fyne app.
	aiKeys *keyStore
}

// ChapterVisit is one entry in the reading history.
type ChapterVisit struct {
	Book    string
	Chapter int
}

func (s *AppState) pal() palette {
	if s.theme != nil {
		return s.theme.palette()
	}
	return lightPalette
}

// keys returns the AI key store, binding it to the app's Preferences on first
// use. Always returns a usable (possibly inert) store.
func (s *AppState) keys() *keyStore {
	if s.aiKeys == nil {
		s.aiKeys = newKeyStore()
	}
	return s.aiKeys
}

// currentVersion returns the active translation's metadata (falls back to the
// default if CurrentVersion is unset, e.g. in unit tests).
func (s *AppState) currentVersion() BibleVersion {
	if v, ok := versionByID(s.CurrentVersion); ok {
		return v
	}
	v, _ := versionByID(defaultVersionID)
	return v
}

// baseBible is the default (public-domain) translation, used as the structural
// template for testing placeholders. nil only before the first load.
func (s *AppState) baseBible() *BibleData {
	if s.loadedVersions != nil {
		if b := s.loadedVersions[defaultVersionID]; b != nil {
			return b
		}
	}
	return s.Bible
}

func (s *AppState) refresh() {
	if s.showReading != nil {
		s.showReading()
	}
	if s.syncSidebar != nil {
		s.syncSidebar()
	}
}

func (s *AppState) refreshReadingOnly() {
	if s.showReading != nil {
		s.showReading()
	}
}

func filterBooks(books []string, query string) []string {
	trimmed := strings.ToLower(strings.TrimSpace(query))
	if trimmed == "" {
		return append([]string(nil), books...)
	}
	filtered := make([]string, 0, len(books))
	for _, book := range books {
		if strings.Contains(strings.ToLower(book), trimmed) {
			filtered = append(filtered, book)
		}
	}
	return filtered
}

func indexOfBook(books []string, target string) int {
	for i, book := range books {
		if book == target {
			return i
		}
	}
	return -1
}

func selectBook(state *AppState, book string, resetChapter bool) {
	state.CurrentBook = book
	if resetChapter {
		chapters := state.Bible.GetChapterNumbersForBook(book)
		if len(chapters) > 0 {
			state.CurrentChapter = chapters[0]
		} else {
			state.CurrentChapter = 1
		}
		addRecentChapter(state, state.CurrentBook, state.CurrentChapter)
	}
	state.IsSearching = false
	state.CanReturnToSearchResults = false
	clearHighlightedVerse(state)
}

func normalizeCurrentChapter(state *AppState, chapters []int) {
	if len(chapters) == 0 {
		state.CurrentChapter = 1
		return
	}
	for _, chapter := range chapters {
		if chapter == state.CurrentChapter {
			return
		}
	}
	state.CurrentChapter = chapters[0]
}

func moveChapter(state *AppState, step int) bool {
	chapters := state.Bible.GetChapterNumbersForBook(state.CurrentBook)
	if len(chapters) == 0 {
		return false
	}
	normalizeCurrentChapter(state, chapters)
	currentIdx := -1
	for i, chapter := range chapters {
		if chapter == state.CurrentChapter {
			currentIdx = i
			break
		}
	}
	if currentIdx == -1 {
		return false
	}
	nextIdx := currentIdx + step
	if nextIdx < 0 || nextIdx >= len(chapters) {
		return false
	}
	state.CurrentChapter = chapters[nextIdx]
	clearHighlightedVerse(state)
	addRecentChapter(state, state.CurrentBook, state.CurrentChapter)
	return true
}

// maxRecent caps the reading history. The slim history bar shows all but the
// current chapter, so this bounds how far back you can jump.
const maxRecent = 7

func addRecentChapter(state *AppState, book string, chapter int) {
	if chapter < 1 || book == "" {
		return
	}
	updated := make([]ChapterVisit, 0, maxRecent)
	updated = append(updated, ChapterVisit{Book: book, Chapter: chapter})
	for _, v := range state.RecentChapters {
		if v.Book == book && v.Chapter == chapter {
			continue
		}
		updated = append(updated, v)
		if len(updated) == maxRecent {
			break
		}
	}
	state.RecentChapters = updated
}

// recentJumpTargets returns previously visited chapters (newest first),
// excluding the current one, for the history bar.
func recentJumpTargets(state *AppState, limit int) []ChapterVisit {
	if len(state.RecentChapters) <= 1 {
		return nil
	}
	out := make([]ChapterVisit, 0, limit)
	for i := 1; i < len(state.RecentChapters) && len(out) < limit; i++ {
		out = append(out, state.RecentChapters[i])
	}
	return out
}

// bookChapters groups recently visited chapters of one book.
type bookChapters struct {
	Book     string
	Chapters []int
}

// groupVisitsByBook consolidates visits so each book appears once with its
// chapters (sorted, de-duplicated), keeping books in most-recent-first order.
// e.g. visits to John 5, Genesis 1, John 1, John 3 -> "John 1,3,5" then "Genesis 1".
func groupVisitsByBook(visits []ChapterVisit) []bookChapters {
	index := make(map[string]int)
	seen := make(map[string]map[int]bool)
	groups := make([]bookChapters, 0, len(visits))
	for _, v := range visits {
		gi, ok := index[v.Book]
		if !ok {
			gi = len(groups)
			index[v.Book] = gi
			groups = append(groups, bookChapters{Book: v.Book})
			seen[v.Book] = make(map[int]bool)
		}
		if !seen[v.Book][v.Chapter] {
			seen[v.Book][v.Chapter] = true
			groups[gi].Chapters = append(groups[gi].Chapters, v.Chapter)
		}
	}
	for i := range groups {
		sort.Ints(groups[i].Chapters)
	}
	return groups
}

func clearHistory(state *AppState) {
	if len(state.RecentChapters) > 1 {
		state.RecentChapters = state.RecentChapters[:1]
	}
}

func navigateToVisit(state *AppState, visit ChapterVisit) {
	selectBook(state, visit.Book, false)
	state.CurrentChapter = visit.Chapter
	addRecentChapter(state, visit.Book, visit.Chapter)
	state.refresh()
}

// executeSearch runs a full search (used on Enter). An exact single-verse
// reference like "John 3:16" jumps straight to the verse in context.
func executeSearch(state *AppState, rawQuery string) {
	trimmed := strings.TrimSpace(rawQuery)
	state.SearchQuery = trimmed

	if trimmed == "" {
		clearSearchState(state)
		state.refreshReadingOnly()
		return
	}

	if book, chapter, verse, hasVerse, ok := state.Bible.parseReferenceQuery(trimmed); ok && hasVerse {
		if match := state.Bible.GetVerse(book, chapter, verse); match != nil {
			openSearchResult(state, *match)
			return
		}
	}

	runSearch(state, trimmed)
}

// searchResultsOnly powers live, as-you-type search. It only lists matches; it
// never navigates away, so typing a reference doesn't jump around mid-keystroke.
// It runs synchronously on the UI goroutine (no background timer), so it is
// race-free.
func searchResultsOnly(state *AppState, rawQuery string) {
	trimmed := strings.TrimSpace(rawQuery)
	state.SearchQuery = trimmed
	if trimmed == "" {
		clearSearchState(state)
		state.refreshReadingOnly()
		return
	}
	runSearch(state, trimmed)
}

func runSearch(state *AppState, trimmed string) {
	state.ActiveSearchQuery = trimmed
	state.IsSearching = true
	state.CanReturnToSearchResults = false
	clearHighlightedVerse(state)

	if len([]rune(trimmed)) < 2 {
		state.SearchResults = nil
		state.SearchTruncated = false
		state.refreshReadingOnly()
		return
	}

	results, truncated := state.Bible.SearchSmartLimited(trimmed, 120)
	state.SearchResults = results
	state.SearchTruncated = truncated
	state.refreshReadingOnly()
}

func openSearchResult(state *AppState, verse Verse) {
	selectBook(state, verse.BookName, false)
	state.CurrentChapter = verse.Chapter
	addRecentChapter(state, verse.BookName, verse.Chapter)
	state.HighlightedBook = verse.BookName
	state.HighlightedChapter = verse.Chapter
	state.HighlightedVerse = verse.Verse
	state.HasHighlightedVerse = true
	state.IsSearching = false
	state.CanReturnToSearchResults = true
	state.refresh()
	if state.surfaceReading != nil {
		state.surfaceReading()
	}
}

func clearHighlightedVerse(state *AppState) {
	state.HighlightedBook = ""
	state.HighlightedChapter = 0
	state.HighlightedVerse = 0
	state.HasHighlightedVerse = false
}

func clearSearchState(state *AppState) {
	state.SearchQuery = ""
	state.ActiveSearchQuery = ""
	state.SearchResults = nil
	state.SearchTruncated = false
	state.IsSearching = false
	state.CanReturnToSearchResults = false
	clearHighlightedVerse(state)
}

func isVerseHighlighted(state *AppState, verse Verse) bool {
	if !state.HasHighlightedVerse {
		return false
	}
	return state.HighlightedBook == verse.BookName &&
		state.HighlightedChapter == verse.Chapter &&
		state.HighlightedVerse == verse.Verse
}
