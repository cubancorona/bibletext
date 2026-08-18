package bibletext

import (
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
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

	// AI Find (passage search) results, kept in state (like keyword results) so they survive a
	// tab switch and so "back to results" can re-show them. aiSearchActive marks the
	// current results context as AI vs keyword (drives buildSearchResultsView and the
	// back-to-results label).
	aiSearchActive  bool
	aiSearchQuery   string
	aiSearchResults []Verse
	// Desktop AI-search progress: results replace the reading pane, so the in-progress
	// and error states are driven from state (mobile drives them in its own results host).
	aiSearchLoading   bool
	aiSearchErr       error
	retryAISearch     func() // re-runs the last AI query (the error view's "Try again")
	cancelAISearch    func() // abandons an in-flight AI query (the searching view's "Cancel")
	aiSearchCancelled bool   // the reader abandoned the last Find — NOT a zero-result answer
	cancelAIAction    func() // abandons an in-flight study request (Explain / Analyze …)
	// searchScrollY remembers the results list's scroll offset so returning to the
	// Search tab lands where you left off. Reset to 0 when a new search runs.
	searchScrollY float32

	// askSession is the ONE Find-supersession guard for the whole app (review
	// finding): it must survive window rebuilds. When it lived as a local of
	// each buildSidebar / buildMobileSearchTab, a rebuild mid-flight (rotation,
	// theme variant change, version switch) minted a fresh zeroed session while
	// the old completion closure still held — and passed — the old one, letting
	// a stale response repaint a newer query's results.
	askSession aiSearchSession

	// mark is the highlight: WHICH verses are lit, in WHAT numbering, and — the
	// part that was missing — WHY. See Mark.
	mark Mark

	// The note attached to that highlight, when the reader arrived on a shared
	// link carrying one. Minimized means the reader collapsed it: the note is
	// kept but neither it nor its highlight is shown until they bring it back.
	ActiveNote    string
	NoteMinimized bool
	// NoteVerseLo is the verse the note is attached to. Kept separately from the
	// highlight because minimizing CLEARS the highlight — without this the note
	// would lose its anchor and its marker would jump to the top of the chapter.
	NoteVerseLo int
	// NoteID is the live note's identity in the scrapbook store
	// (StoredNote.ID) — the ONLY handle Hide, Show and Delete address. It is
	// handed to the mirror by the derive and carried whole; no verb ever
	// rebuilds a key from the version, book or chapter the reader happens to
	// be standing on. Rebuilding the address from the reader's position is
	// what deleted the wrong note (X1), made Hide and Show address different
	// objects (X5), and left a cross-chapter note unreachable by any verb
	// (X13). Zero means "no live note", or a note the store could not keep.
	NoteID uint64

	// NoteNotice is the sentence shown in the note's place when a link's
	// payload could NOT be rendered — a newer note format, or damage
	// (noteOutcomeMessage). Session-only and never stored: it is the app
	// reporting on a payload, not a message from a person, so it is attributed
	// to nobody, carries no action and no link, and the next navigation clears
	// it (addRecentChapter). See docs/NOTE_WIRE_FORMAT.md rule 5.
	NoteNotice string

	// noteFocus is which note is expanded THIS SESSION (notes_plan.go):
	// unset (default rule) / none (the reader closed the open note) / a
	// NoteID (the reader opened this one). Session-only, never stored — the
	// at-most-one-expanded cap is a view rule with zero store residue, and
	// suppression by a foreign mark is derived, so neither ever writes.
	noteFocus noteFocus

	RecentChapters []ChapterVisit

	// IsFullScreen is the mobile "distraction-free reading" toggle. When true,
	// CreateMainUI on iOS/Android renders only the reading area and a tiny exit
	// button — no app header, no chapter toolbar, no bottom tabs.
	IsFullScreen bool

	// CurrentTab is the selected mobile bottom-bar tab: 0 Read, 1 Books,
	// 2 Search. The mobile UI rebuilds the window on tab change (reliable
	// repaint) rather than swapping a content host in place.
	CurrentTab int

	// Regular (iPad) layout sidebar visibility. sidebarCollapsed hides the
	// navigation sidebar so the reading pane gets the full width; the header's
	// sidebar-toggle button (iconSidebarLeft) flips it. By DEFAULT it follows
	// orientation — shown in landscape, collapsed in portrait — re-applied
	// whenever the orientation changes (sidebarInit / sidebarLandscape track
	// that), while an explicit toggle sticks until the next rotation. Only
	// meaningful in the regular layout; the compact and desktop layouts ignore it.
	sidebarCollapsed bool
	sidebarInit      bool // has the orientation default been applied at least once?
	sidebarLandscape bool // the orientation the current default/choice was set for

	// aiSearchMode is the Search tab's mode: false = keyword search, true = the
	// natural-language "Find" passage search. Kept on state so the chosen mode
	// survives the window rebuilds that tab switches trigger.
	aiSearchMode bool

	// NotesMode is the Search tab's third mode: browsing the notes people have
	// shared. Mutually exclusive with aiSearchMode — searchModeOf is the one
	// place that resolves the pair, so no caller has to remember the rule.
	NotesMode bool

	// forceReposition asks the next render to place the view even when nothing
	// about the chapter changed.
	//
	// The reading panes skip their whole push — HTML rebuild AND the scroll
	// cadence that rides on it — when the render fingerprint is unchanged. That
	// is right for a repaint and WRONG for a navigation: tapping a note (or a
	// search result) for the passage you are already reading produces an
	// identical fingerprint, so the view never moved and the tap looked broken.
	// The fingerprint's job is to avoid re-rendering, not to suppress
	// re-positioning. Set by the explicit arrivals — a shared link, a note, a
	// search result — and cleared by the render that honours it.
	forceReposition bool

	// NotesQuery filters the notes browser. It is deliberately NOT the keyword
	// search query: switching Search → Notes with a scripture term still in the
	// box would greet the reader with "no notes match" for a search they never
	// made of their notes.
	NotesQuery string

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
	// surfaceSearch returns to the real Search tab (mobile only; nil on desktop,
	// where search results live in the reading pane). Used by "back to results".
	surfaceSearch func()
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

	// pendingLink holds a shared link (a tapped bibletext.co.uk verse URL) that
	// arrived before the Bible finished loading — the common case, since the OS
	// delivers the link within milliseconds of launch. StartBackgroundLoad
	// consumes it once the data is in place. See share_link_open.go.
	pendingLink *ShareTarget
	// pendingLinkRaw is the URL that target came from. The browser handoff needs
	// the ORIGINAL link, not a reconstruction: only the original is guaranteed to
	// be byte-identical to what the sender wrote.
	pendingLinkRaw string
	// pendingLinkVersion is the translation a parked link is WAITING for. A
	// shared link opens in the translation it was written against, which can
	// mean a download; the target is parked here until applyLoadedVersion lands
	// that translation. Recording WHICH one guards the park: if some other
	// version arrives first (the reader switched by hand mid-download, or the
	// load fell back to a cached epoch of something else), the stale target is
	// dropped rather than yanking the reader to a passage they no longer asked
	// for.
	pendingLinkVersion string
	// preferredVersion is the translation the READER chose, when the one on
	// screen is a forced fallback (a licensed translation that could not be
	// revalidated — offline, or the shared key's quota spent). It exists because
	// the chosen translation is stored ONLY in the reading-state blob's Version
	// field, which is rewritten from CurrentVersion on the first navigation and
	// on every background/stop flush. Without it, one unlucky launch silently
	// overwrites the reader's choice with the default and the licensed
	// translation is forgotten for good rather than returning when they are next
	// online. Cleared by any successful version load, so an explicit switch
	// always wins.
	preferredVersion string
	// notesScroll is the notes browser's scroll position, kept while the reader
	// stays in Notes mode so a rebuild (opening a note and coming back, a theme
	// flip, a sort change) returns them to where they were reading rather than
	// to the top of a long list. Cleared when Notes mode is left — coming back
	// to the notes list fresh should start at the top.
	notesScroll float32
	// notesScrollRead reads the LIVE list's scroll offset. The windowed list
	// (widget.List) does not expose a scroll callback the way a raw VScroll
	// does, so instead of saving continuously the browser leaves a way to ask
	// the current view where it is, and the two moments that replace it — the
	// next buildNotesBrowseView, and openNote leaving for the passage — harvest
	// the offset into notesScroll first. Cleared with notesScroll.
	notesScrollRead func() float32

	// loadPhase drives the startup loading screen. The Bible loads on a
	// background goroutine (so the window appears instantly and the iOS launch
	// watchdog can't SIGKILL us); while loadPhase != loadReady, CreateMainUI
	// renders only a spinner (loadPending) or an error+retry view (loadFailed)
	// and the native reading overlay is NOT attached. See StartBackgroundLoad.
	loadPhase loadPhase
	loadErr   error

	// fullPending is true when the app opened on the embedded Gospels seed (no cache
	// yet) and the complete Bible is still downloading in the background; it flips to
	// false once triggerFullDownload swaps the full text in. Drives the "downloading the
	// full Bible" banner on the book lists (incompleteBibleBanner).
	fullPending bool

	// seedOnly is true only when the displayed text really IS the 4-book
	// embedded Gospels seed. A stale-epoch boot also sets fullPending but
	// serves the reader's COMPLETE previous-epoch canon — the "showing the
	// Gospels for now" banner must never claim otherwise there.
	seedOnly bool

	// fullRetryDelay is the current auto-retry backoff for triggerFullDownload.
	// It doubles on each consecutive failure (capped), so an offline reader who
	// already holds a complete previous-epoch Bible does not burn radio and
	// metered data all session upgrading text they can already read. Reset when
	// a fetch succeeds. UI-goroutine only.
	fullRetryDelay time.Duration

	// fullDownloading guards triggerFullDownload to ONE in-flight fetch: the foreground
	// re-trigger and the post-failure auto-retry all funnel through it, so a flaky
	// connection can never stack overlapping full-Bible downloads. UI-goroutine only.
	fullDownloading bool

	// fullRebuildDeferred: a background data swap (applyFullDownload) landed
	// while a sheet owned the canvas, so its window rebuild — which drains
	// every overlay — is parked rather than yanking the sheet out from under
	// the reader. Set only by applyFullDownload; cleared only by rebuildWindow
	// (any full rebuild satisfies it); consumed by consumeDeferredFullRebuild
	// from the overlay-restore closures and refresh(). UI-goroutine only.
	fullRebuildDeferred bool

	// versionLoading guards switchVersionInteractive to ONE in-flight interactive
	// translation load: the loading modal used to be the interaction block, but
	// rebuildWindow (theme-variant flip, tablet rotation) can evict it
	// mid-download, after which the picker could start a second, racing fetch.
	// UI-goroutine only.
	versionLoading bool

	// stopping is set when the app is tearing down (window close / lifecycle stop)
	// so a late background result (e.g. a version download that finishes during
	// shutdown) can drop itself instead of mutating state off the main thread. On
	// desktop, fyne.Do runs its callback INLINE on the caller's goroutine once the
	// main loop has drained, so an unguarded apply would write state/Preferences
	// off-main during exit; this flag lets that callback bail. Read/written across
	// goroutines, hence atomic. See switchVersionInteractive + InstallReadingStateFlush.
	stopping atomic.Bool

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

	// loadingMsg is the loading-screen status line (buildLoadingView). The first-run
	// fetch updates it per book via loadProgressFn so the spinner shows real progress
	// ("Downloading the Bible… John (43 of 66)") instead of a blind indeterminate bar.
	loadingMsg *canvas.Text
}

