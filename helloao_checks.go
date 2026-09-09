package bibletext

// Two more decode-time witnesses over the helloao feeds — S5 and S8 of
// docs/SCRIPTURE_WORKLIST.md. Both observe and report; neither changes a
// decode, fails a fetch, or drops anything a reader would otherwise see.

import (
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// S5 — every note says which verse it belongs to. Check that it agrees.
// ---------------------------------------------------------------------------

// noteRefAudit compares each note body's own stated chapter and verse against
// the position of the marker that pointed at it.
//
// The feed states the reference on the body, and the marker's position is
// worked out independently from where {"noteId":N} stood in the verse. They
// agree for all 7,752 notes across the three editions today, with a note in a
// Psalm title marked by verse zero. Note ids run continuously across a BOOK
// rather than per chapter, which is exactly the condition under which a future
// change that resolved a marker against the wrong chapter would mis-file a note
// SILENTLY — the body would still be found, and it would be attached to the
// wrong verse.
//
// The note is KEPT on disagreement, not dropped. Dropping it would be a
// reader-visible change, which Stage 2 forbids, and the feed's reference is not
// automatically the more trustworthy of the two: all we actually know when they
// disagree is that one of them is wrong.
type noteRefAudit struct {
	edition   string
	logf      func(format string, v ...any)
	notes     int
	disagreed int
	examples  []string
}

func newNoteRefAudit(edition string, logf func(string, ...any)) *noteRefAudit {
	if edition == "" {
		edition = "helloao"
	}
	return &noteRefAudit{edition: edition, logf: logf}
}

// note records one resolved marker. wantChapter/wantVerse are where the marker
// actually stood; gotChapter/gotVerse are what the body claims. A body that
// states nothing at all (both zero) is not a disagreement — the Psalm-title
// notes legitimately carry verse zero, and a body with no reference block at
// all is simply a feed that does not send one.
func (a *noteRefAudit) note(book string, wantChapter, wantVerse, gotChapter, gotVerse int) {
	if a == nil {
		return
	}
	a.notes++
	if gotChapter == 0 && gotVerse == 0 {
		return // the feed stated no reference; nothing to disagree with
	}
	if gotChapter == wantChapter && gotVerse == wantVerse {
		return
	}
	a.disagreed++
	if len(a.examples) < 5 {
		a.examples = append(a.examples,
			book+" "+itoa(wantChapter)+":"+itoa(wantVerse)+
				" (the note says "+itoa(gotChapter)+":"+itoa(gotVerse)+")")
	}
}

func (a *noteRefAudit) report() {
	if a == nil || a.disagreed == 0 {
		return
	}
	a.logf("bibletext: note references: %s: %d of %d notes sit at a verse they do not name: %s",
		a.edition, a.disagreed, a.notes, strings.Join(a.examples, "; "))
}

// ---------------------------------------------------------------------------
// S8 — the feed's own words-of-Jesus flag, as a second witness.
// ---------------------------------------------------------------------------

// redLetterWitness compares the WEB feed's per-item wordsOfJesus flag against
// the generated red-letter table.
//
// The table is the SOURCE of red and stays so: it is built offline from
// eBible's published USFM (\wj), it carries rune offsets rather than a bare
// per-verse boolean, and it ships with the rune counts and hashes that catch
// runtime text drift. The feed's flag can only say "this run is Jesus", which
// is strictly less. Nothing here changes what is drawn.
//
// What the flag is good for is being a SECOND, INDEPENDENT derivation of the
// same fact. eBible's USFM and helloao's flag are two upstreams; they agree on
// all 2,059 WEB verses today. If they ever stop agreeing, one of them changed,
// and without this nothing would say so — the app would keep painting from the
// table and the divergence would sit there unnoticed.
//
// Both directions matter and mean different things. Flag-but-not-table is the
// dangerous one: the feed thinks words are Jesus's and we are not colouring
// them, so a reader loses red they should have. Table-but-not-flag means we are
// colouring words the feed no longer marks, which is the direction in which we
// would be adding editorial emphasis of our own.
type redLetterWitness struct {
	edition string
	logf    func(format string, v ...any)
	flagged map[string]bool // "Book c:v" the feed marked
	active  bool            // false for editions with no table to compare against
}

func newRedLetterWitness(edition string, logf func(string, ...any), active bool) *redLetterWitness {
	return &redLetterWitness{
		edition: edition,
		logf:    logf,
		flagged: make(map[string]bool, 2200),
		active:  active,
	}
}

// verse records that the feed flagged at least one run of this verse.
func (w *redLetterWitness) verse(ref string) {
	if w == nil || !w.active {
		return
	}
	w.flagged[ref] = true
}

// reportAgainst compares what the feed flagged with what the table holds.
func (w *redLetterWitness) reportAgainst(table map[string][]redLetterSpan) {
	if w == nil || !w.active || len(w.flagged) == 0 {
		return
	}
	var flagOnly, tableOnly []string
	for ref := range w.flagged {
		if _, ok := table[ref]; !ok {
			flagOnly = append(flagOnly, ref)
		}
	}
	for ref := range table {
		if !w.flagged[ref] {
			tableOnly = append(tableOnly, ref)
		}
	}
	if len(flagOnly) == 0 && len(tableOnly) == 0 {
		return
	}
	sort.Strings(flagOnly)
	sort.Strings(tableOnly)
	if len(flagOnly) > 0 {
		w.logf("bibletext: red-letter witness: %s: the feed marks %d verses the table does not "+
			"(a reader loses red we should be drawing): %s",
			w.edition, len(flagOnly), strings.Join(sample(flagOnly, 5), ", "))
	}
	if len(tableOnly) > 0 {
		w.logf("bibletext: red-letter witness: %s: the table marks %d verses the feed does not "+
			"(we would be adding emphasis the publisher no longer marks): %s",
			w.edition, len(tableOnly), strings.Join(sample(tableOnly, 5), ", "))
	}
}

func sample(all []string, n int) []string {
	if len(all) <= n {
		return all
	}
	return append(append([]string(nil), all[:n]...), "…")
}

// ---------------------------------------------------------------------------
// One bundle, so the decoder takes one parameter rather than three.
// ---------------------------------------------------------------------------

// helloAOChecks carries every decode-time witness for one edition's fetch.
// Nil is a working value: a caller with nothing to record — a test, the seed
// generator — passes nil and every method below does nothing.
type helloAOChecks struct {
	Census    *helloAOCensus
	NoteRefs  *noteRefAudit
	RedLetter *redLetterWitness
}

func (c *helloAOChecks) census() *helloAOCensus {
	if c == nil {
		return nil
	}
	return c.Census
}

func (c *helloAOChecks) noteRefs() *noteRefAudit {
	if c == nil {
		return nil
	}
	return c.NoteRefs
}

func (c *helloAOChecks) redLetter() *redLetterWitness {
	if c == nil {
		return nil
	}
	return c.RedLetter
}

// report emits every witness's summary, in a fixed order so a log is readable.
func (c *helloAOChecks) report(table map[string][]redLetterSpan) {
	if c == nil {
		return
	}
	c.Census.report()
	c.NoteRefs.report()
	c.RedLetter.reportAgainst(table)
}

// redLetterTableFor picks the generated table to weigh a feed's flag against.
//
// One decoder serves the BSB and the WEB, so the edition has to come from the
// feed's own name. An edition with no table gets nil and the witness goes
// inactive rather than reporting every flagged verse as a disagreement — which
// is what comparing against an empty map would do.
func redLetterTableFor(shortName string) map[string][]redLetterSpan {
	switch shortName {
	case "WEB":
		return webRedLetterSpans
	case "WEBC":
		return webcRedLetterSpans
	case "BSB":
		return bsbRedLetterSpans
	}
	return nil
}
