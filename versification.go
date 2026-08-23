package bibletext

// Versification: how the same passage is numbered in different translations.
//
// WHY THE APP NEEDS THIS AT ALL. Verse numbers are not a shared address space.
// The WEB puts the Romans doxology at 14:24-26; the BSB and the NKJV put it at
// 16:25-27. The BSB omits eleven verses on textual-critical grounds, so a link
// to Mark 9:44 names nothing there. WEB Catholic inserts the Song of the Three
// into Daniel 3, pushing that chapter's last seven verses from 24-30 down to
// 91-97 — so a WEB Daniel 3:24 opened in WEBC lands on a completely different
// passage. And WEBC's Esther is GREEK Esther: not a renumbering of the WEB's
// Esther but a different book, where no verse-to-verse correspondence exists.
//
// Anything that carries a reference ACROSS translations has to consult this:
// shared links, notes and highlights stored under one translation and read in
// another, and any future "compare translations" view. Red-letter data is not
// in this list: each edition now uses only its own publisher's marks. Ignoring
// versification does not fail loudly — it silently shows the reader the wrong
// verse, which is the worst failure a Bible app has.
//
// The data lives in versification_data.go, generated from the app's OWN cache
// files by scripts/gen-versification.py, so it describes the text actually
// shipped rather than a published standard that may differ from it.
//
// WEB is the reference. Every other translation is stored as a delta against
// it, and a map between two non-reference translations is composed through it.
// That keeps adding a translation to one delta rather than one per existing
// pair, and it means the reference's own numbering never needs a table.

// verseRef is a verse in some translation's numbering.
type verseRef struct {
	Book    string
	Chapter int
	Verse   int
}

// verseMove records a verse that exists in both translations under DIFFERENT
// numbers: reference Book Chapter:Verse is the target's ToChapter:ToVerse.
type verseMove struct {
	Book    string
	Chapter int
	Verse   int

	ToChapter int
	ToVerse   int
}

// versificationDelta is one translation's differences from the reference.
type versificationDelta struct {
	// absent: the reference has this verse and this translation does not — it was
	// not moved elsewhere, it simply is not in this text.
	absent []verseRef
	// moved: same passage, different number.
	moved []verseMove
	// extra: this translation has a verse the reference lacks. Recorded so a
	// reference in THIS translation can be recognised as unmappable back.
	extra []verseRef
	// incommensurable: books whose numbering does not correspond at all, keyed by
	// book with the reason. No verse in such a book can be mapped either way.
	incommensurable map[string]string
}

// verseMapResult says what happened to a reference carried across translations.
type verseMapResult int

const (
	// verseMapExact — the same number means the same passage. The overwhelmingly
	// common case, and the reason the tables hold only exceptions.
	verseMapExact verseMapResult = iota
	// verseMapMoved — the passage is there under a different number, which the
	// returned chapter/verse gives.
	verseMapMoved
	// verseMapAbsent — the target translation does not contain this verse. A
	// caller should offer the surrounding passage, not silently pick a neighbour.
	verseMapAbsent
	// verseMapIncommensurable — the two translations' versions of this BOOK do
	// not correspond verse by verse (WEBC's Greek Esther). Nothing can be mapped;
	// the honest move is to say so rather than to guess.
	verseMapIncommensurable
)

func (r verseMapResult) String() string {
	switch r {
	case verseMapExact:
		return "exact"
	case verseMapMoved:
		return "moved"
	case verseMapAbsent:
		return "absent"
	case verseMapIncommensurable:
		return "incommensurable"
	}
	return "unknown"
}

