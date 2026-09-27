//go:build !windows

package bibletext

import "fyne.io/fyne/v2"

// syncNativeTitleBar has nothing to do off Windows: macOS re-lights its title
// bar with the system appearance, Linux's frame belongs to the window manager,
// and the phones have none (syncTitleBar, appearance.go).
func syncNativeTitleBar(fyne.Window, fyne.ThemeVariant) {}
