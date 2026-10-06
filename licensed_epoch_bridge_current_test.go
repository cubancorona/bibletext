//go:build !next

package bibletext

// The shipping build has no licensed epoch bridge. It never serves a licensed
// translation's superseded epoch, and the startup sweep deletes every one
// (D2), as 1.2.19 does. These ask licensed_epoch_bridge_next_test.go's
// questions of this state, so a bridge that reached the shipping build by
// mistake fails here. The device is licensed_epoch_bridge_test.go's.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Offline at the first launch, a copy inside its window is not served: the
// sweep deletes it, and the launch falls back to the default translation with
// the NKJV remembered, said in the picker.
func TestTheShippingBuildServesNoLicensedPreviousEditionOffline(t *testing.T) {
	d := newBridgeDevice(t, "nkjv")
	d.holdPrevious(licensedRecencyWindow)

	live := d.launch(false)

	if live.CurrentVersion != defaultVersionID || bibleStamp(live.Bible) != "default" {
		t.Fatalf("the shipping build served a licensed superseded epoch: on %s, text %q",
			live.CurrentVersion, bibleStamp(live.Bible))
	}
	if live.preferredVersion != d.nk.ID {
		t.Fatalf("the fallback did not remember the NKJV (remembers %q)", live.preferredVersion)
	}
	if n := fullPendingNotice(live); !strings.Contains(n, d.nk.Name+" could not be opened this time") {
		t.Fatalf("the substitution is not said; footer %q", n)
	}
	if live.staleVersions[d.nk.ID] {
		t.Fatal("the NKJV is recorded as showing a previous edition, which the shipping build never serves")
	}
	if onDisk(d.previous) {
		t.Fatal("the startup sweep kept a licensed superseded copy")
	}
	if onDisk(d.current) {
		t.Fatal("a launch that could not fetch wrote a current-epoch NKJV")
	}
}

// The startup sweep removes every licensed superseded epoch, whatever its age
// and whatever is at the current epoch, and never touches the current one.
func TestTheShippingBuildsSweepRemovesEveryLicensedSupersededEpoch(t *testing.T) {
	day := 24 * time.Hour
	for _, tc := range []struct {
		name    string
		age     time.Duration // of the newest superseded copy
		current string        // "none", "loads", or "torn"
		older   bool          // an older superseded epoch, past its window, beside it
	}{
		{name: "inside its window, no current epoch", age: 10 * day, current: "none"},
		{name: "at the edge of its window, no current epoch", age: licensedRecencyWindow, current: "none"},
		{name: "past its window, no current epoch", age: licensedRecencyWindow + time.Second, current: "none"},
		{name: "inside its window, beside a current epoch that loads", age: 10 * day, current: "loads"},
		{name: "inside its window, beside a current file that does not load", age: 10 * day, current: "torn"},
		{name: "inside its window, beside an older epoch past its own", age: 10 * day, current: "none", older: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(t.TempDir(), cacheFileName))
			setNKJVLicence(t)
			withFakeSharedKeys(t)
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

			sweepLicensedCachesAtLaunch()

			for _, p := range paths {
				if onDisk(p) {
					t.Errorf("the sweep kept the licensed superseded copy %s", filepath.Base(p))
				}
			}
			if tc.current == "loads" {
				if got, err := loadBibleFromCache(current); err != nil || bibleStamp(got) != "current" {
					t.Errorf("the sweep touched the current epoch (err %v)", err)
				}
			}
		})
	}
}

// A copy inside its window, with no current epoch, is served by neither cache
// read: the fallback a failed fetch takes is the cache-only read and nothing
// else. The reader choosing the NKJV offline stays where they were, and the
// choice leaves the copy as it was for the next launch's sweep.
func TestTheShippingBuildsFallbackNeverServesALicensedSupersededEpoch(t *testing.T) {
	d := newBridgeDevice(t, defaultVersionID)
	loads := inStepLoads(t, d.nk.ID)
	live := d.launch(false)
	if live.CurrentVersion != defaultVersionID {
		t.Fatalf("control: the session must open on the default; on %s", live.CurrentVersion)
	}
	held := d.holdPrevious(10 * 24 * time.Hour)

	if data, _, err := loadVersionFallback(d.nk); err == nil {
		t.Fatalf("the fallback served %q, a licensed superseded epoch", bibleStamp(data))
	}
	if _, err := loadLicensedBridge(d.nk); err == nil {
		t.Fatal("the shipping build has a licensed bridge")
	}

	switchVersionInteractive(live, d.nk.ID, byReader)
	if *loads != 1 {
		t.Fatalf("the reader's choice of the NKJV made %d loads, want 1", *loads)
	}
	if live.CurrentVersion != defaultVersionID || bibleStamp(live.Bible) != "default" {
		t.Fatalf("the reader chose the NKJV offline and was shown %s, text %q", live.CurrentVersion, bibleStamp(live.Bible))
	}
	if _, kept := live.loadedVersions[d.nk.ID]; kept || live.staleVersions[d.nk.ID] {
		t.Fatalf("a failed choice left the NKJV in the session (in memory %v, marked %v)", kept, live.staleVersions[d.nk.ID])
	}
	if b, err := os.ReadFile(d.previous); err != nil || !bytes.Equal(b, held) {
		t.Fatalf("the choice changed or removed the copy on disk (err %v)", err)
	}
}

// Without the bridge a switch treats a licensed copy in memory as it treats
// any other, marked or not: taken from memory, with no load.
func TestTheShippingBuildTakesALicensedCopyFromMemory(t *testing.T) {
	d := newBridgeDevice(t, defaultVersionID)
	loads := inStepLoads(t, d.nk.ID)
	live := d.launch(false)
	live.loadedVersions[d.nk.ID] = stampedBible("memory")
	markVersionStale(live, d.nk.ID)

	if bridgeInMemory(live, d.nk) {
		t.Fatal("the shipping build names a licensed copy in memory as the bridge's")
	}
	switchVersionInteractive(live, d.nk.ID, byReader)
	if *loads != 0 || live.CurrentVersion != d.nk.ID || bibleStamp(live.Bible) != "memory" {
		t.Fatalf("the switch made %d loads and shows %s, text %q; want the copy in memory with no load",
			*loads, live.CurrentVersion, bibleStamp(live.Bible))
	}
}
