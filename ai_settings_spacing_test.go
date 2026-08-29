package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
)

// The sheet's section rhythm depends on two spacers of DIFFERENT heights
// rendering the SAME visible gap: a caption widget carries its own trailing
// padding, a card ends at its border with none. Make the two spacers equal —
// the obvious-looking tidy-up — and the sheet goes back to about 42pt of air
// after a captioned section against 34pt after the one section that ends at a
// card. That unevenness is visible on a device and invisible in a diff, which
// is why it is asserted here rather than left to the eye.
func TestSectionSpacersDifferByTheCaptionsOwnPadding(t *testing.T) {
	card := gapHeight(t, sheetGap())
	caption := gapHeight(t, sheetGapAfterCaption())

	if got := card - caption; got != float32(captionTrailingPad) {
		t.Fatalf("card spacer %.1fpt and caption spacer %.1fpt differ by %.1fpt; they must "+
			"differ by the caption's own trailing padding (%dpt), or the two kinds of "+
			"section transition render different gaps", card, caption, got, captionTrailingPad)
	}
	if caption <= 0 {
		t.Fatalf("caption spacer is %.1fpt: sectionGapVisible (%d) cannot absorb the sheet's "+
			"inherent spacing (%d) plus the caption's padding (%d), so a captioned section "+
			"would render a larger gap than the constant asks for",
			caption, sectionGapVisible, sectionGapInherent, captionTrailingPad)
	}
}

func gapHeight(t *testing.T, o fyne.CanvasObject) float32 {
	t.Helper()
	return o.MinSize().Height
}
