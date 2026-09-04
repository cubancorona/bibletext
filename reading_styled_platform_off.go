//go:build ios || darwin || android

package bibletext

// Native-overlay platforms: the styled pane never dispatches. macOS keeps its
// NSTextView overlay (reading_macos.go owns readingScrollArea there), and on
// iOS and Android the Fyne readingScrollArea is dead code — the mobile layouts
// never call it, and Android's bridge-absent fallback is reading_mobile.go's
// RichText pane — so the constant is false to keep that unused path
// byte-identical.
const styledPaneEnabledOnPlatform = false
