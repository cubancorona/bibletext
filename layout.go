package bibletext

// Responsive layout classification. The mobile binary (iOS/Android) runs on both
// phones and tablets; which chrome it shows is decided at runtime from the live
// canvas width, not a build tag. An iPad at full width (or a wide multitasking
// split) gets the regular layout — a persistent sidebar beside the reading pane,
// like the desktop — while a phone, or an iPad squeezed into a narrow Slide
// Over / Split View column, gets the compact bottom-tab layout.
//
// classifyLayout is pure so it can be unit-tested on the host; the AppState
// method feeds it the real canvas width and device idiom.

type layoutClass int

const (
	// layoutCompact is the phone layout: full-screen tabs (Read / Books / Search)
	// across the bottom. Used on all phones and on tablets too narrow for a split.
	layoutCompact layoutClass = iota
	// layoutRegular is the tablet layout: a persistent navigation sidebar beside
	// the reading pane (an HSplit), with the app header on top.
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

// tabletLayoutMinWidth is the canvas width (in logical points) at or above which
// a tablet switches to the regular sidebar+split layout. Below it, even on an
// iPad (e.g. a 1/2 or 1/3 multitasking column), the sidebar would crowd the
// reading column, so the compact layout is used instead.
//
// Chosen so an iPad mini in portrait (744pt) clears the bar while a typical
// half-width multitasking column on an 11" iPad (~397pt portrait, ~507pt
// landscape) stays compact.
const tabletLayoutMinWidth float32 = 700

// classifyLayout decides the layout class from the current canvas width and
// whether the device is a tablet. Phones are always compact. A tablet is regular
// when it has the width for it; the width<=0 case (before the first layout pass,
// when the canvas has no size yet) trusts the tablet idiom so an iPad's first
// real frame is already the regular layout.
// ONE LAYOUT ON EVERY TOUCH DEVICE. The tablet used to get a layout of its own —
// a persistent sidebar beside the reading pane — and that second layout is where
// the iPad's problems lived: a mode row that appeared to govern the sidebar while
// governing the far pane, and results that replaced the reading pane with no way
// back, because the tab bar the phone has was not there to return to.
//
// Both were symptoms of maintaining two shapes for one app. The phone's shape
// already answers both — the tab bar is always present, so "back to reading" is
// a tab, and no sidebar means nothing can imply it governs something it does
// not. So the iPad takes it too, and what the iPad needs beyond it is not a
// different layout but a READABLE MEASURE on the surfaces that would otherwise
// stretch (readableColumn, used by the books, results and notes lists — the
// reading pane already has the reporter measure).
//
// The reason that matters for the years after this is uniformity: the app
// prioritizes compatibility with the other platforms wherever that makes sense,
// so a feature does not have to be reworked once per platform. Every future
// feature now lands on ONE mobile layout.
//
// The tablet parameters stay declared and tested: this decision is recorded as a
// choice, not lost by deleting the concept, and a return to a wide layout is a
// change here rather than an archaeology exercise.
func classifyLayout(width float32, isTablet bool) layoutClass {
	return layoutCompact
}

// layoutClass reports the layout to use right now, from the live canvas width and
// the device idiom. Safe before the window/canvas exists (treated as unknown
// width → the idiom decides).
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

// canvasIsLandscape reports whether the canvas is wider than it is tall. Before
// the canvas is sized it reports true (landscape) so the sidebar defaults to
// shown rather than flashing collapsed. See resolveSidebarDefault.
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

// resolveSidebarDefault applies the orientation-driven sidebar default for the
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

// Sidebar sizing for the regular layout. We aim for a fixed ~logical-point width
// rather than a fixed fraction, so the navigation panel stays a comfortable,
// consistent size whether it's an iPad mini or a 13" iPad in landscape — a fixed
// fraction would make the sidebar balloon on the big canvases.
const (
	regularSidebarTargetPt float32 = 250
	regularSidebarMinFrac  float64 = 0.18
	regularSidebarMaxFrac  float64 = 0.30
)

// regularSplitOffset returns the HSplit offset (sidebar fraction of width) that
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
//   - Regular (tablet) layout: the reading pane is always beside the sidebar, so
//     show whenever a search's results aren't occupying it.
//   - Compact layout: only the Read tab hosts the reading pane, and only when no
//     search is active.
func overlayShouldShow(state *AppState) bool {
	if state.IsFullScreen {
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
