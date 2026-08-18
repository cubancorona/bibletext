package bibletext

// The highlight, as one value that knows where it came from.
//
// Until this file existed the highlight was five loose fields on AppState —
// HasHighlightedVerse plus a book, a chapter and two verse numbers — written by
// five different callers and readable by everyone, with NOTHING recording which
// caller set it. That missing variable is not a tidiness complaint; it is the
// direct cause of three shipped defects, and docs/NOTES_STATE.md names them:
//
//   - ORPHAN_HL: switching translation left a verse lit with no note to explain
//     it, because the code that dropped the note could not tell whether the
//     highlight was the note's to drop.
//   - X10: Hide and Delete cleared a mark they did not own, by GUESSING from a
//     coincidence — "the highlighted verse equals the note's verse, so it must
//     be the note's highlight". A search result on the same verse was collateral.
//   - X9/GHOST_LOC: HasHighlightedVerse=false left the book, chapter and verse
//     behind, so a stale location outlived the flag that said to ignore it.
//
// Two things change here. The origin is recorded, so ownership is an equality
// rather than an inference. And absence IS an origin (hlNone) rather than a
// separate boolean, so there is no way to say "no highlight" while leaving a
// location behind — the ghost state is not merely wrong, it is unwritable.
//
// The verse numbers also carry the translation they are numbered in. A verse
// number means nothing on its own: Romans 14 in one translation is Romans 16 in
// another, and a mark carried unchanged across a version switch lights the
// wrong text. Recording the frame is what lets the read accessor decline.

import "strings"

// hlOrigin says who put the mark there. Every writer sets one.
type hlOrigin int

const (
	// hlNone is the zero value ON PURPOSE: a zeroed AppState has no mark, and
	// clearing a mark is assigning the zero value rather than remembering to
	// blank four other fields.
	hlNone hlOrigin = iota
	// hlNote — the live shared note put it here.
	hlNote
	// hlSearch — a search result, or a row in the notes browser, which reaches
	// the same path through openSearchResultRange.
	hlSearch
	// hlVerseOfDay — goToVerseRange: the verse of the day, a cross-reference,
	// or the Go-to box.
	hlVerseOfDay
	// hlLinkSpan — the verse range carried by a shared link, as distinct from
	// any note that link also carried.
	hlLinkSpan
)

func (o hlOrigin) String() string {
	switch o {
	case hlNote:
		return "note"
	case hlSearch:
		return "search"
	case hlVerseOfDay:
		return "goto"
	case hlLinkSpan:
		return "link"
	default:
		return "none"
	}
}

// VerseSpan is a location AND the numbering it is expressed in.
//
// VersionID is not decoration and not optional. The whole reason a note can be
// shown under a translation it is not stored under is that MapVerse renumbers
// it; a span that does not say which translation numbers it that way cannot be
// renumbered later, and cannot be checked against the translation on screen.
type VerseSpan struct {
	VersionID string
	Book      string
	Chapter   int
	Lo        int // first verse; 0 means the span is chapter-level
	Hi        int // inclusive last verse; 0 or < Lo means a single verse
}

// covers reports whether v falls inside the span, treating Hi <= Lo as single.
func (s VerseSpan) covers(verse int) bool {
	if s.Lo <= 0 || verse <= 0 {
		return false
	}
	if s.Hi > s.Lo {
		return verse >= s.Lo && verse <= s.Hi
	}
	return verse == s.Lo
}

// sameChapter reports whether the span is on the given book and chapter. It
// says nothing about the translation — see inFrame.
func (s VerseSpan) sameChapter(book string, chapter int) bool {
	return s.Book == book && s.Chapter == chapter
}

// Mark is the highlight. The zero Mark is "nothing is highlighted", and it is
// the only way to say that.
type Mark struct {
	Origin hlOrigin
	At     VerseSpan
}

// live reports whether anything is highlighted at all.
func (m Mark) live() bool { return m.Origin != hlNone }

// fromNote reports whether the live note owns this mark. This replaces the
// coincidence test that produced X10: ownership is now recorded at the moment
// the mark is set, not guessed afterwards from matching verse numbers.
func (m Mark) fromNote() bool { return m.Origin == hlNote }

// --- AppState accessors. Everything outside this file goes through these. ---

// setMark lights a span and records why. An empty span with a positive Lo is
// impossible to construct by accident because the caller must name the origin.
func (s *AppState) setMark(origin hlOrigin, at VerseSpan) {
	if s == nil {
		return
	}
	if origin == hlNone || at.Book == "" || at.Chapter <= 0 {
		s.mark = Mark{}
		return
	}
	s.mark = Mark{Origin: origin, At: at}
}

// clearMark puts out the highlight. There is nothing left behind to go stale.
func (s *AppState) clearMark() {
	if s == nil {
		return
	}
	s.mark = Mark{}
}

