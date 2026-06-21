package bibletext

// Reading-position persistence: remember exactly where the reader left off —
// translation, book, chapter, AND the within-chapter scroll position — plus the
// recent-chapters history, so reopening the app lands on the same text and the
// "Recent" bar survives a restart.
//
// Storage is a single JSON blob in fyne.Preferences (per-app, on-device; the
// same store ai_keystore.go / red_letter.go use). It is small (a position + at
// most maxRecent visits), so one key is simpler than a cache file and works
// identically on macOS, iOS, Windows, Linux and Android.
//
// Scroll position is stored as a VERSE ANCHOR (the top-visible verse number plus
// a small within-verse pixel delta) rather than a raw pixel offset: the chapter
// re-wraps when width / orientation / translation change (font size is fixed),
// so a verse anchor re-resolves to the right place where a pixel offset would
// drift. A whole-chapter ScrollFrac is kept as a fallback for when the anchor
// verse can't be resolved (or on platforms without verse geometry).
//
// Saving happens in two places: continuously on navigation (book/chapter/version
// + history, via addRecentChapter / switchVersion — cheap, no native call) and a
// precise scroll flush on app stop/background (the lifecycle hooks in Run /
// cmd/mobile), which is the only moment a pure scroll (no navigation) is caught.
// Restoring happens once, in LoadAndPrepareState, with validation against the
// loaded Bible. The native reading overlay applies the scroll anchor when it
// first lays the chapter out (see armReadingRestore / captureReadingAnchor, which
// are implemented per-platform).

import (
	"encoding/json"
	"sync/atomic"

	"fyne.io/fyne/v2"
)

const prefReadingState = "reading.state"

// readingState is the persisted "where the reader left off" plus history. JSON
// fields are stable on-disk keys; keep them backward-compatible.
type readingState struct {
	Version     string  `json:"version"`
	Book        string  `json:"book"`
	Chapter     int     `json:"chapter"`
	AnchorVerse int     `json:"anchorVerse,omitempty"` // top-visible verse (0 = top/unknown)
	AnchorDelta float64 `json:"anchorDelta,omitempty"` // px scrolled into the anchor verse
	ScrollFrac  float64 `json:"scrollFrac,omitempty"`  // fallback: 0..1 of scrollable height
	// TouchVerse/Delta record where the reader's finger first landed on the LAST
	// scroll — the verse they grabbed to push the text (≈ the line they were
	// reading), with the within-verse pixel offset of the touch. Captured at
	// pan-begin (iOS only; no touch on desktop). On reopen this verse is preferred
	// over the top-visible anchor and softly marked. 0 = none recorded.
	TouchVerse int            `json:"touchVerse,omitempty"`
	TouchDelta float64        `json:"touchDelta,omitempty"`
	Recent     []ChapterVisit `json:"recent,omitempty"` // history, newest first
}

// restoreAnchor is a pending one-shot scroll target carried on AppState from
// launch until the reading overlay first lays out the restored chapter. It is
// gated to a specific book+chapter so it never applies to a different chapter.
type restoreAnchor struct {
	Book    string
	Chapter int
	Verse   int
	Delta   float64
	Frac    float64
	Marker  int // verse to softly mark on reopen ("you left off here"); 0 = none
}

// snapshotReadingState captures the live position + history. anchorVerse/Delta/
// frac come from the platform scroll capture (0 / false ⇒ top of chapter);
// touchVerse/Delta come from the last scroll's initial-touch capture (0 ⇒ none).
func snapshotReadingState(s *AppState, verse int, delta, frac float64, touchVerse int, touchDelta float64) readingState {
	return readingState{
		Version:     s.CurrentVersion,
		Book:        s.CurrentBook,
		Chapter:     s.CurrentChapter,
		AnchorVerse: verse,
		AnchorDelta: delta,
		ScrollFrac:  frac,
		TouchVerse:  touchVerse,
		TouchDelta:  touchDelta,
		Recent:      append([]ChapterVisit(nil), s.RecentChapters...),
	}
}

// writeReadingState / readReadingState are the testable core (a prefStore can be
// an in-memory fake). They no-op / report "absent" when there is no store.
func writeReadingState(p prefStore, rs readingState) {
	if p == nil {
		return
	}
	data, err := json.Marshal(rs)
	if err != nil {
		return
	}
	p.SetString(prefReadingState, string(data))
}

func readReadingState(p prefStore) (readingState, bool) {
	if p == nil {
		return readingState{}, false
	}
	raw := p.String(prefReadingState)
	if raw == "" {
		return readingState{}, false
	}
	var rs readingState
	if err := json.Unmarshal([]byte(raw), &rs); err != nil {
		return readingState{}, false
	}
	if rs.Book == "" || rs.Chapter < 1 {
		return readingState{}, false
	}
	return rs, true
}

// appPrefs returns the running app's preference store, or nil in unit tests
// (no Fyne app), which makes every save/restore call a safe no-op there.
func appPrefs() prefStore {
	if app := fyne.CurrentApp(); app != nil {
		return app.Preferences()
	}
	return nil
}

