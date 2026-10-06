package bibletext

// The licensed epoch bridge (loadLicensedBridge) is the next major release's
// (docs/NEXT.md). There, after a cacheEpoch bump, a licensed translation's
// previous epoch goes on serving a reader whose fetch of the new one fails,
// inside that copy's own recency window and only while no current epoch
// loads; a launch that can fetch replaces it and deletes it, and the startup
// sweep keeps nothing else. The shipping build has no bridge: it never serves
// a licensed superseded epoch and the startup sweep deletes every one. See the
// licensed rows of the transitions in docs/VERSION_STATES.md.
//
// This file holds the device the tests launch and what holds in both states.
// What only the next release does is in licensed_epoch_bridge_next_test.go,
// and what the shipping build does instead in
// licensed_epoch_bridge_current_test.go; each asks the same questions of its
// own state.
//
// Every launch here is the app's own: the startup sweep, loadStateData and
// the hand-off to the live state, in StartBackgroundLoad's order, against a
// local API.Bible fixture the test takes up and down, with the clock held
// still so a window's edge is exact. The text is synthetic throughout.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
)

// bridgeDay is the clock the bridge tests hold still.
var bridgeDay = time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)

// withClock holds currentUTCTime at at for the test and returns a function
// that moves it on.
func withClock(t *testing.T, at time.Time) func(time.Duration) {
	t.Helper()
	prev := currentUTCTime
	now := at
	currentUTCTime = func() time.Time { return now }
	t.Cleanup(func() { currentUTCTime = prev })
	return func(d time.Duration) { now = now.Add(d) }
}

// bridgeDevice is a reader's device on the day a release moves the NKJV's
// cacheEpoch: the default translation's current edition on disk, the NKJV
// licensed with the fixture's key, a saved reading position, and a provider
// that is up or down for each launch.
type bridgeDevice struct {
	t        *testing.T
	nk       BibleVersion
	current  string // the NKJV's current-epoch cache path
	previous string // its newest superseded epoch: the copy the reader holds
	advance  func(time.Duration)
	up, down string // the fixture's URL, and one nothing answers
}

func newBridgeDevice(t *testing.T, saved string) *bridgeDevice {
	t.Helper()
	app := test.NewApp()
	t.Cleanup(app.Quit)
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(t.TempDir(), cacheFileName))
	setNKJVLicence(t)
	withFakeSharedKeys(t)
	advance := withClock(t, bridgeDay)

	nk, ok := versionByID("nkjv")
	if !ok {
		t.Skip("nkjv not registered")
	}
	if !isLicensedSource(nk) || !nk.source.available() {
		t.Fatal("control: the NKJV must be licensed and available, or nothing here is the licensed path")
	}
	prev := supersededCachePaths(nk)
	if len(prev) == 0 {
		t.Fatal("control: the NKJV has no superseded epoch, so there is nothing to bridge")
	}
	mustCache(t, cachePathForVersion(defaultVersionID), stampedBible("default"))

	srv := apiBibleFixture(t)
	t.Cleanup(srv.Close)
	gone := httptest.NewServer(http.NotFoundHandler())
	down := gone.URL
	gone.Close()
	prevURL := apiBibleBaseURL
	t.Cleanup(func() { apiBibleBaseURL = prevURL })

	writeReadingState(appPrefs(), readingState{Version: saved, Book: "John", Chapter: 1})
	return &bridgeDevice{t: t, nk: nk, current: cachePathForVersion(nk.ID), previous: prev[0],
		advance: advance, up: srv.URL, down: down}
}

// holdPrevious writes the copy the reader holds from before the release,
// saved age before the held clock.
func (d *bridgeDevice) holdPrevious(age time.Duration) []byte {
	d.t.Helper()
	at := currentUTCTime().Add(-age)
	if err := saveBibleToCache(d.previous, stampedBible("previous"), func() time.Time { return at }); err != nil {
		d.t.Fatal(err)
	}
	b, err := os.ReadFile(d.previous)
	if err != nil {
		d.t.Fatal(err)
	}
	return b
}

