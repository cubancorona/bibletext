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
// re-wraps when width / orientation / translation / text size change,
// so a verse anchor re-resolves to the right place where a pixel offset would
// drift. A whole-chapter ScrollFrac is kept as a fallback for when the anchor
// verse can't be resolved (or on platforms without verse geometry).
//
// Saving happens in two places: continuously on navigation (book/chapter/version
// + history, via addRecentChapter / switchVersion — cheap, no native call) and a
// precise scroll flush — the only thing that catches a pure scroll with no
// navigation. That flush fires from the per-platform scroll hooks (iOS
// scroll-end via bibleTextReadingScrolled, the macOS/desktop window-close and
// app-stop hooks) AND from the app lifecycle hooks installed by
// InstallReadingStateFlush.
// Restoring happens once, in LoadAndPrepareState, with validation against the
// loaded Bible. The native reading overlay applies the scroll anchor when it
// first lays the chapter out (see armReadingRestore / captureReadingAnchor, which
// are implemented per-platform).

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"fyne.io/fyne/v2"
)

const prefReadingState = "reading.state"

// touchResumeEnabled is the single on/off switch for the "resume at the verse your
// finger last grabbed" feature (iOS): recording the last scroll's initial-touch
// verse, preferring it over the top-visible anchor on reopen, and the accent
// "you left off here" marker. Flip to true to turn the whole feature on.
//
// When false (the default): the touch is never recorded or persisted, reopen uses
// the original top-visible-verse anchor, and no marker is shown — i.e. behaviour is
// exactly as it was before the feature landed. (A var, not a const, only so tests
// can exercise the on-path; flipping this one line is the intended control.)
var touchResumeEnabled = false

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
	// over the top-visible anchor and softly marked. 0 = none recorded. Gated by
	// touchResumeEnabled — left unset (and ignored on reopen) while that is off.
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
	// The reader's CHOSEN translation, which is not always the one on screen: a
	// licensed translation that could not be revalidated falls back to the
	// default canon so the app still opens, and persisting the fallback would
	// erase the choice.
	version := s.CurrentVersion
	if s.preferredVersion != "" {
		version = s.preferredVersion
	}
	return readingState{
		Version:     version,
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
	writeReadingStateSeq(appPrefs(), snapshotReadingState(s, 0, 0, 0, 0, 0), readingStateSeq.Add(1))
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
	// Never flush before the state is actually loaded: during loadPending (and on
	// the loadFailed/Retry screen) CurrentBook is empty, so a background/stop in
	// that window would overwrite the reader's good saved position + history with
	// an empty snapshot — losing their place if iOS jetsams the app before a
	// later good flush.
	if s == nil || s.loadPhase != loadReady || s.CurrentBook == "" {
		return
	}
	p := appPrefs()
	writeReadingStateSeq(p, captureSnapshot(s, p), readingStateSeq.Add(1))
}

// captureAnchorFn indirects the per-platform live scroll capture so tests can
// simulate a failed (or successful) capture deterministically — the real
// functions depend on platform globals other tests may have populated.
var captureAnchorFn = captureReadingAnchor