// persistReadingPosition saves the current book/chapter/version + history with
// the chapter pinned to its top (no scroll anchor). It is the cheap, native-free
// save called from every navigation funnel, so the location and history survive
// even a hard kill. The precise scroll anchor is written separately by
// flushReadingState on app stop/background.
func persistReadingPosition(s *AppState) {
	if s == nil {
		return
	}
	writeReadingState(appPrefs(), snapshotReadingState(s, 0, 0, 0, 0, 0))
}

// flushReadingState captures the exact native scroll position and persists the
// full state. Called from the app-lifecycle stop/background hooks — the one
// moment a scroll with no navigation is otherwise lost.
//
// It can be called more than once around shutdown (a close-intercept while the
// view is alive, then SetOnStopped during teardown). If the live capture fails
// — the native view is already gone, or the platform has no verse geometry — we
// must NOT overwrite a good anchor saved moments earlier with "top": preserve
// the previously-saved anchor when it is for this same chapter.
func flushReadingState(s *AppState) {
	if s == nil {
		return
	}
	p := appPrefs()
	writeReadingState(p, captureSnapshot(s, p))
}

// captureSnapshot reads the live scroll position (captureReadingAnchor /
// captureLastTouch use TextKit and MUST run on the main thread) and builds the
// snapshot, preserving the previously-saved values for this chapter when a live
// read fails (e.g. the native view is already gone, or a lifecycle flush fires
// after a navigation with no fresh scroll).
func captureSnapshot(s *AppState, p prefStore) readingState {
	verse, delta, frac, ok := captureReadingAnchor()
	touchVerse, touchDelta, touchOK := captureLastTouch()

	// Read the previously-saved state once if either live read failed, and only
	// reuse it when it is for the chapter we're currently in.
	var prev readingState
	prevSameChapter := false
	if !ok || !touchOK {
		if pr, had := readReadingState(p); had &&
			pr.Book == s.CurrentBook && pr.Chapter == s.CurrentChapter {
			prev, prevSameChapter = pr, true
		}
	}
	if !ok {
		if prevSameChapter {
			verse, delta, frac = prev.AnchorVerse, prev.AnchorDelta, prev.ScrollFrac
		} else {
			verse, delta, frac = 0, 0, 0
		}
	}
	if !touchOK {
		if prevSameChapter {
			touchVerse, touchDelta = prev.TouchVerse, prev.TouchDelta
		} else {
			touchVerse, touchDelta = 0, 0
		}
	}
	// Mirror the position onto the current history entry so that, once the reader
	// navigates away, tapping this chapter in the history bar returns them here
	// (navigateToVisit reads the visit's anchor). Runs on the main thread.
	updateCurrentVisitAnchor(s, verse, delta, frac)
	return snapshotReadingState(s, verse, delta, frac, touchVerse, touchDelta)
}

// updateCurrentVisitAnchor stamps the live scroll position onto the head of the
// history (the current chapter, by addRecentChapter's invariant). The book/chapter
// guard keeps a late capture from writing the wrong entry after a fast navigation.
func updateCurrentVisitAnchor(s *AppState, verse int, delta, frac float64) {
	if len(s.RecentChapters) == 0 {
		return
	}
	h := &s.RecentChapters[0]
	if h.Book != s.CurrentBook || h.Chapter != s.CurrentChapter {
		return
	}
	h.Verse, h.Delta, h.Frac = verse, delta, frac
}

// readingStateWriting bounds the background prefs writes to one at a time. A fast
// scroller lifts their finger many times; without this, each scroll-end spawned a
// new writeReadingState goroutine, so several JSON-encode+write passes could pile
// up and race on the same preference key. When a write is already in flight we drop
// the newer one — the next scroll-end (or the synchronous lifecycle flush) persists
// the latest position anyway.
var readingStateWriting atomic.Bool

// flushReadingStateAsync captures on the calling (main) thread but writes the
// prefs blob on a goroutine, so a scroll-end never blocks the main thread with a
// synchronous JSON encode + preference write — which made scrolling feel laggy.
// Used from the native scroll-end callback; the lifecycle hooks use the
// synchronous flushReadingState (the write must finish before the app suspends).
func flushReadingStateAsync(s *AppState) {
	if s == nil {
		return
	}
	p := appPrefs()
	snap := captureSnapshot(s, p) // also refreshes the current history entry's anchor
	if !readingStateWriting.CompareAndSwap(false, true) {
		return // a write is already running; drop this one (position is captured above)
	}
	go func() {
		writeReadingState(p, snap)
		readingStateWriting.Store(false)
	}()
}

