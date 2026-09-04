package bibletext

// Mobile layout classification. Every touch device now uses one shared layout:
// Read / Books / Search sits at the bottom in portrait and moves to a leading
// rail on tablets and Android phones in landscape. The old compact/regular enum
// and sizing helpers remain as an explicit record of the former split layout.

type layoutClass int

const (
	// layoutCompact is the shared touch layout. Its navigation is a bottom bar
	// except where the platform's landscape policy selects a leading rail.
	layoutCompact layoutClass = iota
	// layoutRegular is the retained former tablet sidebar + HSplit layout. The
	// current classifier never selects it.
	layoutRegular
)

// androidTabletMinDim is the sw600dp-style tablet threshold used by
// device_android.go: a window whose SMALLEST dimension is at least this many
// logical units is tablet-class. 600 is Android's own convention for when
// tablet resources kick in.
const androidTabletMinDim float32 = 600

// isTabletDimensions applies the smallest-width tablet test. Pure so the
// host test suite covers it even though its only caller is Android-tagged.
func isTabletDimensions(w, h float32) bool {
	m := w
	if h < m {
		m = h
	}
	return m >= androidTabletMinDim
}

// tabletLayoutMinWidth is the former breakpoint for the retained regular
// sidebar+split layout. It is kept for regression tests and documentation; the
// current shared layout does not switch class at this width.
//
// Chosen so an iPad mini in portrait (744pt) clears the bar while a typical
// half-width multitasking column on an 11" iPad (~397pt portrait, ~507pt
// landscape) stays compact.
const tabletLayoutMinWidth float32 = 700

// All touch devices use the shared mobile layout. Persistent Read / Books /
// Search navigation prevents a results surface from stranding the reading
// surface, while readableColumn constrains list width on larger displays. The
// retained tablet parameters keep a future wide-layout change explicit and
// testable without affecting the current classifier.
func classifyLayout(width float32, isTablet bool) layoutClass {
	return layoutCompact
}

// layoutClass reports the shared touch layout. It still passes the live canvas
// width and idiom through the pure classifier so a future deliberate layout
// change has one tested decision point.
func (s *AppState) layoutClass() layoutClass {
	return classifyLayout(s.canvasWidth(), deviceIsTablet())
}

// canvasWidth is the current canvas width in logical points, or 0 before the
// window/canvas exists.
func (s *AppState) canvasWidth() float32 {
	if s != nil && s.window != nil {
		return s.window.Canvas().Size().Width
	}
	return 0
}

// canvasIsLandscape reports whether the canvas is wider than it is tall. Mobile
// navigation uses the same live canvas geometry to choose between bottom bar
// and leading rail; this helper remains the orientation source for the retained
// former sidebar. Before sizing it reports true for that sidebar's initial
// default.
func (s *AppState) canvasIsLandscape() bool {
	if s == nil || s.window == nil {
		return true
	}
	sz := s.window.Canvas().Size()
	if sz.Width <= 0 || sz.Height <= 0 {
		return true
	}
	return sz.Width >= sz.Height
}

// mobileRailWanted is the shared mobile navigation-placement rule with the
// platform policy and live canvas geometry stated explicitly. Tablets use a
// rail in landscape on both mobile platforms. Android phones do too because a
// bottom bar can consume the remaining reading height on a short landscape
// window; iPhone keeps its existing bottom-bar convention. That is where
// navigation is drawn at all: a phone's Read tab reads full-screen in
// landscape by default (readingFullScreen, phone_landscape.go). An unsized
// canvas keeps only the tablet's established initial default, avoiding a rail
// flash on Android before its first real dimensions arrive.
func mobileRailWanted(tablet, phoneLandscapeRail bool, w, h float32) bool {
	if w <= 0 || h <= 0 {
		return tablet
	}
	return w >= h && (tablet || phoneLandscapeRail)
}

// renderedLayout is the layout as the watcher compares it: the class, whether
// the navigation is drawn as a rail, and whether the phone-landscape
// presentation is in force. rail is false whenever the presented mode is
// full-screen — that tree draws no navigation, so a rotation must not rebuild
// it for a rail it would not show (an iPad in chosen full-screen keeps its
// reading position exactly because nothing rebuilds on rotation). landscape is
// its own term so a phone that had chosen full-screen still rebuilds when the
// presentation flips: that rebuild is what re-reads the typography gate and
// pushes the measure.
type renderedLayout struct {
	class     layoutClass
	rail      bool
	landscape bool
}

func (s *AppState) renderedLayout(width float32) renderedLayout {
	return renderedLayout{
		class:     classifyLayout(width, deviceIsTablet()),
		rail:      !s.readingFullScreen() && compactNavRail(s),
		landscape: phoneLandscapeReading(),
	}
}