// toReference converts a verse in translation vid into the reference's numbering.
func toReference(vid, book string, chapter, verse int) (int, int, verseMapResult) {
	if vid == "" || vid == versificationReference {
		return chapter, verse, verseMapExact
	}
	d, ok := versificationDeltas[vid]
	if !ok {
		// An unknown translation is assumed to share the reference's numbering.
		// That is the right default: every translation the app has ever shipped
		// differs from the WEB in a handful of verses and agrees on ~31,000.
		return chapter, verse, verseMapExact
	}
	if why, bad := d.incommensurable[book]; bad && why != "" {
		return 0, 0, verseMapIncommensurable
	}
	for _, m := range d.moved {
		if m.Book == book && m.ToChapter == chapter && m.ToVerse == verse {
			return m.Chapter, m.Verse, verseMapMoved
		}
	}
	for _, e := range d.extra {
		if e.Book == book && e.Chapter == chapter && e.Verse == verse {
			// This translation has a verse the reference does not (Acts 8:37 in
			// the NKJV). There is nowhere in the reference to point at.
			return 0, 0, verseMapAbsent
		}
	}
	return chapter, verse, verseMapExact
}

// fromReference converts a verse in the reference's numbering into translation vid's.
func fromReference(vid, book string, chapter, verse int) (int, int, verseMapResult) {
	if vid == "" || vid == versificationReference {
		return chapter, verse, verseMapExact
	}
	d, ok := versificationDeltas[vid]
	if !ok {
		return chapter, verse, verseMapExact
	}
	if why, bad := d.incommensurable[book]; bad && why != "" {
		return 0, 0, verseMapIncommensurable
	}
	for _, m := range d.moved {
		if m.Book == book && m.Chapter == chapter && m.Verse == verse {
			return m.ToChapter, m.ToVerse, verseMapMoved
		}
	}
	for _, a := range d.absent {
		if a.Book == book && a.Chapter == chapter && a.Verse == verse {
			return 0, 0, verseMapAbsent
		}
	}
	return chapter, verse, verseMapExact
}

// MapVerse carries a verse reference from one translation's numbering into
// another's. It returns the chapter and verse to use in `to`, and what kind of
// correspondence that is — callers must look at the result, because a returned
// 0,0 means "there is no such verse there", not "verse zero".
//
// Composed through the reference, so a BSB→NKJV mapping is BSB→WEB→NKJV. Both
// halves can fail independently: Romans 16:25 in the BSB is the WEB's 14:24
// (moved), which is the NKJV's 16:25 (moved back) — the same passage, two moves,
// and the round trip lands where it started.
func MapVerse(from, to, book string, chapter, verse int) (int, int, verseMapResult) {
	if from == to {
		return chapter, verse, verseMapExact
	}
	refCh, refV, res := toReference(from, book, chapter, verse)
	if res == verseMapAbsent || res == verseMapIncommensurable {
		return 0, 0, res
	}
	outCh, outV, res2 := fromReference(to, book, refCh, refV)
	if res2 == verseMapAbsent || res2 == verseMapIncommensurable {
		return 0, 0, res2
	}
	// A move on either leg is a move overall — unless the two cancelled out and
	// the number is unchanged, which is what a BSB→NKJV Romans doxology does.
	if (res == verseMapMoved || res2 == verseMapMoved) && (outCh != chapter || outV != verse) {
		return outCh, outV, verseMapMoved
	}
	return outCh, outV, verseMapExact
}

// Numbering-difference kinds returned by ChapterNumberingDifference. They are
// plain strings rather than the unexported verseMapResult because the one
// caller outside this package (cmd/websitegen) turns them straight into a
// sentence for a reader, and a leaked enum would be a type it could not name.
const (
	// NumberingSame — every verse keeps its number; a reference may be carried
	// across as-is.
	NumberingSame = ""
	// NumberingMoved — the passage is in `to`, under a different number.
	NumberingMoved = "moved"
	// NumberingAbsent — `from` numbers a verse here that `to` simply does not
	// contain (the NKJV's Acts 8:37). Not a renumbering, and telling a reader it
	// is one would be false.
	NumberingAbsent = "absent"
	// NumberingIncommensurable — the two translations' versions of this BOOK do
	// not correspond verse by verse at all (WEBC's Greek Esther).
	NumberingIncommensurable = "incommensurable"
)

