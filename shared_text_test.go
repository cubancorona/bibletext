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

// The image card draws the quote in one of the share typefaces, and only some
// of them carry the small capitals. typefaceForText must never hand back a face
// that would draw them as missing-glyph boxes, whichever variant Regenerate is
// on.
func TestTheShareCardNeverDrawsAMissingSmallCapital(t *testing.T) {
	text := "“The Lᴏʀᴅ is my shepherd; I shall not want.”"
	for variant := 0; variant < 8; variant++ {
		f, ok := typefaceForText("Psalms 23:1|NKJV", variant, text)
		if !ok {
			// The caller then falls back to the reading face, which carries all
			// of them (build-reading-fonts.sh checks the app's subset).
			continue
		}
		if !faceCanDraw(f, text) {
			t.Errorf("variant %d chose %s, which cannot draw every character of %q", variant, f.name, text)
		}
	}
}
