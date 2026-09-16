//go:build !ios && !android && !windows

package bibletext

// openLinkInBrowser on macOS and Linux: hand the URL to the system browser
// via Fyne. The desktops do receive links from the OS now — the Mac App Store
// build through its Universal Link delegate (share_link_macos.go), Windows
// and Linux through the command line and the single-instance handoff
// (share_link_argv.go, single_instance.go) — and a reader can still paste one
// into Search (executeSearch → HandleShareLink). This is where the notes-off
// offer's "Read it in the browser" and a declined startup link go. No echo is
// possible here (neither OS routes an https link back to the app), so the
// echo guard is not armed; Windows has its own opener
// (share_link_browser_windows.go) because its Store build would intercept
// the toolkit's ShellExecute route.

import (
	"net/url"

	"fyne.io/fyne/v2"
)

func openLinkInBrowser(rawURL string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return
	}
	if app := fyne.CurrentApp(); app != nil {
		openExternalURL(u)
	}
}