// clearMarkFromNote drops the mark ONLY if the live note is what put it there.
// A search result, a shared link's span or a Go-to that happens to sit on the
// same verse is not the note's to clear.
func (s *AppState) clearMarkFromNote() {
	if s == nil || !s.mark.fromNote() {
		return
	}
	s.mark = Mark{}
}

// hasMark reports whether anything is highlighted.
func (s *AppState) hasMark() bool { return s != nil && s.mark.live() }

// markHere returns the highlighted span if it is on the chapter currently being
// read, and reports false otherwise.
//
// THE PAINTERS NO LONGER COME THROUGH HERE, and the frame check must not be
// planted here expecting them to. chapterTint (tint.go) reads markSpan and
// checks book and chapter against each VERSE rather than against AppState,
// because a renderer can be handed verses for a chapter the reader is not
// standing on. That is right, and it is orthogonal to the VERSION frame: a
// version switch renumbers the span into the new translation or takes it down
// (renumberMarkForVersion, called from applyLoadedVersion), so a live mark is
// always expressed in the reading translation's numbering and the painters
// need no frame check of their own.
//
// What still uses this: reading_android.go decides where to SCROLL, and
// notes_store.go asks whether somebody else's mark is already on the page.
// Both are questions about where the READER is, which is exactly what this
// answers.
func (s *AppState) markHere() (VerseSpan, bool) {
	if s == nil || !s.mark.live() {
		return VerseSpan{}, false
	}
	if !s.mark.At.sameChapter(s.CurrentBook, s.CurrentChapter) {
		return VerseSpan{}, false
	}
	return s.mark.At, true
}

// markSpan returns the raw span whatever chapter it is on.
//
// This is what the PAINTERS ask, through chapterTint: they are handed a list of
// verses and must decide per verse, so the span has to arrive unfiltered and be
// checked against each verse's own book and chapter. Callers asking about the
// reader's position want markHere instead.
func (s *AppState) markSpan() (VerseSpan, bool) {
	if s == nil || !s.mark.live() {
		return VerseSpan{}, false
	}
	return s.mark.At, true
}

// renumberMarkForVersion carries the highlight across a version switch — the
// X11/HL_FRAME fix, and the first consumer VerseSpan.VersionID has ever had.
// The span is renumbered into toVersionID through the SAME anchor machinery
// the notes use (resolveNoteAnchor, notes_anchor.go): a transient anchor is
// built from the span and resolved into the new translation, so the mark and
// the note beside it can never again be measured by two rulers (N7).
//
// On anything but a CLEAN landing the mark is cleared rather than left
// lighting the wrong text: the book absent from the new translation, the
// numbering incommensurable (WEBC's Greek Esther), the verses absent (the
// BSB's omissions), a partial landing (a span crossing one of those holes),
// or a landing split across more than one run. A highlight is a pointer, not
// a message — nothing is lost by taking it down, and the one thing it must
// never do is point at the wrong verse.
//
// A mark the live note owns is skipped ON PURPOSE: the note projection
// (applyNoteForCurrentChapter, run by the same apply tail) re-derives the
// note into the new translation and re-places or clears its mark from the
// note itself — renumbering it here too would be a second writer for the same
// fact. Pinned by TestNoteMarkIsRederivedNotRenumberedOnSwitch.
func renumberMarkForVersion(state *AppState, toVersionID string, bible *BibleData) {
	if state == nil || !state.mark.live() || state.mark.fromNote() {
		return
	}
	sp := state.mark.At
	if strings.EqualFold(sp.VersionID, toVersionID) {
		return // already expressed in this frame; nothing to map
	}
	transient := StoredNote{
		VersionID: sp.VersionID,
		Book:      sp.Book,
		Chapter:   sp.Chapter,
		VerseLo:   sp.Lo,
		VerseHi:   sp.Hi,
	}
	pl := resolveNoteAnchor(transient, toVersionID, bible)
	runs := append(append([]anchorRun(nil), pl.Here...), pl.Elsewhere...)
	// Clean means: every verse landed, and landed together. placedExact is the
	// identity case (the numbers survive untouched), placedMoved the renumber,
	// placedOtherChapter the doxology's cross-chapter move — all fine as long
	// as the landing is ONE run. placedPartial and every unplaced kind clear.
	// (placedNative cannot reach here: it is the EqualFold return above.)
	clean := len(runs) == 1 &&
		(pl.Kind == placedExact || pl.Kind == placedMoved || pl.Kind == placedOtherChapter)
	if !clean {
		state.clearMark()
		return
	}
	r := runs[0]
	state.mark.At = VerseSpan{
		VersionID: toVersionID,
		Book:      sp.Book,
		Chapter:   r.Chapter,
		Lo:        r.Lo,
		Hi:        r.Hi,
	}
}
