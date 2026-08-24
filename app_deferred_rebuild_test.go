package bibletext

// A background completion must not close the sheet the reader is inside.
//
// Regression: Settings open → background the iOS app → restore → the
// sheet disappeared. Mechanism: the foreground hook retries the full-Bible
// download on every return to the app; when it completes, the old tail called
// rebuildWindow, whose overlay drain exists for a different, legitimate reason
// (the half-dark sheet after a theme flip) — and the sheet the reader was in
// went with it.
//
// The rule these tests hold: applyFullDownload applies the DATA immediately,
// always — but while a sheet owns the canvas the window rebuild is deferred
// (state.fullRebuildDeferred), consumed the moment the last sheet leaves
// (the overlay-restore closures, or refresh() on the platforms without one),
// and satisfied by ANY intervening full rebuild.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// deferredTestState is the TestRebuildWindowHidesOverlayPopups arrangement: a
// real test window whose content rebuildWindow can actually swap.
func deferredTestState(t *testing.T, app fyne.App) *AppState {
	t.Helper()
	state := sampleState()
	state.CurrentVersion = defaultVersionID
	state.loadedVersions = map[string]*BibleData{}
	win := app.NewWindow("deferred")
	t.Cleanup(win.Close)
	state.app = app
	state.window = win
	win.SetContent(CreateMainUI(app, state, win))
	return state
}

func TestBackgroundApplyDefersWhileASheetIsOpen(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	state := deferredTestState(t, app)
	version, _ := versionByID(defaultVersionID)

	pop := widget.NewModalPopUp(widget.NewLabel("settings stand-in"), state.window.Canvas())
	pop.Show()

	full := sampleState().Bible // a distinct decode, as a fresh download is
	state.fullPending = true
	state.seedOnly = true
	genBefore := windowRebuildGen

	applyFullDownload(state, version, full, modeReal)

	// The DATA landed in full…
	if state.Bible != full {
		t.Fatal("the fresh text must be applied immediately — only the window swap waits")
	}
	if state.fullPending || state.seedOnly {
		t.Errorf("fullPending=%v seedOnly=%v — the swap's bookkeeping must not wait",
			state.fullPending, state.seedOnly)
	}
	if state.loadedVersions[version.ID] != full {
		t.Error("the download must be cached whatever the canvas is doing")
	}
	// …and the sheet did not move.
	if windowRebuildGen != genBefore {
		t.Fatal("the completion rebuilt the window over an open sheet")
	}
	if !pop.Visible() {
		t.Fatal("the sheet the reader is inside must survive the background completion")
	}
	if !state.fullRebuildDeferred {
		t.Fatal("the skipped rebuild must be remembered, or the chrome never catches up")
	}

	// The reader closes the sheet; the platform overlay-restore closure calls
	// consumeDeferredFullRebuild (simulated directly here — the closures are
	// cgo, per platform, and each ends in exactly this call).
	pop.Hide()
	if !consumeDeferredFullRebuild(state) {
		t.Fatal("the last sheet left the canvas: the deferred rebuild must run now")
	}
	if windowRebuildGen != genBefore+1 {
		t.Fatalf("want exactly one rebuild, got %d", windowRebuildGen-genBefore)
	}
	if state.fullRebuildDeferred {
		t.Error("consumed: the flag must be down")
	}
	// Consume is once: a second call must not rebuild again.
	if consumeDeferredFullRebuild(state) {
		t.Error("a second consume rebuilt again — the verb must be idempotent")
	}
}

func TestConsumeWaitsForTheLastSheet(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	state := deferredTestState(t, app)

	// Two stacked sheets (Settings with the goto picker over it, say).
	under := widget.NewModalPopUp(widget.NewLabel("under"), state.window.Canvas())
	under.Show()
	over := widget.NewModalPopUp(widget.NewLabel("over"), state.window.Canvas())
	over.Show()
	state.fullRebuildDeferred = true
	genBefore := windowRebuildGen

	over.Hide()
	if consumeDeferredFullRebuild(state) {
		t.Fatal("a sheet still owns the canvas — the rebuild must keep waiting")
	}
	if windowRebuildGen != genBefore || !state.fullRebuildDeferred {
		t.Fatal("waiting must change nothing")
	}
	under.Hide()
	if !consumeDeferredFullRebuild(state) {
		t.Fatal("the LAST sheet left: now the rebuild runs")
	}
}

