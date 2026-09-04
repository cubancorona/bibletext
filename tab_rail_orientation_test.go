package bibletext

// A PLACEMENT CHANGE HAS TO REBUILD, OR THE RAIL NEVER ARRIVES.
//
// Two things must agree for "a tablet in landscape shows the rail" to be true
// on screen: the rule that decides placement, and the watcher that notices the
// rotation. The second is the one that was quietly broken — layoutWatcher only
// re-evaluated orientation when the layout class was layoutRegular, but
// classifyLayout can no longer return that class. The clause
// was dead code, so the rail would have appeared only on the next rebuild
// triggered by something else entirely, and looked like an intermittent bug.
//
// So this file tests the COUPLING, not just the rule.

import "testing"

// railWantedAt is the placement rule with the canvas stated explicitly, so the
// table below reads as geometry rather than as mocking.
func TestMobileRailPolicy(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		tablet, phoneLandscapeRail bool
		w, h                       float32
		want                       bool
	}{
		{"iPad 13in landscape", true, false, 1366, 1024, true},
		{"iPad 13in portrait", true, false, 1024, 1366, false},
		{"Android tablet landscape", true, true, 1280, 800, true},
		{"Android phone landscape", false, true, 932, 393, true},
		{"Android phone portrait", false, true, 393, 932, false},
		{"iPhone landscape", false, false, 852, 393, false},
		{"unsized Android phone", false, true, 0, 0, false},
		{"unsized iPad initial default", true, false, 0, 0, true},
	} {
		if got := mobileRailWanted(tc.tablet, tc.phoneLandscapeRail, tc.w, tc.h); got != tc.want {
			t.Errorf("%s: rail = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The watcher must schedule a rebuild whenever the resolved placement changes,
// whatever the layout class.
func TestWatcherRebuildsWhenRailPlacementChanges(t *testing.T) {
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

	// With the class identical either way, a navigation-placement change must
	// still be treated as changed. This is the expression under test,
	// transcribed from layoutWatcher.Resize.
	for _, tc := range []struct {
		name        string
		built, want renderedLayout
		rebuild     bool
	}{
		{"tablet portrait to landscape", renderedLayout{layoutCompact, false, false}, renderedLayout{layoutCompact, true, false}, true},
		{"tablet landscape to portrait", renderedLayout{layoutCompact, true, false}, renderedLayout{layoutCompact, false, false}, true},
		{"Android phone portrait to landscape", renderedLayout{layoutCompact, false, false}, renderedLayout{layoutCompact, true, false}, true},
		{"rail unchanged", renderedLayout{layoutCompact, true, false}, renderedLayout{layoutCompact, true, false}, false},
		{"bar unchanged", renderedLayout{layoutCompact, false, false}, renderedLayout{layoutCompact, false, false}, false},
		// The phone-landscape presentation flips with no navigation to move —
		// the full-screen tree draws none — and must still rebuild.
		{"iPhone into landscape reading", renderedLayout{layoutCompact, false, false}, renderedLayout{layoutCompact, false, true}, true},
		{"iPhone back to portrait", renderedLayout{layoutCompact, false, true}, renderedLayout{layoutCompact, false, false}, true},
		// An iPad in chosen full-screen: rail zeroed on both sides, landscape
		// constant — a rotation rebuilds nothing.
		{"iPad full-screen rotation", renderedLayout{layoutCompact, false, false}, renderedLayout{layoutCompact, false, false}, false},
	} {
		if changed := layoutWatcherNeedsRebuild(tc.built, tc.want); changed != tc.rebuild {
			t.Errorf("%s: rebuild = %v, want %v", tc.name, changed, tc.rebuild)
		}
	}
	if !layoutWatcherNeedsRebuild(renderedLayout{layoutCompact, false, false}, renderedLayout{layoutRegular, false, false}) {
		t.Error("a layout-class change must still rebuild when navigation placement is unchanged")
	}
}
