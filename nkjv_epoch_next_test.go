//go:build next

package bibletext

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// nkjvLastEpochBeforeHeadingFixes is the last NKJV cache epoch whose decoder
// dropped every heading after a passage-chunk boundary inside a chapter
// (Psalm 119 kept four of its twenty-two acrostic letters) and read the divine
// name in a heading or a psalm's title as plain words. Both fixes change the
// decoded copy, not the feed, so a reader holding a copy from this epoch keeps
// the old headings until that copy is decoded again. It is the shipping
// build's epoch (nkjv_epoch_current_test.go).
const nkjvLastEpochBeforeHeadingFixes = 7

// In the next major release a reader's NKJV decoded before those fixes is
// fetched again at the first launch that can reach the provider, rather than
// serving until its 30-day recency window runs out. The copy is a superseded
// epoch: the startup fast path never serves it, and a launch that can reach
// the provider replaces it with the current epoch and deletes it. Only a
// launch whose fetch fails reads it, inside its own window
// (licensed_epoch_bridge_next_test.go).
//
// The planted copy is fresh and valid, so nothing but its epoch keeps it from
// serving. While the registry still names that epoch as current, the copy is
// the current cache: the launch serves it without a fetch, and this fails.
func TestAnNKJVCopyDecodedBeforeTheHeadingFixesIsReplacedWhenTheFetchCanBeMade(t *testing.T) {
	d := newBridgeDevice(t, "nkjv")
	old := filepath.Join(filepath.Dir(d.current), fmt.Sprintf("bibletext-nkjv-v%d.json", nkjvLastEpochBeforeHeadingFixes))
	if err := saveBibleToCache(old, stampedBible("before-fixes"), currentUTCTime); err != nil {
		t.Fatal(err)
	}
	if _, err := loadBibleFromCache(old); err != nil {
		t.Fatalf("the planted copy must be servable on its own merits: %v", err)
	}

	if _, _, err := loadVersionFromCacheOnly(d.nk); err == nil {
		t.Fatalf("the startup fast path served an NKJV copy decoded at epoch %d, before the heading fixes",
			nkjvLastEpochBeforeHeadingFixes)
	}
	live := d.launch(true)
	if live.CurrentVersion != d.nk.ID || bibleStamp(live.Bible) == "before-fixes" {
		t.Fatalf("a launch that could fetch showed %s, text %q: the reader keeps the old headings "+
			"until the recency window runs out", live.CurrentVersion, bibleStamp(live.Bible))
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("the copy decoded before the heading fixes is still on disk after the current epoch was saved (stat: %v)", err)
	}
}
