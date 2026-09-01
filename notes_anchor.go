package bibletext

// Where a note points, resolved into whatever translation is on screen
// (docs/NOTES_SPEC.md#anchor-and-placement-contract).
//
// A verse number is not an address. Verified against this tree's own tables:
//
//	MapVerse(web->bsb,  Romans 14:24) = 16:25 moved     one span, two chapters
//	MapVerse(web->bsb,  Mark   9:44)  = 0:0   absent    a hole INSIDE a span
//	MapVerse(webc->web, Esther 1:1)   = 0:0   incommensurable
//	MapVerse(webc->web, Tobit  1:1)   = 1:1   EXACT     <- the table LIES
//	MapVerse(esv->bsb,  Romans 16:25) = 16:25 EXACT     <- unknown id, silent
//
// The last two are why this file exists. versificationDeltas has no entry for
// "web" at all, so nothing stops a WEBC Tobit note claiming an exact landing
// in a translation that does not contain the book; and toReference for an
// unknown id returns "assume the numbering agrees" — the right default for a
// verse, and silently wrong about which BOOKS exist. The table answers "the
// numbering agrees" where it has no knowledge, so book existence is tested
// against the reading translation's actual book list BEFORE the table is
// consulted, and never inferred from it.
//
// So resolution returns a SET of runs plus a REASON, never a span, and the
// reason is consulted before the numbers: "the book is not in this
// translation", "the numbering does not correspond" and "it is here under
// other numbers" are different sentences, and MapVerse alone cannot tell the
// first from "everything is fine".

import (
	"sort"
	"strings"
)

// anchorRun is a contiguous verse run inside ONE chapter, in ONE numbering.
//
// The Chapter is on the run, not on the note, because a resolution can land in
// two chapters at once (the Romans doxology, every day). Without it the second
// half has nowhere to go and the note degrades to the verse it starts at —
// which is exactly what the pre-S6 derive did.
type anchorRun struct {
	Chapter int `json:"c"`
	// Lo == 0 means CHAPTER-LEVEL: a note about the chapter, not about verses
	// in it. A real state (share.go emits chapter links) and a sentinel is
	// cheaper than a second type.
	Lo int `json:"lo,omitempty"`
	// Hi == 0 or <= Lo means a single verse — the same spelling VerseSpan and
	// the store already use.
	Hi int `json:"hi,omitempty"`
}

// placementKind is TOTAL over every way an anchor can meet a translation.
// EIGHT ARMS IN THE MODEL, THREE SENTENCES ON SCREEN (placementCopy). The arms
// exist so the code is honest; the copy is small so four surfaces — two of
// them behind cgo — stay buildable.
type placementKind uint8

const (
	// placedNative — the reading translation IS the note's own. No mapping
	// ran, so the note comes home byte-exact even if the delta tables are
	// wrong for its translation. Distinct from placedExact because the chrome
	// must not print a translation label here.
	placedNative placementKind = iota
	// placedExact — mapped, every verse kept its number.
	placedExact
	// placedMoved — present here under different numbers (same chapter),
	// and/or partly in another chapter of this book (see Placement.Elsewhere).
	placedMoved
	// placedPartial — some verses are here and some are not in this
	// translation at all (a span crossing one of the BSB's omissions: WEB
	// Mark 9:43-46 in the BSB = [43,43] and [45,45], verified). Its own arm
	// because a partial tint that looked complete would misrepresent the
	// sender.
	placedPartial
	// placedOtherChapter — the whole span maps into a DIFFERENT chapter of
	// this book. Nothing on this chapter; the note lives on the other one, and
	// the derive already surfaces it there (X13), so this arm needs no copy.
	placedOtherChapter
	// unplacedAbsent — the book is here, none of the note's verses are.
	unplacedAbsent
	// unplacedIncommensurable — the book is here and does not correspond
	// verse by verse (WEBC's Greek Esther: a different book, not a
	// renumbering).
	unplacedIncommensurable
	// unplacedNoBook — this translation does not contain the book at all.
	//
	// NOT DERIVABLE FROM MapVerse. Verified: versificationDeltas has no "web"
	// entry, so MapVerse("webc","web","Tobit",1,1) answers 1:1 EXACT. This arm
	// exists because the mapping tables have a hole and the anchor must not.
	unplacedNoBook
)

