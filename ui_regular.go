//go:build ios || android

package bibletext

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// buildRegularWidthUI is the tablet (regular-width) layout: a persistent
// navigation sidebar beside the reading pane, with the app header on top —
// structurally the desktop layout, but the reading pane is the mobile native
// overlay (buildReadingViewMobile via rebuildMobileReadingPane) so the reader
// keeps native text selection, the Study-with-AI menu, and scroll persistence.
//
// It reuses the shared, platform-agnostic buildSidebar and buildHeader, so search
// / find / book navigation behave exactly as on the desktop. The native reading
// overlay tracks the reading pane's real rect automatically (setFrameFromObject
// in reading_ios.go / the Android bridge), so it sits over just the right pane
// with no split-specific frame math.
func buildRegularWidthUI(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	// The reading pane swaps between the native reading overlay and the search
	// results list (rebuildMobileReadingPane keys off state.IsSearching), same as
	// the compact Read tab.
	readingHost := container.NewStack(rebuildMobileReadingPane(state))
	state.showReading = func() {
		readingHost.Objects = []fyne.CanvasObject{rebuildMobileReadingPane(state)}
		readingHost.Refresh()
		notifyReadingOverlay(overlayShouldShow(state))
	}

	// On a tablet the sidebar is always on screen, so "surface reading" / "surface
	// results" are just state flips + a refresh — there is no tab to switch to.
	// (selectBook already clears IsSearching; this covers the AI reading-menu
	// paths that call surfaceReading/surfaceSearch directly.)
	state.surfaceReading = func() {
		state.IsSearching = false
		state.refresh()
	}
	state.surfaceSearch = func() {
		state.IsSearching = true
		state.refresh()
	}

	// buildSidebar wires syncSidebar, focusSearch and setSearchText and owns the
	// search / find fields and the book list.
	sidebar := buildSidebar(state)

	split := container.NewHSplit(sidebar, readingHost)
	// Aim for a consistent ~250pt sidebar regardless of iPad size (see
	// regularSplitOffset); the user can still drag the divider from here.
	split.SetOffset(regularSplitOffset(state.canvasWidth()))

	body := container.NewBorder(buildHeader(state), nil, nil, nil, split)

	base := canvas.NewRectangle(pal.Background)
	notifyReadingOverlay(overlayShouldShow(state))
	return container.NewStack(base, body)
}

// layoutWatcher wraps the tablet root so a width change that crosses the
// compact/regular breakpoint (rotation, or resizing an iPad multitasking column)
// rebuilds the window into the other layout. It renders its wrapped content
// unchanged and adds no chrome; only Resize does anything.
//
// It is installed only on tablets (see CreateMainUI): a phone never crosses the
// breakpoint, so its path stays exactly as before.
type layoutWatcher struct {
	widget.BaseWidget
	state   *AppState
	content fyne.CanvasObject
	builtAs layoutClass
	pending bool
}

func newLayoutWatcher(state *AppState, content fyne.CanvasObject) *layoutWatcher {
	w := &layoutWatcher{state: state, content: content, builtAs: state.layoutClass()}
	w.ExtendBaseWidget(w)
	return w
}

func (w *layoutWatcher) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(w.content)
}

// Resize re-evaluates the layout class against the new width. If it changed, a
// rebuild is scheduled on the UI thread (not run inline — we're mid-layout).
// pending coalesces the burst of Resize calls during a live divider/multitasking
// drag into a single rebuild; the watcher is recreated by that rebuild, which
// resets the guard.
func (w *layoutWatcher) Resize(size fyne.Size) {
	w.BaseWidget.Resize(size)
	if w.pending {
		return
	}
	if classifyLayout(size.Width, deviceIsTablet()) != w.builtAs {
		w.pending = true
		fyne.Do(func() { rebuildWindow(w.state) })
	}
}
