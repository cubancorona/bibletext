//go:build ios || android

package bibletext

// The mobile half of startupWorkArea.
//
// app.go carries no build tag, so Run compiles for iOS and Android even though
// neither entry point calls it -- cmd/mobile builds its own window (and notes
// there that Resize is a no-op, because the OS owns the window size). GLFW does
// not exist on either platform, so the desktop probe cannot be compiled there
// and this stands in for it.
//
// Returning a zero size is the honest answer and the right one: there is no
// work area to measure, and startupWindowSize reads a zero size as "no answer"
// and asks for the preferred size, which the OS then ignores.

import "fyne.io/fyne/v2"

func startupWorkArea(fyne.App) fyne.Size { return fyne.Size{} }
