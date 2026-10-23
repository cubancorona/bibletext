//go:build android

package bibletext

import "fyne.io/fyne/v2"

// deviceIsTablet (Android): Android has no UIKit-style idiom, so we use the
// platform's own convention — a device whose smallest window dimension is at
// least ~600dp is a tablet (the classic sw600dp resource qualifier; Fyne's
// logical units track dp on Android). Computed from the live window canvas on
// every call, so it is correct after rotation and in split-screen. Tablet
// identity keeps the landscape presentation to phones (phone_landscape.go) and
// sets the readable measures; the navigation's place is the window's shape on
// phones and tablets alike (mobileRailWanted).
//
// Before the canvas has a size, the activity's configured size in dp stands in
// for it (layoutSize), so a tablet's first build is a tablet's: without it
// every window read as a phone then, and a tablet opened sideways would take
// the phone's full-screen presentation for its first frames. The canvas's
// unit is a dp only approximately, because Fyne rounds the screen's density
// to a few scales (at 420 dpi a unit is 8/7 of a dp), so a window whose short
// side falls between 600 dp and 600 units, or an upright one whose width falls
// between 560 dp and 560 units (the bar's dress, tabBarStyleFor), is answered
// one way before its size arrives and the other way after.
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
	return isTabletDimensions(layoutSize(sz.Width, sz.Height))
}

// launchWindowSize is the activity window's configured size in dp, read over
// the bridge (androidWindowWidthDp, androidWindowHeightDp), which the
// configuration has from the activity's creation and follows rotation and
// multi-window. The layout decides by it until the canvas has a size
// (layoutSize, launch_window.go), so a phone opened while held sideways lays
// out sideways from its first frame. Calls into Java, made only while the
// canvas has no size; none without the bridge.
func launchWindowSize() (w, h float32, ok bool) {
	w, h = float32(androidWindowWidthDp()), float32(androidWindowHeightDp())
	return w, h, w > 0 && h > 0
}

// Android phones read distraction-free in landscape too (phone_landscape.go),
// and on the book page when the pane is wide enough for it, as every surface
// does (reporter_android.go): the dialect's paragraph grammar in
// android_chapter_html.go, the measure centred by the bridge. No Go-side
// anchor is captured on rotation: a rotation recreates the activity, and the
// recovery (foregroundOverlayRecovery, reading_android.go) restores the place
// from the reading position the bridge kept across the recreation
// (BtBridge.lastAnchor: the verse at the top, how far into it, and the scroll
// fraction as the fallback) and forces the re-import; a same-activity width
// change places the verse captured before the frame changed (BtBridge
// pendingReflowVerse, the fraction as the fallback). A Go restore would only
// duplicate that.
func phoneLandscapeReadingSupported() bool { return true }

func rotationRestoreNeeded() bool { return false }