// placed reports whether this kind puts anything on the page here.
func (k placementKind) placed() bool { return k <= placedPartial }

func (k placementKind) String() string {
	switch k {
	case placedNative:
		return "native"
	case placedExact:
		return "exact"
	case placedMoved:
		return "moved"
	case placedPartial:
		return "partial"
	case placedOtherChapter:
		return "other-chapter"
	case unplacedAbsent:
		return "unplaced-absent"
	case unplacedIncommensurable:
		return "unplaced-incommensurable"
	case unplacedNoBook:
		return "unplaced-no-book"
	}
	return "unknown"
}

// placement is the answer to "where does this anchor land, in the translation
// being read?".
type placement struct {
	// Kind names the WORST thing that happened when arms compete, ranked
	// exactly as ChapterNumberingDifference ranks its answers
	// (incommensurable > absent > moved): a reader told only about the milder
	// of two problems acts on the wrong one.
	Kind placementKind
	// Here is what belongs on the page, in the READING translation's
	// numbering, disjoint and ascending. Empty for every unplaced kind and for
	// placedOtherChapter.
	Here []anchorRun
	// Elsewhere is the rest of this anchor, in other chapters of this book —
	// the whole of it for placedOtherChapter, the spill for a placedMoved that
	// straddles a chapter boundary (the doxology never does this today, but
	// the model must have somewhere to put it).
	Elsewhere []anchorRun
}

// placementCopy is the sentence a surface shows for an unplaced arm — THREE
// sentences for eight arms (the collapse to three is deliberate), so the four
// note-drawing surfaces, two of them behind cgo, stay buildable. Same rules as
// the S4 wire notices (noteOutcomeMessage): attributed to nobody, no call to
// action, no link, quiet.
//
// placedOtherChapter is deliberately NOT an unplaced sentence: the note lives
// on the other chapter and the derive already shows it there, so this function
// has nothing to say about it.
func placementCopy(k placementKind) string {
	switch k {
	case unplacedNoBook:
		return "This book is not in the translation being read."
	case unplacedIncommensurable:
		return "The numbering here does not correspond to the note's."
	case unplacedAbsent:
		return "These verses are not in the translation being read."
	}
	return ""
}

// anchorWalkCap bounds how many verses of one run the resolver will ask the
// tables about. The longest chapter in any shipping translation is Psalm 119's
// 176 verses, so no genuine anchor comes near it — but a run's Lo and Hi
// arrive off the wire (parseNoteRuns admits values up to 2^31), and a declared
// size must never drive work (the inflateBytes discipline). A hostile span is
// clamped, not honoured.
const anchorWalkCap = 200

// storedAnchorRuns is the note's anchor as a run set: the full set where the
// wire carried one ('a' record, StoredNote.AnchorRuns), else the legacy
// single span every note has carried since S5.
func storedAnchorRuns(n StoredNote) []anchorRun {
	if len(n.AnchorRuns) > 0 {
		return n.AnchorRuns
	}
	return []anchorRun{{Chapter: n.Chapter, Lo: n.VerseLo, Hi: n.VerseHi}}
}

