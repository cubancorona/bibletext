//go:build !darwin && !android

package bibletext

// Windows and Linux: the styled reading pane draws no sticker of its own, so
// the Fyne banner above the pane (notes_banner.go) is how a note is visible
// there. Android LEFT this set on 19 Aug — it now draws the native in-text
// sticker in both of its reading modes, for parity with iOS (see the `on`
// twin). Whether the styled pane should grow a true in-text band+bubble of its
// own is recorded as future work in [redacted-retired-private-reference].
const nativeNoteStickerOnPlatform = false
