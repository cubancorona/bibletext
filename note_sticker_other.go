//go:build !ios

package bibletext

// Everywhere else the note is still shown as a dismissable card. It is a
// working surface, not the final one: macOS and Android each need their own
// native sticker (an NSView and an Android View respectively), and the desktop
// styled pane can reserve a band directly since it lays out its own lines.
// Until then a reader on those platforms sees the note rather than nothing.
func notePaneHasSticker() bool { return false }
