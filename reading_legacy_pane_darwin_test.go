//go:build darwin && !ios && !race

package bibletext

// The darwin twin of legacyPaneForTest: chapterTextScrollArea lives in
// reading_fyne.go ("ios || !darwin"), which macOS never compiles — and the
// tests that call this helper skip on macOS (skipIfNativeReadingOverlay)
// before reaching it, so it only needs to satisfy the compiler here.

import "fyne.io/fyne/v2"

func legacyPaneForTest(state *AppState) fyne.CanvasObject {
	return buildReadingView(state)
}
