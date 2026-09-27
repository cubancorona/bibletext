package bibletext

// What state.stopping means across activities (activity_life.go).
//
// THE DEFECT these hold: on Android an activity can be destroyed while the
// process lives on — the audio service holds the process through a
// swipe-away, or the system reclaims the activity — and its OnStopped set
// stopping. Fyne runs
// main once per process, so nothing cleared it when the next activity
// started, and for the rest of the process the appearance gate returned at
// once (a light/dark change was never applied), the refresh stood down, and a
// translation load that landed in between left its spinner up and refused
// every later load.
//
// The hooks are the ones InstallReadingStateFlush installs, run in the orders
// the drivers run them in:
//
//	Android   an activity's first redraw, Dead to Focused in one event:
//	          OnStarted, then OnEnteredForeground. Its destroy: the native
//	          window goes (OnExitedForeground), then onDestroy crosses to
//	          Dead, where the driver takes the stop hook and QUEUES it — so
//	          it usually runs at once, and can run after the next start.
//	          A real exit is a stop that no start follows.
//	iOS       OnStarted once at launch; the foreground hooks as the app
//	          comes and goes; OnStopped only at termination.
//	Desktop   OnStarted once as the loop begins; focus moves the foreground
//	          hooks; the close intercept, then OnStopped as the loop ends.

import (
	"errors"
	"testing"

	"fyne.io/fyne/v2"
)

// lifeHooks are the four lifecycle hooks, as the test app's lifecycle hands
// back the ones installed.
type lifeHooks interface {
	OnStarted() func()
	OnStopped() func()
	OnEnteredForeground() func()
	OnExitedForeground() func()
}

// installLifeHooks installs the app's lifecycle hooks on the harness's app,
// with the scroll capture stubbed as the wiring test stubs it, and hands them
// back. window is nil for a phone, where no close intercept is installed.
func installLifeHooks(t *testing.T, h *appearanceHarness, window fyne.Window) lifeHooks {
	t.Helper()
	prevCapture := captureAnchorFn
	captureAnchorFn = func() (int, float64, float64, bool) { return 0, 0, 0, false }
	t.Cleanup(func() { captureAnchorFn = prevCapture })
	InstallReadingStateFlush(h.state.app, window, h.state)
	hooks, ok := h.state.app.Lifecycle().(lifeHooks)
	if !ok || hooks.OnStarted() == nil || hooks.OnStopped() == nil ||
		hooks.OnEnteredForeground() == nil || hooks.OnExitedForeground() == nil {
		t.Fatal("control: InstallReadingStateFlush must install all four lifecycle hooks")
	}
	return hooks
}

// androidLife drives the hooks in the Android glue's order.
type androidLife struct{ hooks lifeHooks }

// starts is an activity's first redraw: one event from Dead to Focused.
func (a androidLife) starts() {
	a.hooks.OnStarted()()
	a.hooks.OnEnteredForeground()()
}

// destroyed is the activity going: the native window (Focused to Alive), then
// onDestroy (Alive to Dead). It returns the stop hook the driver took at the
// crossing, which the driver queues: run it at once for the usual order, or
// after the next start for the late one.
func (a androidLife) destroyed() func() {
	a.hooks.OnExitedForeground()()
	return a.hooks.OnStopped()
}

// captureVersionLoad holds the next interactive translation load in flight
// and hands back its landing.
func captureVersionLoad(t *testing.T) *func(*BibleData, dataMode, error) {
	t.Helper()
	var land func(*BibleData, dataMode, error)
	prev := startVersionLoad
	startVersionLoad = func(_ BibleVersion, _ *BibleData, l func(*BibleData, dataMode, error)) { land = l }
	t.Cleanup(func() { startVersionLoad = prev })
	return &land
}

// captureUpgradeFetches holds every refresh fetch in flight and hands back
// their landings, in order.
func captureUpgradeFetches(t *testing.T) *[]func(*BibleData, dataMode, error) {
	t.Helper()
	var lands []func(*BibleData, dataMode, error)
	prev := startUpgradeFetch
	startUpgradeFetch = func(_ BibleVersion, l func(*BibleData, dataMode, error)) { lands = append(lands, l) }
	t.Cleanup(func() { startUpgradeFetch = prev })
	return &lands
}

