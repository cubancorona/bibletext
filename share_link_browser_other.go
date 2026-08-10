//go:build !ios && !android

package bibletext

// openLinkInBrowser is a no-op off the platforms that receive shared links.
// macOS, Windows and Linux never have one handed to them, so there is nothing
// to hand back.
func openLinkInBrowser(rawURL string) {}
