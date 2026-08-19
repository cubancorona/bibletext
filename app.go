package bibletext

import (
	"fmt"
	"os"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

// applyTheme hands the app the current bibleTheme, but only when it actually
// changed since the last build. SetTheme forces Fyne to walk the whole widget
// tree re-resolving every colour/size and relayout — re-running it on every
// CreateMainUI (i.e. every tab tap, navigation, or full-screen toggle) is a real
// per-interaction cost on a phone. state.pal() reads colours straight from
// state.theme, and ObserveSystemThemeChanges still rebuilds on a real OS variant
// change, so applying once is sufficient.
func applyTheme(app fyne.App, state *AppState) {
	if state.appliedTheme == state.theme {
		return
	}
	app.Settings().SetTheme(state.theme)
	state.appliedTheme = state.theme
}

// NewLoadingState returns a minimal AppState in the loadPending phase, valid for
// CreateMainUI to render the loading spinner before any Bible data exists. The
// entry points hand this to the window, then call StartBackgroundLoad.
func NewLoadingState() *AppState {
	return &AppState{Annotations: NewAnnotationStore(), loadPhase: loadPending}
}

// loadStateData performs the heavy startup load — read cache (or fetch from the
// API on first run), unmarshal ~6.4 MB of JSON, validate, and build the search
// index over ~31k verses — and returns a fully-initialised AppState ready to
// hand to CreateMainUI. It does NOT touch any Fyne widgets, so it is safe to run
// on a background goroutine (see StartBackgroundLoad); unlike the old
// LoadAndPrepareState it returns an error instead of calling os.Exit, because
// killing the process from a non-main goroutine after the window is up is worse
// than surfacing an in-app retry view.
func loadStateData() (*AppState, error) {
	version, _ := versionByID(defaultVersionID)
	savedReading, hasSavedReading := readReadingState(appPrefs())
	// Try the on-disk cache first (fast). On a cache miss, open INSTANTLY on the
	// embedded Gospels seed and download the complete Bible in the background on
	// a genuinely new install. An existing reader's saved position/history must
	// NEVER be restored against that partial seed: after a decoder/cache-epoch
	// migration we wait for the complete Bible here (already off the UI thread),
	// then validate and restore against the full canon. Otherwise the seed makes
	// every non-Gospel visit look invalid and the fallback path overwrites it.
	bibleData, mode, seeded, err := loadStartupBible(
		version,
		hasSavedReading,
		loadVersionFromCacheOnly,
		loadSeedGospels,
		loadVersionData,
	)
	if err != nil {
		return nil, err
	}

	state := &AppState{
		Bible:          bibleData,
		CurrentVersion: version.ID,
		currentMode:    mode,
		loadedVersions: map[string]*BibleData{version.ID: bibleData},
		Annotations:    NewAnnotationStore(),
		loadPhase:      loadReady,
		// The background refresh runs when boot was served by the Gospels seed
		// OR by a superseded-epoch cache (the migration fallback): either way
		// the displayed text is not the current decoder's output, and
		// triggerFullDownload upgrades it in place. Without the stale-epoch
		// half, an epoch bump would be inert for every existing reader — the
		// fallback would serve the old decode forever.
		fullPending: seeded || !versionCacheIsCurrent(version),
		seedOnly:    seeded,
	}

	// Reopen exactly where the reader left off — translation, book, chapter, the
	// within-chapter scroll position, and the recent-chapters history (see
	// reading_state.go). Falls through to the default start position whenever
	// nothing valid is saved (first run, or the saved book no longer exists).
	if hasSavedReading {
		restored, restoreErr := restoreReadingState(state, savedReading, bibleData)
		if restoreErr != nil {
			return nil, restoreErr
		}
		if restored {
			return state, nil
		}
	}

	// A genuinely-gone saved book falls back to the default start — but the
	// still-valid REST of the history survives (dropping one dead entry must
	// not erase the reader's whole trail; incident-hardening).
	if hasSavedReading {
		state.RecentChapters = restoreRecent(savedReading.Recent, bibleData,
			defaultStartBook(bibleData), clampChapter(bibleData, defaultStartBook(bibleData), 1))
	}
	state.CurrentBook = defaultStartBook(bibleData)
	state.CurrentChapter = 1
	if chapters := bibleData.GetChapterNumbersForBook(state.CurrentBook); len(chapters) > 0 {
		state.CurrentChapter = chapters[0]
	}
	addRecentChapter(state, state.CurrentBook, state.CurrentChapter)
	return state, nil
}

// loadStartupBible chooses the startup data without allowing partial data to
// participate in a durable-state migration. cacheOnly/seed/full are parameters
// so the safety policy can be regression-tested without network access.
func loadStartupBible(
	version BibleVersion,
	hasSavedReading bool,
	cacheOnly func(BibleVersion) (*BibleData, dataMode, error),
	seed func() (*BibleData, error),
	full func(BibleVersion, *BibleData) (*BibleData, dataMode, error),
) (*BibleData, dataMode, bool, error) {
	if data, mode, err := cacheOnly(version); err == nil {
		return data, mode, false, nil
	}

	// A saved reading state means this is an upgrade/recovery, not a true first
	// run. Fetch the full canon before restore; on an offline failure the loading
	// screen may show Retry, but the durable history remains untouched.
	if hasSavedReading {
		data, mode, err := full(version, nil)
		return data, mode, false, err
	}

	if data, err := seed(); err == nil {
		return data, modeReal, true, nil
	}

	// Last resort for a genuinely new install if the embedded seed is unusable.
	data, mode, err := full(version, nil)
	return data, mode, false, err
}

// loadProgressFn, when non-nil, is called during the first-run API fetch
// (fetch_bible_data.go) — once as each book starts (chapter == 0) and once per chapter
// that lands — so the loading screen can show live download progress. It is installed
// for the duration of a single background load and read only from that same goroutine,
// so it needs no synchronisation.
var loadProgressFn func(book string, bookNum, totalBooks, chapter int)

// StartBackgroundLoad kicks off the Bible load on a background goroutine and
// swaps the result into the live state on the UI thread when it's ready. The
// caller shows the window FIRST (with state.loadPhase == loadPending, so
// CreateMainUI renders just a spinner and never attaches the native reading
// overlay); this keeps the main thread free, so the iOS launch watchdog can't
// SIGKILL the app on a slow first-run fetch. On success we copy the loaded
// fields into the same *AppState the UI already closed over (never swap the
// pointer — the showReading/surfaceReading closures captured it) and rebuild;
// on failure we show an in-app retry view.
//
// Exported so both entry points (desktop Run, cmd/mobile) use the same path.
func StartBackgroundLoad(myApp fyne.App, window fyne.Window, state *AppState) {
	go func() {
		// Licensed translations whose licence configuration is gone must not
		// keep their on-device copies (the removal obligation that comes with
		// content held under terms). Cheap no-op for everyone else.
		purgeUnavailableLicensedCaches()
		// Show per-book download progress on the loading spinner during a first-run fetch.
		loadProgressFn = func(book string, bookNum, totalBooks, chapter int) {
			ref := book
			if chapter > 0 {
				ref = fmt.Sprintf("%s %d", book, chapter)
			}
			text := fmt.Sprintf("Downloading… %s  ·  %d of %d books", ref, bookNum, totalBooks)
			fyne.Do(func() {
				if state.loadingMsg != nil {
					state.loadingMsg.Text = text
					state.loadingMsg.Refresh()
				}
			})
		}
		loaded, err := loadStateData()
		loadProgressFn = nil
		fyne.Do(func() {
			// Leaving the loading phase either way — stop the spinner so its
			// animation doesn't keep the canvas repainting after it's off-screen.
			state.stopLoadingBar()
			if err != nil {
				fmt.Fprintln(os.Stderr, "BibleText failed to load:", err)
				state.loadPhase = loadFailed
				state.loadErr = err
				rebuildWindow(state)
				return
			}
			// Copy the loaded data into the live state. Only these fields move
			// over; the wiring (app/window/theme/closures, Annotations) the
			// loading-phase UI already installed stays put.
			state.Bible = loaded.Bible
			state.CurrentVersion = loaded.CurrentVersion
			state.currentMode = loaded.currentMode
			state.loadedVersions = loaded.loadedVersions
			state.CurrentBook = loaded.CurrentBook
			state.CurrentChapter = loaded.CurrentChapter
			state.RecentChapters = loaded.RecentChapters
			state.restore = loaded.restore // carry the one-shot scroll target
			state.fullPending = loaded.fullPending
			state.seedOnly = loaded.seedOnly
			state.loadPhase = loadReady
			// Bring back the note on the chapter we are reopening into. It has
			// to happen HERE rather than in the restore itself: the restore runs
			// on the load goroutine against a throwaway state, and only the
			// fields copied just above survive the trip — a note set there would
			// be dropped on the floor. Before consumePendingLink, so a link
			// tapped at cold start still wins.
			applyNoteOnResume(state)
			// A shared link tapped before the data landed was parked; apply it
			// NOW, before the rebuild below, so that rebuild paints the shared
			// chapter directly — one rebuild, no flash of the wrong chapter,
			// and the saved scroll target cleared before it can fire.
			consumePendingLink(state)
			// Full rebuild (not just refresh) so afterRebuild re-pins/re-asserts
			// the iOS native overlay and armPendingRestore re-arms the saved
			// scroll position on the freshly-built reading view.
			rebuildWindow(state)
			// Dev builds only: open a sheet named by BIBLETEXT_DEV_OPEN so it can
			// be screenshotted in the simulator, which has no tap command. No-op
			// (and not compiled in) for shipping builds — dev_autoopen_off.go.
			devAutoOpenSheet(state)
			devAutoSwitchVersion(state)
			devAutoReadAlong(state)
			devAutoTintBench(state)
			devAutoNotesS8(state)
			if state.fullPending {
				// Opened on the embedded Gospels; download the complete Bible in the
				// background (resilient + self-retrying) and swap it in when it lands.
				triggerFullDownload(state)
			}
		})
	}()
}

// triggerFullDownload fetches the complete current-version Bible in the background after
// the app opened on the embedded Gospels seed OR on a superseded-epoch cache
// (loadStateData sets fullPending for both), then swaps the fresh text into the
// live state on the UI thread. It is resilient + self-healing: a single-flight
// guard (fullDownloading) prevents overlapping fetches, and on failure it auto-retries
// after a short delay — so a stalled, dropped, or backgrounded download can't leave the
// reader permanently stuck on stale text. The app-foreground hook and a manual retry
// also funnel through here. MUST be called on the Fyne UI goroutine.
func triggerFullDownload(state *AppState) {
	if state == nil || state.stopping.Load() || !state.fullPending || state.fullDownloading {
		return
	}
	state.fullDownloading = true
	// fullPending is computed for the DEFAULT version's cache, so the refresh
	// must target THAT version. state.CurrentVersion may be a translation the
	// saved reading state restored, which restoreReadingState already brought
	// to its current epoch — refetching it is pure waste, and it would leave
	// the default version's epoch bump permanently unapplied (the implementation requirement).
	version, _ := versionByID(defaultVersionID)
	go func() {
		full, mode, err := loadVersionData(version, nil) // one helloao request; caches on success
		fyne.Do(func() {
			state.fullDownloading = false
			if state.stopping.Load() {
				return // tearing down — don't mutate state or schedule timers
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "BibleText: full-Bible download failed, will retry:", err)
				// Self-heal with BOUNDED exponential backoff (20s → 40s → … →
				// 10m): a reader who stays offline already holds a complete
				// previous-epoch Bible, so retrying every 20s all session only
				// burns radio and metered data. Foreground re-entry still
				// retries immediately via SetOnEnteredForeground.
				switch {
				case state.fullRetryDelay <= 0:
					state.fullRetryDelay = 20 * time.Second
				case state.fullRetryDelay < 10*time.Minute:
					state.fullRetryDelay *= 2
				}
				time.AfterFunc(state.fullRetryDelay, func() { fyne.Do(func() { triggerFullDownload(state) }) })
				return
			}
			state.fullRetryDelay = 0
			applyFullDownload(state, version, full, mode)
		})
	}()
}

