package bibletext

// The decode-time verse-count check — docs/SCRIPTURE_WORKLIST.md, S4.
//
// Every helloao book states its own verse total, and that total counts the
// book's verse NODES, including the ones a critical text omits. This decoder
// turns an omitted verse into an orphan note rather than a verse, so the sum
// that must equal the feed's figure is decoded verses PLUS omitted ones.
//
// Measured against the captured feeds, the identity holds for all 205 books of
// the three editions with no exceptions: BSB 31,086 + 0, WEB 31,098 + 5 (Luke,
// Acts ×3, Romans), WEB Catholic 35,379 + 29 (those three plus Sirach's 24
// versification gaps).
//
// This observes and reports. It never returns an error and never changes a
// decode. A silently truncated book is the failure a cached Bible hides best —
// it reads as a shorter book rather than as a fault — but an edition that has
// stopped adding up is still an edition a reader can read, and taking it away
// from them would be the worse answer.

import "log"

// verseCountAudit reconciles one edition, book by book, and says once at the
// end what it found.
type verseCountAudit struct {
	edition string
	// logf is log.Printf in the app and a recorder in the test. A check nobody
	// can watch fail is a check nobody should trust.
	logf     func(format string, v ...any)
	books    int
	unstated int
	mismatch int
}

func newVerseCountAudit(edition string) *verseCountAudit {
	if edition == "" {
		// The feed named no edition. Worth reporting under a placeholder
		// rather than dropping the audit: the count check is still valid.
		edition = "helloao"
	}
	return &verseCountAudit{edition: edition, logf: log.Printf}
}

// book reconciles ONE decoded book against the total its feed stated.
//
// Orphans are keyed per chapter, and one omitted verse can raise more than one
// orphan record because a verse may carry several note bodies. What reconciles
// with the feed is therefore DISTINCT VERSE NUMBERS, never the number of orphan
// records. No omitted verse in any captured edition carries more than one body
// today, so the two counts agree on every byte of live data — which is exactly
// why the distinction has to be written down rather than discovered later.
func (a *verseCountAudit) book(name string, stated int, chapters map[int][]Verse, orphans map[int][]OrphanFootnote) {
	a.books++
	if stated <= 0 {
		a.unstated++ // nothing to check against; reported once, in the summary
		return
	}
	decoded, omitted := verseTally(chapters, orphans)
	if decoded+omitted == stated {
		return
	}
	a.mismatch++
	a.logf("bibletext: verse count: %s %s: feed states %d, decoded %d + %d omitted = %d (%+d)",
		a.edition, name, stated, decoded, omitted, decoded+omitted, decoded+omitted-stated)
}

// report says once what the whole edition did, and says NOTHING when everything
// reconciled. A check that speaks on every successful download teaches its
// reader to skip past it, and then it is not a check any more.
func (a *verseCountAudit) report() {
	if a.mismatch > 0 {
		a.logf("bibletext: verse count: %s: %d of %d books do not add up",
			a.edition, a.mismatch, a.books)
	}
	if a.unstated > 0 {
		a.logf("bibletext: verse count: %s: %d of %d books state no total",
			a.edition, a.unstated, a.books)
	}
}

// verseTally is what a decoded book actually produced: the verses a reader
// gets, and the distinct verse numbers whose words the translation omits.
func verseTally(chapters map[int][]Verse, orphans map[int][]OrphanFootnote) (decoded, omitted int) {
	for _, verses := range chapters {
		decoded += len(verses)
	}
	for _, notes := range orphans {
		seen := make(map[int]struct{}, len(notes))
		for _, n := range notes {
			seen[n.Verse] = struct{}{}
		}
		omitted += len(seen)
	}
	return decoded, omitted
}
