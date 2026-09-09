package bibletext

// The app header's vertical padding must stay SYMMETRIC.
//
// It was 9pt above and 2pt below. The rationale in the code was that a large bold
// title over a small muted version line reads bottom-heavy at equal margins, so
// the top wants a bias. That argument is about the title column alone, and it
// stops holding once the "Go to" chip shares the band: a bounded shape with two
// crisp horizontal edges turns the bias into a visible error rather than balance.
// Measured on a phone at the old values, the chip sat 24.3pt below the band's top
// edge and 16.7pt above its bottom — a 7.7pt imbalance, which is what prompted
// this. At 3/3 it measures 14.7 and 14.3.
//
// This reads the constants rather than the rendering. A rendered check was the
// first instinct and was abandoned honestly: the chip is a low-importance button
// whose outline is only a shade off the header's own ground, so any pixel
// threshold that finds it also finds the title, and a test that needs a
// hand-tuned threshold to pass is a test that will fail for the wrong reasons
// later. The constants are what a future reader would edit, so the constants are
// what this guards.

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestTheHeaderPadsEquallyAboveAndBelow(t *testing.T) {
	src, err := os.ReadFile("ui.go")
	if err != nil {
		t.Fatalf("cannot read the header: %v", err)
	}
	text := string(src)

	re := regexp.MustCompile(`rowWrap := container\.New\(layout\.NewCustomPaddedLayout\(([0-9.]+), ([0-9.]+),`)
	m := re.FindStringSubmatch(text)
	if m == nil {
		t.Fatal("the header's padded wrapper is no longer in the shape this test reads. " +
			"If the layout was rearranged, re-point the test — do not delete it: the " +
			"asymmetry it guards was shipped once already, defended by a comment")
	}
	top, err1 := strconv.ParseFloat(m[1], 64)
	bottom, err2 := strconv.ParseFloat(m[2], 64)
	if err1 != nil || err2 != nil {
		t.Fatalf("could not read the paddings from %q", m[0])
	}

	if top != bottom {
		t.Errorf("the header pads %vpt above the row and %vpt below. The band carries the "+
			"\"Go to\" chip, a bounded shape whose edges make any vertical bias read as a "+
			"mistake — keep these equal", top, bottom)
	}

	// The control. A test that only compared two numbers would pass just as well
	// on a header that had lost its padding entirely, which is a different bug
	// with the same symmetry.
	if top == 0 {
		t.Error("the header has no vertical padding at all; the title would sit flush " +
			"against the status bar")
	}

	// And the title/version pair reads as one block, so nothing should be
	// reintroduced between them.
	if !strings.Contains(text, "layout.NewCustomPaddedVBoxLayout(0), titleRow, versionSelector(state)") {
		t.Error("the gap between the title and the version line is back; they are one " +
			"two-line block and a padding between them is what made the band feel airy")
	}
}
