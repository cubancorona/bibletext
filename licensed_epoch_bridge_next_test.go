//go:build next

package bibletext

// What the licensed epoch bridge does in the next major release
// (docs/NEXT.md). The device and what holds in both states are in
// licensed_epoch_bridge_test.go; licensed_epoch_bridge_current_test.go asks
// the same questions of the shipping build, which has no bridge.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// previousEditionSentence is the picker's existing sentence for a translation
// showing a previous edition (fullPendingNotice, D3): the bridge says nothing
// new.
func previousEditionSentence(v BibleVersion) string {
	return v.Name + " is showing a previous edition until the update can be downloaded."
}

// The reader whose first launch after the release is offline goes on reading
// the NKJV from the copy they hold — here exactly at the edge of its window,
// which is inside it — said as a previous edition in the picker's existing
// words, never written to, and never served by the fast path that runs
// before a fetch.
func TestAnOfflineFirstLaunchKeepsTheLicensedEditionInsideItsWindow(t *testing.T) {
	d := newBridgeDevice(t, "nkjv")
	held := d.holdPrevious(licensedRecencyWindow)

	live := d.launch(false)

	if live.CurrentVersion != d.nk.ID {
		t.Fatalf("offline, with a copy inside its window, the reader was shown %s instead of the NKJV (remembered %q)",
			live.CurrentVersion, live.preferredVersion)
	}
	if got := bibleStamp(live.Bible); got != "previous" {
		t.Fatalf("the NKJV on screen is %q, want the copy the reader held", got)
	}
	if live.preferredVersion != "" {
		t.Fatalf("the NKJV is on screen and the launch still remembers it as substituted (%q)", live.preferredVersion)
	}
	if !live.staleVersions[d.nk.ID] {
		t.Fatal("the NKJV's previous edition is on screen and nothing records it (D3)")
	}
	if n := fullPendingNotice(live); n != previousEditionSentence(d.nk) {
		t.Fatalf("the picker footer reads %q, want the existing previous-edition sentence %q", n, previousEditionSentence(d.nk))
	}
	for _, v := range owedUpgrades(live) {
		if v.ID == d.nk.ID {
			t.Fatal("the refresh owes the NKJV: the app would spend the provider's quota on its own initiative")
		}
	}
	if b, err := os.ReadFile(d.previous); err != nil || !bytes.Equal(b, held) {
		t.Fatalf("the held copy was changed or removed by the launch that served it (err %v)", err)
	}
	if onDisk(d.current) {
		t.Fatal("a launch that could not fetch wrote a current-epoch NKJV")
	}
	if _, _, err := loadVersionFromCacheOnly(d.nk); err == nil {
		t.Fatal("the startup fast path, which runs before any fetch, served a licensed superseded epoch")
	}
}

// Offline at the first launch the copy bridges; the next launch that can
// fetch replaces it and deletes it.
func TestAnOfflineLaunchThenAnOnlineOneReplacesTheLicensedCopy(t *testing.T) {
	d := newBridgeDevice(t, "nkjv")
	d.holdPrevious(10 * 24 * time.Hour)

	if first := d.launch(false); first.CurrentVersion != d.nk.ID || bibleStamp(first.Bible) != "previous" {
		t.Fatalf("control: the offline launch must bridge; on %s, text %q", first.CurrentVersion, bibleStamp(first.Bible))
	}
	d.advance(24 * time.Hour)
	live := d.launch(true)

	assertBridgeReplaced(t, d, live)
}

