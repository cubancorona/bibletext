package bibletext

import (
	"strconv"
	"strings"
	"unicode"
)

// The share pipeline's use of the selection's lead (selSpan.lead,
// docs/SELECTION_OFFSET.md): where in verse lo the reader's selection begins,
// sent by the pane it was made in. With it the selection is placed where it
// was made rather than at the first copy of its words in the span's verses,
// and the verse numbers in it are told from figures in the text by where they
// stand rather than by what follows them. Without it nothing here runs.

// leadPlacement is placeByLead's answer for one selection.
type leadPlacement struct {
	// at is the corpus offset of the selection's first character, -1 when
	// the lead does not resolve or the selection does not open as it says.
	at int
	// out is the selection with the rest of verse lo's number taken off its
	// front, when the selection began inside that number; otherwise as given.
	out string
	// text is the verse text the selection covers from at, its verse numbers
	// taken out by position (walkSelectionFrom); "" when the walk could not
	// cover the whole selection.
	text string
	// number reports a selection that was nothing but the rest of verse lo's
	// number: apparatus, answered by the verse it labels.
	number bool
}

// placeByLead places out, a selection in the shared form, by the span's lead.
// corpus and spans must be chapterProseIn's for the selection's chapter.
func placeByLead(corpus string, spans []verseSpan, span selSpan, out string) leadPlacement {
	p := leadPlacement{at: -1, out: out}
	at, tail, onNumber, ok := span.leadStart(corpus, spans)
	if !ok {
		return p
	}
	if tail != "" {
		// Begun inside the number: what the drag took of it opens the
		// selection, and is the page's apparatus, not a word.
		switch {
		case out == tail:
			p.number = true
			return p
		case strings.HasPrefix(out, tail+" "):
			p.out = out[len(tail)+1:]
		default:
			return p // the selection does not open as the lead says
		}
	}
	p.at = at
	p.text = walkSelectionFrom(corpus, spans, at, onNumber, p.out)
	return p
}

// walkSelectionFrom reads out against the corpus from at and returns the
// corpus text it covers, or "" at the first character that does not match.
// The page draws each verse's number before its words and the corpus does
// not, so a number is taken out where it stands: only where the corpus is at
// the start of a verse and the token is that verse's number, and at the end
// of the selection any first part of it, a drag ended on the number's first
// digit. At at itself the number is taken only when onNumber says the
// selection begins on it. A figure inside a verse's words is never at a
// verse's start, so it is matched as text whatever words follow it.
//
// A heading inside the selection, or any drawn character the corpus lacks,
// ends the walk with "": the caller then places the selection as it would
// with no lead.
func walkSelectionFrom(corpus string, spans []verseSpan, at int, onNumber bool, out string) string {
	k := 0
	for k < len(spans) && spans[k].start < at {
		k++
	}
	taken := at // the corpus offset whose number has been accounted for
	if onNumber {
		taken = -1
	}
	i, j := 0, at
	for i < len(out) {
		for k < len(spans) && spans[k].start < j {
			k++
		}
		if k < len(spans) && spans[k].start == j && taken != j {
			num := strconv.Itoa(spans[k].verse)
			rest := out[i:]
			if strings.HasPrefix(rest, num) && (len(rest) == len(num) || rest[len(num)] == ' ') {
				i += len(num)
				if i < len(out) {
					i++ // the space after the number
				}
				taken = j
				continue
			}
			if len(rest) < len(num) && strings.HasPrefix(num, rest) {
				break // a drag ended inside the number
			}
		}
		if j >= len(corpus) || out[i] != corpus[j] {
			return ""
		}
		i++
		j++
	}
	return strings.TrimRightFunc(corpus[at:j], unicode.IsSpace)
}

// leadVerses is the first and last verse the corpus stretch [at, at+n)
// overlaps, 0, 0 for none.
func leadVerses(spans []verseSpan, at, n int) (lo, hi int) {
	for _, sp := range spans {
		if sp.start < at+n && at < sp.end {
			if lo == 0 {
				lo = sp.verse
			}
			hi = sp.verse
		}
	}
	return lo, hi
}
