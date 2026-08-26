//go:build !ios && !android

package bibletext

// openLinkInBrowser on the desktops: hand the URL to the system browser via
// Fyne. Desktop never has a link handed to it by the OS, but it can now be
// handed one by the READER — a bibletext.co.uk link pasted into Search opens
// in-app (executeSearch → HandleShareLink) — and the notes-off offer's "Read
// it in the browser" has to actually go somewhere on every platform that can
// show the offer.

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
