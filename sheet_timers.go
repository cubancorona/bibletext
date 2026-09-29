package bibletext

// The timers the sheets arm: a watchdog that catches a close the toolkit
// makes without telling the sheet (a tap outside a non-modal card runs
// PopUp.Hide and nothing else), and the Go to picker's scroll-into-view after
// its panes shrink for the keyboard.
//
// Every one goes through sheetAfter, and never straight to time.AfterFunc,
// because of what the test driver makes of fyne.Do: with no UI thread to
// marshal to it runs the closure on the timer's own goroutine, so a watchdog
// armed by a sheet a host test opened reads the popup's Visible() against the
// test's Hide, and runs its close-out — the native reading pane's restore, on
// a Mac, which reads the appearance seam the next test is writing — while the
// next test is already running. A test that opens one of these sheets holds
// the seam (holdSheetTimers) and runs what was armed on its own goroutine
// where it needs the close-out; the note composer's noteSheetAfter is the same
// seam under its own name, kept for the tests that hold it alone.

import (
	"time"

	"fyne.io/fyne/v2"
)

// sheetAfter runs f on the UI goroutine after d.
var sheetAfter = func(d time.Duration, f func()) {
	time.AfterFunc(d, func() { fyne.Do(f) })
}
