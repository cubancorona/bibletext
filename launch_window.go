package bibletext

// THE WINDOW BEFORE THE CANVAS HAS A SIZE.
//
// A mobile canvas takes its size only at its first paint, and the reader's
// first layout can be built before then: on iOS the data loads while UIKit
// connects the window scene, and the layout built when it lands has 0x0 to
// decide by. Answered by the device, that layout was a phone's resting one
// and an iPad's rail, so an iPhone opened while held sideways drew the bottom
// bar, and an iPad opened upright the rail, for a few frames before the
// first paint found the other wanted and the layout watcher rebuilt.
//
// Each platform now reports what it knows of the window before then
// (launchWindowSize, device_*.go), and the layout decides by that size as it
// would by the canvas's (layoutSize): the rail, the phone's landscape
// presentation, the bar's dress and, on Android, the tablet idiom.
//
//	iOS      the window scene's size, read on the main thread by a goroutine
//	         of its own and kept for the launch (keptWindowReport)
//	Android  the activity's configured width and height in dp, read over
//	         the bridge
//	desktop  nothing: a desktop window has a size from its first layout

import (
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
)

// canvasSized reports whether a canvas w x h has a size to decide a layout
// by. A canvas a unit or less on a side has none: Android's first surface can
// be a single pixel, which a dense screen makes a third of a unit each way,
// square and so "wide", and a width a page would be chosen from. No window
// the app runs in is that small.
func canvasSized(w, h float32) bool { return w > 1 && h > 1 }

// launchSize is the seam over launchWindowSize (device_*.go), pinned by tests:
// the size the platform reports for the app's window before its canvas has
// one; ok is false with no report. iOS reports points, the canvas's own unit
// there. Android reports dp, which its canvas unit only approximates: Fyne
// rounds the screen's density to a few scales, so a unit is a dp at 160,
// 240, 320, 480 and 640 dpi and within about a quarter of one between them
// (device_android.go says what that leaves).
var launchSize = launchWindowSize

// layoutSize is the size the layout decides by: the canvas's own once it has
// one (canvasSized), and before then the size the platform reports for the
// window (heldLaunchSize), or 0x0 when it reports none, which leaves each
// decision to its device answer.
func layoutSize(w, h float32) (float32, float32) {
	if canvasSized(w, h) {
		return w, h
	}
	if lw, lh := heldLaunchSize(); canvasSized(lw, lh) {
		return lw, lh
	}
	return 0, 0
}

// launchSizeHeld is the platform's report as the layout has read it since the
// current build began; nil until it is next asked. A build decides its tree
// at its start and the layout watcher records the decision at its end, so
// every question in between must get the same answer: Android's
// configuration is read live, and a rotation in the middle of a build would
// otherwise have the watcher record a layout the tree does not have, which no
// later resize would see as a change. The canvas, the other input, changes
// only between the Fyne goroutine's tasks.
var launchSizeHeld atomic.Pointer[[2]float32]

// heldLaunchSize answers from the held report, asking the platform when
// nothing is held; 0x0 when it reports nothing.
func heldLaunchSize() (float32, float32) {
	if p := launchSizeHeld.Load(); p != nil {
		return p[0], p[1]
	}
	var held [2]float32
	if w, h, ok := launchSize(); ok {
		held = [2]float32{w, h}
	}
	launchSizeHeld.Store(&held)
	return held[0], held[1]
}

// releaseLaunchSize lets the next question ask the platform again: at the
// start of every build (CreateMainUI, ui_mobile.go), and when a new report is
// kept (keepWindowReport).
func releaseLaunchSize() { launchSizeHeld.Store(nil) }

// windowReport is the app's window as a platform's own read of it describes
// it, in the canvas's units: its size, and the left and right safe-area
// insets the canvas lays its content inside, together.
type windowReport struct {
	w, h  float32
	sides float32
}

// paneWidth is the width the window lays the app's tree in: its width less
// the side insets. It is the reading page's estimate for a chapter pushed
// before the canvas has a size (readingColdWidth), the full-screen pane's
// width on a phone held sideways.
func (r windowReport) paneWidth() float64 {
	if w := r.w - r.sides; w > 0 {
		return float64(w)
	}
	return 0
}

// windowReportRead is the platform's read of its window where that read can
// wait: iOS reads its window scene on the main thread, which is busy with
// UIKit's own launch for the first few hundred milliseconds of it. ok is false
// while there is no window to report yet. Nil where no such read is made.
//
// The read is made on a goroutine of its own and kept (keptWindowReport), so
// no layout decision waits for it. Made on the Fyne goroutine, it would hold
// back every launch's first build, upright ones included, for as long as the
// main thread is busy.
var windowReportRead func() (windowReport, bool)

var (
	windowReportStart sync.Once
	windowReportKept  atomic.Pointer[windowReport]
)

// windowReportTries and windowReportRetry bound the read's retries while no
// window is connected yet: a launch into the background connects none.
const windowReportTries = 100

var windowReportRetry = 20 * time.Millisecond

// keptWindowReport is the platform's read of its window, kept for the
// launch: it answers at once, with ok false until the read has landed, and
// the first question starts the read. Safe from any goroutine.
func keptWindowReport() (windowReport, bool) {
	windowReportStart.Do(startWindowReportRead)
	if r := windowReportKept.Load(); r != nil {
		return *r, true
	}
	return windowReport{}, false
}

func startWindowReportRead() {
	read := windowReportRead
	if read == nil {
		return
	}
	go func() {
		for try := 0; try < windowReportTries; try++ {
			if r, ok := read(); ok {
				windowReportLanded(r)
				return
			}
			time.Sleep(windowReportRetry)
		}
	}()
}

// windowReportLanded hands a landed read to the Fyne goroutine, which keeps
// it (keepWindowReport). The report is kept there and nowhere else, between
// two of the goroutine's tasks, so it never changes in the middle of a build:
// a build decides the tree at its start and the layout watcher records the
// decision at its end, and a report kept in between would have the watcher
// record a layout the tree does not have.
func windowReportLanded(r windowReport) { windowReportHandOff(func() { keepWindowReport(r) }) }

// windowReportHandOff runs f on the Fyne goroutine; a test holds it instead.
var windowReportHandOff = func(f func()) { fyne.Do(f) }

// keepWindowReport keeps the report and asks the layout again. The first
// build can be made before the read lands, from the device's answer, and the
// canvas may still have no size when it does: the layout watcher then rebuilds
// before the first paint, rather than at it (launchReportRelayout).
func keepWindowReport(r windowReport) {
	windowReportKept.Store(&r)
	releaseLaunchSize()
	launchReportRelayout()
}

// launchReportRelayout is the layout watcher's second look at an unsized
// window (ui_regular.go); the desktop, which reports nothing, has none.
var launchReportRelayout = func() {}
