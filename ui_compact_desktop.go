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

// compactReadingPane is the desktop's reading view for the compact layout: the
// ordinary desktop pane (NSTextView on macOS, the styled pane on
// Windows/Linux), with search results taking its place while a search is live —
// exactly as the mobile twin does.
func compactReadingPane(state *AppState) fyne.CanvasObject {
	if state.IsSearching {
		return buildSearchResultsView(state)
	}
	return buildReadingPane(state)
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
	desktopNavSidebar desktopNavStyle = iota // opt-out: the former sidebar + HSplit
	desktopNavBar                            // comparison: compact layout, bottom bar
	desktopNavRail                           // SHIPPED: compact layout, left rail
)

// desktopNav is the ONLY reader of BIBLETEXT_DESKTOP_TABS.
//
// The left rail IS the desktop layout now, so the variable is an override
// rather than a way in, and an unset or unrecognised value means "no override":
//
//	sidebar | 0   the former sidebar + HSplit, for comparison or escape
//	1 | bar       the compact layout with the phone/iPad bottom bar
//	rail          the rail, stated explicitly (same as unset)
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
	}
	return desktopNavRail
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
		return // the shipped rail, chosen by default: nothing to report
	}
	name := map[desktopNavStyle]string{
		desktopNavSidebar: "the former sidebar + split",
		desktopNavBar:     "compact layout, bottom bar",
		desktopNavRail:    "compact layout, LEFT RAIL",
	}[desktopNav()]
	fmt.Fprintf(os.Stderr, "BibleText: desktop navigation override — %s "+
		"(BIBLETEXT_DESKTOP_TABS=%q)\n", name, set)
}

// compactNavRail reports whether the navigation draws as a left rail. Desktop
// only — a rail is a pointer-and-window convention, not a touch one, so the
// phones and the iPad never ask (their answer is the constant in ui_mobile.go).
func compactNavRail(*AppState) bool { return desktopNav() == desktopNavRail }
