package bibletext

import "testing"


// assert the opposite for tablets — a wide iPad got layoutRegular, the sidebar
// layout — and it is inverted here deliberately rather than deleted, because
// the inversion IS the decision: the iPad's two reported problems (a mode row
// that appeared to govern the sidebar, and results replacing the reading pane
// with no way back) were both consequences of maintaining a second shape for
// one app, and the phone's shape answers both.
//
// Every width and idiom the old table covered is kept, so the sweep still says
// what happens at the boundary, in a narrow multitasking column, and before the
// first layout pass — it just says one answer now.
func TestClassifyLayout(t *testing.T) {
	cases := []struct {
		name     string
		width    float32
		isTablet bool
	}{
		{"phone portrait", 393, false},
		{"phone landscape (wide but not a tablet)", 932, false},
		{"phone before first layout", 0, false},
		{"ipad full-screen portrait", 834, true},
		{"ipad full-screen landscape", 1194, true},
		{"ipad mini portrait at the boundary", 744, true},
		{"ipad exactly at the old breakpoint", tabletLayoutMinWidth, true},
		{"ipad just below the old breakpoint", tabletLayoutMinWidth - 1, true},
		{"ipad 1/3 multitasking column", 320, true},
		{"ipad before first layout", 0, true},
	}
	for _, c := range cases {
		if got := classifyLayout(c.width, c.isTablet); got != layoutCompact {
			t.Errorf("%s: classifyLayout(%v, %v) = %v, want layoutCompact — every touch "+
				"device takes the one mobile layout now", c.name, c.width, c.isTablet, got)
		}
	}
}

// A phone is compact at every width — the tablet layout must never appear on a
// phone, no matter how wide the landscape canvas gets.
func TestClassifyLayoutPhoneNeverRegular(t *testing.T) {
	for w := float32(0); w <= 2000; w += 37 {
		if classifyLayout(w, false) != layoutCompact {
			t.Fatalf("phone at width %v must be compact", w)
		}
	}
}

func TestRegularSplitOffset(t *testing.T) {
	// Across every regular-layout width the resulting sidebar stays within a
	// sensible band — never a sliver, never a crushing majority.
	for _, w := range []float32{700, 744, 834, 1024, 1194, 1366, 2048} {
		off := regularSplitOffset(w)
		if off < regularSidebarMinFrac-1e-9 || off > regularSidebarMaxFrac+1e-9 {
			t.Errorf("width %v: offset %v out of [%v,%v]", w, off, regularSidebarMinFrac, regularSidebarMaxFrac)
		}
		sidebarPt := off * float64(w)
		if sidebarPt < 200 {
			t.Errorf("width %v: sidebar %vpt too narrow", w, sidebarPt)
		}
	}
	// A big canvas is clamped to the max fraction, not left as a sliver.
	if got := regularSplitOffset(4000); got != regularSidebarMaxFrac && got != regularSidebarMinFrac {
		// 250/4000 = 0.0625 < min → clamps to the min fraction.
		if got != regularSidebarMinFrac {
			t.Errorf("huge canvas should clamp to min frac, got %v", got)
		}
	}
	// A mid canvas hits the target width (~250pt) exactly, not a clamp.
	if got := regularSplitOffset(1000); got != float64(regularSidebarTargetPt/1000) {
		t.Errorf("mid canvas should target ~250pt, got frac %v (=%vpt)", got, got*1000)
	}
	// Unknown width (pre-layout) falls back to the max fraction.
	if got := regularSplitOffset(0); got != regularSidebarMaxFrac {
		t.Errorf("width 0 should fall back to max frac, got %v", got)
	}
}

func TestIsTabletDimensions(t *testing.T) {
	cases := []struct {
		w, h float32
		want bool
	}{
		{393, 852, false},  // phone portrait
		{852, 393, false},  // phone landscape — smallest dim still phone-class
		{600, 960, true},   // exactly at the sw600dp threshold
		{599, 1200, false}, // just under
		{800, 1280, true},  // classic 10" tablet
		{1280, 800, true},  // same, landscape
		{0, 0, false},      // pre-layout: not a tablet yet
	}
	for _, c := range cases {
		if got := isTabletDimensions(c.w, c.h); got != c.want {
			t.Errorf("isTabletDimensions(%v,%v) = %v, want %v", c.w, c.h, got, c.want)
		}
	}
}
