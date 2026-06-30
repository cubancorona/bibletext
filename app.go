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
	// Try the on-disk cache first (fast). On a cache miss, open INSTANTLY on the
	// embedded Gospels seed and download the complete Bible in the background
	// (StartBackgroundLoad → triggerFullDownload) rather than blocking on the
	// chapter-by-chapter API fetch — which can take minutes and is at the mercy of
	// bible-api.com rate-limiting. The seed is never cached, so once the background
	// fetch lands it caches the full text for every later launch.
	bibleData, mode, err := loadVersionFromCacheOnly(version)
	seeded := false
	if err != nil {
		if seed, serr := loadSeedGospels(); serr == nil {
			bibleData, mode, seeded = seed, modeReal, true
		} else if full, fmode, ferr := loadVersionData(version, nil); ferr == nil {
			bibleData, mode = full, fmode // last resort, if the embedded seed is somehow unusable
		} else {
			return nil, ferr
		}
	}

	state := &AppState{
		Bible:          bibleData,
		CurrentVersion: version.ID,
		currentMode:    mode,
		loadedVersions: map[string]*BibleData{version.ID: bibleData},
		Annotations:    NewAnnotationStore(),
		loadPhase:      loadReady,
		fullPending:    seeded,
	}

	// Reopen exactly where the reader left off — translation, book, chapter, the
	// within-chapter scroll position, and the recent-chapters history (see
	// reading_state.go). Falls through to the default start position whenever
	// nothing valid is saved (first run, or the saved book no longer exists).
	if rs, ok := readReadingState(appPrefs()); ok && applyRestoredState(state, rs, bibleData) {
		return state, nil
	}

	state.CurrentBook = defaultStartBook(bibleData)
	state.CurrentChapter = 1
	if chapters := bibleData.GetChapterNumbersForBook(state.CurrentBook); len(chapters) > 0 {
		state.CurrentChapter = chapters[0]
	}
	addRecentChapter(state, state.CurrentBook, state.CurrentChapter)
	return state, nil
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
			state.loadPhase = loadReady
			// Full rebuild (not just refresh) so afterRebuild re-pins/re-asserts
			// the iOS native overlay and armPendingRestore re-arms the saved
			// scroll position on the freshly-built reading view.
			rebuildWindow(state)
			if state.fullPending {
				// Opened on the embedded Gospels; download the complete Bible in the
				// background (resilient + self-retrying) and swap it in when it lands.
				triggerFullDownload(state)
			}
		})
	}()
}

// triggerFullDownload fetches the complete default-version Bible in the background after
// the app opened on the embedded Gospels seed (loadStateData), then swaps the full text
// into the live state on the UI thread. It is resilient + self-healing: a single-flight
// guard (fullDownloading) prevents overlapping fetches, and on failure it auto-retries
// after a short delay — so a stalled, dropped, or backgrounded download can't leave the
// reader permanently stuck on the Gospels. The app-foreground hook and a manual retry
// also funnel through here. MUST be called on the Fyne UI goroutine.
func triggerFullDownload(state *AppState) {
	if state == nil || state.stopping.Load() || !state.fullPending || state.fullDownloading {
		return
	}
	state.fullDownloading = true
	go func() {
		version, _ := versionByID(defaultVersionID)
		full, mode, err := loadVersionData(version, nil) // one helloao request; caches on success
		fyne.Do(func() {
			state.fullDownloading = false
			if state.stopping.Load() {
				return // tearing down — don't mutate state or schedule timers
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "BibleText: full-Bible download failed, will retry:", err)
				// Self-heal: try again shortly (also retried on app-foreground). The guard
				// above keeps retries from stacking; fullPending gates it to "still needed".
				time.AfterFunc(20*time.Second, func() { fyne.Do(func() { triggerFullDownload(state) }) })
				return
			}
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
			rebuildWindow(state)
		})
	}()
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
	lc.SetOnEnteredForeground(func() { fyne.Do(func() { triggerFullDownload(state) }) })
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
	myApp := app.NewWithID("bibletext")
	// Start in loadPending: the window shows a spinner while the Bible loads on a
	// background goroutine, then swaps to the reader.
	state := NewLoadingState()

	window := myApp.NewWindow("BibleText")
	window.Resize(fyne.NewSize(1280, 860))
	window.SetContent(CreateMainUI(myApp, state, window))
	ObserveSystemThemeChanges(myApp, state)
	InstallReadingStateFlush(myApp, window, state)
	StartBackgroundLoad(myApp, window, state)
	window.ShowAndRun()
}

// systemThemeOnce guarantees we install the system-appearance listener exactly
// once per process — both cmd/desktop (via Run) and cmd/mobile call
// ObserveSystemThemeChanges, and we don't want stacked subscribers.
var systemThemeOnce sync.Once

// ObserveSystemThemeChanges subscribes to Fyne's settings-change channel so a
// system light/dark switch rebuilds the window content. Fyne re-runs Color()
// automatically when the variant changes, but anything generated outside the
// theme callback (like the HTML the iOS UITextView consumes) is stale until we
// rebuild the tree.
//
// applyTheme calls app.Settings().SetTheme() the first time (and on a real theme
// change), which ALSO fires this listener — so we guard against a rebuild loop by
// only acting when the actual light/dark variant has changed since last time.
func ObserveSystemThemeChanges(myApp fyne.App, state *AppState) {
	systemThemeOnce.Do(func() {
		ch := make(chan fyne.Settings, 1)
		myApp.Settings().AddChangeListener(ch)
		lastVariant := myApp.Settings().ThemeVariant()
		go func() {
			for range ch {
				v := myApp.Settings().ThemeVariant()
				if v == lastVariant {
					continue // theme object changed but not the variant — ignore
				}
				lastVariant = v
				fyne.Do(func() {
					if state.window != nil && state.app != nil {
						state.window.SetContent(CreateMainUI(state.app, state, state.window))
					}
				})
			}
		}()
	})
}

// defaultStartBook opens on John when available, else the first loaded book.
func defaultStartBook(bd *BibleData) string {
	if bd.GetChaptersForBook("John") > 0 {
		return "John"
	}
	if len(bd.Books) > 0 {
		return bd.Books[0]
	}
	return "John"
}

func currentUTCTime() time.Time {
	return time.Now().UTC()
}
