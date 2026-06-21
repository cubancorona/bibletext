//go:build !ios && !darwin

package bibletext

// Scroll capture/restore relies on per-verse glyph geometry, which the app reads
// from the native text overlays on iOS and macOS. On the Fyne fallback platforms
// (Linux, Windows, Android) the translation, book, chapter and recent-chapters
// history still persist and restore (see reading_state.go); only the precise
// within-chapter scroll position is a no-op for now — a container.Scroll-fraction
// restore is a worthwhile future addition.

// captureReadingAnchor reports "no anchor available" so flushReadingState saves
// the position at the chapter top.
func captureReadingAnchor() (verse int, delta, frac float64, ok bool) {
	return 0, 0, 0, false
}

// armReadingRestore is a no-op: there is no native scroll target to arm.
func armReadingRestore(verse int, delta, frac float64) {}

// captureLastTouch / armReadingMarker back the initial-touch ("where I left off")
// feature, which is iOS-only (it needs a touch gesture and native verse geometry).
// On the Fyne fallback platforms there is nothing to record or draw.
func captureLastTouch() (verse int, delta float64, ok bool) { return 0, 0, false }

func armReadingMarker(verse int, r, g, b float64) {}
