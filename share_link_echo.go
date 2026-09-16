package bibletext

import (
	"sync"
	"time"
)

// browserEchoGuard remembers the last URL the app handed to the browser, for
// a few seconds. On the Windows Store build the web-to-app handler catches
// every https://bibletext.co.uk link launched through the shell — including
// the one the app itself just launched for "Read it in the browser" when the
// direct browser command (share_link_browser_windows.go) is unavailable. That
// launch starts a second BibleText, which forwards the URL to this one; the
// guard recognises it as an echo and drops it, so the loop ends after one
// hop instead of never. Every arrival path on the desktops (argv and the
// listener) asks it first.
type browserEchoGuard struct {
	mu     sync.Mutex
	recent []echoEntry
	now    func() time.Time
}

type echoEntry struct {
	url string
	at  time.Time
}

// browserEchoWindow is how long a handed-off URL counts as an echo. Long
// enough for the shell, the handler and a cold second process; short enough
// that a reader who really does open the same link again a minute later is
// served. It is armed only on the one route that can echo — the toolkit's
// ShellExecute fallback on Windows (share_link_browser_command.go) — so a
// deliberate "Open in BibleText" right after "Read it in the browser" is
// only ever dropped when that fallback was just taken.
const browserEchoWindow = 10 * time.Second

// browserEchoKeep is how many handed-off spellings are remembered: the raw
// URL and the toolkit's re-encoding of it are noted together.
const browserEchoKeep = 4

var browserEcho = &browserEchoGuard{now: time.Now}

func (g *browserEchoGuard) key(raw string) string {
	if u, ok := normaliseSiteURL(raw); ok {
		return u
	}
	return raw
}

// note records that each of raws is about to be handed to the browser.
func (g *browserEchoGuard) note(raws ...string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	at := g.now()
	for _, raw := range raws {
		g.recent = append(g.recent, echoEntry{url: g.key(raw), at: at})
	}
	if n := len(g.recent); n > browserEchoKeep {
		g.recent = g.recent[n-browserEchoKeep:]
	}
}

// isEcho reports whether raw is a URL just handed to the browser, in either
// spelling (https or bibletext scheme), within the window.
func (g *browserEchoGuard) isEcho(raw string) bool {
	k := g.key(raw)
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	for _, e := range g.recent {
		if e.url == k && now.Sub(e.at) < browserEchoWindow {
			return true
		}
	}
	return false
}