// applyFullDownload is the download's success tail, on the UI goroutine: swap
// the fresh text into the live state, then rebuild the window — UNLESS the
// reader is inside a sheet. A named function rather than the tail of the
// goroutine closure for the same reason as consumeSeedParkedLink: the rule it
// carries has to be callable to be provable.
func applyFullDownload(state *AppState, version BibleVersion, full *BibleData, mode dataMode) {
	if state.loadedVersions != nil {
		state.loadedVersions[version.ID] = full
	}
	// Only swap the live view if the reader is still on the default version (they
	// may have switched translations while it downloaded); the cache is warm either way.
	if state.CurrentVersion != version.ID {
		state.fullPending = false
		return
	}
	state.Bible = full
	state.currentMode = mode
	state.fullPending = false
	// The displayed text is no longer the four-book seed. Nothing else
	// cleared this: the "showing the Gospels" banner keys off fullPending,
	// so seedOnly stayed true for the rest of the session and any later
	// reader of it (applyShareTarget's park below) would have been told
	// the reader is still on the seed.
	state.seedOnly = false
	// A rebuild drains every open overlay — that is its job (the half-dark
	// sheet). But THIS rebuild is a background completion, not something the
	// reader did, and the foreground hook retries the download on every
	// return to the app — so a Settings sheet that was open across a
	// backgrounding vanished the moment the retry landed (verification, and
	// the sim reproduces it: the cache file's mtime matches the restore to
	// the second). The DATA is applied above either way; the window swap
	// waits for the reader to leave the sheet (consumeDeferredFullRebuild),
	// and any other full rebuild satisfies it too (rebuildWindow clears the
	// flag and consumes the parked link itself).
	if state.window != nil && state.window.Canvas().Overlays().Top() != nil {
		if os.Getenv("BT_SHEET_DEBUG") != "" {
			fmt.Fprintln(os.Stderr, "[sheet] applyFullDownload: overlay open, rebuild DEFERRED")
		}
		state.fullRebuildDeferred = true
		return
	}
	if os.Getenv("BT_SHEET_DEBUG") != "" {
		fmt.Fprintln(os.Stderr, "[sheet] applyFullDownload: no overlay, rebuilding now")
	}
	// A link for a book the seed does not carry was parked rather than
	// dropped (applyShareTarget). The whole Bible is now in place, so it
	// can finally be honoured — BEFORE the rebuild, so that rebuild paints
	// the shared passage instead of flashing this chapter first. This is
	// the same ordering StartBackgroundLoad and applyLoadedVersion use.
	consumeSeedParkedLink(state)
	rebuildWindow(state)
}

