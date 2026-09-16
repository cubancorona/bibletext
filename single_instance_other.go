//go:build !windows

package bibletext

import "fyne.io/fyne/v2"

// Everywhere but Windows: no package identity to detect, no foreground grant
// to give, and RequestFocus alone brings the window forward.

func packagedWindows() bool { return false }

func allowSetForeground(int) {}

func restoreNative(fyne.Window) {}
