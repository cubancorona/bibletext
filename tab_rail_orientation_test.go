package bibletext

// A PLACEMENT CHANGE HAS TO REBUILD, OR THE RAIL NEVER ARRIVES.
//
// Two things must agree for "a window wider than tall shows the rail" to be
// true on screen: the rule that decides placement, and the watcher that notices the
// rotation. The second is the one that was quietly broken — layoutWatcher only
// re-evaluated orientation when the layout class was layoutRegular, but
// classifyLayout can no longer return that class. The clause
// was dead code, so the rail would have appeared only on the next rebuild
// triggered by something else entirely, and looked like an intermittent bug.
//
// So this file tests the COUPLING, not just the rule.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// navDevice is a phone or a tablet as the navigation rule sees it: its
// platform, whether it is tablet-class, and its full-screen window in points,
// upright.
type navDevice struct {
	name    string
	android bool
	tablet  bool
	w, h    float32
}

// tabletAt is what deviceIsTablet answers for d's window at w x h: the UIKit
// idiom on iOS, fixed for the process; on Android the window's own short side
// (isTabletDimensions), which an unsized window does not have.
func (d navDevice) tabletAt(w, h float32) bool {
	if d.android {
		return isTabletDimensions(w, h)
	}
	return d.tablet
}

// railRule is the rule as compactNavRail asks it on d's platform.
func railRule(d navDevice, w, h float32) bool {
	return mobileRailWanted(d.tabletAt(w, h), w, h)
}

var navDevices = []navDevice{
	{"iPhone 17 Pro Max", false, false, 440, 956},
	{"iPhone 16 Pro", false, false, 402, 874},
	{"iPhone SE", false, false, 375, 667},
	{"Android phone", true, false, 412, 915},
	{"iPad Pro 11-inch", false, true, 834, 1210},
	{"iPad Pro 13-inch", false, true, 1032, 1376},
	{"Android tablet", true, true, 800, 1280},
}

// ONE RULE ON EVERY PHONE AND TABLET. The navigation is a rail when the
// window is wider than it is tall and a bottom bar otherwise, on iOS and
// Android, phone and tablet, full screen and in a window. Upright every one
// keeps the bottom bar it had; sideways every one has the rail, the iPhone
// included, whose Books and Search kept the bar there before. A square window
// counts as wide. Before the canvas has a size the idiom answers: an iPad's
// first frame is the rail, and an iPhone's and any Android window's (whose
// idiom is its size) the bar. Shown to fail on the rule before it, which gave
// the rail to a phone only on Android: the three iPhones held sideways, and
// only they, failed.
func TestOneNavigationRuleOnEveryPhoneAndTablet(t *testing.T) {
	type geometry struct {
		how  string
		w, h float32
		rail bool
	}
	for _, d := range navDevices {
		for _, g := range []geometry{
			{"upright", d.w, d.h, false},
			{"sideways", d.h, d.w, true},
		} {
			if got := railRule(d, g.w, g.h); got != g.rail {
				t.Errorf("%s held %s (%vx%v): rail = %v, want %v", d.name, g.how, g.w, g.h, got, g.rail)
			}
		}
	}
	for _, tc := range []struct {
		name string
		d    navDevice
		w, h float32
		rail bool
	}{
		{"an iPad Split View column", navDevices[4], 320, 1210, false},
		{"an iPad Stage Manager window wider than tall", navDevices[4], 900, 640, true},
		{"an iPad Stage Manager window taller than wide", navDevices[4], 640, 900, false},
		{"an Android phone's half of a split screen, upright", navDevices[3], 412, 450, false},
		{"an Android phone's half of a split screen, sideways", navDevices[3], 457, 412, true},
		{"an Android tablet window narrower than 600, wider than tall", navDevices[6], 590, 400, true},
		{"a square iPad window", navDevices[4], 600, 600, true},
		{"an unsized iPhone", navDevices[0], 0, 0, false},
		{"an unsized iPad", navDevices[4], 0, 0, true},
		{"an unsized Android phone", navDevices[3], 0, 0, false},
		{"an unsized Android tablet", navDevices[6], 0, 0, false},
	} {
		if got := railRule(tc.d, tc.w, tc.h); got != tc.rail {
			t.Errorf("%s (%s, %vx%v): rail = %v, want %v", tc.name, tc.d.name, tc.w, tc.h, got, tc.rail)
		}
	}
}

// THE RULE READS THE WINDOW'S CANVAS. railForWindow is what compactNavRail
// asks on a phone or a tablet; it reads the canvas's size, so the soft
// keyboard, which shrinks only the laid-out tree, cannot read as a rotation,
// and it answers for a window resized across square at once. With no window
// it answers as an unsized canvas does: the bar, on a host that is not a
// tablet.
func TestRailForWindowReadsTheCanvas(t *testing.T) {
	test.NewTempApp(t)
	st := sampleState()
	if railForWindow(st) {
		t.Error("with no window the navigation is the rail; an unsized phone's is the bar")
	}
	w := test.NewWindow(nil)
	t.Cleanup(w.Close)
	st.window = w
	for _, sz := range []fyne.Size{{Width: 440, Height: 956}, {Width: 956, Height: 440}, {Width: 640, Height: 641}, {Width: 641, Height: 641}, {Width: 402, Height: 874}} {
		w.Resize(sz)
		want := sz.Width >= sz.Height
		if got := railForWindow(st); got != want {
			t.Errorf("a %vx%v window: rail = %v, want %v", sz.Width, sz.Height, got, want)
		}
	}
	if c := w.Canvas().Size(); c.Width != 402 || c.Height != 874 {
		t.Fatalf("control: the canvas is %vx%v, not the window's last size", c.Width, c.Height)
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
		// An iPhone on Books or Search: the rail arrives with the landscape
		// presentation's term, and either alone would rebuild.
		{"iPhone Books upright to sideways", renderedLayout{layoutCompact, false, false}, renderedLayout{layoutCompact, true, true}, true},
		{"iPhone Books sideways to upright", renderedLayout{layoutCompact, true, true}, renderedLayout{layoutCompact, false, false}, true},
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
