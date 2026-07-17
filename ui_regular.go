//go:build ios || android

package bibletext

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// buildRegularWidthUI is the tablet (regular-width) layout: a navigation sidebar
// beside the reading pane, with the app header on top — structurally the desktop
// layout, but the reading pane is the mobile native overlay (buildReadingViewMobile
// via rebuildMobileReadingPane) so the reader keeps native text selection, the
// Study-with-AI menu, and scroll persistence.
//
// The sidebar can be hidden (header sidebar-toggle button) to give the reading
// pane the full width; by default it follows orientation — shown in landscape,
// collapsed in portrait (resolveSidebarDefault). It reuses the shared,
// platform-agnostic buildSidebar and buildHeader, so search / find / book
// navigation behave exactly as on the desktop. The native reading overlay tracks
// the reading pane's real rect automatically (setFrameFromObject in reading_ios.go
// / the Android bridge), so it sits over just the reading pane — full width or the
// right pane of the split — with no layout-specific frame math.
func buildRegularWidthUI(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	// Resolve the orientation default (portrait → collapsed) unless the user has
	// toggled it within the current orientation.
	collapsed := state.resolveSidebarDefault()

	// Invariant: in this layout, search results live in the reading pane but the
	// search / find FIELD lives only in the sidebar — so an active search must
	// keep the sidebar shown. Collapsing it (the toggle, or the portrait
	// auto-collapse) while a search is active would strand full-width results with
	// no field to edit or clear them, AND would leave any in-flight AI Find running
	// on a session the next sidebar build can't cancel. So collapsing ends an active
	// search and returns to reading. clearSearchState also drops aiSearchActive, so
	// a late Find completion is discarded rather than clobbering the reader's view.
	// (surfaceSearch, below, is the other half: "back to results" re-shows the
	// sidebar so the field comes back with the results.)
	if collapsed && state.IsSearching {
		clearSearchState(state)
	}

	// The reading pane swaps between the native reading overlay and the search
	// results list (rebuildMobileReadingPane keys off state.IsSearching), same as
	// the compact Read tab.
	readingHost := container.NewStack(rebuildMobileReadingPane(state))
	state.showReading = func() {
		readingHost.Objects = []fyne.CanvasObject{rebuildMobileReadingPane(state)}
		readingHost.Refresh()
		notifyReadingOverlay(overlayShouldShow(state))
	}

	// On a tablet the reading pane is always on screen, so "surface reading" /
	// "surface results" are just state flips + a refresh — there is no tab to
	// switch to. (selectBook already clears IsSearching; this covers the AI
	// reading-menu paths that call surfaceReading/surfaceSearch directly.)
	state.surfaceReading = func() {
		state.IsSearching = false
		state.refresh()
	}
	state.surfaceSearch = func() {
		state.IsSearching = true
		if state.sidebarCollapsed {
			// "Back to results" while collapsed: the results need the sidebar's
			// field, so reveal the sidebar (a full rebuild switches the layout;
			// state.refresh only swaps the reading pane, which can't add the sidebar).
			state.sidebarCollapsed = false
			rebuildWindow(state)
			return
		}
		state.refresh()
	}

	var content fyne.CanvasObject
	if collapsed {
		// Sidebar hidden: the reading pane gets the whole width. buildSidebar isn't
		// constructed, so the hooks that drive its widgets (focus, set text, re-sync
		// the book list) would otherwise operate on a detached sidebar — point them
		// at safe no-ops. (Search / book navigation are reached by revealing the
		// sidebar again via the header toggle.) No search can be active here — the
		// invariant above cleared it — so there is no results view or AI "Try again"
		// to worry about, and retryAISearch is unreachable while collapsed.
		state.syncSidebar = func() {}
		state.focusSearch = func() {}
		state.setSearchText = func(string) {}
		content = readingHost
	} else {
		// buildSidebar wires syncSidebar, focusSearch, setSearchText and retryAISearch
		// and owns the search / find fields and the book list.
		sidebar := buildSidebar(state)
		split := container.NewHSplit(sidebar, readingHost)
		// Aim for a consistent ~250pt sidebar regardless of iPad size (see
		// regularSplitOffset); the user can still drag the divider from here.
		split.SetOffset(regularSplitOffset(state.canvasWidth()))
		content = split
	}

	body := container.NewBorder(buildHeader(state), nil, nil, nil, content)

	base := canvas.NewRectangle(pal.Background)
	notifyReadingOverlay(overlayShouldShow(state))
	return container.NewStack(base, body)
}

// layoutWatcher wraps the tablet root so a size change that matters rebuilds the
// window: (1) a width crossing the compact/regular breakpoint (a narrow
// multitasking column), or (2) a portrait↔landscape flip while regular, so the
// orientation-driven sidebar default re-applies and the split offset recomputes.
// It renders its wrapped content unchanged and adds no chrome; only Resize acts.
//
// It is installed only on tablets (see CreateMainUI): a phone never crosses the
// breakpoint and doesn't have the sidebar layout, so its path is unchanged.
type layoutWatcher struct {
	widget.BaseWidget
	state          *AppState
	content        fyne.CanvasObject
	builtAs        layoutClass
	builtLandscape bool // orientation at build (only meaningful when builtAs is regular)
	pending        bool
}

func newLayoutWatcher(state *AppState, content fyne.CanvasObject) *layoutWatcher {
	w := &layoutWatcher{
		state:          state,
		content:        content,
		builtAs:        state.layoutClass(),
		builtLandscape: state.canvasIsLandscape(),
	}
	w.ExtendBaseWidget(w)
	return w
}

func (w *layoutWatcher) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(w.content)
}

// Resize re-evaluates the layout against the new size. If the layout class or (in
// the regular layout) the orientation changed, a rebuild is scheduled on the UI
// thread — not run inline, since we're mid-layout. pending coalesces the burst of
// Resize calls during a live divider/rotation into a single rebuild; the watcher
// is recreated by that rebuild, which resets the guard.
//
// CRITICAL: orientation must be derived from the WINDOW CANVAS size
// (state.canvasIsLandscape), NEVER from this Resize's size argument. When the
// soft keyboard rises, Fyne lays the content out at the canvas size MINUS the
// keyboard — on an iPad in portrait that squeezed height makes width >= height,
// which read as a landscape flip here and triggered a rebuild; the rebuild's
// fresh watcher then sampled the UNsqueezed canvas (portrait), so the next
// layout pass "flipped" again, rebuilding forever — a visible reading-pane
// flicker whenever the search/Find field had the keyboard up (field-reported
// on iPad, reproduced in the sim: 3,000+ rebuilds in under a minute). Canvas
// size ignores the keyboard, and it is the SAME source newLayoutWatcher
// samples, so the two can never disagree and oscillate. The layout-class check
// keys off width, which the keyboard never changes.
func (w *layoutWatcher) Resize(size fyne.Size) {
	w.BaseWidget.Resize(size)
	if w.pending {
		return
	}
	want := classifyLayout(size.Width, deviceIsTablet())
	landscape := w.state.canvasIsLandscape()
	changed := want != w.builtAs || (want == layoutRegular && landscape != w.builtLandscape)
	if changed {
		w.pending = true
		fyne.Do(func() { rebuildWindow(w.state) })
	}
}
