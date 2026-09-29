package bibletext

// The sheets' watchdogs are armed through sheetAfter (sheet_timers.go), so a
// host test that opens a sheet holds them rather than racing them: under the
// test driver a timer's fyne.Do runs on the timer's own goroutine.

import (
	"fmt"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// holdSheetTimers keeps what the sheets schedule through sheetAfter — the
// audio source menu's, the Ask sheet's and the phone Go to picker's
// watchdogs, the picker's scroll after its panes shrink — for the test to run
// on its own goroutine. The note composer's noteSheetAfter goes through the
// same seam, so this holds it too unless a test holds it by name.
func holdSheetTimers(t *testing.T) *[]func() {
	t.Helper()
	held := &[]func(){}
	prev := sheetAfter
	sheetAfter = func(_ time.Duration, f func()) { *held = append(*held, f) }
	t.Cleanup(func() { sheetAfter = prev })
	return held
}

// runHeld runs one pass of what is held: a watchdog re-arms itself on every
// pass, so the held list never empties by itself.
func runHeld(held *[]func()) {
	fs := *held
	*held = nil
	for _, f := range fs {
		f()
	}
}

// EVERY SHEET WATCHDOG IS HELD BY THE SEAM. A watchdog armed straight on a
// timer ran its close-out — the reading pane's restore — on the timer's own
// goroutine, 150ms after a test had hidden the popup and moved on: the audio
// source menu's did exactly that under the sheet tests, and the Ask sheet's
// and the phone Go to picker's were armed the same way. Held, nothing runs
// until the test runs it, and the close-out still runs then. Mutation
// guarded: any of the three arming time.AfterFunc directly (the restore
// lands during the sleep).
func TestSheetWatchdogsArmThroughTheSeam(t *testing.T) {
	for _, tc := range []struct {
		name  string
		phone bool
		open  func(*AppState)
	}{
		{"the audio source menu", false, showAudioSourceMenu},
		{"the Ask sheet", true, func(s *AppState) { promptAskQuestion(s, "For God so loved the world") }},
		{"the Go to picker", true, showGotoPicker},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var st *AppState
			var top func() *widget.PopUp
			if tc.phone {
				h := phoneSheetHarness(t)
				st, top = h.state, h.top
			} else {
				st, _ = desktopWindow(t, fyne.NewSize(1280, 800))
				top = func() *widget.PopUp { p, _ := st.window.Canvas().Overlays().Top().(*widget.PopUp); return p }
			}
			held := holdSheetTimers(t) // after the harness, which holds the seam itself
			tc.open(st)
			popup := top()
			if popup == nil {
				t.Fatal("control: the sheet did not open")
			}
			restores := 0
			st.showReadingOverlay = func() { restores++ }
			popup.Hide() // as a tap outside the card would, telling the sheet nothing
			time.Sleep(350 * time.Millisecond)
			if restores != 0 {
				t.Fatalf("the reading pane was restored %d times while the watchdog was held: it ran on a timer", restores)
			}
			if len(*held) == 0 {
				t.Fatal("the sheet armed no watchdog through the seam")
			}
			runHeld(held)
			if restores != 1 {
				t.Errorf("the held watchdog's pass restored the reading pane %d times, want 1", restores)
			}
		})
	}
}

// phoneSheetHarness is the appearance harness on a phone: the device answers
// mobile, the window is an iPhone's 390x844, and the sheets' timers are
// held. What it opens is the phone's sheet, not the desktop's.
func phoneSheetHarness(t *testing.T) *appearanceHarness {
	t.Helper()
	h := newAppearanceHarness(t, true)
	fyne.SetCurrentApp(phoneTestApp{h.state.app})
	h.state.window.Resize(fyne.NewSize(390, 844))
	rebuildWindow(h.state)
	return h
}

// A held closure is one thing to run, whatever it was armed for.
var _ = fmt.Sprint