// (a) and (b). THE NEXT ACTIVITY STARTS NOT STOPPING, AND FOLLOWS LIGHT/DARK.
// A change made while no activity was live is applied as the next one starts
// (its reconcile), and a change made in its foreground rebuilds at once.
// Mutation guarded: the start not clearing stopping (both halves fail: the
// flag stays up and neither change is applied).
func TestANewActivityInALivingProcessStartsNotStopping(t *testing.T) {
	t.Run("a change made while no activity was live", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		life := androidLife{installLifeHooks(t, h, nil)}
		life.starts()
		if h.state.stopping.Load() {
			t.Fatal("control: a launched activity must not be stopping")
		}
		life.destroyed()()
		if !h.state.stopping.Load() {
			t.Fatal("control: the activity's stop must mark the teardown")
		}
		gen := windowRebuildGen
		h.flip() // the system switches with no activity to show it
		if windowRebuildGen != gen {
			t.Fatal("control: a change heard with no activity live must wait")
		}
		life.starts()
		if h.state.stopping.Load() {
			t.Fatal("the next activity in the same process started out stopping")
		}
		if windowRebuildGen != gen+1 || h.state.appearance.built != h.variant {
			t.Errorf("the next activity must apply the change made while none was live: %d rebuilds, built %v, system %v",
				windowRebuildGen-gen, h.state.appearance.built, h.variant)
		}
	})
	t.Run("a change made in the next activity", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		life := androidLife{installLifeHooks(t, h, nil)}
		life.starts()
		life.destroyed()()
		life.starts()
		gen := windowRebuildGen
		// Android's order: the new activity's size event, carrying the
		// variant, lands after the start's event, and its closure after it.
		h.flip()
		if windowRebuildGen != gen+1 || h.state.appearance.built != h.variant {
			t.Fatalf("a light/dark change in the next activity was not applied: %d rebuilds, built %v, system %v",
				windowRebuildGen-gen, h.state.appearance.built, h.variant)
		}
		h.flip()
		if windowRebuildGen != gen+2 {
			t.Errorf("a second change must rebuild again: %d rebuilds", windowRebuildGen-gen)
		}
	})
}

// (c) THE REFRESH IS NOT REFUSED IN THE NEXT ACTIVITY. The refresh's own
// retry fired while no activity was live and stood down, and the fetch in
// flight at the destroy landed then and was dropped; the next activity's
// foreground hook retries it, and the retry lands. Mutation guarded: the
// start not clearing stopping (the foreground hook's retry is refused, and
// the default stays owed for the rest of the process).
func TestTheRefreshRunsAgainInTheNextActivity(t *testing.T) {
	lands := captureUpgradeFetches(t)
	h := newAppearanceHarness(t, true)
	life := androidLife{installLifeHooks(t, h, nil)}
	life.starts()
	h.state.fullPending = true // the default is owed its full text
	triggerFullDownload(h.state)
	if len(*lands) != 1 {
		t.Fatalf("control: the refresh must be in flight (%d fetches)", len(*lands))
	}
	life.destroyed()()
	before := h.state.Bible
	(*lands)[0](stampedBible("current"), modeReal, nil) // lands with no activity live
	if h.state.Bible != before || !h.state.fullPending || h.state.fullDownloading {
		t.Fatalf("control: a landing while stopping must drop itself and end the flight: pending %v, fetching %v",
			h.state.fullPending, h.state.fullDownloading)
	}
	triggerFullDownload(h.state) // the backoff's retry firing now
	if len(*lands) != 1 {
		t.Fatal("control: the refresh must stand down while stopping")
	}
	life.starts()
	if len(*lands) != 2 || !h.state.fullDownloading {
		t.Fatalf("the next activity's foreground hook did not retry the refresh (%d fetches, fetching %v)",
			len(*lands), h.state.fullDownloading)
	}
	(*lands)[1](stampedBible("current"), modeReal, nil)
	if bibleStamp(h.state.Bible) != "current" || h.state.fullPending || h.state.fullDownloading {
		t.Errorf("the retry must land in the next activity: on %q, pending %v, fetching %v",
			bibleStamp(h.state.Bible), h.state.fullPending, h.state.fullDownloading)
	}
}

