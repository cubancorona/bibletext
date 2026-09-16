package bibletext

// Receiving a link on the desktops that have no application delegate.
//
// Windows and Linux hand a launched URL to the program the way they hand it
// anything: on the command line. The Store build's manifest asks for that
// (desktop2:Parameters="%1" on the web-to-app handler and "%1" on the
// bibletext: scheme — msstore/AppxManifest.xml.in), and a Linux desktop
// entry's Exec=… %u does the same. So the third arrival path, after the
// Apple delegates and the Android intent, is os.Args: read once at the top of
// Run, before the window exists, and delivered through the very same
// HandleShareLink the other paths use, so a cold start from a link parks
// until the Bible has loaded and opens with the one startup rebuild.
//
// Two things differ from the delegate paths, and both are deliberate:
//   - an ACTIVATED desktop app has no OS fallback. On iOS a declined link
//     makes the system offer the browser; on Windows the app has already been
//     launched, so a URL that is ours but not a passage (a book index matched
//     by the handler's /web/* claim) is opened in the browser HERE, by the
//     loop-safe opener — invariant I2 in docs/NKJV_FLOW.md.
//   - a link the app itself just handed to the browser can come straight back
//     (the Store build's web-to-app handler catches its own ShellExecute); the
//     echo guard in share_link_echo.go drops that within a few seconds.

import (
	"net/url"
	"strings"
)

// bibleTextScheme is the custom scheme the site's "Open in BibleText" button
// and the Store manifest register. Its payload is the https link with only
// the scheme swapped, so the grammar stays ParseShareLink's.
const bibleTextScheme = "bibletext://"

// startupShareLink returns the first command-line argument that is a URL on
// our host: https://, http:// or bibletext://, any capitalisation, with any
// quotes a shell left around it stripped. Position-independent so a future
// flag never breaks it, and deliberately NOT ParseShareLink's looser grammar
// (www., a bare host): only a scheme-qualified URL can have come from the OS.
func startupShareLink(args []string) (string, bool) {
	if len(args) < 2 {
		return "", false
	}
	for _, a := range args[1:] {
		a = strings.Trim(strings.TrimSpace(a), `"'`)
		if u, ok := normaliseSiteURL(a); ok {
			return u, true
		}
	}
	return "", false
}

// maxStartupLinkLen bounds what the intake will carry: longer than any link
// the app mints by an order of magnitude, shorter than the handoff's line.
const maxStartupLinkLen = 8 << 10

// normaliseSiteURL swaps bibletext:// for https:// (ParseShareLink strips
// only http(s):// and //) and reports whether the URL is on our host at all —
// the host check the single-instance listener also relies on, so a forwarded
// string can never be anything but one of our own URLs. Parsed, not
// prefix-matched: bibletext.co.uk.evil.com is not our host, and userinfo
// (https://bibletext.co.uk@evil/…) is refused outright; an explicit default
// port and a trailing dot on the host are the same host.
func normaliseSiteURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxStartupLinkLen {
		return "", false
	}
	if n := len(bibleTextScheme); len(raw) >= n && strings.EqualFold(raw[:n], bibleTextScheme) {
		raw = "https://" + raw[n:]
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil {
		return "", false
	}
	switch u.Scheme {
	case "https", "http":
	default:
		return "", false
	}
	host := strings.TrimPrefix(strings.TrimSuffix(strings.ToLower(u.Hostname()), "."), "www.")
	if host != shareLinkHost {
		return "", false
	}
	switch u.Port() {
	case "", "443", "80":
	default:
		return "", false
	}
	// A spelled-out default port or a trailing dot names the same host, but
	// ParseShareLink matches the host as a prefix: canonicalise those two.
	if canonical := strings.TrimSuffix(u.Hostname(), "."); u.Host != canonical {
		u.Host = canonical
		return u.String(), true
	}
	return raw, true
}

// deliverStartupLink hands a startup link to HandleShareLink. Called after
// CreateMainUI and before StartBackgroundLoad, on the main goroutine, so the
// link parks (loadPhase is still loadPending) and consumePendingLink opens it
// ahead of the startup rebuild — the macOS cold-start shape exactly. A URL
// that is ours but not a passage goes to the browser here (I2: no OS fallback
// after activation); an echo of one the app just opened there is dropped.
func deliverStartupLink(state *AppState, raw string) bool {
	if state == nil || raw == "" || browserEcho.isEcho(raw) {
		return false
	}
	if HandleShareLink(state, raw) {
		return true
	}
	openLinkInBrowser(raw)
	return false
}
