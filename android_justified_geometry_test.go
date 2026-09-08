package bibletext

// The Android reading pane is Java behind JNI, so no Go test can render it.
// What a Go test CAN do is hold the one rule that went wrong, as a property of
// the source: every horizontal the pane measures must go through the
// justification-aware helper, never through Layout.getPrimaryHorizontal
// directly.
//
// THE DEFECT THIS EXISTS FOR. Layout.getHorizontal never applies justification
// on any release from 26 to 36, while the line-level extents (getLineLeft /
// getLineRight) do. The verse wash took one edge from each convention, so on a
// justified line — Android 15 and newer, where the pane justifies — a verse
// ending mid-line had its right edge measured short and the full stop closing it
// fell OUTSIDE the wash. Confirmed against the ragged build on Android 13, where
// the two conventions agree and the same verse washed correctly.

import (
	"os"
	"strings"
	"testing"
)

func TestAndroidMeasuresHorizontalsThroughTheJustificationAwareHelper(t *testing.T) {
	src, err := os.ReadFile("android/BtBridge.java")
	if err != nil {
		t.Fatalf("cannot read the bridge: %v", err)
	}
	text := string(src)

	if !strings.Contains(text, "private static float drawnHorizontal(") {
		t.Fatal("drawnHorizontal is gone. Every offset-to-x measurement on this pane " +
			"depends on it to add justification back; without it the wash, and the " +
			"selection popup's anchor, are measured where the glyphs would be if the " +
			"line were ragged")
	}

	// getPrimaryHorizontal may appear ONLY inside the helper's own two functions.
	// Anywhere else it is the blind measurement the helper exists to replace.
	var offenders []string
	for i, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, "getPrimaryHorizontal(") {
			continue
		}
		trimmed := strings.TrimSpace(line)
		// The helper's own uses, and any line that is a comment about them.
		if strings.HasPrefix(trimmed, "//") ||
			strings.Contains(line, "lay.getPrimaryHorizontal(probe)") ||
			strings.Contains(line, "float x = lay.getPrimaryHorizontal(offset)") {
			continue
		}
		offenders = append(offenders, strings.TrimSpace(line)+"  (line "+itoaLine(i+1)+")")
	}
	if len(offenders) > 0 {
		t.Errorf("these measure a horizontal without the justification correction, so on "+
			"a justified line they answer where the glyph WOULD be if the text were "+
			"ragged — route them through drawnHorizontal:\n  %s",
			strings.Join(offenders, "\n  "))
	}

	// The control. If the pattern this test searches for were absent from the
	// file entirely, the loop above would pass for a bridge that had lost the
	// measurement code altogether.
	if strings.Count(text, "getPrimaryHorizontal(") < 2 {
		t.Errorf("only %d getPrimaryHorizontal call(s) in the bridge — the helper itself "+
			"needs one and the check above needs something to find, so this test can no "+
			"longer tell a corrected pane from an empty one",
			strings.Count(text, "getPrimaryHorizontal("))
	}
}

func itoaLine(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
