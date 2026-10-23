package bibletext

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// A heading's note, set apart from its words.
//
// A publisher can put a footnote inside a section heading. The WEB Catholic
// does so once in the three helloao editions: Greek Daniel 3 sets the heading
// "THE SONG OF THE THREE HOLY CHILDREN" above verse 24, with a translators'
// note on it whose reference is 3:24 and which explains where the addition
// stands and how the verses after it are numbered. The helloao feed delivers
// that heading as one string, the note's reference and words run straight on
// from the heading's last letter:
//
//	THE SONG OF THE THREE HOLY CHILDREN3:24 The Song of the Three ...
//
// Drawn as it arrives, that is a 510-character bold heading. The page sets it
// as the publisher does instead: the heading's words as the heading, and the
// note as that heading's footnote, filed in the chapter's footnote section
// under the verse the heading stands above, as every other note is filed
// under its verse, and shown exactly when the section is.
//
// This is done where a heading is read for the page (headingsFor), not when
// the feed is decoded. The stored heading keeps the string the feed sent, so
// a translation already on a device draws correctly without being downloaded
// again, and the fused form stays the record of what the feed carried.

// feedHeadingStyle is the one style the helloao feeds give a heading (bsb.go).
// Their decoder keeps no note on a heading, so a note on a heading of this
// style is one settleHeadingNote set apart, and only those notes are listed
// in the chapter's footnote section (chapterFootnoteEntries). The notes the
// API.Bible decoder keeps on an NKJV heading (Heading.Footnotes, apibible.go)
// stay unlisted, as they were before any heading's note was listed: how many
// of them are not cross-references has not been measured, and the display of
// the NKJV's apparatus is an open licensing question.
const feedHeadingStyle = "heading"

// settleHeadingNote returns h with a note the feed ran into its words set
// apart, and whether there was one.
//
// The shape it recognises is the one the feed writes: the heading's words,
// then, with no space between, the note's reference, which is the chapter
// and the verse the heading stands above ("3:24"), then a space and the
// note's words. Requiring the reference to be that verse's, and to follow a
// letter or punctuation directly, keeps as it is a heading whose own words
// cite a chapter and verse, which a space or a digit precedes. Only the
// helloao feeds' headings are read this way (feedHeadingStyle); the
// API.Bible decoder already keeps a heading's notes apart
// (Heading.Footnotes).
func settleHeadingNote(h Heading, chapter int) (Heading, bool) {
	if h.Style != feedHeadingStyle || h.BeforeVerse <= 0 || chapter <= 0 {
		return h, false
	}
	ref := strconv.Itoa(chapter) + ":" + strconv.Itoa(h.BeforeVerse) + " "
	at := strings.Index(h.Text, ref)
	if at <= 0 {
		return h, false
	}
	if last, _ := utf8.DecodeLastRuneInString(h.Text[:at]); unicode.IsSpace(last) || unicode.IsDigit(last) {
		return h, false
	}
	title := strings.TrimSpace(h.Text[:at])
	note := strings.TrimSpace(h.Text[at+len(ref):])
	if title == "" || note == "" {
		return h, false
	}
	n := utf8.RuneCountInString(title)
	out := h
	out.Text = title
	out.SmallCaps = spansWithin(h.SmallCaps, n)
	// The marker stood at the end of the heading's words, so that is the
	// note's anchor. The feed's string does not carry the note's caller.
	out.Footnotes = append(append([]Footnote(nil), h.Footnotes...), Footnote{Anchor: n, Text: note})
	return out, true
}

// spansWithin is spans cut to the first n runes: a span past n is dropped and
// one that crosses it ends there.
func spansWithin(spans []TextSpan, n int) []TextSpan {
	var out []TextSpan
	for _, s := range spans {
		if s.Start >= n {
			continue
		}
		if s.End > n {
			s.End = n
		}
		out = append(out, s)
	}
	return out
}
