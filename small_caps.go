package bibletext

// HOW THE DIVINE NAME IS SET, and why the feature to ask for is not the same
// every time.
//
// The edition marks the divine name with a char span and sets it in small
// capitals. The feed sends that span in two shapes, and they need different
// OpenType features:
//
//	"Lord"        the whole word inside the span, remainder in lower case.
//	              The printed form is a full-size L followed by small-capital
//	              ORD, which is what smcp gives: it maps lower case to small
//	              capitals and leaves the capital alone.
//
//	"G" + "OD"    a full-size capital OUTSIDE the span and the remainder
//	              already capitalised INSIDE it. smcp does nothing here,
//	              because there is no lower case to map. c2sc is the one that
//	              acts on capitals.
//
// Asking for both every time is wrong, and the reason is particular to the
// face. Spectral's c2sc covers lower case as well as capitals — verified
// against the font's own GSUB, where c2sc carries 673 substitutions including
// all 26 lower-case letters, against smcp's 377 which include none of the
// capitals. So c2sc on "Lord" sets the L as a small capital too, and the word
// loses the full-size initial that makes it read as the divine name.
//
// Hence: smcp always, and c2sc only for a span with no lower case in it.

import "unicode"

// smallCapsFeatures reports the OpenType features to apply to one small-capital
// span, given the publisher's own characters inside it. Every surface asks this
// rather than deciding for itself, so a reader sees the same word set the same
// way on a phone, a desktop and the web.
func smallCapsFeatures(span string) (smcp, c2sc bool) {
	for _, r := range span {
		if unicode.IsLower(r) {
			// Lower case present: smcp alone sets it, and the capital that
			// opens the word stays full size.
			return true, false
		}
	}
	// Nothing but capitals: only c2sc can act on them.
	return true, true
}

// smallCapsCSS renders those features as a font-feature-settings value for the
// surfaces that take CSS. The features the reading text already asks for are
// restated because font-feature-settings REPLACES an inherited list rather than
// merging with it: a span that named only the small-capital feature would
// silently drop the old-style figures and the kerning the reading rule sets.
func smallCapsCSS(span string) string {
	smcp, c2sc := smallCapsFeatures(span)
	s := `"kern" 1,"liga" 1,"calt" 1,"onum" 1`
	if smcp {
		s += `,"smcp" 1`
	}
	if c2sc {
		s += `,"c2sc" 1`
	}
	return s
}
