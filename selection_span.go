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
// The zero value means "position unknown" — a layer that could not resolve
// the range — and every consumer then falls back to the old text matching
// (selectionVerses, normalizeShareSelection), with its interim rules: a
// partial word the matching places in more than one verse cites the chapter
// (selectionVersesIn), and a title or a heading selected whole is answered
// as a heading (normalizeShareSelectionProse). The page the Apple panes show
// when a chapter fails to import was such a layer until it carried its verse
// numbers (reading_apple_fallback.go); it now sends a span and a lead.
//
// lead is where in verse lo the selection begins, as the pane's own text of
// verse lo from the first character of its number up to the selection's first
// character (docs/SELECTION_OFFSET.md). The verses alone cannot tell a later
// copy of repeated wording from the first; without a lead the share pipeline
// places the selection at the first. hasLead is false when the pane sent
// none; an empty lead is a value, a selection that begins on the number.
//
// book and chapter are the chapter the selection was made in, as the pane
// that made it knows it: the native panes key the text they hold with the
// chapter Go pushed it as, and the Fyne panes read it off the verses they
// laid out. The verses, the lead and the words are that chapter's, and the
// reader's chapter can move before a verb reads them: continuous narration
// turns it when a chapter ends, and a desktop menu stays open across the
// turn. Read against the chapter then current, Psalm 135:3's words were
// cited as Psalms 136:3. Every consumer therefore reads the selection in
// selectionChapter. Where book is empty the pane named no chapter, and the
// reader's chapter at the time of use stands in, as it did for every
// selection before.
//
// heading reports a selection the pane found wholly inside the publisher's
// heading matter standing above verse lo: every character of it that is not
// white space lies in a heading drawn between the verse before and verse
// lo's number. lo and hi are then that verse, and there is no lead. Such a
// selection means the section the heading introduces: it is cited by verse
// lo and quotes nothing, as a heading selected whole on its own is answered
// (normalizeShareSelectionProse). The words cannot tell it apart: a word of
// a heading is often a word of the verse above it too, and searched for, it
// was quoted from that verse and cited by it. Only the pane knows where the
// selection stands, from the heading range it keeps beside its verse
// numbers (reading_ios.go's verse index, reading_macos.go's heading
// paragraphs, BtBridge's verse ends). The panes that draw a heading outside
// their selectable text, the Windows and Linux pane among them, never set it.
type selSpan struct {
	lo, hi  int
	lead    string
	hasLead bool
	heading bool
	book    string
	chapter int
}

func (s selSpan) valid() bool { return s.lo >= 1 && s.hi >= s.lo }

// selSpanInHeading is the span of a selection that lies wholly in the
// heading matter standing above verse below (selSpan.heading), or the zero
// span when below names no verse.
func selSpanInHeading(below int) selSpan {
	if below < 1 {
		return selSpan{}
	}
	return selSpan{lo: below, hi: below, heading: true}
}

// inChapter is s made in chapter of book. A span that already names its
// chapter keeps it: the pane's own record outranks any later stand-in. An
// empty book or a chapter below 1 names nothing and leaves s as it is.
func (s selSpan) inChapter(book string, chapter int) selSpan {
	if s.book != "" || book == "" || chapter < 1 {
		return s
	}
	s.book, s.chapter = book, chapter
	return s
}

// selectionChapter is the chapter a selection is read against: the one its
// pane recorded with it, else the reader's chapter now.
func selectionChapter(state *AppState, span selSpan) (book string, chapter int) {
	if span.book != "" && span.chapter >= 1 {
		return span.book, span.chapter
	}
	return readerChapter(state)
}

// paneSelSpan is the span a native pane dispatches with its selection: the
// verses (selSpanFromNative), the lead when the pane sent one
// (selSpanWithLead), and the chapter its text held. One rule for the Apple
// exports (ai_menu_darwin.go) and Android's (reading_android_export.go),
// which then mark a selection the pane found inside a heading (inHeading).
func paneSelSpan(lo, hi int, lead string, hasLead bool, book string, chapter int) selSpan {
	s := selSpanFromNative(lo, hi)
	if hasLead {
		s = selSpanWithLead(lo, hi, lead)
	}
	return s.inChapter(book, chapter)
}

// inHeading is s for a selection the pane found wholly inside the heading
// matter standing above verse s.lo, when heading says it did: that verse
// alone, with no lead, in s's chapter (selSpanInHeading). A span that names
// no verse, or heading false, is s as it is.
func (s selSpan) inHeading(heading bool) selSpan {
	if !heading || !s.valid() {
		return s
	}
	h := selSpanInHeading(s.lo)
	h.book, h.chapter = s.book, s.chapter
	return h
}

// versesChapter is the chapter verses belong to, as a pane that laid them
// out records it: the first verse that names its book and chapter. Empty
// when none does.
func versesChapter(verses []Verse) (book string, chapter int) {
	for _, v := range verses {
		if v.BookName != "" && v.Chapter >= 1 {
			return v.BookName, v.Chapter
		}
	}
	return "", 0
}

// selectedChapter is the chapter a selection's verses are read in by a
// function handed the verses rather than the span: theirs (versesChapter),
// which selectionVerses took from the chapter the selection was made in,
// else the reader's chapter now.
func selectedChapter(state *AppState, selected []Verse) (book string, chapter int) {
	if b, ch := versesChapter(selected); b != "" {
		return b, ch
	}
	return readerChapter(state)
}

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