// The reader choosing the NKJV while offline, in a session that opened on
// another translation, is served the held copy through the same door — and
// once the clock has carried that copy past its window in the same session,
// with no sweep in between, it is not served.
func TestAReaderChoosingTheLicensedTranslationOfflineReadsTheCopyInsideItsWindow(t *testing.T) {
	d := newBridgeDevice(t, defaultVersionID)
	held := d.holdPrevious(licensedRecencyWindow - time.Hour)

	prevLoad := startVersionLoad
	startVersionLoad = func(v BibleVersion, base *BibleData, land func(*BibleData, dataMode, error)) {
		land(loadVersionData(v, base)) // the real load, in step, against the provider that is down
	}
	t.Cleanup(func() { startVersionLoad = prevLoad })

	live := d.launch(false)
	if live.CurrentVersion != defaultVersionID || !onDisk(d.previous) {
		t.Fatalf("control: the session must open on the default with the copy kept; on %s, copy kept %v",
			live.CurrentVersion, onDisk(d.previous))
	}

	switchVersionInteractive(live, d.nk.ID, byReader)
	if live.CurrentVersion != d.nk.ID || bibleStamp(live.Bible) != "previous" {
		t.Fatalf("the reader chose the NKJV offline and was shown %s, text %q", live.CurrentVersion, bibleStamp(live.Bible))
	}
	if !live.staleVersions[d.nk.ID] || fullPendingNotice(live) != previousEditionSentence(d.nk) {
		t.Fatalf("the previous edition is not said in the existing words: footer %q", fullPendingNotice(live))
	}
	if b, _ := os.ReadFile(d.previous); !bytes.Equal(b, held) {
		t.Fatal("serving the held copy changed it")
	}

	// Two hours on, the copy is an hour past its window. A session that
	// opened on the default and does not hold the NKJV in memory chooses it,
	// offline.
	d.advance(2 * time.Hour)
	writeReadingState(appPrefs(), readingState{Version: defaultVersionID, Book: "John", Chapter: 1})
	later := d.launchWithoutSweep()
	if later.CurrentVersion != defaultVersionID || !onDisk(d.previous) {
		t.Fatalf("control: the later session must open on the default with the copy still on disk; on %s, copy kept %v",
			later.CurrentVersion, onDisk(d.previous))
	}
	switchVersionInteractive(later, d.nk.ID, byReader)
	if later.CurrentVersion != defaultVersionID {
		t.Fatalf("a licensed copy past its window was served when the reader chose it: on %s, text %q",
			later.CurrentVersion, bibleStamp(later.Bible))
	}
}

// The startup sweep keeps a licensed superseded copy only while it can bridge:
// inside its own window, with no current epoch that loads.
func TestTheStartupSweepKeepsOnlyALicensedCopyThatCanBridge(t *testing.T) {
	day := 24 * time.Hour
	for _, tc := range []struct {
		name    string
		age     time.Duration // of the newest superseded copy
		current string        // "none", "loads", or "torn"
		older   bool          // an older superseded epoch, past its window, beside it
		gone    bool          // the licence is gone for good
		kept    bool
	}{
		{name: "inside its window, no current epoch", age: 10 * day, current: "none", kept: true},
		{name: "at the edge of its window, no current epoch", age: licensedRecencyWindow, current: "none", kept: true},
		{name: "past its window, no current epoch", age: licensedRecencyWindow + time.Second, current: "none"},
		{name: "inside its window, beside a current epoch that loads", age: 10 * day, current: "loads"},
		{name: "inside its window, beside a current file that does not load", age: 10 * day, current: "torn", kept: true},
		{name: "inside its window, beside an older epoch past its own", age: 10 * day, current: "none", older: true, kept: true},
		{name: "inside its window, the licence gone for good", age: 10 * day, current: "none", gone: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(t.TempDir(), cacheFileName))
			setNKJVLicence(t)
			keys := withFakeSharedKeys(t)
			withClock(t, bridgeDay)
			nk, ok := versionByID("nkjv")
			if !ok {
				t.Skip("nkjv not registered")
			}
			paths := supersededCachePaths(nk)
			if tc.older && len(paths) < 2 {
				t.Fatal("control: the NKJV needs two superseded epochs for this case")
			}
			at := func(age time.Duration) func() time.Time {
				return func() time.Time { return bridgeDay.Add(-age) }
			}
			if err := saveBibleToCache(paths[0], stampedBible("previous"), at(tc.age)); err != nil {
				t.Fatal(err)
			}
			held, _ := os.ReadFile(paths[0])
			if tc.older {
				if err := saveBibleToCache(paths[1], stampedBible("older"), at(licensedRecencyWindow+day)); err != nil {
					t.Fatal(err)
				}
			}
			current := cachePathForVersion(nk.ID)
			switch tc.current {
			case "loads":
				mustCache(t, current, stampedBible("current"))
			case "torn":
				if err := os.WriteFile(current, nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.gone {
				t.Setenv("BIBLE_API_KEY", "")
				t.Setenv("BIBLETEXT_LICENSE_NKJV", "")
				t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "")
				keys.noteBibleKeyCleared(true)
				if nk.source.available() || !keys.bibleKeyKnownAbsent() {
					t.Fatal("control: the licence must be gone for good, or this is not the case it names")
				}
			}

			sweepLicensedCachesAtLaunch()

			b, err := os.ReadFile(paths[0])
			if tc.kept && (err != nil || !bytes.Equal(b, held)) {
				t.Fatalf("the sweep removed or changed a copy that can still bridge (err %v)", err)
			}
			if !tc.kept && err == nil {
				t.Fatal("the sweep kept a licensed superseded copy that can never be served")
			}
			if tc.older && onDisk(paths[1]) {
				t.Error("the sweep kept an older superseded copy past its window")
			}
			if tc.current == "loads" {
				if got, err := loadBibleFromCache(current); err != nil || bibleStamp(got) != "current" {
					t.Errorf("the sweep touched the current epoch (err %v)", err)
				}
			}
		})
	}
}