// A TRANSLATION LOAD THAT LANDS BETWEEN ACTIVITIES LANDS IN THE NEXT. Dropped,
// it left versionLoading set and the Downloading spinner up, and every later
// load was refused as "already in flight". It is held while stopping and
// landed by the next start, and only by that one: once the reader has moved
// on, a later recreation must not land it again. Mutations guarded: the
// landing dropping itself again, and the start not landing what was held
// (either way the spinner stays up and the next load is refused); and the
// start keeping what it landed (every later recreation then lands it again and
// moves the reader back to that translation).
func TestATranslationLoadLandingBetweenActivitiesLandsInTheNext(t *testing.T) {
	land := captureVersionLoad(t)
	h := newAppearanceHarness(t, true)
	life := androidLife{installLifeHooks(t, h, nil)}
	life.starts()
	switchVersionInteractive(h.state, "bsb", byReader)
	if *land == nil || !sheetHas(h.top(), "Downloading Berean Standard Bible…") {
		t.Fatalf("control: the download must be in flight behind its spinner; texts %v", sheetTexts(h.top()))
	}
	life.destroyed()()
	(*land)(stampedBible("bsb"), modeReal, nil)
	if h.state.CurrentVersion == "bsb" || !h.state.versionLoading {
		t.Fatalf("a landing with no activity live must not apply: on %q, loading %v", h.state.CurrentVersion, h.state.versionLoading)
	}
	life.starts()
	if h.state.CurrentVersion != "bsb" || bibleStamp(h.state.Bible) != "bsb" || h.state.versionLoading {
		t.Fatalf("the next activity must land the translation: on %q with %q, loading %v",
			h.state.CurrentVersion, bibleStamp(h.state.Bible), h.state.versionLoading)
	}
	if h.overlays() != 0 {
		t.Errorf("the spinner must be down once its download has landed: %v", sheetTexts(h.top()))
	}
	*land = nil
	switchVersionInteractive(h.state, defaultVersionID, byReader)
	if *land == nil {
		t.Fatal("the next load was refused as if the landed one were still in flight")
	}
	(*land)(stampedBible(defaultVersionID), modeReal, nil)
	if h.state.CurrentVersion != defaultVersionID || bibleStamp(h.state.Bible) != defaultVersionID || h.state.versionLoading {
		t.Fatalf("control: the reader's next load must land in the live activity: on %q with %q, loading %v",
			h.state.CurrentVersion, bibleStamp(h.state.Bible), h.state.versionLoading)
	}
	life.destroyed()() // a later recreation, with nothing in flight
	life.starts()
	if h.state.CurrentVersion != defaultVersionID || bibleStamp(h.state.Bible) != defaultVersionID ||
		h.state.versionLoading || h.overlays() != 0 {
		t.Errorf("a later recreation landed again what an earlier start had landed: on %q with %q, loading %v, overlays %v",
			h.state.CurrentVersion, bibleStamp(h.state.Bible), h.state.versionLoading, sheetTexts(h.top()))
	}
}

// THE PREVIOUS ACTIVITY'S STOP, RUN AFTER THE NEXT ONE'S START, STOPS NOTHING.
// The driver queues the stop and runs the start inside its event, so a
// recreation's two can meet in that order; the late stop still flushes, and the
// new activity's own stop still marks. Mutations guarded: the stop ignoring
// its generation (the late stop marks the live activity as stopping), and the
// start not registering its own stop hook (the live activity's stop then
// marks nothing, so a real exit would apply what lands).
func TestAStopArrivingAfterTheNextStartIsTheOldActivitys(t *testing.T) {
	h := newAppearanceHarness(t, true)
	life := androidLife{installLifeHooks(t, h, nil)}
	life.starts()
	late := life.destroyed() // taken at the crossing, still queued
	life.starts()
	late()
	if h.state.stopping.Load() {
		t.Fatal("the destroyed activity's late stop marked the live one as stopping")
	}
	gen := windowRebuildGen
	h.flip()
	if windowRebuildGen != gen+1 {
		t.Error("a light/dark change after the late stop was not applied")
	}
	life.destroyed()()
	if !h.state.stopping.Load() {
		t.Error("the live activity's own stop must still mark the teardown")
	}
}

