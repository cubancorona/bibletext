package bibletext

import "testing"

// TestIncompleteBibleBannerGate locks when the "downloading the full Bible" banner shows:
// only while on the embedded Gospels seed of the default (WEB) version, never once the full
// text has landed or the reader is on a different (complete) translation.
func TestIncompleteBibleBannerGate(t *testing.T) {
	_ = themedTestApp() // banner builds widgets that resolve theme colors
	s := sampleState()
	s.CurrentVersion = defaultVersionID

	s.fullPending = false
	if incompleteBibleBanner(s) != nil {
		t.Error("banner should be nil once the full Bible is loaded (not fullPending)")
	}

	s.fullPending = true
	if incompleteBibleBanner(s) == nil {
		t.Error("banner should appear on the WEB Gospels seed (fullPending)")
	}

	s.CurrentVersion = "bsb" // a different, complete version
	if incompleteBibleBanner(s) != nil {
		t.Error("banner should be nil when the reader is not on the seeded default version")
	}
}