// captureSnapshot reads the live scroll position (captureReadingAnchor /
// captureLastTouch use TextKit and MUST run on the main thread) and builds the
// snapshot, preserving the previously-saved values for this chapter when a live
// read fails (e.g. the native view is already gone, or a lifecycle flush fires
// after a navigation with no fresh scroll).
func captureSnapshot(s *AppState, p prefStore) readingState {
	verse, delta, frac, ok := captureAnchorFn()

	// Initial-touch capture only runs when the feature is on; otherwise the touch
	// stays unset (0) so it is never persisted. touchOK=true here means "resolved
	// to none", which skips the preserve-previous branch below.
	touchVerse, touchDelta, touchOK := 0, 0.0, true
	if touchResumeEnabled {
		touchVerse, touchDelta, touchOK = captureLastTouch()
	}

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

// Writes are additionally serialized latest-wins: every snapshot takes a sequence
// number on the UI goroutine at capture time, and a writer persists it only if no
// NEWER snapshot has already been written. Without this, an in-flight async write
// (spawned by a scroll moments before close) could land AFTER the synchronous
// close-time flush and resurrect the older position.
var (
	readingStateSeq     atomic.Uint64
	readingStateWriteMu sync.Mutex
	readingStateWritten uint64 // highest seq persisted; guarded by readingStateWriteMu
)

// writeReadingStateSeq persists a snapshot unless a newer one already landed.
func writeReadingStateSeq(p prefStore, snap readingState, seq uint64) {
	readingStateWriteMu.Lock()
	defer readingStateWriteMu.Unlock()
	if seq <= readingStateWritten {
		return // stale: a newer snapshot has already been persisted
	}
	readingStateWritten = seq
	writeReadingState(p, snap)
}

// flushReadingStateAsync captures on the calling (main) thread but writes the
// prefs blob on a goroutine, so a scroll-end never blocks the main thread with a
// synchronous JSON encode + preference write — which made scrolling feel laggy.
// Used from the native scroll-end callback; the lifecycle hooks use the
// synchronous flushReadingState (the write must finish before the app suspends).
func flushReadingStateAsync(s *AppState) {
	if s == nil || s.loadPhase != loadReady || s.CurrentBook == "" {
		return // same jetsam/loading guard as flushReadingState: a stray
		// scroll-end during load must not overwrite the saved position
	}
	p := appPrefs()
	snap := captureSnapshot(s, p) // also refreshes the current history entry's anchor
	seq := readingStateSeq.Add(1) // stamped on the UI goroutine, before the hop
	if !readingStateWriting.CompareAndSwap(false, true) {
		return // a write is already running; drop this one (position is captured above)
	}
	go func() {
		writeReadingStateSeq(p, snap, seq)
		readingStateWriting.Store(false)
	}()
}

// loadVersionForRestore is indirected so tests can prove that a saved
// translation is loaded before its wider canon is validated without making a
// network request.
var loadVersionForRestore = loadVersionData

// applyRestoredState is the bool-only test/helper surface. Production startup
// uses restoreReadingState so a transient saved-translation load error can keep
// the loading screen on Retry rather than falling back and pruning history.
func applyRestoredState(state *AppState, rs readingState, base *BibleData) bool {
	restored, _ := restoreReadingState(state, rs, base)
	return restored
}

// restoreReadingState validates a persisted state against the loaded Bible and,
// if usable, sets the version/book/chapter/history on state and stashes a
// pending scroll restore. It returns (false, nil) when the saved book genuinely
// no longer exists, so the caller may establish a fresh default. A transient
// error loading the saved translation is returned instead: callers must not
// fall back and overwrite durable history in that case. base is the already-
// loaded default translation's data.
func restoreReadingState(state *AppState, rs readingState, base *BibleData) (bool, error) {
	if base == nil {
		return false, nil
	}
	bible := base
	versionID := state.CurrentVersion
	mode := state.currentMode
	book := rs.Book

	// Saved-translation restore. A load ERROR aborts the whole restore (returned
	// to the caller, which keeps the loading screen on Retry): validating durable
	// history against the WRONG canon is exactly how the deuterocanon history of
	// a WEBC reader would be silently pruned by the 66-book base. Only a nil
	// data result (version genuinely unavailable) falls through to the default
	// canon, where the saved book may still legitimately exist.
	if rs.Version != "" && rs.Version != state.CurrentVersion {
		v, known := versionByID(rs.Version)
		switch {
		case known && !v.canSelect():
			// THE CHOICE OUTLIVES THE CONFIGURATION. canSelect() is false
			// whenever the licence configuration cannot be READ, and "cannot
			// be read" is not "the reader gave it up": a credential store that
			// has not unlocked yet answers exactly like one that is empty, and
			// on iOS an app launched before first unlock is a routine morning,
			// not an error. The block below is skipped in that case, so
			// without this line preferredVersion is never set, the fallback's
			// id is what the next navigation persists, and the ONLY record of
			// the reader's chosen translation is gone for good — from a
			// condition that fixed itself seconds later. See D9 in
			// docs/VERSION_STATES.md.
			state.preferredVersion = v.ID
		case known:
			data, loadedMode, err := loadVersionForRestore(v, base)
			if err != nil {
				// OFFLINE EPOCH-BUMP UPGRADE: loadVersionData
				// resolves only the CURRENT epoch's filename, so right after a
				// cacheEpoch bump it misses and tries the network — and with no
				// network the whole launch aborted to Retry even though this
				// version's own previous-epoch cache is a complete, valid canon
				// sitting on disk. Serve that instead of refusing to open; a
				// later online launch re-fetches and upgrades in place.
				old, oldMode, cerr := loadVersionFromCacheOnly(v)
				// If that succeeded it may be the SUPERSEDED epoch — a
				// complete canon from the previous decoder. Record it so the
				// picker can say so; nothing else would (D3).
				if cerr == nil && !versionCacheIsCurrent(v) {
					markVersionStale(state, v.ID)
				}
				if cerr != nil {
					// A LICENSED translation that cannot be revalidated must not
					// abort the launch. Its cache was DELETED before the refetch
					// (versions.go: §11 says stale licensed text is revalidated,
					// not served) and the cache-only path refuses a stale or
					// superseded copy for the same reason — so offline, or with
					// the shared key's quota spent, both routes fail and the
					// reader was left staring at a Retry button that could not
					// succeed, locked out of the WHOLE app because their last
					// translation happened to be the licensed one.
					//
					// Fall through to the default canon instead: the app opens,
					// and the saved book and chapter survive (all our
					// translations share the structure). Nothing stale is
					// served, so the compliance line is untouched — this is only
					// about refusing to open the app.
					//
					// preferredVersion is what makes it a fallback rather than a
					// forgetting, and it is why the licensed text DOES come back
					// on the next launch with a connection — an earlier version
					// of this comment promised that while nothing implemented it. The chosen translation lives ONLY in this
					// blob's Version field, which the next navigation rewrites
					// from CurrentVersion — so without this, one offline launch
					// would overwrite "nkjv" with "web" permanently and the
					// reader would never get their translation back.
					if isLicensedSource(v) {
						data, loadedMode = nil, mode
						state.preferredVersion = v.ID
					} else {
						return false, fmt.Errorf("restore saved %s reading state: %w", v.ID, err)
					}
				} else {
					data, loadedMode = old, oldMode
				}
			}
			if data != nil {
				bible, versionID, mode = data, v.ID, loadedMode
			}
		}
	}

	// Validate only after the saved translation has had a chance to load. This is
	// essential for wider canons such as WEBC: Tobit is absent from WEB but valid
	// in the translation that owns the saved history.
	if bible.GetChaptersForBook(book) == 0 {
		return false, nil
	}
	chapter := clampChapter(bible, book, rs.Chapter)

	state.Bible = bible
	state.CurrentVersion = versionID
	state.currentMode = mode
	if state.loadedVersions == nil {
		state.loadedVersions = map[string]*BibleData{}
	}
	state.loadedVersions[versionID] = bible

	state.CurrentBook = book
	state.CurrentChapter = chapter
	state.RecentChapters = restoreRecent(rs.Recent, bible, book, chapter)

	useTouch := touchResumeEnabled && rs.TouchVerse > 0
	if useTouch || rs.AnchorVerse > 0 || rs.ScrollFrac > 0 {
		a := &restoreAnchor{Book: book, Chapter: chapter, Frac: rs.ScrollFrac}
		if useTouch {
			// Prefer the verse the reader's finger last grabbed: bring it to the top
			// (a predictable "resume where I was reading") and softly mark it.
			a.Verse = rs.TouchVerse
			a.Delta = 0
			a.Marker = rs.TouchVerse
		} else {
			// Feature off, or no recorded touch (older state, or a scroll we couldn't
			// map): reproduce the exact top-visible screen, no marker.
			a.Verse = rs.AnchorVerse
			a.Delta = rs.AnchorDelta
		}
		state.restore = a
	}
	return true, nil
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
			// Already at head — but carry over its saved scroll anchor, or a
			// later history-bar tap back to this chapter would land at the top
			// even though a mid-chapter position was persisted. The anchor is
			// re-stamped only when the reader actually scrolls.
			out[0].Verse, out[0].Delta, out[0].Frac = v.Verse, v.Delta, v.Frac
			continue
		}
		if bd.GetChaptersForBook(v.Book) == 0 {
			// DORMANT, NOT GONE. This used to delete the entry, and the two
			// cases it cannot tell apart are not alike: a book dropped from
			// the build is gone, but a book missing from the translation the
			// reader HAPPENS to be in is still theirs. An evening in the WEBC
			// followed by a switch to the WEB persisted the trail under "web",
			// and this loop then deleted Tobit, Sirach and 1 Maccabees from
			// the only copy — the reader's own history, erased for reading a
			// different translation, with nothing said and no way back. Keep
			// anything the app's widest canon knows; recentJumpTargets does
			// not offer what the loaded canon cannot resolve, so nothing dead
			// is shown, and the entries come alive again when the reader
			// returns to a translation that has them. See D16 in
			// docs/VERSION_STATES.md.
			if !bookKnownToApp(v.Book) {
				continue // genuinely not a book this app has ever had
			}
			out = append(out, v)
			if len(out) == maxRecent {
				break
			}
			continue
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

// bookKnownToApp reports whether a book name belongs to any canon this build
// ships. catholicBooks is the widest (73), and a probe over the registry
// confirms it is a strict superset of the 66-book list, so one lookup answers
// for every translation without loading any of them — which is the point: this
// is asked at launch, when exactly one canon is in hand.
func bookKnownToApp(name string) bool {
	for _, b := range catholicBooks {
		if b == name {
			return true
		}
	}
	return false
}
