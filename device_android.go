//go:build android

package bibletext

import "fyne.io/fyne/v2"

// deviceIsTablet (Android): Android has no UIKit-style idiom, so we use the
// platform's own convention — a device whose smallest window dimension is at
// least ~600dp is a tablet (the classic sw600dp resource qualifier; Fyne's
// logical units track dp on Android). Computed from the live window canvas on
// every call, so it is correct after rotation and in split-screen. Tablet
// identity controls the shared layout's landscape rail and readable measures.
func deviceIsTablet() bool {
	app := fyne.CurrentApp()
	if app == nil {
		return false
	}
	wins := app.Driver().AllWindows()
	if len(wins) == 0 {
		return false
	}
	sz := wins[0].Canvas().Size()
	return isTabletDimensions(sz.Width, sz.Height)
}

// phoneLandscapeNavRail moves phone navigation to the leading edge in
// landscape. The fixed-height app, chapter, history and bottom-navigation
// chrome can otherwise leave no height for the reading host on a short window.
// Portrait keeps the usual bottom bar.
func phoneLandscapeNavRail() bool { return true }

// layoutMayChange: always watch on Android — before the first layout the
// canvas reports 0×0, so the watcher must catch the real size. It also owns the
// phone bar/rail transition on rotation.
func layoutMayChange() bool { return true }
