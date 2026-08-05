//go:build !race

// This test builds Fyne widgets via themedTestApp, which is defined only in the
// !race test helpers (the Fyne test app doesn't run cleanly under -race), so this
// file must carry the same tag or `go test -race ./...` fails to compile.

package bibletext

import "testing"

// TestIncompleteBibleBannerGate locks when the "downloading the full Bible" banner shows:
// only while the DISPLAYED text is the embedded Gospels seed of the default (WEB) version
// (seedOnly) — never once the full text has landed, never on a stale-epoch boot (which is
// fullPending but serves the reader's complete previous-epoch canon), and never while the
// reader is on a different (complete) translation.
func TestIncompleteBibleBannerGate(t *testing.T) {
	_ = themedTestApp() // banner builds widgets that resolve theme colors
	s := sampleState()
	s.CurrentVersion = defaultVersionID

	if incompleteBibleBanner(s) != nil {
		t.Error("banner should be nil once the full Bible is loaded")
	}

	// A stale-epoch boot re-downloads in the background (fullPending) but shows a
	// complete canon — the banner's "showing the Gospels for now" would be false.
	s.fullPending = true
	if incompleteBibleBanner(s) != nil {
		t.Error("banner should be nil on a stale-epoch boot (fullPending but not seedOnly)")
	}

	s.seedOnly = true
	if incompleteBibleBanner(s) == nil {
		t.Error("banner should appear on the WEB Gospels seed (seedOnly)")
	}

	s.CurrentVersion = "bsb" // a different, complete version
	if incompleteBibleBanner(s) != nil {
		t.Error("banner should be nil when the reader is not on the seeded default version")
	}
}
