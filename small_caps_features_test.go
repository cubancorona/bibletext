package bibletext

import (
	"strings"
	"testing"
)

func TestSmallCapsFeaturesFollowTheSpansOwnCharacters(t *testing.T) {
	cases := []struct {
		span       string
		smcp, c2sc bool
		why        string
	}{
		{"Lord", true, false, "a full-size L and small-capital ord: smcp alone"},
		{"ord", true, false, "lower case only"},
		{"OD", true, true, "capitals only, so c2sc is the one that acts"},
		{"LORD", true, true, "capitals only"},
		{"", true, true, "empty spans cannot be wrong either way"},
		{"O Lord", true, false, "any lower case at all is enough"},
	}
	for _, tc := range cases {
		smcp, c2sc := smallCapsFeatures(tc.span)
		if smcp != tc.smcp || c2sc != tc.c2sc {
			t.Errorf("%q: smcp=%v c2sc=%v, want smcp=%v c2sc=%v (%s)",
				tc.span, smcp, c2sc, tc.smcp, tc.c2sc, tc.why)
		}
	}
}

// The CSS must carry the reading text's own features forward. A span that
// named only the small-capital feature would drop the old-style figures,
// because font-feature-settings replaces an inherited list rather than
// merging with it.
func TestSmallCapsCSSKeepsTheReadingFeatures(t *testing.T) {
	got := smallCapsCSS("Lord")
	for _, want := range []string{`"kern" 1`, `"liga" 1`, `"calt" 1`, `"onum" 1`, `"smcp" 1`} {
		if !strings.Contains(got, want) {
			t.Errorf("smallCapsCSS(%q) = %q, missing %s", "Lord", got, want)
		}
	}
	if strings.Contains(got, `"c2sc"`) {
		t.Errorf("smallCapsCSS(%q) asked for c2sc, which would shrink the initial: %q", "Lord", got)
	}
	if up := smallCapsCSS("OD"); !strings.Contains(up, `"c2sc" 1`) {
		t.Errorf("smallCapsCSS(%q) = %q, wants c2sc for an all-capital span", "OD", up)
	}
}
