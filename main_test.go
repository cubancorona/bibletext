package bibletext

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
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

// NOR DOES ANY TEST READ OR WRITE THE MACHINE'S TRANSLATION CACHE.
//
// Every translation cache path the app uses hangs off defaultCachePath, which
// is BIBLETEXT_CACHE_PATH when set and the user's cache directory otherwise.
// So the whole binary gets a directory of its own, made here, and a test that
// needs a particular disk builds it in a directory of its own — t.Setenv the
// variable to a t.TempDir, then mustCache — as the version harnesses do, so no
// test leaves a cache behind for the next.
//
// Before this, sixteen tests that switch translation read whatever the machine
// running them had downloaded: applyLoadedVersion asks versionCacheIsCurrent,
// which decodes the whole 7 MB file. On a machine that had opened the BSB and
// the Catholic WEB, the arrivals journeys decoded one 849 times — ten minutes
// under the race detector, most of the race run — and walked a different disk
// from CI's, which has none. The paths that delete and write the cache
// (purgeSupersededCaches, saveBibleToCache, the dev tab's Clear) hang off the
// same function, and only convention kept a test from reaching them.
//
// A test that reads real downloaded text on purpose, and says so, reaches the
// machine's directory through realCachePath: read-only, and never by default.
var realCacheDir string

// realCachePath is the machine's copy of the file the app keeps at p — for the
// opt-in tests that check real downloaded text. BIBLETEXT_CACHE_PATH set when
// the suite starts names the machine's cache for them (a simulator's, say).
func realCachePath(p string) string { return filepath.Join(realCacheDir, filepath.Base(p)) }

func TestMain(m *testing.M) {
	externalOpener = func(u *url.URL) error {
		openedMu.Lock()
		opened = append(opened, u.String())
		openedMu.Unlock()
		return nil
	}
	realCacheDir = filepath.Dir(defaultCachePath())
	suiteCache, err := os.MkdirTemp("", "bibletext-test-cache-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "no directory for the suite's translation cache:", err)
		os.Exit(2)
	}
	os.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(suiteCache, cacheFileName))
	code := m.Run()
	os.RemoveAll(suiteCache)
	if left := openedInTests(); len(left) > 0 && os.Getenv("BT_SHOW_OPENS") != "" {
		fmt.Fprintf(os.Stderr, "tests would have opened %d URL(s); first: %s\n", len(left), left[0])
	}
	os.Exit(code)
}

// The suite's translation cache is its own: every path the app reads, writes
// or deletes a translation through resolves inside the directory TestMain
// made, and none in the machine's. The control clears the redirect and shows
// the same check would fire — it resolves paths and reads nothing.
func TestTheSuiteNeverTouchesTheMachinesTranslationCache(t *testing.T) {
	suite := filepath.Dir(defaultCachePath())
	if suite == realCacheDir {
		t.Fatalf("the suite's translation cache is the machine's: %s", suite)
	}
	check := func() []string {
		var stray []string
		paths := []string{defaultCachePath(), crossRefCachePath()}
		for _, v := range bibleVersions() {
			paths = append(paths, cachePathForVersion(v.ID))
			paths = append(paths, supersededCachePaths(v)...)
		}
		for _, p := range paths {
			if filepath.Dir(p) != suite {
				stray = append(stray, p)
			}
		}
		return stray
	}
	if stray := check(); len(stray) > 0 {
		t.Fatalf("%d cache paths lie outside the suite's directory %s, first %s", len(stray), suite, stray[0])
	}
	t.Setenv("BIBLETEXT_CACHE_PATH", "")
	if len(check()) == 0 {
		t.Fatal("control: with the redirect cleared every path still lies in the suite's directory, so the check above proves nothing")
	}
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