// applyRestoredState validates a persisted state against the loaded Bible and,
// if usable, sets the version/book/chapter/history on state and stashes a
// pending scroll restore. Returns false when the saved book no longer exists, so
// the caller falls back to the default start position. base is the
// already-loaded default translation's data.
func applyRestoredState(state *AppState, rs readingState, base *BibleData) bool {
	if base == nil || base.GetChaptersForBook(rs.Book) == 0 {
		return false
	}
	bible := base
	book := rs.Book
	chapter := clampChapter(bible, book, rs.Chapter)

	// Best-effort translation restore. Only the default is selectable today, so
	// this branch is dormant until other versions are licensed; it must never
	// fail the whole restore (fall through to the default version on any error).
	if rs.Version != "" && rs.Version != state.CurrentVersion {
		if v, ok := versionByID(rs.Version); ok && v.canSelect() {
			if data, mode, err := loadVersionData(v, base); err == nil && data != nil &&
				data.GetChaptersForBook(book) > 0 {
				bible = data
				state.Bible = data
				state.CurrentVersion = v.ID
				state.currentMode = mode
				if state.loadedVersions == nil {
					state.loadedVersions = map[string]*BibleData{}
				}
				state.loadedVersions[v.ID] = data
				chapter = clampChapter(bible, book, rs.Chapter)
			}
		}
	}

	state.CurrentBook = book
	state.CurrentChapter = chapter
	state.RecentChapters = restoreRecent(rs.Recent, bible, book, chapter)

	if rs.TouchVerse > 0 || rs.AnchorVerse > 0 || rs.ScrollFrac > 0 {
		a := &restoreAnchor{Book: book, Chapter: chapter, Frac: rs.ScrollFrac}
		if rs.TouchVerse > 0 {
			// Prefer the verse the reader's finger last grabbed: bring it to the top
			// (a predictable "resume where I was reading") and softly mark it.
			a.Verse = rs.TouchVerse
			a.Delta = 0
			a.Marker = rs.TouchVerse
		} else {
			// No recorded touch (older state, or a scroll we couldn't map): fall back
			// to reproducing the exact top-visible screen.
			a.Verse = rs.AnchorVerse
			a.Delta = rs.AnchorDelta
		}
		state.restore = a
	}
	return true
}

// clampChapter keeps a chapter valid for the book (all translations share the
// canonical structure), falling back to the book's first chapter.
func clampChapter(bd *BibleData, book string, chapter int) int {
	nums := bd.GetChapterNumbersForBook(book)
	for _, n := range nums {
		if n == chapter {
			return chapter
		}
	}
	if len(nums) > 0 {
		return nums[0]
	}
	return 1
}

// restoreRecent rebuilds the history from saved visits: drop entries whose
// book/chapter no longer exist, de-duplicate, cap at maxRecent, and guarantee
// the current chapter sits at the head (index 0 == current, as addRecentChapter
// maintains).
func restoreRecent(saved []ChapterVisit, bd *BibleData, book string, chapter int) []ChapterVisit {
	out := make([]ChapterVisit, 0, maxRecent)
	out = append(out, ChapterVisit{Book: book, Chapter: chapter})
	for _, v := range saved {
		if v.Book == book && v.Chapter == chapter {
			continue // already at head
		}
		if bd.GetChaptersForBook(v.Book) == 0 {
			continue // book gone from this build/translation
		}
		if !chapterExists(bd, v.Book, v.Chapter) {
			continue
		}
		out = append(out, v)
		if len(out) == maxRecent {
			break
		}
	}
	return out
}

func chapterExists(bd *BibleData, book string, chapter int) bool {
	for _, n := range bd.GetChapterNumbersForBook(book) {
		if n == chapter {
			return true
		}
	}
	return false
}

// armPendingRestore is called by the native reading overlay just before it
// pushes a chapter's text. If a pending restore matches the chapter about to be
// shown, it (re-)arms the native one-shot scroll target; if a different chapter
// is shown the restore is stale and dropped; otherwise it disarms.
//
// It deliberately does NOT consume the pending anchor on a match: the overlay
// rebuilds and re-pushes the chapter a couple of times during launch, and
// consuming on the first push would let the second push disarm it and pin the
// reader back to the top. The native side stops applying it the moment the user
// scrolls (scrollViewDidScroll), and navigating to another chapter drops it here.
func armPendingRestore(state *AppState) {
	if state == nil {
		return
	}
	r := state.restore
	switch {
	case r == nil:
		armReadingRestore(0, 0, 0)
		armReadingMarkerFor(state, 0)
	case r.Book == state.CurrentBook && r.Chapter == state.CurrentChapter:
		armReadingRestore(r.Verse, r.Delta, r.Frac)
		armReadingMarkerFor(state, r.Marker)
	default:
		state.restore = nil
		armReadingRestore(0, 0, 0)
		armReadingMarkerFor(state, 0)
	}
}

// armReadingMarkerFor arms (or clears, when verse<=0) the native "you left off
// here" marker on the given verse, passing the palette's accent colour. The
// marker is an iOS-only render concern; armReadingMarker is a no-op elsewhere.
func armReadingMarkerFor(state *AppState, verse int) {
	if verse <= 0 {
		armReadingMarker(0, 0, 0, 0)
		return
	}
	c := state.pal().Accent
	armReadingMarker(verse, float64(c.R)/255, float64(c.G)/255, float64(c.B)/255)
}
