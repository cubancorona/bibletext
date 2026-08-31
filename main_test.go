package bibletext

import (
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
)

// NO TEST IN THIS PACKAGE OPENS ANYTHING FOR REAL.
//
// The opener is substituted for the WHOLE test binary here, not per test. A
// per-test rule was tried and is not enough: it only binds tests that call the
// opener directly, and the one that actually opened browsers reached it three
// calls deep. TestNoteLinkIsDeclinedWhenNotesAreOff hands a note-bearing link
// to HandleShareLink with notes off and no window, and offerNoteLinkChoice's
// no-window fallback is — correctly, for a reader on a cold start — to hand the
// link to the browser. Every run of the suite opened a real tab, and on desktop
// macOS that is -[NSWorkspace openURL:], which Fyne's test app cannot stub.
//
// Nothing in CI shows it. It is visible only as tabs piling up on the machine
// of whoever ran the tests, which is how it survived.
//
// A test that wants to assert routing substitutes its own opener over this one
// and restores it; that still works, and openedInTests records what would have
// been opened for any test that wants to check.
var (
	openedMu sync.Mutex
	opened   []string
)

// openedInTests returns, and clears, the URLs the suite would have opened.
func openedInTests() []string {
	openedMu.Lock()
	defer openedMu.Unlock()
	out := opened
	opened = nil
	return out
}

func TestMain(m *testing.M) {
	externalOpener = func(u *url.URL) error {
		openedMu.Lock()
		opened = append(opened, u.String())
		openedMu.Unlock()
		return nil
	}
	code := m.Run()
	if left := openedInTests(); len(left) > 0 && os.Getenv("BT_SHOW_OPENS") != "" {
		fmt.Fprintf(os.Stderr, "tests would have opened %d URL(s); first: %s\n", len(left), left[0])
	}
	os.Exit(code)
}

// The no-window fallback really does hand the link to the browser, and the
// stub above is really what stops it reaching the machine.
//
// This is the mechanism that opened a real tab on every run of the suite:
// TestNoteLinkIsDeclinedWhenNotesAreOff hands a note-bearing link to
// HandleShareLink with notes off and no window, and offerNoteLinkChoice's
// no-window arm hands it onward — correct for a reader on a cold start, and
// invisible in CI, where nobody sees a window open.
func TestTheNoWindowOfferHandsTheLinkOnwardButNotToTheMachine(t *testing.T) {
	openedInTests() // drain anything an earlier test recorded

	st := psalm23State()
	st.window = nil
	raw := "https://bibletext.co.uk/bsb/psalms/23/#v1-4&n=" + EncodeNote("a note")
	target, ok := ParseShareLink(raw)
	if !ok {
		t.Fatal("fixture URL does not parse as a share link")
	}
	offerNoteLinkChoice(st, raw, target)

	got := openedInTests()
	if len(got) != 1 || got[0] != raw {
		t.Fatalf("the no-window fallback routed %v, want exactly the raw link — if this "+
			"is empty the fallback is gone; if it is a real open, the suite is opening "+
			"browser tabs again", got)
	}
}
