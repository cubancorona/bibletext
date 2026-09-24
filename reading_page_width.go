package bibletext

import (
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

// readingWindowWidth is the estimate before any pane has reported a width. It
// answers 0 — "not known", which chooses the book page — unless a platform turns
// on widestWindowWidth: the phones do, where the first push is made before the
// pane has a frame and a phone's window is its pane's width. The Mac does not:
// its pane reports a width before the reader can see the page, and the host
// tests, which build chapters with no pane at all, keep the book page.
var readingWindowWidth = func() float64 { return 0 }

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

// readingPaneWidthNow is the width to choose a page from now.
func readingPaneWidthNow() float64 {
	if readingPaneWidth > 0 {
		return readingPaneWidth
	}
	return readingWindowWidth()
}

// currentReadingPage is the page a native push is made at now. reporterLayout
// answers from the same width, so the stylesheet and the pushes agree within one
// push.
func currentReadingPage() readingPage {
	return readingPageAt(readingPaneWidthNow(), readingReferencePx())
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
	if width <= 0 || width == readingPaneWidth {
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