// The copy the bridge served is not the NKJV's text for the rest of the
// session. Back online, a reader who leaves it and asks for the NKJV again —
// from the picker, by a link to it, or through the synchronous core the
// development hooks call — is given the current edition: the choice loads,
// which asks the provider first, rather than taking the copy from memory. The
// copy is deleted, and nothing says a previous edition any more.
func TestAChoiceOfTheBridgedEditionAsksTheProviderAgain(t *testing.T) {
	for _, route := range []string{"picker", "link", "synchronous core"} {
		t.Run(route, func(t *testing.T) {
			d := newBridgeDevice(t, "nkjv")
			d.holdPrevious(10 * 24 * time.Hour)
			loads := inStepLoads(t, d.nk.ID)

			live := d.launch(false)
			if live.CurrentVersion != d.nk.ID || bibleStamp(live.Bible) != "previous" || !live.staleVersions[d.nk.ID] {
				t.Fatalf("control: the offline launch must bridge; on %s, text %q", live.CurrentVersion, bibleStamp(live.Bible))
			}
			apiBibleBaseURL = d.up // the network comes back
			switchVersionInteractive(live, defaultVersionID, byReader)
			if live.CurrentVersion != defaultVersionID {
				t.Fatalf("control: the reader must be able to leave the NKJV; on %s", live.CurrentVersion)
			}
			if _, held := live.loadedVersions[d.nk.ID]; !held {
				t.Fatal("control: the bridge's copy must still be in memory, or the choice cannot be served it from there")
			}

			want := 1
			switch route {
			case "picker":
				switchVersionInteractive(live, d.nk.ID, byReader)
			case "link":
				switchToLinkVersion(live, ShareTarget{VersionID: d.nk.ID, Book: "John", Chapter: 1})
			case "synchronous core":
				switchVersion(live, d.nk.ID, byReader)
				want = 0 // it loads in step, not through the interactive door
			}
			if *loads != want {
				t.Fatalf("the reader's choice of the NKJV made %d interactive loads, want %d", *loads, want)
			}
			if bibleStamp(live.Bible) == "previous" {
				t.Fatal("back online, the reader chose the NKJV again and was handed the bridge's copy from memory")
			}
			assertBridgeReplaced(t, d, live)

			// The current edition in memory is an ordinary loaded
			// translation again: choosing it once more makes no load.
			switchVersionInteractive(live, defaultVersionID, byReader)
			switchVersionInteractive(live, d.nk.ID, byReader)
			if *loads != want || live.CurrentVersion != d.nk.ID {
				t.Fatalf("the current edition in memory was loaded again (%d interactive loads, want %d), on %s",
					*loads, want, live.CurrentVersion)
			}
		})
	}
}

