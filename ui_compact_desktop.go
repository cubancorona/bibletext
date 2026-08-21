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
type desktopNavPreview int

const (
	desktopNavSidebar desktopNavPreview = iota // shipped: sidebar + HSplit
	desktopNavBar                              // preview: shared compact layout, bottom bar
	desktopNavRail                             // preview: shared compact layout, left rail
)

// desktopNav is the ONLY reader of BIBLETEXT_DESKTOP_TABS.
//
//	1 | bar   the shared compact layout with the phone/iPad bottom bar
//	rail      the shared compact layout with the vertical rail
//	anything else (including unset) leaves the shipped sidebar alone
func desktopNav() desktopNavPreview {
	switch os.Getenv("BIBLETEXT_DESKTOP_TABS") {
	case "1", "bar":
		return desktopNavBar
	case "rail":
		return desktopNavRail
	}
	return desktopNavSidebar
}

// announceDesktopNav prints the chosen preview once per run, and only when the
// variable is set — a shipped run says nothing.
//
// It exists because "the app shows the old sidebar" was indistinguishable from
// "the app shows the rail" from outside the process: the window cannot be
// screenshotted from here, and the render galleries bypass the gate. A line on
// stderr is the cheapest way for a preview to be checkable rather than claimed.
func announceDesktopNav() {
	name := map[desktopNavPreview]string{
		desktopNavBar:  "compact layout, bottom bar",
		desktopNavRail: "compact layout, LEFT RAIL",
	}[desktopNav()]
	if name == "" {
		return // sidebar: the shipped layout, nothing to announce
	}
	fmt.Fprintf(os.Stderr, "BibleText: desktop navigation preview — %s "+
		"(BIBLETEXT_DESKTOP_TABS=%q)\n", name, os.Getenv("BIBLETEXT_DESKTOP_TABS"))
}

// compactNavRail reports whether the navigation draws as a left rail. Desktop
// only — a rail is a pointer-and-window convention, not a touch one, so the
// phones and the iPad never ask (their answer is the constant in ui_mobile.go).
func compactNavRail(*AppState) bool { return desktopNav() == desktopNavRail }
