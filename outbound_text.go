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
// The small capitals are the one exception, and it depends on who is on the
// other end. The edition marks the divine name and the app draws it with the
// Unicode small-capital letters. A READER sending or copying a verse keeps them
// as drawn (sharedText): that is what the page shows, and the account holder
// chose it with the costs in view — a pasted "Lᴏʀᴅ" is not found by a later
// search for "Lord". A MACHINE gets capitals (outboundText): an AI request
// reads standard text, so a small capital resolves to the CAPITAL and an
// edition that stored "Lord" sends "LORD", the plain-text convention. The
// reasoning is at smallCapitalToLetter (small_caps_draw.go) and in
// docs/DIVINE_NAME.md.
//
// So every way out goes through here first, by one of the two doors. Either way
// the reader keeps the verse number as an ordinary number, the publisher's own
// spacing, and nothing else this app invented for its own page.

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
// any shipped edition is believed to contain one, and by shape rather than by
// state so every funnel is covered by this one place.
//
// THAT BELIEF IS NOT TESTED. This comment used to claim outbound_text_test.go
// "walks the feeds to keep that true"; it does not, and there is no sign it
// ever did -- the file is a table of hand-written cases. The exposure is narrow
// but real: a publisher verse containing a "[12]"-shaped token would have it
// silently deleted on the way out, and because that strip changes the BYTE
// LENGTH it would also shift the offsets the share pipeline matches on. A
// superscript digit or an em/en space in publisher text does the same. Worth a
// test that walks the shipped translations; recorded in docs/BACKLOG.md. The space after it goes with it, or "left. [36]
// They" would become "left.  They".
func outboundText(s string) string { return outboundForm(s, false) }

// sharedText is the form a reader's own shares and copies carry: the page's
// typography off exactly as outboundText takes it off — superscript numbers,
// the no-break join, the indent, an omitted verse's mark — but the divine
// name's small capitals KEPT, as drawn.
//
// Why two forms. The small capitals are the one piece of the page's typography
// that is also the edition's meaning: they are how the NKJV marks the divine
// name, and a reader sending a verse to a friend is sending what the page
// shows. The account holder chose that for everything a reader sends or copies
// — the text share, the image card, the verse of the day, Copy — having seen a
// shared `Lᴏʀᴅ` arrive in a message and wanted it kept. The costs were weighed
// and accepted: a pasted `Lᴏʀᴅ` is not found by a later search for "Lord", and
// a document font without the characters draws them from a fallback.
//
// Text the app hands to a MACHINE still goes out as capitals, through
// outboundText: an AI request reads standard text, and there is no reader on
// the other end to see the typography.
func sharedText(s string) string { return outboundForm(s, true) }

func outboundForm(s string, keepSmallCaps bool) string {
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
		case smallCapitalToLetter[r] != 0 && !keepSmallCaps:
			// To the CAPITAL, not back to the publisher's own letter -- see
			// smallCapitalToLetter. The small capital was the app's way of
			// SETTING the word, never the word itself, but it records nothing
			// about which case it replaced, so there is no letter to go back
			// to. Capitals are the plain-text convention and keep the divine
			// name distinct from an ordinary "Lord".
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

// verseSharedText is a verse as a reader's share carries it: drawn the way the
// page draws it — the divine name in small capitals — with the rest of the
// page's typography taken off by sharedText.
//
// It exists because the share pipeline has to compare two things that start in
// different forms. A selection is taken from the DRAWN page, where the divine
// name is set in small capitals; the verses it is located among are stored in
// the publisher's letters, where the same word is "Lord". Neither can simply be
// stripped to the other — a small capital records nothing about which case it
// replaced — so both are put in ONE form before they meet, and since the share
// sends the drawn form, that is the form: the stored verse is drawn, and the
// selection already is.
//
// The form has moved once. It used to be the capitals (outboundText), when a
// share sent "LORD"; it moved with the decision to send what the page shows.
// What must never happen is the two sides being in different forms: when only
// one corpus was converted, the psalms silently lost their line breaks.
//
// Running the real applySmallCaps rather than re-deriving the rule is
// deliberate — a second implementation of which letters shrink would drift
// from the first, and the locate would start failing on whichever verses the
// two disagreed about.
func verseSharedText(v Verse) string {
	if len(v.SmallCaps) == 0 {
		return sharedText(v.Text)
	}
	var b strings.Builder
	b.Grow(len(v.Text))
	for _, r := range applySmallCaps(v, []verseRun{{Text: v.Text}}) {
		b.WriteString(r.Text)
	}
	return sharedText(b.String())
}