// deferOrRebuild is the background-completion spelling of rebuildWindow: the
// rebuild happens NOW when nothing would be lost, and waits for the sheet the
// reader is inside otherwise. Used by the paths a reader never triggered from
// the sheet itself — the theme observer today; applyFullDownload keeps its own
// copy of the check because its immediate path must consume the seed-parked
// link first.
func deferOrRebuild(state *AppState) {
	if state != nil && state.window != nil && state.window.Canvas().Overlays().Top() != nil {
		if os.Getenv("BT_SHEET_DEBUG") != "" {
			fmt.Fprintln(os.Stderr, "[sheet] deferOrRebuild: overlay open, rebuild DEFERRED")
		}
		state.fullRebuildDeferred = true
		return
	}
	rebuildWindow(state)
}

// consumeDeferredFullRebuild runs the window rebuild a background data swap
// deferred because the reader was inside a sheet (applyFullDownload). Called
// from the overlay-restore closures — the moment the last sheet leaves the
// canvas; on Windows/Linux that closure is the stand-in
// installSheetCloseConsume assigns, whose whole body is this call — and from
// refresh() as the catch-all for any close path that runs no restore closure.
// Reports whether it rebuilt, so refresh() can skip its own repaint.
//
// The flag itself is CLEARED inside rebuildWindow, not here: any full rebuild
// satisfies a deferred one (a version switch, a theme flip), and clearing at
// the one place every rebuild passes through makes double-consume impossible
// — including this function's own call, which cannot recurse because the flag
// is already down by the time rebuildWindow re-runs the restore closure.
func consumeDeferredFullRebuild(state *AppState) bool {
	if state == nil || !state.fullRebuildDeferred || state.stopping.Load() {
		return false
	}
	if state.window == nil || state.window.Canvas().Overlays().Top() != nil {
		return false // another sheet still owns the canvas; its close consumes
	}
	rebuildWindow(state)
	return true
}

