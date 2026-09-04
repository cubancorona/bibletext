//go:build ios || android

package bibletext

// classifyLayout currently selects the shared compact layout for every touch
// device. buildRegularWidthUI is retained only as an inactive wide-layout
// implementation and must not receive feature work while it is unreachable.
// layoutWatcher remains active and moves shared navigation between bar and rail
// when the resolved mobile policy changes.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// buildRegularWidthUI is the former tablet (regular-width) layout: a navigation sidebar
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
	readingHost := container.NewStack(compactReadingPane(state))
	state.showReading = func() {
		readingHost.Objects = []fyne.CanvasObject{compactReadingPane(state)}
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

// layoutWatcher wraps the shared mobile root so rotation rebuilds when the
// navigation placement changes. That covers tablets on both mobile platforms
// and Android phones, whose landscape rail preserves reading height. The
// retained layout-class comparison also makes any future deliberate classifier
// change rebuild safely. It adds no chrome; only Resize acts.
type layoutWatcher struct {
	widget.BaseWidget
	state   *AppState
	content fyne.CanvasObject
	built   renderedLayout
	pending bool
}

func newLayoutWatcher(state *AppState, content fyne.CanvasObject) *layoutWatcher {
	w := &layoutWatcher{
		state:   state,
		content: content,
		built:   state.renderedLayout(state.canvasWidth()),
	}
	w.ExtendBaseWidget(w)
	return w
}

func (w *layoutWatcher) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(w.content)
}

// Resize re-evaluates the layout against the new size. If the class or the
// resolved bar/rail placement changes, a rebuild is scheduled on the UI
// thread — not run inline, since we're mid-layout. pending coalesces the burst of
// Resize calls during a live divider/rotation into a single rebuild; the watcher
// is recreated by that rebuild, which resets the guard.
//
// Orientation comes from the window canvas rather than this content size. The
// soft keyboard reduces content height and can otherwise make a portrait layout
// look landscape. Using the same canvas source as newLayoutWatcher prevents
// alternating rebuild decisions; layout-class selection still keys off width.
func (w *layoutWatcher) Resize(size fyne.Size) {
	// Compare the rendered decision itself instead of reconstructing it from
	// orientation and device class. That keeps the watcher coupled to the policy
	// used by buildCompactUI and avoids a rotation appearing only after some
	// unrelated later rebuild.
	want := w.state.renderedLayout(size.Width)
	changed := !w.pending && layoutWatcherNeedsRebuild(w.built, want)
	if changed && want.landscape != w.built.landscape {
		// The phone-landscape presentation is about to flip on this rotation:
		// read the reader's place NOW, before BaseWidget.Resize lets the old
		// host push the new-width frame. The capture is synchronous on the
		// main queue and the frame push is asynchronous, so ordering here is
		// ordering there; captured after the rewrap, the anchor would name the
		// verse the rewrap scrolled to, not the one the reader was on.
		captureRotationAnchor(w.state)
	}
	w.BaseWidget.Resize(size)
	if changed {
		w.pending = true
		fyne.Do(func() { rebuildWindow(w.state) })
	}
}