// launch runs the startup the app runs on its load goroutine, with the
// provider up or down, and returns the live state the reader sees.
func (d *bridgeDevice) launch(online bool) *AppState {
	d.t.Helper()
	apiBibleBaseURL = d.down
	if online {
		apiBibleBaseURL = d.up
	}
	sweepLicensedCachesAtLaunch()
	loaded, err := loadStateData()
	if err != nil {
		d.t.Fatalf("the launch did not open: %v", err)
	}
	live := NewLoadingState()
	adoptLaunch(live, loaded)
	return live
}

// launchWithoutSweep opens a session on the saved translation as the launch
// does but without the startup sweep, so a copy that crossed its window since
// the last sweep is still on disk — the state a long-running session is in.
func (d *bridgeDevice) launchWithoutSweep() *AppState {
	d.t.Helper()
	apiBibleBaseURL = d.down
	loaded, err := loadStateData()
	if err != nil {
		d.t.Fatalf("the launch did not open: %v", err)
	}
	live := NewLoadingState()
	adoptLaunch(live, loaded)
	return live
}

func onDisk(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// inStepLoads makes every interactive translation load the real one, in step,
// against whichever provider the device has up, and counts the loads of id.
func inStepLoads(t *testing.T, id string) *int {
	t.Helper()
	n := 0
	prev := startVersionLoad
	startVersionLoad = func(v BibleVersion, base *BibleData, land func(*BibleData, dataMode, error)) {
		if v.ID == id {
			n++
		}
		land(loadVersionData(v, base))
	}
	t.Cleanup(func() { startVersionLoad = prev })
	return &n
}

// Past its own window by a second, the copy is never served: the sweep
// removes it, and the launch falls back to the default translation with the
// NKJV remembered. The same in both states.
func TestAnOfflineLaunchDoesNotServeALicensedCopyPastItsWindow(t *testing.T) {
	d := newBridgeDevice(t, "nkjv")
	d.holdPrevious(licensedRecencyWindow + time.Second)

	live := d.launch(false)

	if live.CurrentVersion != defaultVersionID || bibleStamp(live.Bible) != "default" {
		t.Fatalf("a licensed copy past its window was served: on %s, text %q", live.CurrentVersion, bibleStamp(live.Bible))
	}
	if live.preferredVersion != d.nk.ID {
		t.Fatalf("the fallback did not remember the NKJV (remembers %q)", live.preferredVersion)
	}
	if n := fullPendingNotice(live); !strings.Contains(n, d.nk.Name+" could not be opened this time") {
		t.Fatalf("the substitution is not said; footer %q", n)
	}
	if onDisk(d.previous) {
		t.Fatal("the startup sweep kept a licensed superseded copy past its window")
	}
}

// Online at the first launch, the edition is fetched and saved, and the copy
// the reader held is deleted: a superseded epoch is never served when the
// fetch succeeds.
func TestAnOnlineFirstLaunchReplacesTheLicensedCopy(t *testing.T) {
	d := newBridgeDevice(t, "nkjv")
	d.holdPrevious(10 * 24 * time.Hour)

	live := d.launch(true)

	assertBridgeReplaced(t, d, live)
}

func assertBridgeReplaced(t *testing.T, d *bridgeDevice, live *AppState) {
	t.Helper()
	if live.CurrentVersion != d.nk.ID {
		t.Fatalf("online, the reader was shown %s instead of the NKJV", live.CurrentVersion)
	}
	saved, err := loadBibleFromCache(d.current)
	if err != nil {
		t.Fatalf("the fetched edition was not saved at the current epoch: %v", err)
	}
	if got := bibleStamp(live.Bible); got == "previous" || got != bibleStamp(saved) {
		t.Fatalf("the NKJV on screen is %q, want the fetched edition %q", got, bibleStamp(saved))
	}
	if onDisk(d.previous) {
		t.Fatal("the current epoch is saved and the superseded copy is still on disk")
	}
	if live.staleVersions[d.nk.ID] || live.preferredVersion != "" || fullPendingNotice(live) != "" {
		t.Fatalf("the current edition is on screen and the app still says otherwise: stale %v, remembered %q, footer %q",
			live.staleVersions[d.nk.ID], live.preferredVersion, fullPendingNotice(live))
	}
}

// A licensed previous edition is never served beside a current epoch that
// loads. The current one serves while it is inside its window; past it, the
// load deletes it and asks the provider, and when that fails the fallback
// finds no copy it may serve. Here the stale current file could not be
// removed, so it still loads — in the next release the bridge's own first
// condition is what refuses the previous copy, which is inside its window.
func TestALicensedPreviousEditionIsNeverServedBesideACurrentOne(t *testing.T) {
	d := newBridgeDevice(t, "nkjv")
	d.holdPrevious(24 * time.Hour)

	t.Run("a current epoch inside its window", func(t *testing.T) {
		mustCache(t, d.current, stampedBible("current"))
		live := d.launch(false)
		if live.CurrentVersion != d.nk.ID || bibleStamp(live.Bible) != "current" {
			t.Fatalf("on %s, text %q, want the current epoch", live.CurrentVersion, bibleStamp(live.Bible))
		}
		if onDisk(d.previous) {
			t.Fatal("the superseded copy outlived the launch beside a current epoch that loads")
		}
	})

	t.Run("a current epoch past its window that is still on disk", func(t *testing.T) {
		d.holdPrevious(24 * time.Hour)
		stale := currentUTCTime().Add(-licensedRecencyWindow - time.Hour)
		if err := saveBibleToCache(d.current, stampedBible("current"), func() time.Time { return stale }); err != nil {
			t.Fatal(err)
		}
		if !versionCacheIsCurrent(d.nk) || !licensedCacheStale(d.current) {
			t.Fatal("control: the current epoch must load and be past its window")
		}
		data, _, err := loadVersionFallback(d.nk)
		if err == nil {
			t.Fatalf("the fallback served %q beside a current epoch that loads", bibleStamp(data))
		}
		if _, err := loadLicensedBridge(d.nk); err == nil {
			t.Fatal("the bridge served a superseded copy beside a current epoch that loads")
		}
	})
}

// Public-domain epochs are untouched: a previous epoch far older than any
// licensed window is kept by the sweep and served by both cache reads.
func TestThePublicDomainPreviousEpochHasNoRecencyWindow(t *testing.T) {
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(t.TempDir(), cacheFileName))
	withFakeSharedKeys(t)
	withClock(t, bridgeDay)
	bsb, ok := versionByID("bsb")
	if !ok || isLicensedSource(bsb) {
		t.Skip("bsb not registered as a public-domain translation")
	}
	old := supersededCachePaths(bsb)[0]
	at := bridgeDay.Add(-400 * 24 * time.Hour)
	if err := saveBibleToCache(old, stampedBible("previous"), func() time.Time { return at }); err != nil {
		t.Fatal(err)
	}
	if licensedCopyInsideWindow(old) {
		t.Fatal("control: the copy must be far past the licensed window, or this proves nothing")
	}

	sweepLicensedCachesAtLaunch()
	if !onDisk(old) {
		t.Fatal("the licensed sweep removed a public-domain previous epoch")
	}
	for name, read := range map[string]func(BibleVersion) (*BibleData, dataMode, error){
		"cache-only": loadVersionFromCacheOnly,
		"fallback":   loadVersionFallback,
	} {
		if data, _, err := read(bsb); err != nil || bibleStamp(data) != "previous" {
			t.Errorf("the %s read did not serve the public-domain previous epoch (err %v)", name, err)
		}
	}
}

// The window's edge, from the copy's own stamp, with the clock held: inside
// at exactly the window, outside a nanosecond past it, and outside when the
// stamp cannot be read at all.
func TestALicensedCopysWindowIsMeasuredFromItsOwnStamp(t *testing.T) {
	withClock(t, bridgeDay)
	dir := t.TempDir()
	at := func(name string, age time.Duration) string {
		p := filepath.Join(dir, name)
		if err := saveBibleToCache(p, fullValidBible(), func() time.Time { return bridgeDay.Add(-age) }); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if !licensedCopyInsideWindow(at("edge.json", licensedRecencyWindow)) {
		t.Error("a copy exactly licensedRecencyWindow old is inside its window")
	}
	if licensedCopyInsideWindow(at("past.json", licensedRecencyWindow+time.Nanosecond)) {
		t.Error("a copy a nanosecond past its window is inside it")
	}
	stampless := filepath.Join(dir, "stampless.json")
	if err := os.WriteFile(stampless, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{stampless, filepath.Join(dir, "missing.json")} {
		if licensedCopyInsideWindow(p) {
			t.Errorf("%s: a copy whose age cannot be read is inside its window", filepath.Base(p))
		}
	}
}
