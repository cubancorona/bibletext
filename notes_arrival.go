package bibletext

// notes_arrival.go — WHERE AN ARRIVAL PUTS THE VIEW, decided once.
//
// Five surfaces each answered this for themselves, in five dialects, and they
// were never the same rule:
//
//	iOS      same PARAGRAPH, via -paragraphRangeForRange: on the note's anchor
//	         and on the highlight — and a tautology whenever the note is
//	         anchorless, because the anchor range then FALLS BACK to the
//	         highlight range and the test compares a paragraph with itself
//	macOS    the iOS rule, transcribed
//	Android  same VERSE (noteAnchorVerse == pendingVerse) under a comment that
//	         says "when the band belongs to this verse's paragraph". Bible
//	         paragraphs run to many verses, so a link to any other verse of the
//	         note's own paragraph scrolled to the verse's line while the band
//	         pushed the card above the fold — the exact failure the comment
//	         claims to prevent
//	styled   the band's LINE SPAN, which is the paragraph by another name, and
//	         the closest of the four
//	web      no predicate at all: `if (noteBox || noteChip)` is a null-check on
//	         an element assigned two statements earlier, so the note always won
//
// The question needs no layout to answer. Paragraphs come from
// groupVersesIntoParagraphs on every surface — an invariant that is now
// enforced rather than assumed (TestEverySurfaceBreaksParagraphsWhereTheModelDoes)
// — so "does the note's band belong to the paragraph I am arriving at" is a
// fact about verses, and Go owns it.

// noteArrival is WHERE an arrival places the view. A renderer resolves the case
// against geometry it owns; it never decides which case applies.
type noteArrival uint8

const (
	// arriveNothing: this arrival contributes no placement. NOT "scroll to the
	// top" — whatever else places (a one-shot restore, a read-along follow, the
	// browser's own :target, or simply leaving the offset alone) stands.
	arriveNothing noteArrival = iota
	// arriveVerse: the top of ArrivalVerse's first laid-out line, Lead below
	// the top of the viewport.
	arriveVerse
	// arriveBand: the top of the reservation above that verse's paragraph, Lead
	// below the top of the viewport. A renderer that cannot resolve the band
	// yet — its reservation not built, its layout not run — falls back to
	// arriveVerse, NEVER to nothing: silence is indistinguishable from
	// "the reader is already there" and leaves them wherever they were.
	arriveBand
)

func (a noteArrival) String() string {
	return [...]string{"nothing", "verse", "band"}[a]
}

// noteParagraphOf is the index of the paragraph carrying a verse, or -1. The
// whole predicate rests on this being the SAME grouping every surface renders.
func noteParagraphOf(paras [][]Verse, verse int) int {
	if verse <= 0 {
		return -1
	}
	for i, para := range paras {
		if paraCarriesVerse(para, verse) {
			return i
		}
	}
	return -1
}

// chapterNoteArrival is the rule. It reads only facts, so it is exhaustively
// testable on a host with no device and no layout.
//
//	following      a read-along is steering the viewport
//	restoreArmed   a one-shot saved position is waiting to be consumed
//	explicit       the reader ASKED for this render (a tapped link, a note, a
//	               search result) rather than arriving by reopening
//
// Precedence, and each step is a decision somebody made once and can now stop
// re-making:
//
//  1. Narration owns the viewport while it is following. On three surfaces the
//     follow scroll simply ran LAST in the same layout pass and won by
//     accident; it wins by declaration now.
//  2. Reopening beats both targets, which is why an explicit arrival must clear
//     the restore first. Without this a note restored on reopen dragged the
//     reader back to it every launch.
//  3. The arriving verse is the mark's, or the note's own when there is no
//     mark. With neither there is nothing to place.
//  4. The band wins when it belongs to the arriving verse's PARAGRAPH — never
//     when it merely shares its verse, and never merely because a note exists.
func chapterNoteArrival(state *AppState, c noteChrome, verses []Verse, groups []noteParagraphGroup, following, restoreArmed, explicit bool) (noteArrival, int) {
	if state == nil || len(verses) == 0 {
		return arriveNothing, 0
	}
	if following {
		return arriveNothing, 0
	}
	if restoreArmed && !explicit {
		return arriveNothing, 0
	}

	verse := 0
	if span, ok := state.markHere(); ok && span.Lo > 0 {
		verse = span.Lo
	} else if c.Anchor > 0 {
		verse = c.Anchor
	}
	if verse <= 0 {
		return arriveNothing, 0
	}
	if !c.present() {
		return arriveVerse, verse
	}

	paras := groupVersesIntoParagraphs(verses)
	arriving := noteParagraphOf(paras, verse)
	if arriving < 0 {
		return arriveNothing, 0
	}
	// ANY RESERVED BAND on the arriving paragraph, not only the open card's.
	// Since the set can be drawn as one pill per paragraph, the paragraph a
	// reader arrives at may carry a PILL while the open card sits elsewhere;
	// scrolling to the verse then puts that pill above the fold, which is the
	// same failure one step smaller. The groups are the reservations
	// (chapterNoteGroups), so this asks the list rather than a single anchor.
	for _, g := range groups {
		if noteParagraphOf(paras, g.BandVerse) == arriving {
			return arriveBand, verse
		}
	}
	// An ANCHORLESS note (chapter scope, or a set whose notes land nowhere in
	// this chapter) has no paragraph of its own. Its band is reserved above the
	// paragraph the reader is arriving at, which is where the Apple panes
	// effectively put it — by accident, through the anchor-range fallback, but
	// it is the right answer: a note explaining this passage is useless parked
	// at the chapter top, out of view exactly when it is wanted.
	if c.Anchor <= 0 {
		return arriveBand, verse
	}
	if noteParagraphOf(paras, c.Anchor) == arriving {
		return arriveBand, verse
	}
	return arriveVerse, verse
}
