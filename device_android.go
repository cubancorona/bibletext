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

// Android phones read distraction-free in landscape too (phone_landscape.go),
// and on the book page when the pane is wide enough for it, as every surface
// does (reporter_android.go): the dialect's paragraph grammar in
// android_chapter_html.go, the measure centred by the bridge. No Go-side
// anchor is captured on rotation: a rotation recreates the
// activity, the bridge's own
// recovery restores the place from its surviving scroll fraction and forces
// the re-import (foregroundOverlayRecovery, reading_android.go), and a
// same-activity width change re-places by fraction too (BtBridge
// pendingReflowFrac); a Go restore would only duplicate that.
func phoneLandscapeReadingSupported() bool { return true }

func rotationRestoreNeeded() bool { return false }
