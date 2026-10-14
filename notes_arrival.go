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
	if following && !explicit {
		// Narration owns the viewport while it is following — for every
		// render the reader did not ask for. An EXPLICIT arrival (a search
		// jump, a link) still places its one target; the follow's own channel
		// re-takes the view on its next tick, which is the shipped behaviour
		// the styled read-along tests pin.
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
	if verse = arrivalVerseOnPage(c, verses, verse); verse <= 0 {
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

// drawnArrivalVerse is the verse an arrival at verse lands on in a chapter
// drawn as verses: verse itself when the chapter has it, and otherwise the
// first verse after it, the one whose number the hole's mark stands before
// (gapsBefore). A link names a verse in the sender's edition, and the
// reader's may omit it: the NKJV prints Matthew 17:21, the BSB does not.
// Arriving at the 21 the page does not have, Android found no row for it and
// opened the chapter at the top, and so did every pane for a link to the one
// verse; a link to 21-22 lit 22 on the others and still opened Android at
// the top. A verse past the chapter's last is left as it is, and lands
// nowhere.
func drawnArrivalVerse(verses []Verse, verse int) int {
	next := 0
	for _, v := range verses {
		if v.Verse == verse {
			return verse
		}
		if v.Verse > verse && (next == 0 || v.Verse < next) {
			next = v.Verse
		}
	}
	if next == 0 {
		return verse
	}
	return next
}

// arrivalVerseOnPage is the verse an explicit arrival at verse lands on, with
// c the render's chrome: drawnArrivalVerse, except where the page omits both
// the verse and the open card's own verse, as for a note sent on Matthew
// 17:21 and opened in the BSB. Every surface draws that card at the chapter
// top: the canvas pane opens its band above the first verse
// (noteAnchorVerse), the bridge parks the sticker at the top of the text,
// the Apple panes reserve it at the top when the mark lights nothing, and
// the website parks it above the text and lands on it. Moved on past the
// hole, the arrival scrolled the note out of view; there it places nothing
// (0), and the chapter opens at its top with the card in view. For a note on
// a range that starts at the hole (21-22) the Apple panes and the website
// put the card above the first lit verse's paragraph instead, and the
// canvas pane and the bridge keep it at the top (PLATFORM_MATRIX.md,
// divergence 34); the arrival places nothing there too, as it did before
// drawnArrivalVerse. Android pushes this verse to the bridge beside the
// class chapterNoteArrival gives, so the two stay one answer.
func arrivalVerseOnPage(c noteChrome, verses []Verse, verse int) int {
	drawn := drawnArrivalVerse(verses, verse)
	if drawn != verse && c.present() && c.Anchor > 0 && !paraCarriesVerse(verses, c.Anchor) {
		return 0
	}
	return drawn
}
