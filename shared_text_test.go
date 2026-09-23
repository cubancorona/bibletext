package bibletext

import (
	"strings"
	"testing"
)

// ONE ANSWER FOR WHAT A READER SENDS, AND ANOTHER FOR WHAT A MACHINE READS.
//
// A share, the image card, the verse of the day and Copy send the divine name
// as the page draws it; an AI request sends capitals. Both take the rest of the
// page's typography off identically — that half must not diverge, or a paste
// carries "¹⁶" or an omitted verse's "[36]" depending on the button pressed.

func TestSharedTextKeepsTheSmallCapitalsAndOutboundTextResolvesThem(t *testing.T) {
	in := "¹⁶ The Lᴏʀᴅ is good; " + verseGapMark(17) + " He reigns."
	shared, out := sharedText(in), outboundText(in)

	if !strings.Contains(shared, "Lᴏʀᴅ") {
		t.Errorf("sharedText lost the drawn small capitals: %q", shared)
	}
	if !strings.Contains(out, "LORD") || strings.Contains(out, "ᴏ") {
		t.Errorf("outboundText did not resolve the small capitals to capitals: %q", out)
	}
	// Everything else is taken off identically.
	if strings.Replace(shared, "Lᴏʀᴅ", "LORD", 1) != out {
		t.Errorf("the two forms differ in more than the divine name:\n shared %q\n    out %q", shared, out)
	}
	for _, page := range []string{"¹", "⁶", "[17]", " "} {
		if strings.Contains(shared, page) {
			t.Errorf("sharedText kept the page's own %q: %q", page, shared)
		}
	}
}

func TestCopyingAChapterSendsTheNameAsDrawn(t *testing.T) {
	st := psalm23SmallCapsState()
	got := chapterCopyText(st)
	if !strings.Contains(got, "Lᴏʀᴅ") {
		t.Errorf("the chapter copy does not carry the name as drawn: %q", got)
	}
	// The stored spelling is the third form the copy used to send, and the one
	// no surface shows.
	if strings.Contains(got, "The Lord is") {
		t.Errorf("the chapter copy sent the stored mixed case: %q", got)
	}
}