// ChapterNumberingDifference reports HOW one chapter's verse numbers differ
// between two translations — NumberingSame when they do not.
//
// When more than one kind applies the most severe wins, in the order
// incommensurable, absent, moved: they describe the same chapter at different
// scales, and a reader told only about the smallest would act on the wrong one.
func ChapterNumberingDifference(from, to, book string, chapter, spanEnd int) string {
	if from == to {
		return NumberingSame
	}
	worst := NumberingSame
	rank := map[string]int{NumberingSame: 0, NumberingMoved: 1, NumberingAbsent: 2, NumberingIncommensurable: 3}
	note := func(kind string) {
		if rank[kind] > rank[worst] {
			worst = kind
		}
	}
	check := func(verse int) {
		ch, v, res := MapVerse(from, to, book, chapter, verse)
		switch {
		case res == verseMapIncommensurable:
			note(NumberingIncommensurable)
		case res == verseMapAbsent:
			note(NumberingAbsent)
		case ch != chapter || v != verse:
			note(NumberingMoved)
		}
	}
	for verse := 1; verse <= spanEnd; verse++ {
		check(verse)
		if worst == NumberingIncommensurable {
			return worst
		}
	}
	if d, ok := versificationDeltas[from]; ok {
		for _, e := range d.extra {
			if e.Book == book && e.Chapter == chapter {
				check(e.Verse)
			}
		}
		for _, m := range d.moved {
			if m.Book == book && m.ToChapter == chapter {
				check(m.ToVerse)
			}
		}
	}
	return worst
}

// ChapterNumberingAgrees reports whether EVERY verse of one chapter keeps its
// own number when a reference is carried from translation `from` into `to`.
//
// It exists for the web reader's parallel-passage links. The server never sees
// the verse — it rides in the fragment (share_link.go) — so the page has to
// decide at BUILD time whether "the same verse, in another translation" is a
// link it may offer at all. When this returns false the page offers the CHAPTER
// and says the numbering differs there, which is the honest answer; pointing
// confidently at a number that means a different passage is the worst failure
// this file exists to prevent.
//
// spanEnd is the last verse number the chapter reaches in the REFERENCE
// translation (the WEB) — the caller has that, because the reference is one of
// the translations it loaded, and it does NOT have `from`'s own verse list (the
// point of the NKJV pages is that the site holds no NKJV data at all).
//
// TWO THINGS THE REFERENCE'S SPAN ALONE WOULD MISS, both found by measurement
// rather than by reading:
//
//   - verses `from` HAS and the reference LACKS — the NKJV's Acts 8:37,
//     Acts 15:34, Acts 24:7 and Luke 17:36. All four happen to be numbered
//     inside the WEB chapter's span today, but relying on that is luck.
//   - verses `from` numbers INTO this chapter that the reference numbers
//     elsewhere — the NKJV's Romans 16:25-27, which the WEB carries as
//     14:24-26. The WEB's Romans 16 stops at 24, so a 1..spanEnd scan never
//     asks about 25 and calls the chapter safe. It is not: those three verses
//     are exactly the ones a shared link would land in the wrong place.
//
// It is deliberately CONSERVATIVE in the other direction: spanEnd may name
// verses `from` does not have (the WEB's Romans 14:24-26 against the NKJV's
// 23-verse chapter), so a chapter can be reported as disagreeing when no
// reachable reference would actually move. The cost of that is a chapter-level
// link and one extra sentence; the cost of the opposite is the reader being
// shown different words from the ones they were sent.
func ChapterNumberingAgrees(from, to, book string, chapter, spanEnd int) bool {
	return ChapterNumberingDifference(from, to, book, chapter, spanEnd) == NumberingSame
}

// VerseExistsIn reports whether a verse named in the reference's numbering is
// present in a translation at all. Cheaper to read than MapVerse when the caller
// only needs to know whether to offer something.
func VerseExistsIn(vid, book string, chapter, verse int) bool {
	_, _, res := fromReference(vid, book, chapter, verse)
	return res != verseMapAbsent && res != verseMapIncommensurable
}

// IncommensurableBook explains why a book cannot be mapped between the reference
// and vid, or "" when it can. Callers show this rather than inventing a verse.
func IncommensurableBook(vid, book string) string {
	d, ok := versificationDeltas[vid]
	if !ok {
		return ""
	}
	return d.incommensurable[book]
}

// versificationReference is the translation every delta is measured against.
// The WEB, because it is the app's default and the one the words-of-Christ
// table was generated from.
const versificationReference = "web"
