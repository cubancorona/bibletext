package bibletext

// WHAT state.stopping MEANS, AND WHEN IT ENDS.
//
// state.stopping tells late background work that the app is tearing down, so
// a result that lands then drops itself instead of changing state (state.go).
// OnStopped sets it. What OnStopped means is not the same on every platform,
// and neither is what can follow it. As Fyne 2.7.4's drivers and the repo's
// patches report the lifecycle:
//
//   - Desktop (glfw). OnStarted once, as the run loop begins; OnStopped once,
//     queued as the loop ends. The close intercept marks the teardown before
//     that stop (InstallReadingStateFlush). Nothing starts after a stop.
//   - iOS (the scene life-cycle patch). OnStarted once, at launch, when
//     didFinishLaunching takes the stage from Dead to Alive; OnStopped only
//     from applicationWillTerminate, the one way to Dead. A scene the system
//     disconnects goes back to Alive, not Dead, so it neither stops nor
//     starts. Nothing starts after a stop.
//   - Android (the NativeActivity glue). Go's main runs ONCE per process —
//     ANativeActivity_onCreate calls it only while main_running is 0 — but
//     the activity comes and goes. Each activity's first redraw takes the
//     stage from Dead straight to Focused: OnStarted, then
//     OnEnteredForeground, in that order, in one event. Its onDestroy takes
//     the stage to Dead: OnStopped. Going to the background and coming back
//     move between Focused and Alive and never cross Alive, so they run the
//     foreground hooks alone. An activity is destroyed with the process
//     living on when the reader swipes the app away while the audio service
//     holds the process, when Back finishes the root activity (Android 11 and
//     older), and when the system reclaims it (Developer options > "Don't
//     keep activities" forces that); the next activity then starts in the
//     same process, on the same AppState. A rotation is not one of them: the
//     manifest's configChanges lacks screenSize, yet on the Android 15
//     emulator a rotation keeps the same activity instance and process. A
//     real exit is the process being killed, which runs no Go at all: seen
//     from Go it is a stop that no start follows.
//
// So a stop ends an ACTIVITY, and on Android only a later start can tell that
// the process lived on. OnStarted therefore clears stopping: the new activity
// is live, whatever an earlier activity's stop said. A real teardown still
// sets it, and nothing clears it after, on every platform. OnStarted is the
// only point that can tell: at the stop there is nothing to tell a destroyed
// activity from a process about to be killed, and OnEnteredForeground is also
// every return from the background, and on desktop every regained focus,
// which would clear a desktop teardown.
//
// ONE STOP CAN ARRIVE LATE. OnStarted runs inside the lifecycle event, on the
// UI goroutine. OnStopped does not: the mobile driver queues it on the
// lifecycle's own queue, whose goroutine hands it back to the UI goroutine's
// function queue. The UI loop picks between that queue and the event queue in
// no fixed order, so when both are waiting as it comes free — a recreation
// destroys one activity and creates the next at once — the old activity's
// stop can run after the new activity's start. Each start therefore registers
// the stop hook for its own activity, carrying the activity's generation. The
// driver takes the hook as the stage crosses to Dead, so the hook it queues
// names the activity that ended; a stop for an activity older than the one
// started last flushes the reading position and stops the audio as ever
// (nothing about those changes here), and leaves stopping clear.
//
// WHAT THE NEW ACTIVITY PICKS UP. Between a stop and the next start every
// consumer of stopping refuses, and each is picked up as follows:
//
//   - observeAppearance: the new activity's OnEnteredForeground follows its
//     OnStarted in the same event and reconciles the settled variant against
//     the one the window was built with; a change carried by the new
//     activity's own size event arrives after that, in the foreground, and
//     rebuilds by itself (appearance.go).
//   - triggerFullDownload and ensureUpgradeScheduled: the same
//     OnEnteredForeground retries the refresh. A landing upgradeLanded dropped
//     in between had written its cache first, so the retry reads it from disk
//     (or fetches again, if that write failed: D6).
//   - consumeDeferredFullRebuild: the flag stays up until the next sheet
//     close, refresh() or rebuild takes it — on Android the overlay recovery
//     rebuilds as soon as the native pane finds itself in a new activity.
//   - An interactive translation load's landing: nothing retries it, and
//     dropping it left versionLoading set and the Downloading spinner up for
//     the rest of the process, refusing every later load. It is held instead
//     (holdWhileStopped) and landed by the next start, through fyne.Do, so it
//     runs once the start's event is over, as it would have run had it landed
//     while the activity was live. If no start comes, it never lands.

import "sync"

// activityLife is the record of the activities the process has run in: which
// started last, and the work held for the next. mu guards both and stopping's
// changes with them: on mobile every caller is on the UI goroutine, but on
// desktop a late landing runs inline on its own goroutine once the loop has
// drained.
type activityLife struct {
	mu   sync.Mutex
	gen  uint64   // the activity started last; every OnStarted moves it on
	held []func() // work that landed while stopping was set
}

// activityStarted is OnStarted: an activity is live, so the app is not
// stopping. It returns the new activity's generation, for the stop hook the
// start registers, and the work held for it, for the start to land.
func activityStarted(state *AppState) (uint64, []func()) {
	a := &state.activity
	a.mu.Lock()
	defer a.mu.Unlock()
	a.gen++
	state.stopping.Store(false)
	held := a.held
	a.held = nil
	return a.gen, held
}

// activityStopped is OnStopped for the activity of generation gen. It marks
// the teardown while that activity is still the one started last, and reports
// whether it did; a stop that arrives after a newer start is the older
// activity's and marks nothing.
func activityStopped(state *AppState, gen uint64) bool {
	a := &state.activity
	a.mu.Lock()
	defer a.mu.Unlock()
	if gen != a.gen {
		return false
	}
	state.stopping.Store(true)
	return true
}

// holdWhileStopped keeps f for the next activity when the app is stopping, and
// reports whether it did. The check and the hold are one step under the lock,
// so a start cannot clear stopping between them and leave f held with no
// start to land it.
func holdWhileStopped(state *AppState, f func()) bool {
	a := &state.activity
	a.mu.Lock()
	defer a.mu.Unlock()
	if !state.stopping.Load() {
		return false
	}
	a.held = append(a.held, f)
	return true
}
