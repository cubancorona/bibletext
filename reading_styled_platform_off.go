//go:build ios || darwin || android

package bibletext

// Native-overlay platforms (and the Android Fyne fallback): the styled pane
// never dispatches — readingScrollArea's chapterText path stays byte-identical
// where it is still reachable (iOS dead code, Android bridge-absent fallback),
// and macOS keeps its NSTextView overlay.
const styledPaneEnabledOnPlatform = false
