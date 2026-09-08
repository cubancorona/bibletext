package bibletext

import "testing"

// A share card is rasterised here with NO system fallback, so a glyph the
// chosen face lacks is a silent blank in the middle of a shared verse. One of
// the display faces has no macron vowels, and the NKJV prints them in Genesis
// 4 — so the chooser must pass over a face that cannot set the verse it is
// given, through every variant a reader can reach by tapping Regenerate.
func TestShareCardNeverPicksAFaceThatCannotDrawTheVerse(t *testing.T) {
	const verse = "Ē´noch begot Irad, and Irad begot Mehujael, ō and ē."
	faces := loadShareTypefaces()
	if len(faces) == 0 {
		t.Skip("no card typefaces in this build")
	}
	// The control: at least one face must actually fail the verse, or this
	// test would pass no matter what the chooser did.
	blind := 0
	for _, f := range faces {
		if !faceCanDraw(f, verse) {
			blind++
		}
	}
	if blind == 0 {
		t.Skip("every face covers this verse — nothing to prove")
	}
	for variant := 0; variant < len(faces)*2; variant++ {
		tf, ok := typefaceForText("Genesis 4:18|web", variant, verse)
		if !ok {
			t.Fatalf("variant %d returned no face", variant)
		}
		if !faceCanDraw(tf, verse) {
			t.Errorf("variant %d chose %s, which cannot draw the verse", variant, tf.name)
		}
	}
	// And a reference-only caller keeps its face: passing no text must not
	// narrow the rotation.
	seen := map[string]bool{}
	for variant := 0; variant < len(faces); variant++ {
		tf, _ := typefaceForText("Genesis 4:18|web", variant, "")
		seen[tf.name] = true
	}
	if len(seen) != len(faces) {
		t.Errorf("with no text the rotation reached %d of %d faces", len(seen), len(faces))
	}
}