// sheetConsumeClosure answers whether THIS platform needs the stand-in
// overlay-restore closure below. A var-function seam (the nativeNoteSticker
// arrangement): the host for the tests is darwin, where the constant is false,
// and the closure still has to be provable there.
var sheetConsumeClosure = func() bool { return sheetConsumeClosureOnPlatform }

// installSheetCloseConsume gives the platforms WITHOUT a native reading overlay
// (Windows/Linux) the sheet-close consume point the native platforms get from
// their overlay-restore closures. Every popup close path already calls
// state.showReadingOverlay when it is set; on these platforms there is no
// overlay to restore, so the closure's only duty is consuming a rebuild that a
// theme flip or a background data swap deferred while the sheet was open.
// Without it the deferral had no on-close consume here at all: a REAL
// light/dark flip under an open sheet parked fullRebuildDeferred and the WHOLE
// window kept the stale palette after the sheet closed, until the next
// navigation's refresh() happened to catch it — the very half-dark-window
// class deferOrRebuild exists to fix, sitting as the entire page instead of
// one sheet. consumeDeferredFullRebuild itself declines while any overlay
// still owns the canvas, so the guarded and unguarded restore callers are both
// safe to hand this closure.
// sheetConsumeInstallGen counts installer INVOCATIONS (not installs): on the
// native platforms the gate stands the installer down, so the only host-
// observable truth about the WIRING — that desktop CreateMainUI still calls it
// — is that this moved. The refuters proved the wiring was otherwise unpinned:
// deleting the ui_desktop.go call left the whole suite green, and with it the
// Windows/Linux stale-palette-after-sheet-close fix could silently vanish in
// any CreateMainUI refactor. UI-goroutine only, like windowRebuildGen.
var sheetConsumeInstallGen uint64

