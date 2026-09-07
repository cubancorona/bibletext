package bibletext

// DRAWING THE SMALL CAPITALS, with characters rather than with a font feature.
//
// The edition sets the divine name in small capitals, and that is the whole of
// the distinction between the Tetragrammaton and Adonai: the letters are the
// same and only the letterforms differ. The decoder keeps the publisher's own
// characters and records where the feature applies (Verse.SmallCaps); this is
// where it is realised.
//
// It is realised by SUBSTITUTING CHARACTERS, not by asking for the OpenType
// feature, and that decision is what makes it possible at all. Unicode carries
// real codepoints for the Latin small capitals, and the shipped reading face
// has them in every cut. So the same three lines of substitution serve all six
// surfaces: no stylesheet, no sweep over an imported attributed string, no
// per-span setting on a text paint, and no patched toolkit on the two
// platforms whose text stack exposes no OpenType control whatsoever.
//
// It is the same move the verse numbers already make. They are written as real
// superscript characters rather than requested as a feature, and outboundText
// turns them back into digits on the way out. Small capitals travel the same
// road, and back along it.
//
// TWO PROPERTIES MAKE IT SAFE.
//
// The substitution belongs to the DRAWN runs. BibleData keeps the publisher's
// characters, so search, sharing, links, speech and the red-letter
// fingerprints all see the text the publisher sent.
//
// And it preserves rune counts exactly — "Lord" is four runes and "Lᴏʀᴅ" is
// four runes — so every offset the app records stays valid: footnote anchors,
// supplied-word spans, red-letter spans and selection alike.

// smallCapitals maps a letter to its Unicode small capital. They are scattered
// across three blocks, which is why a font subset that keeps only one of them
// loses letters from the divine name in silence.
//
// X has no small capital in Unicode at all. No word an edition sets this way
// contains one, so a missing entry is left as itself rather than faked.
var smallCapitals = map[rune]rune{
	'a': 'ᴀ', 'b': 'ʙ', 'c': 'ᴄ', 'd': 'ᴅ', 'e': 'ᴇ', 'f': 'ꜰ', 'g': 'ɢ',
	'h': 'ʜ', 'i': 'ɪ', 'j': 'ᴊ', 'k': 'ᴋ', 'l': 'ʟ', 'm': 'ᴍ', 'n': 'ɴ',
	'o': 'ᴏ', 'p': 'ᴘ', 'q': 'ꞯ', 'r': 'ʀ', 's': 'ꜱ', 't': 'ᴛ', 'u': 'ᴜ',
	'v': 'ᴠ', 'w': 'ᴡ', 'y': 'ʏ', 'z': 'ᴢ',
}

// smallCapitalToLetter is smallCapitals reversed for the way out, and it maps
// to the CAPITAL rather than to the letter that was substituted.
//
// A small capital does not record whether it stood for an upper or a lower case
// letter, so the way out has to choose one. Uppercase is the right choice
// twice over. It is the conventional plain-text realisation of a small-capital
// divine name — what every other edition and reader puts on a clipboard — so it
// PRESERVES the distinction between the Tetragrammaton and Adonai that the
// small capitals exist to carry. And where the publisher sent capitals inside
// the span, as it does in "G" + "OD", it returns exactly what was sent.
var smallCapitalToLetter = func() map[rune]rune {
	m := make(map[rune]rune, len(smallCapitals))
	for letter, cap := range smallCapitals {
		m[cap] = letter - ('a' - 'A')
	}
	return m
}()

// applySmallCaps rewrites the runs so the words the edition sets in small
// capitals are drawn in them. The runs' rune offsets into v.Text are what tie
// the two together, so this must run AFTER the red-letter split, whose table
// is keyed on the publisher's own text.
func applySmallCaps(v Verse, runs []verseRun) []verseRun {
	if len(v.SmallCaps) == 0 || len(runs) == 0 {
		return runs
	}
	text := []rune(v.Text)
	// Which runes to substitute. The rule follows the span's own characters,
	// exactly as the OpenType features would: a span with lower case in it
	// keeps its capital and lowers the rest, so "Lord" becomes a full-size L
	// and a small-capital ord. A span of capitals alone — the shape the feed
	// sends as "G" outside and "OD" inside — has no lower case to work on, so
	// its capitals are the ones that shrink.
	mark := make(map[int]bool)
	for _, sp := range v.SmallCaps {
		if sp.Start < 0 || sp.End > len(text) || sp.Start >= sp.End {
			continue
		}
		_, capsToo := smallCapsFeatures(string(text[sp.Start:sp.End]))
		for i := sp.Start; i < sp.End; i++ {
			r := text[i]
			switch {
			case r >= 'a' && r <= 'z':
				mark[i] = true
			case capsToo && r >= 'A' && r <= 'Z':
				mark[i] = true
			}
		}
	}
	if len(mark) == 0 {
		return runs
	}
	out := make([]verseRun, len(runs))
	at := 0
	for i, run := range runs {
		rs := []rune(run.Text)
		changed := false
		for j, r := range rs {
			if !mark[at+j] {
				continue
			}
			lower := r
			if lower >= 'A' && lower <= 'Z' {
				lower += 'a' - 'A'
			}
			if sc, ok := smallCapitals[lower]; ok {
				rs[j] = sc
				changed = true
			}
		}
		out[i] = run
		if changed {
			out[i].Text = string(rs)
		}
		at += len(rs)
	}
	return out
}

// smallCapsText is v.Text as it should be DRAWN. Same length in runes, same
// words in the same places; only the letterforms of the marked spans differ.
//
// The canvas pane needs this rather than the runs, because it draws word
// tokens and asks the red-letter machinery only which of them are Christ's —
// and that machinery matches its tokens back against the publisher's own text,
// so the substitution has to happen after it has answered, never before.
func smallCapsText(v Verse) string {
	if len(v.SmallCaps) == 0 {
		return v.Text
	}
	return applySmallCaps(v, []verseRun{{Text: v.Text}})[0].Text
}
