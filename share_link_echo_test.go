package bibletext

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
)

// A URL the app just handed to the browser can come straight back through
// the Store build's web-to-app handler. Within the window it is an echo, in
// either spelling; after it, or for another URL, it is a real arrival.
func TestABrowserHandoffEchoIsNotReopened(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	g := &browserEchoGuard{now: func() time.Time { return now }}
	link := "https://bibletext.co.uk/web/john/3/#v16"
	if g.isEcho(link) {
		t.Fatal("an echo before any handoff")
	}
	g.note(link)
	now = now.Add(time.Second)
	if !g.isEcho(link) {
		t.Error("the handed-off URL is not recognised a second later")
	}
	if !g.isEcho("bibletext://bibletext.co.uk/web/john/3/#v16") {
		t.Error("the scheme spelling of the same URL is not recognised")
	}
	if g.isEcho("https://bibletext.co.uk/web/john/4/") {
		t.Error("a different URL counted as an echo")
	}
	now = now.Add(browserEchoWindow)
	if g.isEcho(link) {
		t.Error("an echo after the window")
	}

	// And the arrival paths honour it: an echoed URL neither parks nor opens.
	app := test.NewApp()
	defer app.Quit()
	prev := browserEcho
	browserEcho = g
	defer func() { browserEcho = prev }()
	now = now.Add(time.Second)
	g.note(link)
	st := NewLoadingState()
	if deliverStartupLink(st, link) || st.pendingLink != nil {
		t.Error("an echoed startup link was taken")
	}
}