func installSheetCloseConsume(state *AppState) {
	sheetConsumeInstallGen++
	if state == nil || !sheetConsumeClosure() {
		return
	}
	state.showReadingOverlay = func() { consumeDeferredFullRebuild(state) }
}

// consumeSeedParkedLink honours a link that was parked because the app was still
// on the four-book seed — and ONLY such a link.
//
// pendingLinkVersion is set when switchToLinkVersion parks a target waiting for a
// TRANSLATION to arrive. This download is not that translation: it is the
// reader's own, filling in behind the seed. Two downloads really can be in
// flight at once on a fresh install, and consuming unconditionally let whichever
// finished first apply the target — so a link that asked for the NKJV was opened
// by the WEB landing, in wording the sender never chose, and silently:
// linkVersionUnavailable says nothing for a translation the reader can select, so
// nothing told them.
//
// applyShareTarget's park already refuses the mirror image of this ("stops a seed
// park from stealing a target already waiting on a translation switch") and
// applyLoadedVersion has always checked the id before consuming. This is the same
// rule on the third path, which is the one that was missing it.
//
// A named function rather than an inline check because the download tail runs on a
// goroutine after a network fetch and cannot be reached from a host test; the rule
// has to be callable to be provable.
func consumeSeedParkedLink(state *AppState) {
	if state == nil || state.pendingLinkVersion != "" {
		return
	}
	consumePendingLink(state)
}

