package bibletext

import (
	"math"
	"time"

	"fyne.io/fyne/v2"
)

// THE WIDTH THE READING PAGE IS CHOSEN FROM, on the three native panes.
//
// The canvas pane lays its page out from the width it is given (relayout). The
// native panes are handed a chapter before they know their width, and learn it
// from the frames the toolkit pushes afterwards, so the page is decided in two
// moves: the first push is made at the best width known — the last one a pane
// reported, or the window's before any has — and when a reported width settles
// on the other page, the chapter is pushed again at that page. The two agree
// almost always (a phone is narrow, a tablet or a desktop window wide), so the
// second push happens only when a window is resized across the switch or opens
// at a width the estimate had wrong. The same-chapter re-push keeps the
// reader's place (shouldCaptureScrollRestore), as a theme flip does.
//
// All of this runs on the Fyne goroutine: the width arrives through fyne.Do.

var (
	// readingPaneWidth is the width, in the surface's unit, that a native pane
	// last reported for its text; 0 until one has.
	readingPaneWidth float64
	// readingPaneWindow is the window's width (readingWindowWidth) when the
	// pane last reported; 0 where the platform gives no window estimate.
	readingPaneWindow float64
	// readingPaneUnit is the pane's unit per window unit: 1 where the pane
	// reports in the window's own unit (macOS, iOS), the ratio of dp to Fyne's
	// unit on Android, measured from the host each time the bridge reports.
	readingPaneUnit = 1.0
	// readingPagePushed is the page the last chapter push was made at.
	readingPagePushed      readingPageKind
	readingPagePushedValid bool
	readingPageTimer       *time.Timer
)

// readingPageSettle is how long a new width must hold before a change of page
// re-pushes the chapter — long enough that a window being dragged across the
// switch does not re-import it on every frame, short enough to read as the
// window's own response.
var readingPageSettle = 120 * time.Millisecond

// readingPageRun runs the re-push on the Fyne goroutine; a test replaces it.
var readingPageRun = func(f func()) { fyne.Do(f) }

// readingWindowWidth is the widest window's canvas width, the estimate before
// any pane has reported a width. It answers 0 — "not known" — unless a platform
// turns on widestWindowWidth, as the native panes all do. The Windows and Linux
// pane lays out from its own width and never asks.
var readingWindowWidth = func() float64 { return 0 }

// readingColdWidth is the pane's width, in its own unit, before the window has
// any size: the canvas has none until its first paint, and a chapter can be
// pushed before that. Android reads its activity's configured width
// (androidWindowWidthDp) — a call into Java, made only then; 0 elsewhere.
var readingColdWidth = func() float64 { return 0 }

// readingUnsizedPage is the page for a push made with no width known at all —
// no report from the pane and no window size. The book page by default (the
// Mac, the host tests); the phones answer from the device, as the layout
// already does for a canvas with no size (phoneLandscapeReadingWanted): an
// iPhone's resting page is the phone page, an iPad's the book page. The one
// place a page is chosen by device, and only for want of a width.
var readingUnsizedPage = func() readingPageKind { return readingPageBook }

// widestWindowWidth is the widest open window's canvas width.
func widestWindowWidth() float64 {
	app := fyne.CurrentApp()
	if app == nil {
		return 0
	}
	best := float32(0)
	for _, w := range app.Driver().AllWindows() {
		if c := w.Canvas(); c != nil && c.Size().Width > best {
			best = c.Size().Width
		}
	}
	return float64(best)
}

// readingPaneWidthNow is the width to choose a page from now: the pane's last
// report, moved by as much as the window has moved since. A rotation or a
// split-screen change rebuilds the window, and the chapter is pushed into the
// new pane before that pane has been laid out; its last report is the OLD
// orientation's, and read as it stands would push the old page and correct it
// a moment later with a second import. The window has its new size by then,
// and the pane's width changes by what the window's did — exactly so with a
// sidebar or rail of fixed width beside it — converted to the pane's unit
// (readingPaneUnit). The pane's own report follows and settles it.
func readingPaneWidthNow() float64 {
	if readingPaneWidth <= 0 {
		if w := readingWindowWidth(); w > 0 {
			return w
		}
		return readingColdWidth()
	}
	if readingPaneWindow > 0 {
		if w := readingWindowWidth(); w > 0 && w != readingPaneWindow {
			return math.Max(1, readingPaneWidth+(w-readingPaneWindow)*readingPaneUnit)
		}
	}
	return readingPaneWidth
}

// currentReadingPage is the page a native push is made at now. reporterLayout
// answers from the same width, so the stylesheet and the pushes agree within one
// push. With no width known at all, the device's resting page
// (readingUnsizedPage), unless the dev override names one.
func currentReadingPage() readingPage {
	ref := readingReferencePx()
	w := readingPaneWidthNow()
	if w <= 0 {
		if readingPageOverride != nil {
			if k, ok := readingPageOverride(); ok {
				return readingPageOf(k, 0, ref)
			}
		}
		return readingPageOf(readingUnsizedPage(), 0, ref)
	}
	return readingPageAt(w, ref)
}

// markReadingPagePushed records the page a chapter push was made at.
func markReadingPagePushed(k readingPageKind) {
	readingPagePushed, readingPagePushedValid = k, true
}

// noteReadingPaneWidth records a width a native pane reported and, when the page
// for it differs from the one last pushed, re-pushes after the width settles. A
// later width re-arms the wait, and the page is asked again when it fires, so a
// drag that crosses the switch and comes back re-pushes nothing.
func noteReadingPaneWidth(width float64, repush func()) {
	if width <= 0 {
		return
	}
	readingPaneWindow = readingWindowWidth()
	if width == readingPaneWidth {
		return
	}
	readingPaneWidth = width
	if readingPageTimer != nil {
		readingPageTimer.Stop()
	}
	readingPageTimer = time.AfterFunc(readingPageSettle, func() {
		readingPageRun(func() {
			if readingPagePushedValid && currentReadingPage().Kind != readingPagePushed {
				repush()
			}
		})
	})
}
