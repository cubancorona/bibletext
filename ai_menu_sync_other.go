//go:build !darwin && !android

package bibletext

// syncNativeAIMenu is a no-op on platforms whose reading pane is the plain Fyne
// widget (Linux / Windows): there is no native selection menu to gate — the Fyne
// fallback pane's menu has no AI items. The darwin (macOS + iOS) and android
// twins push the Settings → Assistant choice into their native menu gates.
func syncNativeAIMenu(state *AppState) {}