// resolveNoteAnchor resolves a stored note's anchor into the translation being
// read, answering with a set of runs plus a reason.
//
// bible is the READING translation's loaded data — the only honest authority
// on which books it contains, because the delta tables answer "the numbering
// agrees" for books they have never heard of. nil (a caller with no data, as
// in most store-level tests) skips the existence test and lets the tables'
// answer stand.
//
// It reads nothing from AppState, by construction: a resolver that can see
// where the reader is standing will eventually record where the reader is
// standing (the mechanism behind X1, X5 and the Chapter:
// state.CurrentChapter arrival bug).
func resolveNoteAnchor(n StoredNote, readingVersionID string, bible *BibleData) placement {
	runs := storedAnchorRuns(n)

	// 1. The reading translation IS the note's own: home, byte-exact, even if
	// the delta tables are wrong about it. No MAPPING runs — but existence is
	// still the TEXT's to answer: a run can name a verse this data does not
	// carry (an inflated link, a revision drift, an omitted verse), and
	// trusting it verbatim produced a PLACED note anchored on a verse with no
	// line — a tail pointing at nothing, a mirror verse no surface can find
	// (the enumeration's N11). The check is against the loaded text, never
	// the delta tables, so the tables-are-wrong concern stays answered.
	// Verses the data lacks fall out of the runs; a note none of whose verses
	// survive has no home here at all and says so as R4.
	if strings.EqualFold(n.VersionID, readingVersionID) {
		var hereHits []verseHit
		var hereChapters []int
		for _, r := range runs {
			if r.Lo <= 0 {
				if bibleHasChapter(bible, n.Book, r.Chapter) {
					hereChapters = append(hereChapters, r.Chapter)
				}
				continue
			}
			hi := r.Hi
			if hi < r.Lo {
				hi = r.Lo
			}
			if hi-r.Lo+1 > anchorWalkCap {
				hi = r.Lo + anchorWalkCap - 1
			}
			for v := r.Lo; v <= hi; v++ {
				if bibleHasVerse(bible, n.Book, r.Chapter, v) {
					hereHits = append(hereHits, verseHit{r.Chapter, v})
				}
			}
		}
		here := append(coalesceChapterRuns(hereChapters), coalesceVerseHits(hereHits)...)
		if len(here) == 0 {
			return placement{Kind: unplacedAbsent}
		}
		return placement{Kind: placedNative, Here: here}
	}

	// 2. Book existence, BEFORE MapVerse is consulted — the table lies about
	// missing books (Tobit "maps exactly" into a WEB that does not contain it,
	// and an unknown translation id claims exact everywhere).
	if bible != nil && bible.GetChaptersForBook(n.Book) == 0 {
		return placement{Kind: unplacedNoBook}
	}

	// 3. Per-verse MapVerse over the run set, collecting what actually lands.
	var hereHits, elseHits []verseHit
	var hereChapters, elseChapters []int
	sawAbsent, sawMoved := false, false
	for _, r := range runs {
		if r.Lo <= 0 {
			// A chapter-level run resolves to the chapter. Verse 1 is the
			// probe for HOW this book's numbering carries across; whether the
			// chapter is present at all is the reading data's to answer.
			ch, _, res := MapVerse(n.VersionID, readingVersionID, n.Book, r.Chapter, 1)
			switch res {
			case verseMapIncommensurable:
				return placement{Kind: unplacedIncommensurable}
			case verseMapAbsent:
				sawAbsent = true
				continue
			case verseMapMoved:
				sawMoved = true
			}
			if !bibleHasChapter(bible, n.Book, ch) {
				sawAbsent = true
				continue
			}
			if ch == r.Chapter {
				hereChapters = append(hereChapters, ch)
			} else {
				elseChapters = append(elseChapters, ch)
			}
			continue
		}
		hi := r.Hi
		if hi < r.Lo {
			hi = r.Lo
		}
		if hi-r.Lo+1 > anchorWalkCap {
			hi = r.Lo + anchorWalkCap - 1
		}
		for v := r.Lo; v <= hi; v++ {
			ch, mv, res := MapVerse(n.VersionID, readingVersionID, n.Book, r.Chapter, v)
			switch res {
			case verseMapIncommensurable:
				return placement{Kind: unplacedIncommensurable}
			case verseMapAbsent:
				// A hole inside the span. It stays OUT of Here — a partial
				// mark that looked complete would misrepresent the sender.
				sawAbsent = true
				continue
			case verseMapMoved:
				sawMoved = true
			}
			// The destination chapter must EXIST in the reading translation,
			// the same test the chapter-level branch already applies. Without it,
			// a verse-level anchor from an unknown
			// translation id naming an out-of-canon chapter classified as
			// placedExact — MapVerse's identity default trusted past the point
			// the canon supports. Unreachable with the shipping tables, but
			// "unreachable" is a property of today's data, not of this code.
			// The destination VERSE too, not just its chapter: MapVerse's
			// identity default carries any number through for a version pair
			// (or an unknown id) the tables treat as exact, and only the text
			// can say whether a line is actually there.
			if !bibleHasVerse(bible, n.Book, ch, mv) {
				sawAbsent = true
				continue
			}
			if ch == r.Chapter {
				hereHits = append(hereHits, verseHit{ch, mv})
			} else {
				elseHits = append(elseHits, verseHit{ch, mv})
			}
		}
	}

	here := append(coalesceChapterRuns(hereChapters), coalesceVerseHits(hereHits)...)
	elsewhere := append(coalesceChapterRuns(elseChapters), coalesceVerseHits(elseHits)...)

	// 4. Name the WORST thing that happened.
	switch {
	case len(here) == 0 && len(elsewhere) == 0:
		return placement{Kind: unplacedAbsent}
	case sawAbsent:
		return placement{Kind: placedPartial, Here: here, Elsewhere: elsewhere}
	case len(here) == 0:
		// The whole anchor maps into other chapters of this book (BSB Romans
		// 16:25 read in the WEB, which files the doxology at 14:24).
		return placement{Kind: placedOtherChapter, Elsewhere: elsewhere}
	case len(elsewhere) > 0 || sawMoved:
		return placement{Kind: placedMoved, Here: here, Elsewhere: elsewhere}
	default:
		return placement{Kind: placedExact, Here: here}
	}
}

