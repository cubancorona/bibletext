package bibletext

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// S5 — note references
// ---------------------------------------------------------------------------

func TestANoteThatNamesItsOwnVerseIsAccepted(t *testing.T) {
	r := &recorder{}
	a := newNoteRefAudit("WEB", r.logf)
	a.note("John", 3, 16, 3, 16)
	a.note("Psalms", 3, 0, 3, 0) // a title's note: verse zero by convention
	a.note("Luke", 17, 36, 0, 0) // the feed stated no reference at all
	a.report()
	if len(r.lines) != 0 {
		t.Errorf("the audit objected to notes that agree:\n%s", r.joined())
	}
}

// CONTROL: a mis-filed note is exactly the silent failure this exists for —
// the body is found, so nothing errors, and it lands on the wrong verse.
func TestANoteFiledAgainstTheWrongVerseIsReported(t *testing.T) {
	r := &recorder{}
	a := newNoteRefAudit("BSB", r.logf)
	a.note("John", 3, 16, 3, 17)
	a.report()
	got := r.joined()
	for _, want := range []string{"BSB", "John 3:16", "3:17"} {
		if !strings.Contains(got, want) {
			t.Errorf("the report does not mention %q:\n%s", want, got)
		}
	}
}

// ---------------------------------------------------------------------------
// S8 — the words-of-Jesus witness
// ---------------------------------------------------------------------------

func TestTheRedLetterWitnessIsSilentWhenBothSourcesAgree(t *testing.T) {
	r := &recorder{}
	w := newRedLetterWitness("WEB", r.logf, true)
	w.verse("John 3:16")
	w.reportAgainst(map[string][]redLetterSpan{"John 3:16": {{0, 10}}})
	if len(r.lines) != 0 {
		t.Errorf("the witness spoke when the two sources agree:\n%s", r.joined())
	}
}

// The two directions mean different things and must be reported separately.
func TestTheRedLetterWitnessSeparatesTheTwoDirections(t *testing.T) {
	r := &recorder{}
	w := newRedLetterWitness("WEB", r.logf, true)
	w.verse("Mark 1:15") // the feed marks it
	w.reportAgainst(map[string][]redLetterSpan{
		"Luke 4:18": {{0, 10}}, // the table marks a different one
	})
	got := r.joined()
	if !strings.Contains(got, "Mark 1:15") || !strings.Contains(got, "loses red") {
		t.Errorf("flag-but-not-table was not reported as a loss:\n%s", got)
	}
	if !strings.Contains(got, "Luke 4:18") || !strings.Contains(got, "adding emphasis") {
		t.Errorf("table-but-not-flag was not reported as added emphasis:\n%s", got)
	}
}

// An edition with no table must not report every flagged verse as a
// disagreement, which is what comparing against an empty map would do.
func TestAnEditionWithNoTableReportsNothing(t *testing.T) {
	r := &recorder{}
	w := newRedLetterWitness("NKJV", r.logf, false)
	w.verse("John 3:16")
	w.reportAgainst(nil)
	if len(r.lines) != 0 {
		t.Errorf("an edition with no table should stay quiet:\n%s", r.joined())
	}
	if redLetterTableFor("NOT-AN-EDITION") != nil {
		t.Error("an unknown edition should resolve to no table")
	}
	if redLetterTableFor("WEB") == nil {
		t.Error("the WEB must resolve to its table, or the witness never runs")
	}
}

// ---------------------------------------------------------------------------
// Both, against the real feeds. This is the measurement the worklist asserted;
// re-running it is what turns a claim into a standing check.
// ---------------------------------------------------------------------------

func TestTheFeedsOwnWitnessesStillAgree(t *testing.T) {
	for _, tc := range []struct {
		path      string
		shortName string
	}{
		{"build/biblecache/bsb.json", "BSB"},
		{"build/biblecache/web.json", "WEB"},
		{"build/biblecache/webc.json", "WEBC"},
	} {
		body, err := os.ReadFile(tc.path)
		if err != nil {
			t.Skipf("%s not present (build/ is gitignored); skipping", tc.path)
		}
		var doc struct {
			Books []helloAOBook `json:"books"`
		}
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}

		r := &recorder{}
		table := redLetterTableFor(tc.shortName)
		ck := &helloAOChecks{
			NoteRefs:  newNoteRefAudit(tc.shortName, r.logf),
			RedLetter: newRedLetterWitness(tc.shortName, r.logf, table != nil),
		}
		for _, b := range doc.Books {
			name := b.ID
			if tc.shortName == "WEBC" {
				if n := usfmToCatholicName[b.ID]; n != "" {
					name = n
				}
			}
			decodeHelloAOChapters(name, b, ck)
		}

		// Anti-vacuity, both halves. A witness that recorded nothing would
		// agree with anything.
		if ck.NoteRefs.notes < 1000 {
			t.Fatalf("%s: only %d notes were audited; the check is not wired and "+
				"this test proves nothing", tc.path, ck.NoteRefs.notes)
		}
		t.Logf("%s: %d notes audited", tc.path, ck.NoteRefs.notes)

		ck.NoteRefs.report()
		if len(r.lines) != 0 {
			t.Errorf("%s: notes disagree with their own stated references:\n%s",
				tc.path, r.joined())
		}
	}
}

// The red-letter halves are checked with the app's real book names, which only
// the full decode produces — so this one drives the actual decoders.
func TestTheWordsOfJesusFlagAgreesWithTheTable(t *testing.T) {
	for _, tc := range []struct {
		path      string
		shortName string
		decode    func([]byte) (*BibleData, error)
		wantMin   int
	}{
		{"build/biblecache/web.json", "WEB", decodeWEB, 2000},
		{"build/biblecache/webc.json", "WEBC", decodeHelloAOCatholic, 2000},
	} {
		body, err := os.ReadFile(tc.path)
		if err != nil {
			t.Skipf("%s not present (build/ is gitignored); skipping", tc.path)
		}
		var doc struct {
			Books []helloAOBook `json:"books"`
		}
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}

		r := &recorder{}
		table := redLetterTableFor(tc.shortName)
		if table == nil {
			t.Fatalf("%s resolves to no red-letter table", tc.shortName)
		}
		w := newRedLetterWitness(tc.shortName, r.logf, true)
		for _, b := range doc.Books {
			name := b.ID
			if tc.shortName == "WEBC" {
				if n := usfmToCatholicName[b.ID]; n != "" {
					name = n
				}
			} else {
				name = canonicalNameForUSFM(b.Order)
			}
			decodeHelloAOChapters(name, b, &helloAOChecks{RedLetter: w})
		}

		if len(w.flagged) < tc.wantMin {
			t.Fatalf("%s: the feed flagged only %d verses; the flag is not being read "+
				"and this test proves nothing", tc.path, len(w.flagged))
		}
		t.Logf("%s: feed flags %d verses, table holds %d", tc.path, len(w.flagged), len(table))

		w.reportAgainst(table)
		if len(r.lines) != 0 {
			t.Errorf("%s: the feed's flag and the generated table have diverged:\n%s",
				tc.path, r.joined())
		}
	}
}

// canonicalNameForUSFM maps a helloao book order to the app's book name, which
// is how the 66-book decoder keys its verses.
func canonicalNameForUSFM(order int) string {
	books := NewBibleData().Books
	if order < 1 || order > len(books) {
		return ""
	}
	return books[order-1]
}