// InstallReadingStateFlush captures the precise within-chapter scroll position
// when the app stops or backgrounds (and, on desktop, when the window is closed
// while the native text view is still alive). Navigation already saves the
// location + history continuously via persistReadingPosition; this is the only
// hook that catches a pure scroll with no navigation. Exported so both entry
// points (desktop Run and cmd/mobile) can install it.
func InstallReadingStateFlush(myApp fyne.App, window fyne.Window, state *AppState) {
	lc := myApp.Lifecycle()
	lc.SetOnStopped(func() {
		state.stopping.Store(true)
		// Release the audio session / player on quit. Call the raw native stop, NOT
		// gAudio.stop(): OnStopped can run off the main thread during shutdown, and
		// the native stop is fire-and-forget (dispatch_async) with no UI callback, so
		// it can't hang the way a fyne.Do / dispatch_sync(main) would. (Background —
		// SetOnExitedForeground — deliberately does NOT stop: lock-screen controls and
		// background playback are the whole point.)
		nativeAudioStop()
		flushReadingState(state)
	})
	lc.SetOnExitedForeground(func() { flushReadingState(state) }) // iOS/Android background
	// Retry the full-Bible download whenever the app returns to the foreground — covers a
	// fetch that stalled or dropped while backgrounded. No-op once the full text has landed
	// (triggerFullDownload guards on fullPending + single-flight).
	// foregroundOverlayRecovery (Android-only) re-renders the native reading
	// overlay when Android recreated the activity while we were away — without
	// it the reading pane comes back blank after a swipe-away relaunch (common
	// now that the audio foreground service keeps the process alive).
	lc.SetOnEnteredForeground(func() {
		foregroundOverlayRecovery(state)
		fyne.Do(func() { triggerFullDownload(state) })
	})
	if window != nil && !fyne.CurrentDevice().IsMobile() {
		// Desktop: the window-close button bypasses the lifecycle "stopped" hook
		// until teardown, so capture here while the NSTextView is still alive.
		window.SetCloseIntercept(func() {
			// Mark teardown BEFORE Close() drains the main loop, so an in-flight
			// background apply (e.g. a version download) drops itself rather than
			// running inline off the main thread during exit.
			state.stopping.Store(true)
			nativeAudioStop() // release any audio session before the window goes away
			flushReadingState(state)
			window.Close()
		})
	}
}

// Run is the desktop entry: loads the data, opens a sized window, and starts the
// event loop. Mobile entries (Fyne iOS) use the same data path but configure the
// window differently — see cmd/mobile/main.go.
func Run() {
	// Dev builds only: BIBLETEXT_MIMIC=windows|linux flips the runtime seams so
	// this build follows the Windows/Linux code paths (docs/PLATFORM_MIMIC.md).
	// Must run before CreateMainUI (installSheetCloseConsume reads a seam) and
	// before loadBookFonts (the font candidates are one). No-op — and not
	// compiled in — for shipping builds (dev_mimic_off.go).
	devApplyMimic()
	myApp := app.NewWithID("bibletext")
	// Start in loadPending: the window shows a spinner while the Bible loads on a
	// background goroutine, then swaps to the reader.
	state := NewLoadingState()

	window := myApp.NewWindow("BibleText")
	window.Resize(fyne.NewSize(1280, 860))
	window.SetContent(CreateMainUI(myApp, state, window))
	ObserveSystemThemeChanges(myApp, state)
	InstallReadingStateFlush(myApp, window, state)
	InstallDebugCapture() // dev builds only; empty in release (debug_capture_off.go)
	StartBackgroundLoad(myApp, window, state)
	window.ShowAndRun()
}

// systemThemeOnce guarantees we install the system-appearance listener exactly
// once per process — both cmd/desktop (via Run) and cmd/mobile call
// ObserveSystemThemeChanges, and we don't want stacked subscribers.
var systemThemeOnce sync.Once

