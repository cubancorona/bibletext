//go:build darwin

package bibletext

// Platforms whose reading pane draws the note ITSELF — in the text, anchored to
// the verse it is about, scrolling with the passage: the iOS UITextView bubble
// (reading_ios.go) and its macOS NSTextView twin (reading_macos.go). The
// `darwin` build tag is set for BOTH, which is exactly the set that has one.
//
// Where this is true the Fyne banner must stay out of the way, or the reader
// sees the same note twice. Android's overlay has no sticker yet and still
// shows the banner (task #19); Windows/Linux styled panes likewise.
const nativeNoteStickerOnPlatform = true
