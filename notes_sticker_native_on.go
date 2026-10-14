//go:build darwin || android

package bibletext

// Platforms whose reading pane draws the note ITSELF — in the text, anchored to
// the verse it is about, scrolling with the passage: the iOS UITextView bubble
// (reading_ios.go), its macOS NSTextView twin (reading_macos.go), and the
// Android TextView sticker (BtBridge.setNote), which serves BOTH
// of that platform's reading modes.
//
// Where this is true the Fyne banner must stay out of the way, or the reader
// sees the same note twice. Windows and Linux have no native sticker (the
// `off` twin), but their styled pane draws its own band, card and tail, and
// the banner stands down for it there too.
const nativeNoteStickerOnPlatform = true
