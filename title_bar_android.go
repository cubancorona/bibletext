//go:build android

package bibletext

import "fyne.io/fyne/v2"

// syncNativeTitleBar puts Android's window chrome into the variant the
// content was built in. The window has no title bar; its chrome is the status
// bar and the navigation bar, whose icons nothing else sets. Their background
// is the canvas's paper, and the activity's theme is the platform's dark one,
// so a window starts with white icons whatever the page is (chromeAtStart):
// over the light page the clock and battery stood at about 1.2:1. This asks
// for dark icons over the light page and light ones over the dark
// (systemBarsLight, BtBridge.setSystemBarsLight); the bridge keeps the answer
// and gives it to a recreated activity's new window as well, which no rebuild
// here would resend, since the variant has not moved.
func syncNativeTitleBar(_ fyne.Window, v fyne.ThemeVariant) {
	setAndroidSystemBarsLight(systemBarsLight(v))
}
