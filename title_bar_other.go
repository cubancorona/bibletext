//go:build !windows && !android

package bibletext

import "fyne.io/fyne/v2"

// syncNativeTitleBar has nothing to do on these platforms: macOS re-lights its
// title bar with the system appearance, Linux's frame belongs to the window
// manager, and the iPhone's and iPad's status bar follows the system
// appearance by itself, as the app does (syncTitleBar, appearance.go).
// Windows (title_bar_windows.go) and Android's system bars
// (title_bar_android.go) have to be told.
func syncNativeTitleBar(fyne.Window, fyne.ThemeVariant) {}
