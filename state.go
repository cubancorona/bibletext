package bibletext

import (
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
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

	// restore is a pending one-shot scroll target set by applyRestoredState when
	// reopening into the last-read chapter; the native reading overlay consumes it
	// on first layout (see armPendingRestore / reading_state.go). nil in the
	// common case (fresh navigation pins to the chapter top).
	restore *restoreAnchor

	// loadPhase drives the startup loading screen. The Bible loads on a
	// background goroutine (so the window appears instantly and the iOS launch
	// watchdog can't SIGKILL us); while loadPhase != loadReady, CreateMainUI
	// renders only a spinner (loadPending) or an error+retry view (loadFailed)
	// and the native reading overlay is NOT attached. See StartBackgroundLoad.
	loadPhase loadPhase
	loadErr   error

	// appliedTheme tracks the theme object last handed to app.Settings().SetTheme
	// so CreateMainUI re-applies it only when it actually changes — re-applying on
	// every rebuild forces a full canvas theme-walk + relayout (a real cost on a
	// phone, on every tab tap / navigation).
	appliedTheme fyne.Theme

	// loadingBar is the startup spinner (buildLoadingView). It animates every frame,
	// which pins the canvas dirty and forces full-tree repaints; once the Bible has
	// loaded and the reading view replaces it, the orphaned animation would keep
	// running (and the canvas repainting) until renderer-cache expiry. stopLoadingBar
	// halts it the moment we leave the loading phase.
	loadingBar *widget.ProgressBarInfinite
}

// stopLoadingBar halts the startup spinner's animation (safe to call repeatedly /
// when absent) so it stops pinning the canvas dirty once loading is done.
func (s *AppState) stopLoadingBar() {
	if s.loadingBar != nil {
		s.loadingBar.Stop()
		s.loadingBar = nil
	}
}

// loadPhase is the startup state machine for the background Bible load.
type loadPhase int

const (
	loadReady   loadPhase = iota // data is in; render the normal UI (the zero value, so a bare AppState — tests, helpers — is "ready" and renders the real UI)
	loadPending                  // loading; show the spinner
	loadFailed                   // first-run fetch failed (offline); show retry
)

// ChapterVisit is one entry in the reading history. The scroll anchor (top
// verse + within-verse delta, with a whole-chapter Frac fallback) records where
// the reader was when they left this chapter, so tapping it in the history bar
// returns them there instead of to the top. A zero anchor means top-of-chapter.
// The anchor fields are omitempty so plain (top-of-chapter) entries and pre-anchor
// saved blobs stay compact and backward-compatible.
type ChapterVisit struct {
	Book    string
	Chapter int
	Verse   int     `json:"v,omitempty"`
	Delta   float64 `json:"d,omitempty"`
	Frac    float64 `json:"f,omitempty"`
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
	// Plain navigation (arrows, picker, reference, search-jump) lands at the top of
	// the new chapter, so drop any pending restore target. navigateToVisit re-arms
	// one *after* calling us when the reader taps a history entry. (The launch
	// restore is set directly on AppState and never routes through here.)
	state.restore = nil
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
	// Every book/chapter navigation funnels through here, so this is the single
	// place to persist the current location + history (no-op without a Fyne app).
	persistReadingPosition(state)
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
// chapters, de-duplicated and in most-recently-read-first order (visits arrive
// newest-first, so append order already is). Books stay in most-recent-first
// order too. e.g. visits John 5, Genesis 1, John 1, John 3 -> "John 5,1,3" then
// "Genesis 1".
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
	return groups
}

func clearHistory(state *AppState) {
	if len(state.RecentChapters) > 1 {
		state.RecentChapters = state.RecentChapters[:1]
	}
	persistReadingPosition(state)
}

func navigateToVisit(state *AppState, visit ChapterVisit) {
	selectBook(state, visit.Book, false)
	state.CurrentChapter = visit.Chapter
	addRecentChapter(state, visit.Book, visit.Chapter) // clears state.restore
	// Return the reader to where they left off in this chapter, not the top, when
	// the visit carries a scroll anchor. Gated to this exact chapter; the native
	// overlay applies it on the next push (armPendingRestore) and the first user
	// scroll drops it (bibleTextReadingScrolled).
	if visit.Verse > 0 || visit.Frac > 0 {
		state.restore = &restoreAnchor{
			Book:    visit.Book,
			Chapter: visit.Chapter,
			Verse:   visit.Verse,
			Delta:   visit.Delta,
			Frac:    visit.Frac,
		}
	}
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

// searchDebounceDelay is how long a search field waits for typing to settle
// before running the (synchronous, whole-corpus) search. Short enough to feel
// live, long enough that a fast typist doesn't queue a scan + results rebuild on
// every keystroke.
const searchDebounceDelay = 150 * time.Millisecond

// newSearchDebouncer returns an OnChanged handler that defers the search until
// typing pauses, plus a stop() to cancel a pending run. The trailing timer fires
// on its own goroutine, so the search (which mutates state + repaints widgets) is
// marshaled back to the UI goroutine with fyne.Do. Both returned closures are
// only ever called from the UI goroutine (Entry callbacks), so the timer pointer
// needs no lock. Call stop() from OnSubmitted, which searches immediately.
func newSearchDebouncer(state *AppState) (onChanged func(string), stop func()) {
	var timer *time.Timer
	stop = func() {
		if timer != nil {
			timer.Stop()
			timer = nil
		}
	}
	onChanged = func(s string) {
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(searchDebounceDelay, func() {
			fyne.Do(func() { searchResultsOnly(state, s) })
		})
	}
	return onChanged, stop
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
