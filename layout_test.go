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
