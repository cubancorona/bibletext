//go:build !darwin && !android

package bibletext

// Read-along entry points for Windows/Linux: the audio controller's calls
// (readAlongHighlight from the oto engine's time observer, readAlongClear on
// stop/nav, readAlongFollowButton for the pill) land here and are forwarded to
// the styled pane's helpers (reading_styled_readalong.go) on the Fyne UI
// goroutine. The native platforms have their own implementations
// (reading_macos.go / reading_ios.go / readalong_android.go).
//
// When the styled pane isn't active — the chapterText fallback via the
// styledPaneEnabledOnPlatform revert constant, or no reading view up — the
// helpers no-op, which is exactly the old stub behaviour.

import "fyne.io/fyne/v2"

func readAlongHighlight(verse int, follow bool) {
	fyne.Do(func() { styledReadAlongApply(verse, follow) })
}

func readAlongClear() {
	fyne.Do(styledReadAlongClearTint)
}

func readAlongFollowButton(show bool) {
	fyne.Do(func() { styledReadAlongSetPill(show) })
}