// (d) A REAL EXIT STILL STOPS, AND WHAT LANDS IN IT DROPS ITSELF. On Android
// a process that is killed after its activity's stop runs nothing more, so
// what the stop marks is all there is: a refresh landing changes nothing and
// arms nothing, a translation landing is not applied, and the refresh stands
// down. A light/dark change is not applied either, but not because of the
// stop's mark: the destroy's own foreground exit ran first and closed the
// appearance gate, and the gate refuses the change before it reads stopping.
// Where stopping alone holds a change off — the desktop teardown, an iOS
// termination from the foreground — TestDesktopAndIOSStopAsTheyDid proves it.
// Mutations guarded: the stop not marking the teardown, and the translation
// landing applying while stopping.
func TestARealExitStillStopsAndWhatLandsDropsItself(t *testing.T) {
	lands := captureUpgradeFetches(t)
	land := captureVersionLoad(t)
	h := newAppearanceHarness(t, true)
	life := androidLife{installLifeHooks(t, h, nil)}
	life.starts()
	h.state.fullPending = true
	triggerFullDownload(h.state)
	switchVersionInteractive(h.state, "bsb", byReader)
	if len(*lands) != 1 || *land == nil {
		t.Fatal("control: a refresh and a translation load must be in flight")
	}
	life.destroyed()() // and no start follows: the process is killed
	if !h.state.stopping.Load() {
		t.Fatal("the last activity's stop must mark the teardown")
	}
	before := h.state.Bible
	armed := upgradeRetriesArmed.Load()
	(*lands)[0](nil, modeReal, errors.New("offline"))
	if upgradeRetriesArmed.Load() != armed || h.state.fullRetryDelay != 0 {
		t.Errorf("a refresh failing in the teardown armed a retry (%d) or moved the backoff to %v",
			upgradeRetriesArmed.Load()-armed, h.state.fullRetryDelay)
	}
	(*land)(stampedBible("bsb"), modeReal, nil)
	if h.state.CurrentVersion == "bsb" || h.state.Bible != before {
		t.Errorf("a translation landing in the teardown was applied: on %q", h.state.CurrentVersion)
	}
	triggerFullDownload(h.state)
	if len(*lands) != 1 {
		t.Error("the refresh must stand down in the teardown")
	}
	if !h.state.appearance.background {
		t.Fatal("control: the destroy's foreground exit must have closed the appearance gate")
	}
	gen := windowRebuildGen
	h.flip()
	if windowRebuildGen != gen {
		t.Error("a light/dark change in the teardown rebuilt the window")
	}
}

// interceptWindow keeps the close intercept InstallReadingStateFlush sets, so
// a test can close the window as its close button does. The intercept's own
// Close is recorded, not passed on: the harness closes the test window at
// cleanup, and the test driver cannot close one twice.
type interceptWindow struct {
	fyne.Window
	intercept func()
	closed    bool
}

func (w *interceptWindow) SetCloseIntercept(f func()) { w.intercept = f }
func (w *interceptWindow) Close()                     { w.closed = true }

