package bibletext

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// Opening a link, and every link the app shows, in one place.
//
// WHY THIS EXISTS RATHER THAN widget.NewHyperlink(label, u). Fyne's desktop
// macOS OpenURL shells out to /usr/bin/open and blocks the calling goroutine
// until that process exits. Two problems follow. Spawning an executable from
// outside the bundle is the kind of thing the App Sandbox refuses, so a
// sandboxed build — the only kind the Mac App Store accepts — risks every
// link silently doing nothing; and Fyne's Hyperlink discards the error, so
// there is no log, no dialog, and no way for a reader to tell a dead link
// from a slow one.
//
// externalLink keeps the Hyperlink's appearance and focus behaviour exactly —
// it IS a Hyperlink — and only takes over what happens on tap, which
// Hyperlink.invokeAction lets us do by preferring OnTapped over its own
// openURL. openExternalURL then routes to the platform's supported API
// (NSWorkspace on macOS, external_link_darwin.go) instead of a subprocess.
//
// Use this for every link the app opens. A bare widget.NewHyperlink with a
// URL bypasses all of the above; external_link_test.go fails the build if one
// reappears.

// externalLink is a Hyperlink that opens through openExternalURL.
//
// A nil url yields a plain label-styled Hyperlink with no action, matching
// what NewHyperlink does with a nil target.
func externalLink(label string, u *url.URL) *widget.Hyperlink {
	hl := widget.NewHyperlink(label, nil)
	if u == nil {
		return hl
	}
	hl.OnTapped = func() { openExternalURL(u) }
	return hl
}

// externalOpener is the seam the platform implementation is reached through.
// It is a variable rather than a direct call for one reason: a test that
// invokes the real one on macOS launches a browser or a mail composer on the
// machine running it. A test asserting where a tap is ROUTED has no business
// opening anything, so it substitutes this instead.
var externalOpener = openExternalURLPlatform

// openExternalURL hands a URL to the platform. The error is reported here
// rather than discarded, because a link that quietly fails is indistinguishable
// from an app that has hung.
func openExternalURL(u *url.URL) {
	if u == nil {
		return
	}
	if err := externalOpener(u); err != nil {
		fyne.LogError("could not open "+u.Scheme+" link", err)
	}
}

// openExternalURLDefault is the portable implementation: Fyne's own OpenURL.
// Platforms with a supported native API override it in their own file.
func openExternalURLDefault(u *url.URL) error {
	app := fyne.CurrentApp()
	if app == nil {
		return nil // no running app (tests): nothing to open, nothing to report
	}
	return app.OpenURL(u)
}