func layoutWatcherNeedsRebuild(built, want renderedLayout) bool {
	return built != want
}

// resolveSidebarDefault retains the orientation-driven default for the former
// regular layout: shown in landscape, collapsed in portrait. It is applied only
// the first time the regular layout is built and thereafter whenever the
// orientation flips — so a rotation re-asserts the default, while an explicit
// toggle of state.sidebarCollapsed (same orientation) is preserved. Returns the
// resolved collapsed state and records the orientation it was resolved for.
func (s *AppState) resolveSidebarDefault() bool {
	landscape := s.canvasIsLandscape()
	if !s.sidebarInit || landscape != s.sidebarLandscape {
		s.sidebarCollapsed = !landscape // portrait → collapsed by default
		s.sidebarLandscape = landscape
		s.sidebarInit = true
	}
	return s.sidebarCollapsed
}

// Retained sidebar sizing for the former regular layout. It aims for a fixed
// rather than a fixed fraction, so the navigation panel stays a comfortable,
// consistent size whether it's an iPad mini or a 13" iPad in landscape — a fixed
// fraction would make the sidebar balloon on the big canvases.
const (
	regularSidebarTargetPt float32 = 250
	regularSidebarMinFrac  float64 = 0.18
	regularSidebarMaxFrac  float64 = 0.30
)

// regularSplitOffset returns the former HSplit offset (sidebar fraction) that
// yields ~regularSidebarTargetPt of sidebar, clamped so it is never a sliver on a
// huge canvas nor a crushing majority on a small one. width<=0 (canvas not sized
// yet) falls back to the max fraction, matching the small-canvas default.
func regularSplitOffset(width float32) float64 {
	if width <= 0 {
		return regularSidebarMaxFrac
	}
	frac := float64(regularSidebarTargetPt / width)
	if frac < regularSidebarMinFrac {
		return regularSidebarMinFrac
	}
	if frac > regularSidebarMaxFrac {
		return regularSidebarMaxFrac
	}
	return frac
}

// --- What the mobile shell shows, as pure state ------------------------------
//
// These live here rather than in ui_mobile.go so they can be TESTED. That file
// is tagged `ios || android`, packaging does not compile tests, and the bug
// leaveSearchForRead fixes shipped twice. A test in that tagged file would not
// run in the default host test suite. The styled reading pane is untagged for
// the same reason.

// overlayShouldShow is the single source of truth for native reading-overlay
// visibility on mobile: the iOS UITextView must be visible exactly when the
// reading view is the content actually on screen. Every place that toggles the
// overlay derives the answer from here, and afterRebuild re-asserts it as the
// last word after each window rebuild, so a stray async show/hide during the
// rebuild can't leave the overlay floating over the wrong content as a blank
// (black) rectangle.
//
//   - Full-screen (distraction-free) reading: always show.
//   - Retained former regular layout: show unless results occupy its reading pane.
//   - Current shared layout: only Read hosts the pane, and only with no search.
func overlayShouldShow(state *AppState) bool {
	if state.readingFullScreen() {
		return true
	}
	if state.layoutClass() == layoutRegular {
		return !state.IsSearching
	}
	return state.CurrentTab == 0 && !state.IsSearching
}

// leaveSearchForRead turns the "showing results" flag off when the reader picks
// the Read tab.
//
// THE BUG THIS FIXES, which shipped: search, get results, then tap Read. The tab
// changed and IsSearching did not, so rebuildMobileReadingPane went on returning
// the RESULTS view while overlayShouldShow — CurrentTab == 0 && !IsSearching —
// went false and took the native reading overlay down with it. The reader landed
// on a Read tab holding the search results, with no reading view and without the
// search field and buttons that at least made the results look intentional. The
// only way out was to tap a result. Present in 1.1.6 and 1.1.7: the tab bar,
// rebuildMobileReadingPane and overlayShouldShow are byte-identical there, and
// nothing on the tab-switch path has ever cleared the flag.
//
// Clearing it here is safe because the Search tab does not read it: its keyword
// branch renders buildSearchResultsView unconditionally from state.SearchResults,
// which survive. So the results are still there when the reader goes back, and
// CanReturnToSearchResults — the reading view's own way back — is deliberately
// left alone.
//
// IsSearching means "results are occupying the reading pane", and on the phone
// that can only be true while the reader is looking at the pane it occupies.
func leaveSearchForRead(state *AppState, tab int) {
	if state == nil || tab != 0 {
		return
	}
	state.IsSearching = false
}