// stopLoadingBar halts the startup spinner's animation (safe to call repeatedly /
// when absent) so it stops pinning the canvas dirty once loading is done.
func (s *AppState) stopLoadingBar() {
	if s.loadingBar != nil {
		s.loadingBar.Stop()
		s.loadingBar = nil
	}
	s.loadingMsg = nil
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
	// The catch-all consume for a deferred background rebuild
	// (applyFullDownload): the sheet-close consume points (the overlay-restore
	// closures, including the Windows/Linux stand-in installSheetCloseConsume
	// hands out) normally get there first; this is the backstop for any close
	// path that never runs one, upgrading the first navigation after it to the
	// full rebuild — which repaints everything this refresh would have, plus
	// the chrome the swap changed (the downloading banner).
	if consumeDeferredFullRebuild(s) {
		return
	}
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

// advanceToNextChapter moves to the next chapter in reading order, crossing into
// the first chapter of the next book at a book boundary. Returns false at the very
// end of the Bible (nothing after Revelation 22). Used by continuous audio
// playback to roll onto the next chapter when one finishes.
func advanceToNextChapter(state *AppState) bool {
	if state == nil || state.Bible == nil {
		return false
	}
	if moveChapter(state, 1) {
		return true // next chapter within the current book
	}
	// At the last chapter of the book → first chapter of the next book.
	books := state.Bible.Books
	idx := indexOfBook(books, state.CurrentBook)
	if idx < 0 || idx+1 >= len(books) {
		return false // end of the Bible
	}
	// Mirror moveChapter, NOT selectBook: selectBook also resets IsSearching /
	// CanReturnToSearchResults, and this runs from a background audio event —
	// it must never yank away results the reader is browsing (within-book
	// advances already leave them alone; implementation verification).
	state.CurrentBook = books[idx+1]
	state.CurrentChapter = clampChapter(state.Bible, state.CurrentBook, 1)
	clearHighlightedVerse(state)
	addRecentChapter(state, state.CurrentBook, state.CurrentChapter)
	return true
}

// maxRecent caps the reading history. The slim history bar shows all but the
// current chapter, so this bounds how far back you can jump. The bar scrolls
// horizontally (it never wraps or truncates), so the cap is about keeping the
// history a jump-back tool rather than an archive — not about layout.
const maxRecent = 13

func addRecentChapter(state *AppState, book string, chapter int) {
	if chapter < 1 || book == "" {
		return
	}
	// Chapter audio is bound to the displayed text, so stop it when the reader moves
	// off the chapter that's playing. This single hook covers every navigation path
	// (arrows, picker, reference, search-jump, book select, history, VOTD/cross-ref),
	// since they all funnel through here; the fingerprint guard leaves a same-chapter
	// re-add (e.g. a history tap restoring a scroll anchor) playing.
	stopAudioForNav(state)
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
	// A could-not-read-the-note notice belongs to the arrival that raised it,
	// and every navigation funnels through here — so this is the single place
	// it expires. (applyShareTarget sets it AFTER calling us, so the arrival's
	// own pass through here does not eat it.)
	state.NoteNotice = ""
	// Navigation resets focus to the default: every chapter arrival starts
	// from the default rule, and an explicit Show or Hide lasts until the
	// reader moves. (A version SWITCH deliberately does not come through here
	// and keeps focus — hard case 12: the note the reader opened survives the
	// switch.) applyShareTarget sets the arriving note's focus AFTER calling
	// us, so an arrival's own pass does not eat it.
	state.resetNoteFocus()
	// Every book/chapter navigation funnels through here, so this is also the
	// single place to pick up whatever note belongs to where the reader has just
	// landed — that is what makes a note reappear on a later visit rather than
	// living only as long as the link that brought it.
	applyNoteForCurrentChapter(state)
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

// bookChapters groups recently visited chapters of one book. Chapters are the full
// visits (not bare numbers) so a history tap can restore the saved within-chapter
// scroll anchor, not just land at the top.
type bookChapters struct {
	Book     string
	Chapters []ChapterVisit
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
			groups[gi].Chapters = append(groups[gi].Chapters, v)
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

// goToReference parses a free-form citation ("John 3:16", "Ps 23", "1 cor 13:4",
// "jn 3") and navigates to it: highlighting the verse when one is given, otherwise
// opening the chapter at the top. Returns false when the text is not a resolvable
// reference, so a caller (the Goto box) can show a gentle hint instead of jumping.
func goToReference(state *AppState, rawQuery string) bool {
	if state.Bible == nil {
		return false
	}
	book, chapter, verse, hasVerse, ok := state.Bible.parseReferenceQuery(rawQuery)
	if !ok {
		return false
	}
	if hasVerse {
		if match := state.Bible.GetVerse(book, chapter, verse); match != nil {
			goToVerse(state, *match)
			return true
		}
		// Verse out of range — fall back to opening the chapter rather than failing.
	}
	navigateToReference(state, book, chapter)
	return true
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

	// A PASTED SHARE LINK opens like a tapped one. This is how notes reach the
	// desktop at all: macOS/Windows/Linux are never handed a universal link by
	// the OS, but a reader can paste one from a message into the box they
	// already think of as "where I type things" — and it routes through the
	// very same HandleShareLink the OS entry points use, notes gate, offer
	// dialog and all. Mobile gets the same trick for free.
	if _, isLink := ParseShareLink(trimmed); isLink {
		if HandleShareLink(state, trimmed) {
			clearSearchState(state)
			if state.setSearchText != nil {
				state.setSearchText("")
			}
			return
		}
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
// marshaled back to the UI goroutine. Call stop() from OnSubmitted, which
// searches immediately.
func newSearchDebouncer(state *AppState) (onChanged func(string), stop func()) {
	return newTrailingDebouncer(searchDebounceDelay, fyne.Do, func(s string) {
		searchResultsOnly(state, s)
	})
}

// newTrailingDebouncer is the mechanism behind newSearchDebouncer, split out so
// its STALE-FIRE guard is testable: timer.Stop() cannot cancel a timer whose
// callback has already fired and queued its marshalled run — without the
// generation check, a debounced search for an OLDER prefix could land AFTER an
// Enter-submitted search and silently overwrite its results. Every keystroke
// and every stop() bumps the generation; a queued run re-checks it inside the
// marshalled closure (the UI goroutine) and drops itself if superseded.
// onChanged/stop run on the UI goroutine (Entry callbacks) and gen is only read
// back inside the marshalled closure, so the counter needs no lock.
func newTrailingDebouncer(delay time.Duration, marshal func(func()), fire func(string)) (onChanged func(string), stop func()) {
	var timer *time.Timer
	gen := 0
	stop = func() {
		gen++
		if timer != nil {
			timer.Stop()
			timer = nil
		}
	}
	onChanged = func(s string) {
		gen++
		g := gen
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(delay, func() {
			marshal(func() {
				if g != gen {
					return // superseded by a newer keystroke or a submit
				}
				fire(s)
			})
		})
	}
	return onChanged, stop
}

func runSearch(state *AppState, trimmed string) {
	state.ActiveSearchQuery = trimmed
	state.aiSearchActive = false // a keyword search switches the results context
	state.searchScrollY = 0      // new results start at the top
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

// openSearchResult opens a single verse. openSearchResultRange is the same act
// for a passage that spans several — a note's range, say — kept as one function
// so the two arrivals cannot drift apart.
func openSearchResult(state *AppState, verse Verse) {
	openSearchResultRange(state, verse, 0)
}

func openSearchResultRange(state *AppState, verse Verse, endVerse int) {
	// Drop the search field's focus (and the soft keyboard) BEFORE rebuilding. When a
	// result is tapped the Search-tab field is usually still focused; jumping to the
	// Read tab then dismissing the keyboard can leave the field's pixels ghosting over
	// the reading header (Fyne doesn't always fully repaint the strip above the native
	// text overlay). Unfocusing first removes the field cleanly.
	if state.window != nil {
		if c := state.window.Canvas(); c != nil {
			c.Unfocus()
		}
	}

	selectBook(state, verse.BookName, false)
	state.CurrentChapter = verse.Chapter
	addRecentChapter(state, verse.BookName, verse.Chapter)
	// The reader asked to go here. Place the view even if this is the chapter
	// already on screen with the very same verse already lit — otherwise the tap
	// does nothing visible (see AppState.forceReposition).
	state.forceReposition = true
	// And drop any pending "reopen where you left off" target: it now outranks the
	// highlight in the reading panes, so leaving it armed would send an explicit
	// arrival to the saved position instead of the verse just asked for.
	state.restore = nil
	if verse.Verse > 0 {
		state.setMark(hlSearch, VerseSpan{
			VersionID: state.currentVersion().ID,
			Book:      verse.BookName,
			Chapter:   verse.Chapter,
			Lo:        verse.Verse,
			Hi:        endVerse,
		})
	} else {
		state.clearMark()
	}
	state.IsSearching = false
	state.CanReturnToSearchResults = true
	state.refresh()
	if state.surfaceReading != nil {
		state.surfaceReading()
	}

	// Belt-and-braces: force one full repaint a beat after the rebuild so any stale
	// paint of the now-removed search field is cleared from the header strip.
	if state.window != nil {
		time.AfterFunc(160*time.Millisecond, func() {
			fyne.Do(func() {
				if state.window == nil {
					return
				}
				if c := state.window.Canvas(); c != nil && c.Content() != nil {
					c.Content().Refresh()
				}
			})
		})
	}
}

func clearHighlightedVerse(state *AppState) {
	state.clearMark()
}

// clearHighlightAndRederive is the clear verb the highlight's own controls end
// on — the native "Clear highlight" tap (bibleTextHighlightCleared) and the
// back-to-results bar's X and Back. Clearing a foreign mark releases the
// suppression it caused (the plan derives Open again at the next render), but
// only the projection re-raises the note's own hlNote mark — setMark REPLACED
// the note's mark when the foreign one went up — so a bare clear re-opened the
// bubble with its verse unwashed until the next navigation re-derived. The
// navigation paths keep calling clearHighlightedVerse bare: they all funnel
// through addRecentChapter, whose own tail ends on this same projection.
func clearHighlightAndRederive(state *AppState) {
	clearHighlightedVerse(state)
	applyNoteForCurrentChapter(state)
}

func clearSearchState(state *AppState) {
	abandonAISearch(state) // never leave a Find running behind a torn-down view
	state.SearchQuery = ""
	state.ActiveSearchQuery = ""
	state.SearchResults = nil
	state.SearchTruncated = false
	state.IsSearching = false
	state.CanReturnToSearchResults = false
	state.aiSearchActive = false
	state.aiSearchQuery = ""
	state.aiSearchResults = nil
	state.aiSearchErr = nil
	state.aiSearchLoading = false
	state.aiSearchCancelled = false
	clearHighlightedVerse(state)
}

// isVerseHighlighted USED TO LIVE HERE. It is deleted, not shimmed.
//
// It answered a bool, and every one of its five callers was a renderer deciding
// what to paint. That is now chapterTint(state).of(verse) — one answer per
// chapter, a VALUE rather than a bool, in tint.go. Keeping this as a thin
// wrapper would have left a second way to ask the same question, spelled as the
// bool the notes rework has to stop it being; the next person adding a surface
// would have found the easier one.
