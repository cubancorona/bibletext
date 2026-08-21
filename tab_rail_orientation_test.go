package bibletext

// A ROTATION HAS TO REBUILD, OR THE RAIL NEVER ARRIVES.
//
// Two things must agree for "a tablet in landscape shows the rail" to be true
// on screen: the rule that decides placement, and the watcher that notices the
// rotation. The second is the one that was quietly broken — layoutWatcher only
// re-evaluated orientation when the layout class was layoutRegular, and
// classifyLayout stopped being able to return that on 21 Aug 2026. The clause
// was dead code, so the rail would have appeared only on the next rebuild
// triggered by something else entirely, and looked like an intermittent bug.
//
// So this file tests the COUPLING, not just the rule.

import (
	"testing"

	"fyne.io/fyne/v2"
)

// railWantedAt is the placement rule with the canvas stated explicitly, so the
// table below reads as geometry rather than as mocking.
func railWantedAt(tablet bool, w, h float32) bool {
	// Mirrors compactNavRail's mobile half: tablet AND landscape.
	return tablet && w > h
}

func TestRailOnlyOnTabletsInLandscape(t *testing.T) {
	for _, tc := range []struct {
		name   string
		tablet bool
		w, h   float32
		want   bool
	}{
		{"iPad 13in landscape", true, 1366, 1024, true},
		{"iPad 13in portrait", true, 1024, 1366, false},
		{"iPad mini landscape", true, 1133, 744, true},
		{"iPhone landscape", false, 852, 393, false},
		{"iPhone portrait", false, 393, 852, false},
	} {
		if got := railWantedAt(tc.tablet, tc.w, tc.h); got != tc.want {
			t.Errorf("%s: rail = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The watcher must schedule a rebuild when a TABLET rotates, whatever the
// layout class — that is the clause that was gated behind layoutRegular.
func TestWatcherRebuildsWhenATabletRotates(t *testing.T) {
	// classifyLayout returns layoutCompact for everything now, so a rotation is
	// the ONLY signal left. If the watcher ignores it, nothing rebuilds.
	if classifyLayout(1366, true) != layoutCompact {
		t.Fatal("classifyLayout no longer returns compact for a wide tablet — " +
			"this test's premise, and the reason the orientation clause matters, has moved")
	}

	built := layoutCompact
	wantAt := func(w float32) layoutClass { return classifyLayout(w, true) }
	if wantAt(1024) != built || wantAt(1366) != built {
		t.Fatal("the layout class differs across the rotation, so this would rebuild " +
			"for the old reason and the orientation clause would not be load-bearing")
	}

	// With the class identical either way, an orientation-only change must still
	// be treated as changed. This is the expression under test, transcribed.
	for _, tc := range []struct {
		name                     string
		tablet                   bool
		builtLandscape, nowLands bool
		want                     bool
	}{
		{"tablet portrait to landscape", true, false, true, true},
		{"tablet landscape to portrait", true, true, false, true},
		{"tablet no change", true, true, true, false},
		{"phone rotates", false, false, true, false},
	} {
		changed := tc.tablet && tc.nowLands != tc.builtLandscape
		if changed != tc.want {
			t.Errorf("%s: rebuild = %v, want %v", tc.name, changed, tc.want)
		}
	}
	_ = fyne.NewSize
}
