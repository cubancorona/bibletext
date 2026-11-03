//go:build !ios && !android

package bibletext

import (
	"fmt"
	"os"

	"fyne.io/fyne/v2"
)

// THE DESKTOP HALF OF THE SHARED COMPACT LAYOUT.
//
// Three seams, and nothing else, separate the desktop from the layout the
// phones and the iPad run. That is the point: if the desktop ever adopts the
// tab chrome, it adopts the SAME chrome — same tab bar, same books grid, same
// search tab, same readable measures — rather than a fourth thing to keep in
// step by hand.

// compactReadingView is the desktop's reading view for the compact layout: the
// plain chapter view (NSTextView on macOS, the styled pane on Windows/Linux),
// never the search results. compactReadingPane (ui_compact.go) puts the
// results in its place while a search is live, and the full-screen branch
// relies on this being the plain view — exactly as the mobile twin is.
// buildReadingPane, which substitutes on its own and toggles the overlay,
// stays with the former sidebar layout (ui_desktop.go); here the shared
// layout's own notifyReadingOverlay calls decide the overlay.
func compactReadingView(state *AppState) fyne.CanvasObject {
	return buildReadingView(state)
}

// notifyReadingOverlay keeps a native reading overlay in step with the tab
// selection. macOS has a real NSTextView to hide; Windows and Linux draw the
// pane inside the canvas and their setReadingOverlayVisible is already a no-op,
// so this is one call for all three.
func notifyReadingOverlay(visible bool) {
	setReadingOverlayVisible(visible)
}

// dismissKeyboard is a no-op off mobile — there is no soft keyboard to drop, and
// unfocusing here would steal focus from a field the user is still typing in.
func dismissKeyboard(*AppState) {}

// THE DESKTOP NAVIGATION PREVIEW, PARSED IN ONE PLACE.
//
// One function reads the variable and both questions derive from it. They were
// two functions reading it independently — one asking `== "1"` to decide whether
// the shared compact layout runs at all, the other asking `== "rail"` to decide
// how it draws its navigation — and `rail` answered the second while failing the
// first, so it fell straight through to the shipped sidebar. The app showed the
// old sidebar while every rendered gallery showed the rail, because the gallery
// test calls buildCompactUI directly and never crosses the gate.
//
// That is the shape of the bug worth not repeating: not a wrong string, but a
// value that two readers of the same variable disagreed about.
type desktopNavStyle int

const (
	desktopNavSidebar  desktopNavStyle = iota // opt-out: the former sidebar + HSplit
	desktopNavBar                             // comparison: compact layout, bottom bar
	desktopNavRail                            // comparison: compact layout, left rail
	desktopNavAdaptive                        // shared layout, navigation follows the window
)

// desktopNav is the ONLY reader of BIBLETEXT_DESKTOP_TABS.
//
// An unset or unrecognised value follows the window's shape, using the same
// bar/rail rule as phones and tablets. Explicit values remain overrides:
//
//	sidebar | 0   the former sidebar + HSplit, for comparison or escape
//	1 | bar       the compact layout with the phone/iPad bottom bar
//	rail          the compact layout with the left rail in every window shape
//
// Both readers of this decision — whether the compact layout runs at all, and
// how it draws its navigation — must keep agreeing. They once disagreed, and
// "rail" satisfied the second while failing the first, so the app showed the
// sidebar while every rendered gallery showed the rail. desktop_nav_preview_test.go
// pins the coupling.
func desktopNav() desktopNavStyle {
	switch os.Getenv("BIBLETEXT_DESKTOP_TABS") {
	case "sidebar", "0":
		return desktopNavSidebar
	case "1", "bar":
		return desktopNavBar
	case "rail":
		return desktopNavRail
	}
	return desktopNavAdaptive
}

// announceDesktopNav names the layout on stderr, and only when the variable is
// explicitly set — a default run says nothing.
//
// It exists because the layout in force is otherwise unobservable from outside
// the process: the window cannot be screenshotted from here, and the render
// galleries call buildCompactUI directly, bypassing the gate. A line on stderr
// is the cheapest way for an override to be checkable rather than claimed.
func announceDesktopNav() {
	set := os.Getenv("BIBLETEXT_DESKTOP_TABS")
	if set == "" {
		return
	}
	name := map[desktopNavStyle]string{
		desktopNavSidebar:  "the former sidebar + split",
		desktopNavBar:      "compact layout, bottom bar",
		desktopNavRail:     "compact layout, LEFT RAIL",
		desktopNavAdaptive: "compact layout, navigation follows the window",
	}[desktopNav()]
	fmt.Fprintf(os.Stderr, "BibleText: desktop navigation override — %s "+
		"(BIBLETEXT_DESKTOP_TABS=%q)\n", name, set)
}

// compactNavRail reports whether the navigation draws as a left rail. The
// desktop uses the same shape rule as phones and tablets unless an override
// pins an edge. An unsized desktop keeps its initial rail until a real size
// arrives; touch devices retain their own launch-window/device fallback.
func compactNavRail(state *AppState) bool {
	switch desktopNav() {
	case desktopNavRail:
		return true
	case desktopNavBar, desktopNavSidebar:
		return false
	}
	if state == nil || state.window == nil {
		return true
	}
	size := state.window.Canvas().Size()
	if !canvasSized(size.Width, size.Height) {
		return true
	}
	return railForWindow(state)
}
