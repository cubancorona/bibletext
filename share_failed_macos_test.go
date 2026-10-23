//go:build darwin && !ios

package bibletext

import (
	"path/filepath"
	"testing"
)

// THE MAC'S OWN SHARE OF A CARD GONE AT THE TAP SAYS THE CARD COULD NOT BE
// SHARED, as iOS and Android say it, and hands the picker nothing. Before,
// the picker was handed the address of a file that was not there, and the
// share said nothing of it (docs/PLATFORM_MATRIX.md, divergence 27). The
// release path, not the mimic's (devMimicTarget is "" here). The card that
// is there is not driven: it goes to the picker, which a test cannot show.
func TestTheMacsShareOfACardGoneAtTheTapSaysSo(t *testing.T) {
	if devMimicTarget() != "" {
		t.Skip("the mimic routes the Mac's share to the desktop fallback")
	}
	h := newShareSheetHarness(t)
	nativeShareImage(filepath.Join(t.TempDir(), "gone.png"))
	if p := failedSheetProblems(h, shareFailedCard); p != nil {
		t.Error(p)
	}
}