// placementRunOn returns the first resolved run on the given chapter — Here
// first, then Elsewhere. Elsewhere is consulted because a placedOtherChapter
// note surfaces ON the chapter it mapped into, exactly as the pre-S6 derive
// showed the doxology (X13); the unplaced kinds carry no runs, so they answer
// false without a special case.
func placementRunOn(pl placement, chapter int) (anchorRun, bool) {
	for _, r := range pl.Here {
		if r.Chapter == chapter {
			return r, true
		}
	}
	for _, r := range pl.Elsewhere {
		if r.Chapter == chapter {
			return r, true
		}
	}
	return anchorRun{}, false
}

// bibleHasVerse reports whether the loaded reading data carries a verse — the
// question bibleHasChapter answers, one level down. The text refuses a verse
// only on POSITIVE contrary knowledge: a chapter with verses loaded that does
// not include this one. A nil bible or an empty chapter has no opinion and
// answers yes, so the tables' verdict stands alone — the same stance
// bibleHasChapter documents, extended to data that has not (or cannot have)
// been loaded.
func bibleHasVerse(bible *BibleData, book string, chapter, verse int) bool {
	if bible == nil {
		return true
	}
	verses := bible.GetChapter(book, chapter)
	if len(verses) == 0 {
		return true
	}
	for _, v := range verses {
		if v.Verse == verse {
			return true
		}
	}
	return false
}

// bibleHasChapter reports whether the loaded reading data contains a chapter.
// A nil bible has nothing to consult and answers yes, so the tables' verdict
// stands alone — the same stance resolveNoteAnchor takes on book existence.
func bibleHasChapter(bible *BibleData, book string, chapter int) bool {
	if bible == nil {
		return true
	}
	for _, c := range bible.GetChapterNumbersForBook(book) {
		if c == chapter {
			return true
		}
	}
	return false
}

// verseHit is one verse that landed somewhere: chapter and verse, both in the
// READING translation's numbering.
type verseHit struct{ ch, v int }

// coalesceVerseHits turns landed verses into maximal disjoint runs, ascending
// by chapter then verse — the shape every consumer of a placement reads.
func coalesceVerseHits(hits []verseHit) []anchorRun {
	if len(hits) == 0 {
		return nil
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].ch != hits[j].ch {
			return hits[i].ch < hits[j].ch
		}
		return hits[i].v < hits[j].v
	})
	var runs []anchorRun
	for _, h := range hits {
		if n := len(runs); n > 0 && runs[n-1].Chapter == h.ch {
			if h.v == runs[n-1].Hi {
				continue // duplicate landing
			}
			if h.v == runs[n-1].Hi+1 {
				runs[n-1].Hi = h.v
				continue
			}
		}
		runs = append(runs, anchorRun{Chapter: h.ch, Lo: h.v, Hi: h.v})
	}
	return runs
}

// coalesceChapterRuns is the chapter-level twin: each surviving chapter is its
// own Lo==0 run.
func coalesceChapterRuns(chapters []int) []anchorRun {
	if len(chapters) == 0 {
		return nil
	}
	sort.Ints(chapters)
	var runs []anchorRun
	for _, ch := range chapters {
		if n := len(runs); n > 0 && runs[n-1].Chapter == ch {
			continue
		}
		runs = append(runs, anchorRun{Chapter: ch})
	}
	return runs
}