func TestAnyFullRebuildSatisfiesTheDeferredOne(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	state := deferredTestState(t, app)

	state.fullRebuildDeferred = true
	genBefore := windowRebuildGen

	// Something else rebuilds — a theme flip, a version switch, a layout
	// change. That rebuild already painted everything from live state.
	rebuildWindow(state)

	if state.fullRebuildDeferred {
		t.Fatal("any full rebuild satisfies the deferred one — the flag must come down inside rebuildWindow")
	}
	if consumeDeferredFullRebuild(state) {
		t.Error("nothing left to consume: a second rebuild here would be a duplicate window swap")
	}
	if windowRebuildGen != genBefore+1 {
		t.Fatalf("want exactly one rebuild, got %d", windowRebuildGen-genBefore)
	}
}

// The Windows/Linux sheet-close consume point: installSheetCloseConsume hands
// those platforms a stand-in overlay-restore closure whose whole duty is the
// consume, so a theme flip (the whole palette!) deferred under an open sheet
// repaints the moment the sheet closes — not at whenever the next navigation
// happens to run refresh(). The host is darwin, so the platform seam is
// overridden to prove the closure.
func TestSheetCloseConsumeStandInOnFynePanes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	state := deferredTestState(t, app)

	orig := sheetConsumeClosure
	sheetConsumeClosure = func() bool { return true }
	defer func() { sheetConsumeClosure = orig }()

	state.showReadingOverlay = nil // the Windows/Linux fact: no native overlay
	installSheetCloseConsume(state)
	if state.showReadingOverlay == nil {
		t.Fatal("the stand-in closure must be installed where no native overlay exists")
	}

	// A real light/dark flip lands while the reader is inside a sheet…
	pop := widget.NewModalPopUp(widget.NewLabel("settings stand-in"), state.window.Canvas())
	pop.Show()
	genBefore := windowRebuildGen
	deferOrRebuild(state)
	if windowRebuildGen != genBefore || !state.fullRebuildDeferred {
		t.Fatal("precondition: the flip must defer under the sheet")
	}
	// …a restore that fires while a sheet still owns the canvas declines
	// (the guarded close paths call it exactly so)…
	state.showReadingOverlay()
	if windowRebuildGen != genBefore {
		t.Fatal("a sheet still owns the canvas: the stand-in must decline")
	}
	// …and the sheet's close is the consume.
	pop.Hide()
	state.showReadingOverlay()
	if windowRebuildGen != genBefore+1 {
		t.Fatalf("sheet closed: the deferred palette repaint must run NOW, got %d rebuilds",
			windowRebuildGen-genBefore)
	}
	if state.fullRebuildDeferred {
		t.Error("consumed: the flag must be down")
	}

	// On the native platforms the installer stands aside — their builders own
	// the real closure, and a generic one must never shadow it.
	sheetConsumeClosure = func() bool { return false }
	state.showReadingOverlay = nil
	installSheetCloseConsume(state)
	if state.showReadingOverlay != nil {
		t.Error("native platforms: installSheetCloseConsume must install nothing")
	}
}

// The refresh() catch-all remains beneath the consume points above: any close
// path that runs no restore closure still upgrades the first navigation.
func TestRefreshConsumesTheDeferredRebuild(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	state := deferredTestState(t, app)

	state.fullRebuildDeferred = true
	genBefore := windowRebuildGen

	state.refresh()

	if windowRebuildGen != genBefore+1 {
		t.Fatalf("refresh with a deferred rebuild pending must run it, got %d rebuilds",
			windowRebuildGen-genBefore)
	}
	if state.fullRebuildDeferred {
		t.Error("consumed: the flag must be down")
	}
}

