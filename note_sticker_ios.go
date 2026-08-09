//go:build ios

package bibletext

// iOS draws the note as a native sticker floating in a band the text reserves
// (reading_ios.go), so the fallback card must not also appear — it would hide
// the passage the note is about.
func notePaneHasSticker() bool { return true }
