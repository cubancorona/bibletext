package bibletext

import (
	"strconv"
	"strings"
)

// selSpan is the verse range a reading-pane selection covers, resolved
// POSITIONALLY by the pane the selection was made in — the native overlays map
// the selected character range onto their verse-number index (reading_ios.go /
// reading_macos.go / BtBridge.java), the styled pane reads it off its own runs
// (reading_styled_select.go), and the legacy Fyne Entry pane from the verse
// offsets it records as it lays the chapter out (chapterText.selectionSpan).
// Position is authoritative: text matching cannot
// tell apart repeated wording (Psalm 136's refrain appears in all 26 verses)
// and its 8-rune probe floor resolves short selections to nothing, so the
// selection's position must survive the trip through the dispatch layer instead
// of being rediscovered from the words.
//
// The zero value means "position unknown" — the plain-text page the Apple
// panes show when a chapter fails to import, or a layer that could not resolve
// the range — and every consumer then falls back to the old text matching
// (selectionVerses, normalizeShareSelection).
//
// lead is where in verse lo the selection begins, as the pane's own text of
// verse lo from the first character of its number up to the selection's first
// character (docs/SELECTION_OFFSET.md). The verses alone cannot tell a later
// copy of repeated wording from the first; without a lead the share pipeline
// places the selection at the first. hasLead is false when the pane sent
// none; an empty lead is a value, a selection that begins on the number.
type selSpan struct {
	lo, hi  int
	lead    string
	hasLead bool
}

func (s selSpan) valid() bool { return s.lo >= 1 && s.hi >= s.lo }

// selSpanFromNative normalizes a lo/hi pair as the panes report it. hi<=0 is
// "no span". lo==0 with hi>0 is a selection that starts ABOVE verse 1's number
// (the chapter heading) — the first scripture it can touch is verse 1, so it
// clamps there rather than degrading the whole span to the matching fallback.
func selSpanFromNative(lo, hi int) selSpan {
	if hi <= 0 {
		return selSpan{}
	}
	if lo < 1 {
		lo = 1
	}
	if hi < lo {
		lo, hi = hi, lo
	}
	return selSpan{lo: lo, hi: hi}
}

// selSpanWithLead is selSpanFromNative for a pane that also sends the
// selection's lead. The lead was measured from verse lo's number, so it is
// kept only while the span still starts at that verse: the clamp of a
// selection begun above verse 1 and the swap of an inverted pair both change
// lo, and the lead then describes another verse.
func selSpanWithLead(lo, hi int, lead string) selSpan {
	s := selSpanFromNative(lo, hi)
	if s.valid() && s.lo == lo {
		s.lead, s.hasLead = lead, true
	}
	return s
}

// leadStart resolves the span's lead against a chapter built by
// chapterProseIn: at is the corpus offset of the selection's first character,
// and tail, when the selection began inside verse lo's number, is the rest of
// that number, which the selection then opens with. onNumber reports a
// selection that may open with the number of the verse starting at at: verse
// lo's, for an empty lead, or the next verse's, for a lead holding all of
// verse lo.
//
// The lead is put into the form the selection is put into (collapseSpaces,
// sharedText), so whatever makes the pane's drawn text differ from the
// corpus falls away by the same rules for both, and after the number it must
// be a prefix of verse lo's body byte for byte. A lead from another chapter,
// one that runs through a heading or holds any drawn character the corpus
// lacks, is no lead: ok is false and the selection is placed as if none had
// been sent.
func (s selSpan) leadStart(corpus string, spans []verseSpan) (at int, tail string, onNumber, ok bool) {
	if !s.hasLead || !s.valid() {
		return -1, "", false, false
	}
	k := -1
	for i, sp := range spans {
		if sp.verse == s.lo {
			k = i
			break
		}
	}
	if k < 0 {
		return -1, "", false, false
	}
	sp := spans[k]
	p := sharedText(collapseSpaces(s.lead))
	num := strconv.Itoa(s.lo)
	switch {
	case p == "":
		return sp.start, "", true, true
	case p == num:
		return sp.start, "", false, true
	case strings.HasPrefix(num, p):
		return sp.start, num[len(p):], false, true
	case strings.HasPrefix(p, num+" "):
		before := p[len(num)+1:]
		if !strings.HasPrefix(corpus[sp.start:sp.end], before) {
			return -1, "", false, false
		}
		at = sp.start + len(before)
		// The lead's trailing space, if it had one, was collapsed away; a
		// selection that begins after a space begins after the corpus's.
		if at < len(corpus) && corpus[at] == ' ' {
			at++
		}
		// A lead that holds the whole verse is a selection begun after it,
		// in the space before the next verse's number or on that number.
		onNext := k+1 < len(spans) && at == spans[k+1].start
		return at, "", onNext, true
	}
	return -1, "", false, false
}