// (e) DESKTOP AND iOS STOP AS THEY DID. Each starts once, at launch, where the
// start changes nothing, and nothing starts after its stop.
//
// A light/dark change heard in the teardown is held off by stopping alone in
// two of these. The desktop gate has no background flag: it acts on every
// change, so the teardown's mark is all that keeps a change heard as the
// window closes from rebuilding it. And an iOS termination that comes while
// the app is still active reaches the mobile driver as one event from Focused
// to Dead, in which the foreground-exit hook never runs: the driver drops the
// GL context at the Visible crossing, and on iOS its Focused crossing returns
// when the GL context is gone, before that hook. The gate is left open, and
// only the stop's mark refuses the change.
//
// Mutations guarded: the desktop close intercept no longer marking the
// teardown (a landing then applies during the exit); stopping cleared by a
// return to the foreground as well, the other point the fix could have taken
// (a desktop window focused during its teardown then applies what lands, and
// rebuilds on a light/dark change); the start not registering its own stop
// hook (iOS's termination marks nothing); and the appearance gate no longer
// refusing while stopping (the desktop teardown, and an iOS termination from
// the foreground, rebuild the window on a light/dark change).
func TestDesktopAndIOSStopAsTheyDid(t *testing.T) {
	launch := func(t *testing.T, h *appearanceHarness, hooks lifeHooks) {
		t.Helper()
		gen, fetches, gate := windowRebuildGen, upgradeFetchesStarted.Load(), h.state.appearance
		hooks.OnStarted()()
		if h.state.stopping.Load() || windowRebuildGen != gen || upgradeFetchesStarted.Load() != fetches || h.state.appearance != gate {
			t.Fatalf("the launch's start changed something: stopping %v, %d rebuilds, %d fetches, gate %+v -> %+v",
				h.state.stopping.Load(), windowRebuildGen-gen, upgradeFetchesStarted.Load()-fetches, gate, h.state.appearance)
		}
	}

	t.Run("desktop", func(t *testing.T) {
		land := captureVersionLoad(t)
		h := newAppearanceHarness(t, false)
		win := &interceptWindow{Window: h.state.window}
		hooks := installLifeHooks(t, h, win)
		if win.intercept == nil {
			t.Fatal("control: a desktop window must get the close intercept")
		}
		launch(t, h, hooks)
		hooks.OnExitedForeground()() // another window takes the focus, and gives it back
		hooks.OnEnteredForeground()()
		if h.state.stopping.Load() {
			t.Fatal("control: focus moving must not mark a teardown")
		}
		gen := windowRebuildGen
		h.flip()
		if windowRebuildGen != gen+1 {
			t.Fatalf("control: a light/dark change on a live desktop window must rebuild it (%d rebuilds)", windowRebuildGen-gen)
		}
		switchVersionInteractive(h.state, "bsb", byReader)
		if *land == nil {
			t.Fatal("control: the download must be in flight")
		}
		win.intercept() // the close button
		if !h.state.stopping.Load() || !win.closed {
			t.Fatalf("the close button must mark the teardown and close the window: stopping %v, closed %v",
				h.state.stopping.Load(), win.closed)
		}
		gen = windowRebuildGen
		h.flip() // the system switches as the window closes
		if windowRebuildGen != gen {
			t.Errorf("a light/dark change during the desktop teardown rebuilt the window (%d rebuilds)", windowRebuildGen-gen)
		}
		hooks.OnEnteredForeground()() // the window focused as it closes
		(*land)(stampedBible("bsb"), modeReal, nil)
		if !h.state.stopping.Load() || h.state.CurrentVersion == "bsb" {
			t.Errorf("a landing during the exit was applied: stopping %v, on %q", h.state.stopping.Load(), h.state.CurrentVersion)
		}
		gen = windowRebuildGen
		h.flip()
		if windowRebuildGen != gen {
			t.Errorf("a light/dark change after the window was focused in its teardown rebuilt it (%d rebuilds)", windowRebuildGen-gen)
		}
		hooks.OnStopped()() // the run loop's end
		if !h.state.stopping.Load() || h.state.CurrentVersion == "bsb" {
			t.Errorf("the loop's stop must leave the teardown marked and the landing unapplied: stopping %v, on %q",
				h.state.stopping.Load(), h.state.CurrentVersion)
		}
	})

	t.Run("iOS", func(t *testing.T) {
		land := captureVersionLoad(t)
		h := newAppearanceHarness(t, true)
		hooks := installLifeHooks(t, h, nil)
		launch(t, h, hooks)
		hooks.OnEnteredForeground()()
		for range 2 { // to the background and back, twice
			hooks.OnExitedForeground()()
			hooks.OnEnteredForeground()()
		}
		if h.state.stopping.Load() {
			t.Fatal("control: the background must not mark a teardown")
		}
		switchVersionInteractive(h.state, "bsb", byReader)
		if *land == nil {
			t.Fatal("control: the download must be in flight")
		}
		hooks.OnExitedForeground()()
		hooks.OnStopped()() // applicationWillTerminate
		if !h.state.stopping.Load() {
			t.Fatal("termination must mark the teardown")
		}
		(*land)(stampedBible("bsb"), modeReal, nil)
		if h.state.CurrentVersion == "bsb" {
			t.Error("a landing after termination was applied")
		}
	})

	t.Run("iOS terminated from the foreground", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		hooks := installLifeHooks(t, h, nil)
		launch(t, h, hooks)
		hooks.OnEnteredForeground()()
		gen := windowRebuildGen
		h.flip()
		if windowRebuildGen != gen+1 {
			t.Fatalf("control: a light/dark change in the foreground must rebuild (%d rebuilds)", windowRebuildGen-gen)
		}
		hooks.OnStopped()() // Focused to Dead in one event: no foreground exit
		if !h.state.stopping.Load() || h.state.appearance.background {
			t.Fatalf("control: termination must mark the teardown with the gate still open: stopping %v, background %v",
				h.state.stopping.Load(), h.state.appearance.background)
		}
		gen = windowRebuildGen
		h.flip()
		if windowRebuildGen != gen {
			t.Errorf("a light/dark change after termination rebuilt the window (%d rebuilds)", windowRebuildGen-gen)
		}
	})
}
