//go:build !darwin

package bibletext

// Windows, Linux and Android: no in-text sticker, so the Fyne banner above the
// pane (notes_banner.go) is how a note is visible there. See the `on` twin.
const nativeNoteStickerOnPlatform = false
