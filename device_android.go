//go:build android

package bibletext

import "fyne.io/fyne/v2"

// deviceIsTablet (Android): Android has no UIKit-style idiom, so we use the
// platform's own convention — a device whose smallest window dimension is at
// least ~600dp is a tablet (the classic sw600dp resource qualifier; Fyne's
// logical units track dp on Android). Computed from the live window canvas on
// every call, so it is correct after rotation and in split-screen: an app
// squeezed to phone-ish width behaves as a phone, exactly like the iPad's
// narrow Split View fallback.
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

// layoutMayChange: always watch on Android — before the first layout the
// canvas reports 0×0 (not a tablet), so the watcher must be armed regardless
// to catch the real size when it arrives. On phones it stays inert (the
// layout class never changes), costing one comparison per resize.
func layoutMayChange() bool { return true }
