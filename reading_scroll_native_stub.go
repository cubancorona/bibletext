//go:build darwin || android

package bibletext

// Stubs for the Fyne-pane scroll plumbing on the native-overlay platforms.
// reading.go (untagged) calls applyFyneReadingRestore from readingColumn.Layout,
// and reading_fyne.go (ios || !darwin) calls wireFyneReadingScroll — but on
// macOS/iOS/Android the native text overlays own scroll capture/restore
// (reading_macos.go / reading_ios.go / reading_android.go), and the Fyne pane
// is either absent (macOS) or an inert fallback beneath the overlay (mobile).
// The real implementations live in reading_scroll_fyne.go (Linux/Windows).

import "fyne.io/fyne/v2/container"

func wireFyneReadingScroll(state *AppState, scroll *container.Scroll, chapter *chapterText) {}

func applyFyneReadingRestore(l *readingColumn) {}
