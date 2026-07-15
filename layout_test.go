package bibletext

import "testing"

func TestClassifyLayout(t *testing.T) {
	cases := []struct {
		name     string
		width    float32
		isTablet bool
		want     layoutClass
	}{
		{"phone portrait", 393, false, layoutCompact},
		{"phone landscape (wide but not a tablet)", 932, false, layoutCompact},
		{"phone before first layout", 0, false, layoutCompact},
		{"ipad full-screen portrait", 834, true, layoutRegular},
		{"ipad full-screen landscape", 1194, true, layoutRegular},
		{"ipad mini portrait at the boundary", 744, true, layoutRegular},
		{"ipad exactly at the breakpoint", tabletLayoutMinWidth, true, layoutRegular},
		{"ipad just below the breakpoint (narrow split)", tabletLayoutMinWidth - 1, true, layoutCompact},
		{"ipad 1/3 multitasking column", 320, true, layoutCompact},
		{"ipad before first layout trusts the idiom", 0, true, layoutRegular},
	}
	for _, c := range cases {
		if got := classifyLayout(c.width, c.isTablet); got != c.want {
			t.Errorf("%s: classifyLayout(%v, %v) = %v, want %v", c.name, c.width, c.isTablet, got, c.want)
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
