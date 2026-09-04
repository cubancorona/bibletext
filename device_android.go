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
// landscape, where navigation is drawn — Books, Search and the dev Links tab;
// the Read tab reads full-screen in landscape by default (phone_landscape.go).
// The fixed-height app, chapter, history and bottom-navigation chrome can
// otherwise leave no height for the reading host on a short window. Portrait
// keeps the usual bottom bar.
func phoneLandscapeNavRail() bool { return true }

// Android phones read distraction-free in landscape too, with the reporter
// page under it (phone_landscape.go, reporter_android.go): the dialect's
// paragraph grammar in android_chapter_html.go, the measure centred by the
// bridge. No Go-side anchor is captured on rotation: a rotation recreates the
// activity, the bridge's own
// recovery restores the place from its surviving scroll fraction and forces
// the re-import (foregroundOverlayRecovery, reading_android.go), and a
// same-activity width change re-places by fraction too (BtBridge
// pendingReflowFrac); a Go restore would only duplicate that.
func phoneLandscapeReadingSupported() bool { return true }

var phoneLandscapeTypographySupported = func() bool { return true }

func rotationRestoreNeeded() bool { return false }

// layoutMayChange: always watch on Android — before the first layout the
// canvas reports 0×0, so the watcher must catch the real size. It also owns the
// phone bar/rail transition on rotation.
func layoutMayChange() bool { return true }
