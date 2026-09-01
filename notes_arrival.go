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
	// A PLAIN entry — the chapter arrows, the strip, a reopen with nothing
	// armed — is browsing, not a request to be taken to the note. Every
	// explicit route sets forceReposition (a tapped link, a note row, a
	// search result, the verse of the day); without it this classifier used
	// to fall through to "the note's own verse", so merely entering a
	// chapter that carried a collapsed note dragged the reader to its pill.
	// The chapter opens at the top like any other; the bands and the wash
	// still say where the notes are.
	if !explicit {
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
	// With one pill per paragraph the reader can arrive at a paragraph whose
	// pill is not the open note's, and scrolling to the verse puts that pill
	// above the fold — the same failure one step smaller.
	//
	// The list is empty unless per-paragraph pills are on, because
	// chapterNoteGroups is itself gated on that: with the single chapter pill
	// there is exactly ONE reservation, and it is the anchor below that finds
	// it. So this loop needs no flag of its own, and must not grow one — a
	// second gate on the same fact is a second place for the two to disagree.
	for _, g := range groups {
		if noteParagraphOf(paras, g.BandVerse) == arriving {
			return arriveBand, verse
		}
	}
	// An ANCHORLESS card — a chapter-scope note, or a collapsed set parked at
	// chapter scope — has no paragraph of its own. Its band is reserved at the
	// CHAPTER TOP (chapterTopGroup, notes_plan.go), so it wins only when the
	// reader is arriving at the first paragraph, and nowhere else.
	//
	// The Apple panes appeared to say otherwise, reserving it above whatever
	// paragraph was being arrived at — but only because their anchor range fell
	// back to the highlight range, which is the same tautology that made their
	// same-paragraph guard vacuous. Following that would send a reader arriving
	// mid-chapter to a band that is drawn at the top.
	if c.Anchor <= 0 {
		if arriving == 0 {
			return arriveBand, verse
		}
		return arriveVerse, verse
	}
	if noteParagraphOf(paras, c.Anchor) == arriving {
		return arriveBand, verse
	}
	return arriveVerse, verse
}
