//go:build darwin || android

package bibletext

// iOS, macOS and Android: the reading pane is a native overlay, and its builder
// assigns the real state.showReadingOverlay closure (reading_ios.go,
// reading_macos.go, reading_android.go) — each of which already ends on
// consumeDeferredFullRebuild. installSheetCloseConsume must stand aside, or a
// generic closure installed first could be the one a search-time build leaves
// standing, and the native un-suppress would never run.
const sheetConsumeClosureOnPlatform = false
