//go:build !(darwin && !ios)

package bibletext

import "net/url"

// Everywhere except desktop macOS, Fyne's own OpenURL is already the supported
// path: Windows uses rundll32, Linux xdg-open, iOS UIApplication and Android an
// Intent — none of which is the sandboxed-subprocess problem macOS has.
func openExternalURLPlatform(u *url.URL) error { return openExternalURLDefault(u) }
