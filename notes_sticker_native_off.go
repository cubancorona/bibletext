//go:build !darwin

package bibletext

// Windows, Linux and Android: the Fyne banner above the pane (notes_banner.go)
// is how a note is visible in NORMAL reading, so this stays false. Android
// additionally draws a native in-text sticker in FULL-SCREEN reading only
// (the implementation requirement: pushNoteToOverlay in reading_android.go gates on IsFullScreen,
// where the banner is never built) — flipping this constant there would kill
// the banner the compact layout still depends on. See the `on` twin.
const nativeNoteStickerOnPlatform = false
