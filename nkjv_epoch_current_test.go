//go:build !next

package bibletext

import (
	"fmt"
	"path/filepath"
	"testing"
)

// The shipping build keeps the NKJV at cache epoch 7, as 1.2.19 does. The
// decoder fixes epoch 8 would carry in the next major release (the headings
// after a passage-chunk boundary, and the small capitals in headings and psalm
// titles) reach a reader's copy when it is next fetched, within its 30-day
// recency window, and no launch fetches the edition early
// (nkjv_epoch_next_test.go asks the opposite of the next release).
//
// So a fresh, valid epoch-7 copy is the current cache: the startup fast path
// serves it without a fetch, the launch shows it, and the sweep leaves it.
func TestTheShippingBuildServesTheNKJVCopyItHoldsAtEpoch7(t *testing.T) {
	d := newBridgeDevice(t, "nkjv")
	want := filepath.Join(filepath.Dir(d.current), fmt.Sprintf("bibletext-nkjv-v%d.json", 7))
	if d.current != want {
		t.Fatalf("the NKJV's cache is %s, want %s: the shipping build's epoch moved",
			filepath.Base(d.current), filepath.Base(want))
	}
	if err := saveBibleToCache(d.current, stampedBible("held"), currentUTCTime); err != nil {
		t.Fatal(err)
	}

	if data, _, err := loadVersionFromCacheOnly(d.nk); err != nil || bibleStamp(data) != "held" {
		t.Fatalf("the startup fast path did not serve the epoch-7 copy (err %v)", err)
	}
	live := d.launch(false)
	if live.CurrentVersion != d.nk.ID || bibleStamp(live.Bible) != "held" {
		t.Fatalf("the launch showed %s, text %q; want the NKJV the reader holds", live.CurrentVersion, bibleStamp(live.Bible))
	}
	if live.staleVersions[d.nk.ID] || fullPendingNotice(live) != "" {
		t.Fatalf("the current epoch is on screen and the app says otherwise: footer %q", fullPendingNotice(live))
	}
	if !onDisk(d.current) {
		t.Fatal("the launch removed the epoch-7 copy")
	}
}
