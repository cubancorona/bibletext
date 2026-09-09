package bibletext

// TEXT THAT LEAVES THE APP MUST BE THE PUBLISHER'S.
//
// The reading surfaces draw more than the publisher sent, and they have to:
// the verse number is set as a superscript, it is glued to its verse with a
// no-break space so it cannot be orphaned at a line end, and in the presented
// reporter layout a paragraph opens with an em-space and an en-space because
// the platform HTML importers drop the CSS that would indent it. All three are
// the app's own characters, chosen for the page.
//
// They are only the page's as long as they stay on it. A selection carries
// them out through every verb that turns a selection into text — copying,
// sharing, asking an assistant, following a link — and there they stop being
// typography and become words the reader believes a publisher wrote. A
// no-break space pasted into a document is a character nobody typed; a
// superscript "¹⁶" pasted into a search box finds nothing.
//
// The small capitals are the same kind of thing. The edition marks the divine
// name and the app draws it with the Unicode small-capital letters, which are
// its own characters and not the publisher's; a reader who pasted them would
// have a word no edition prints and no search box matches. They go back to the
// letters the publisher actually sent.
//
// So every outbound path goes through here first. What the reader keeps is the
// verse number as an ordinary number, the publisher's own letters and spacing,
// and nothing this app invented for its own page.

import "strings"

const (
	// noBreakSpace joins a verse number to its verse on every HTML surface, so
	// the number cannot be orphaned at a line end.
	noBreakSpace = '\u00a0'
	// emSpace and enSpace are the reporter layout's first-line indent, written
	// as literal characters because the importers ignore text-indent.
	emSpace = '\u2003'
	enSpace = '\u2002'
)

// outboundText strips the app's own typography from text on its way out, and
// changes nothing else: the publisher's words, their spacing and their
// authored line breaks all survive byte for byte.
//
// An omitted verse's mark — "[36]" in the hole Luke 17:36 leaves (verse_gaps.go)
// — is the newest of the page's own characters and goes the same way. It is
// stripped by SHAPE, a bracketed run of digits, because no publisher text in
// any shipped edition contains one (outbound_text_test.go walks the feeds to
// keep that true), and by shape rather than by state so every funnel is
// covered by this one place. The space after it goes with it, or "left. [36]
// They" would become "left.  They".
func outboundText(s string) string {
	if s == "" {
		return s
	}
	s = stripVerseGapMarks(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case superToDigit[r] != 0:
			b.WriteRune(superToDigit[r])
		case r == noBreakSpace:
			// The join is a space; only its unbreakability was ours.
			b.WriteByte(' ')
		case r == emSpace || r == enSpace:
			// The indent is the page's alone. Dropped rather than turned into
			// spaces, which would leave the paragraph looking hand-indented.
		case smallCapitalToLetter[r] != 0:
			// Back to the publisher's own letter. The small capital was the
			// app's way of SETTING the word, never the word itself.
			b.WriteRune(smallCapitalToLetter[r])
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// stripVerseGapMarks removes every "[digits]" token and one following space.
// Hand-rolled rather than a regexp: this runs on every share, copy and AI
// request, and the shape is three cases.
func stripVerseGapMarks(s string) string {
	if !strings.Contains(s, "[") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '[' {
			j := i + 1
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			if j > i+1 && j < len(s) && s[j] == ']' {
				i = j + 1
				if i < len(s) && s[i] == ' ' {
					i++
				}
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