// ObserveSystemThemeChanges subscribes to Fyne's settings-change channel so a
// system light/dark switch rebuilds the window. Fyne re-runs Color()
// automatically when the variant changes, but anything generated outside the
// theme callback (like the HTML the iOS UITextView consumes, or the palette
// colors baked into canvas objects at build time) is stale until we rebuild.
//
// The rebuild goes through rebuildWindow, NOT a bare SetContent: SetContent
// replaces only the content tree and never touches Canvas().Overlays(), so an
// OPEN popup (the Settings sheet, a picker) survived a variant flip with its
// captured colors while Fyne re-lit its stock widgets — the observed in practice
// dark-panel/dark-text sheet after an overnight dark→light switch with the
// app suspended. rebuildWindow drains the overlay stack (popups close;
// reopening shows fresh colors) and re-pins the native reading overlay.
//
// applyTheme calls app.Settings().SetTheme() the first time (and on a real theme
// change), which ALSO fires this listener — so we guard against a rebuild loop by
// only acting when the actual light/dark variant has changed since last time.
func ObserveSystemThemeChanges(myApp fyne.App, state *AppState) {
	systemThemeOnce.Do(func() {
		ch := make(chan fyne.Settings, 1)
		myApp.Settings().AddChangeListener(ch)
		// The variant the WINDOW was last built with — compared at rebuild
		// time, on the UI goroutine, not event-to-event on the listener
		// goroutine. The difference is not pedantry: when iOS backgrounds the
		// app it snapshots it in BOTH appearances for the app switcher, so
		// the variant flips away and back and the listener hears two
		// changes. Event-to-event each leg looks like a real change, the
		// queued rebuilds run on restore, and the drain takes the sheet the
		// reader left open with it (verification: "Settings open, background,
		// restore — the sheet is gone"). Against the built variant, a round
		// trip nets to no change and both queued closures no-op; a REAL
		// overnight flip still differs and still rebuilds — with the drain
		// that exists precisely for that flip's stale-palette sheet.
		builtVariant := myApp.Settings().ThemeVariant()
		go func() {
			for range ch {
				fyne.Do(func() {
					if state.stopping.Load() {
						return
					}
					v := myApp.Settings().ThemeVariant()
					if os.Getenv("BT_SHEET_DEBUG") != "" {
						fmt.Fprintf(os.Stderr, "[sheet] theme event: built=%v now=%v\n", builtVariant, v)
					}
					if v == builtVariant {
						return // net no-change: a snapshot round trip, or no variant in it at all
					}
					builtVariant = v
					// The built-variant compare alone cannot save an open sheet:
					// the round trip's two closures interleave with the settings
					// updates, so each leg reads as a real change at execution
					// time (measured — the instrumented sim logs built=0 now=1
					// then built=1 now=0, one rebuild each, sheet drained). No
					// timer or flag can outrace that delivery. What CAN hold is
					// pure state: while a sheet owns the canvas, defer the
					// rebuild to the moment it leaves — the same machinery as
					// applyFullDownload, consumed by the overlay-restore
					// closures and satisfied by any other rebuild. The round
					// trip then nets to one repaint with the settled variant on
					// close; a REAL flip with a sheet open repaints on close
					// too, trading the sheet-yank for a briefly stale palette
					// behind an overlay the reader is actively using.
					deferOrRebuild(state)
				})
			}
		}()
	})
}

// defaultStartBook opens on Matthew when available — the New Testament's
// first page — else the first loaded book. Used for fresh installs and as
// the fallback when a saved book no longer exists in the loaded canon.
func defaultStartBook(bd *BibleData) string {
	if bd.GetChaptersForBook("Matthew") > 0 {
		return "Matthew"
	}
	if len(bd.Books) > 0 {
		return bd.Books[0]
	}
	return "Matthew"
}

func currentUTCTime() time.Time {
	return time.Now().UTC()
}