// The reader switched translations while the default version downloaded: the
// completion caches the data and touches nothing else — no swap, no rebuild,
// no deferral, sheet or no sheet. (The pre-existing rule, held against the
// new branch.)
func TestSwitchedAwayCompletionOnlyCaches(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	state := deferredTestState(t, app)
	version, _ := versionByID(defaultVersionID)

	pop := widget.NewModalPopUp(widget.NewLabel("sheet"), state.window.Canvas())
	pop.Show()
	defer pop.Hide()

	state.CurrentVersion = "bsb"
	shown := state.Bible
	full := sampleState().Bible
	state.fullPending = true
	genBefore := windowRebuildGen

	applyFullDownload(state, version, full, modeReal)

	if state.Bible != shown {
		t.Fatal("the reader is on another translation — the live view must not be swapped")
	}
	if state.loadedVersions[version.ID] != full {
		t.Error("the cache must still be warmed")
	}
	if state.fullRebuildDeferred || windowRebuildGen != genBefore {
		t.Error("nothing visible changed: nothing to rebuild, now or later")
	}
	if state.fullPending {
		t.Error("the download did land; fullPending must clear")
	}
}

// The theme observer's spelling: iOS snapshots a backgrounding app in BOTH
// appearances, so the variant round-trips and each leg reads as a real change
// at execution time. deferOrRebuild is what keeps the sheet alive:
// rebuild now with a clear canvas, defer to sheet-close otherwise.
func TestThemeRebuildDefersWhileASheetIsOpen(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	state := deferredTestState(t, app)

	genBefore := windowRebuildGen
	pop := widget.NewModalPopUp(widget.NewLabel("settings stand-in"), state.window.Canvas())
	pop.Show()

	deferOrRebuild(state)

	if windowRebuildGen != genBefore {
		t.Fatal("the variant flip rebuilt over an open sheet — the snapshot round trip kills it again")
	}
	if !pop.Visible() || !state.fullRebuildDeferred {
		t.Fatal("the sheet must survive, and the rebuild must be remembered")
	}

	pop.Hide()
	if !consumeDeferredFullRebuild(state) {
		t.Fatal("sheet closed: the deferred repaint must run")
	}

	// And with a clear canvas the same call rebuilds immediately — today's
	// behavior for a variant change while the reader is just reading.
	genBefore = windowRebuildGen
	deferOrRebuild(state)
	if windowRebuildGen != genBefore+1 {
		t.Fatal("no sheet: the variant change must rebuild right away")
	}
}

// A link parked while the app was on the seed rides the DEFERRED rebuild just
// as it rides the immediate one: the rebuild that finally runs paints the
// shared passage, not the chapter the reader happened to be on.
func TestDeferredRebuildCarriesTheSeedParkedLink(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	state := deferredTestState(t, app)
	version, _ := versionByID(defaultVersionID)

	pop := widget.NewModalPopUp(widget.NewLabel("sheet"), state.window.Canvas())
	pop.Show()

	state.pendingLink = &ShareTarget{VersionID: defaultVersionID, Book: "John", Chapter: 1, VerseLo: 1}
	state.fullPending = true
	applyFullDownload(state, version, sampleState().Bible, modeReal)

	if state.pendingLink == nil {
		t.Fatal("the parked link must wait with the rebuild — consuming it now would navigate under the sheet")
	}

	pop.Hide()
	if !consumeDeferredFullRebuild(state) {
		t.Fatal("the sheet left: the deferred rebuild must run")
	}
	if state.pendingLink != nil {
		t.Error("the deferred rebuild must honour the parked link exactly as the immediate one does")
	}
}

// The WIRING pin: deleting installSheetCloseConsume's
// call from desktop CreateMainUI left the whole suite green, because the
// existing test calls the installer directly — proving the function, never the
// app. On this darwin host the native builder overwrites the closure later in
// the same build, so the only observable wiring truth is the installer's
// invocation counter moving during CreateMainUI.
func TestDesktopBuildWiresTheSheetCloseConsume(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	state := sampleState()
	win := app.NewWindow("wiring")
	t.Cleanup(win.Close)
	state.app = app
	state.window = win

	genBefore := sheetConsumeInstallGen
	win.SetContent(CreateMainUI(app, state, win))
	if sheetConsumeInstallGen == genBefore {
		t.Fatal("CreateMainUI no longer calls installSheetCloseConsume — the " +
			"Windows/Linux sheet-close consume point is unwired, and the " +
			"stale-palette-after-sheet-close bug it exists for is back there")
	}
}