// Offline, the reader's choice of the bridged NKJV still asks the provider,
// and when that fails it is served the copy again from disk, through the
// bridge, which measures the window again. Inside it the reader reads the same
// edition, marked as before. Past it they stay where they were, and the
// copy's decode goes from memory with its mark, so nothing in the session
// serves it or says it is showing.
func TestAChoiceOfTheBridgedEditionMeasuresItsWindowAgain(t *testing.T) {
	d := newBridgeDevice(t, "nkjv")
	held := d.holdPrevious(licensedRecencyWindow - time.Hour)
	loads := inStepLoads(t, d.nk.ID)

	live := d.launch(false)
	if live.CurrentVersion != d.nk.ID || bibleStamp(live.Bible) != "previous" {
		t.Fatalf("control: the offline launch must bridge; on %s, text %q", live.CurrentVersion, bibleStamp(live.Bible))
	}
	away := func() {
		t.Helper()
		switchVersionInteractive(live, defaultVersionID, byReader)
		if live.CurrentVersion != defaultVersionID {
			t.Fatalf("control: the reader must be able to leave the NKJV; on %s", live.CurrentVersion)
		}
	}

	away()
	switchVersionInteractive(live, d.nk.ID, byReader)
	if *loads != 1 {
		t.Fatalf("the reader's choice of the bridged NKJV made %d loads, want 1: it was taken from memory", *loads)
	}
	if live.CurrentVersion != d.nk.ID || bibleStamp(live.Bible) != "previous" {
		t.Fatalf("offline, inside its window, the reader chose the NKJV and was shown %s, text %q",
			live.CurrentVersion, bibleStamp(live.Bible))
	}
	if !live.staleVersions[d.nk.ID] || fullPendingNotice(live) != previousEditionSentence(d.nk) {
		t.Fatalf("the previous edition is not said in the existing words: footer %q", fullPendingNotice(live))
	}

	d.advance(2 * time.Hour) // the copy is an hour past its window
	away()
	if _, held := live.loadedVersions[d.nk.ID]; !held || !live.staleVersions[d.nk.ID] {
		t.Fatal("control: the bridge's copy must still be in memory and marked, or nothing here asks whether it is served from there")
	}
	switchVersionInteractive(live, d.nk.ID, byReader)
	if *loads != 2 {
		t.Fatalf("the reader's choice of the NKJV past its window made %d loads in all, want 2", *loads)
	}
	if live.CurrentVersion != defaultVersionID {
		t.Fatalf("a licensed copy past its window was served: on %s, text %q", live.CurrentVersion, bibleStamp(live.Bible))
	}
	if _, kept := live.loadedVersions[d.nk.ID]; kept || live.staleVersions[d.nk.ID] {
		t.Fatalf("the session still holds the copy past its window (in memory %v, marked %v)", kept, live.staleVersions[d.nk.ID])
	}
	if n := fullPendingNotice(live); strings.Contains(n, "previous edition") {
		t.Fatalf("the picker still names a previous edition no one may be shown: %q", n)
	}
	if b, err := os.ReadFile(d.previous); err != nil || !bytes.Equal(b, held) {
		t.Fatalf("the choices changed or removed the copy on disk (err %v)", err)
	}
	if onDisk(d.current) {
		t.Fatal("a choice that could not fetch wrote a current-epoch NKJV")
	}
	switchVersionInteractive(live, d.nk.ID, byReader)
	if live.CurrentVersion != defaultVersionID {
		t.Fatalf("a second choice past the window served the copy: on %s", live.CurrentVersion)
	}
}
