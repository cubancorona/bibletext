//go:build bibletextdev

package bibletext

import "testing"

// BIBLETEXT_DEV_JUSTIFY and the Links box force either page; anything else
// leaves it to the spec.
func TestTheDevJustifySwitchForcesEitherPage(t *testing.T) {
	prev := devJustify
	t.Cleanup(func() { devJustify = prev })
	for _, c := range []struct {
		env  string
		want bool
	}{
		{"on", true}, {" ON ", true}, {"off", false}, {"Off", false},
		{"", readingJustifyProse}, {"sometimes", readingJustifyProse},
	} {
		devJustify = devJustifyFrom(c.env)
		if got := readingJustify(); got != c.want {
			t.Errorf("BIBLETEXT_DEV_JUSTIFY=%q: justify %v, want %v", c.env, got, c.want)
		}
	}
}
