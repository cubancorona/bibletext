package bibletext

import (
	"errors"
	"net/url"
	"strings"
)

// errNoDirectBrowser is the answer where no direct browser launch exists:
// every platform but Windows, whose opener installs the real one at init.
var errNoDirectBrowser = errors.New("no direct browser launch on this platform")

// directBrowserLaunch starts the default browser's own command line. A seam,
// so tests assert the routing without starting a browser on the machine —
// the Windows CI runner included.
var directBrowserLaunch = func(cmdline, exe string) error { return errNoDirectBrowser }

// openInBrowserWithFallback is the Windows opener's wiring, kept platform-free
// so it is tested everywhere: the browser's own command first, and only when
// that is unavailable the toolkit's ShellExecute route — the one route the
// Store build's web-to-app handler can catch and hand straight back, which
// is why the echo guard is armed here and nowhere else. Both spellings the
// OS may see (the raw URL and the toolkit's re-encoding) are noted.
func openInBrowserWithFallback(rawURL string, command func(string) (cmdline, exe string, ok bool)) {
	if cmdline, exe, ok := command(rawURL); ok {
		if err := directBrowserLaunch(cmdline, exe); err == nil {
			return
		}
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return
	}
	browserEcho.note(rawURL, u.String())
	openExternalURL(u)
}

// browserCommandLine turns a shell "open" command template — what Windows
// records for the default browser, e.g.
//
//	"C:\Program Files\Microsoft\Edge\Application\msedge.exe" --single-argument %1
//	"C:\Program Files\Google\Chrome\Application\chrome.exe" -- "%1"
//	"C:\Program Files\Mozilla Firefox\firefox.exe" -osint -url "%1"
//
// — into the exact command line to start and the executable it names. The
// URL replaces the first %1 (or is appended, quoted, when the template has
// none); the executable is the first token, quoted or not. Pure, so the
// string handling is tested on every platform; only the query that produces
// the template is Windows-only (share_link_browser_windows.go).
func browserCommandLine(template, rawURL string) (cmdline, exe string, ok bool) {
	template = strings.TrimSpace(template)
	if template == "" || rawURL == "" || strings.ContainsAny(rawURL, "\"\r\n") {
		return "", "", false
	}
	if strings.HasPrefix(template, `"`) {
		end := strings.Index(template[1:], `"`)
		if end < 0 {
			return "", "", false
		}
		exe = template[1 : 1+end]
	} else if i := strings.IndexByte(template, ' '); i >= 0 {
		exe = template[:i]
	} else {
		exe = template
	}
	if exe == "" {
		return "", "", false
	}
	if strings.Contains(template, "%1") {
		cmdline = strings.Replace(template, "%1", rawURL, 1)
	} else {
		cmdline = template + ` "` + rawURL + `"`
	}
	return cmdline, exe, true
}
