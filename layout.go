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
func classifyLayout(width float32, isTablet bool) layoutClass {
	if !isTablet {
		return layoutCompact
	}
	if width <= 0 || width >= tabletLayoutMinWidth {
		return layoutRegular
	}
	return layoutCompact
}

// layoutClass reports the layout to use right now, from the live canvas width and
// the device idiom. Safe before the window/canvas exists (treated as unknown
// width → the idiom decides).
func (s *AppState) layoutClass() layoutClass {
	var width float32
	if s != nil && s.window != nil {
		width = s.window.Canvas().Size().Width
	}
	return classifyLayout(width, deviceIsTablet())
}
