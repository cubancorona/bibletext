//go:build darwin

package bibletext

// Platforms whose reading pane draws the note ITSELF — in the text, anchored to
// the verse it is about, scrolling with the passage: the iOS UITextView bubble
// (reading_ios.go) and its macOS NSTextView twin (reading_macos.go). The
// `darwin` build tag is set for BOTH, which is exactly the set that has one.
//
// Where this is true the Fyne banner must stay out of the way, or the reader
// sees the same note twice. Android stays false: its native sticker exists
// only in FULL-SCREEN reading (the implementation requirement, pushNoteToOverlay), where the banner
// is never built, while normal reading still needs the banner; Windows/Linux
// styled panes have no sticker at all.
const nativeNoteStickerOnPlatform = true
