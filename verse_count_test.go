package bibletext

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// The verse-count check exists to catch a book that arrived short. The thing
// worth testing is therefore not that it stays quiet on good data — silence is
// cheap and proves nothing — but that it SPEAKS on bad data, and names the
// shortfall correctly.

// recorder stands in for log.Printf so a test can read what the audit said.
type recorder struct{ lines []string }

func (r *recorder) logf(format string, v ...any) {
	r.lines = append(r.lines, fmt.Sprintf(format, v...))
}

func (r *recorder) joined() string { return strings.Join(r.lines, "\n") }

func TestTheVerseCountAuditIsSilentWhenABookAddsUp(t *testing.T) {
	r := &recorder{}
	a := newVerseCountAudit("WEB")
	a.logf = r.logf

	// Three verses decoded, one omitted, feed states four.
	a.book("Luke", 4,
		map[int][]Verse{1: {{Verse: 1}, {Verse: 2}}, 2: {{Verse: 1}}},
		map[int][]OrphanFootnote{17: {{Verse: 36, Text: "omitted"}}})
	a.report()

	if len(r.lines) != 0 {
		t.Errorf("the audit spoke on a book that reconciles:\n%s", r.joined())
	}
}

// CONTROL: the whole point. A short book must produce a line naming the book
// and the size of the shortfall, or this check would pass a truncated Bible.
func TestTheVerseCountAuditCatchesATruncatedBook(t *testing.T) {
	r := &recorder{}
	a := newVerseCountAudit("BSB")
	a.logf = r.logf

	// The feed says 1,533; the decode produced 1,000 and no omissions.
	chapters := map[int][]Verse{}
	for i := 1; i <= 1000; i++ {
		chapters[i] = []Verse{{Verse: 1}}
	}
	a.book("Genesis", 1533, chapters, nil)
	a.report()

	got := r.joined()
	for _, want := range []string{"BSB", "Genesis", "1533", "1000", "-533"} {
		if !strings.Contains(got, want) {
			t.Errorf("the shortfall report does not mention %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "1 of 1 books do not add up") {
		t.Errorf("no summary line:\n%s", got)
	}
}

// An omitted verse can carry more than one note body. The feed counts the
// VERSE once, so the audit must count distinct verse numbers — not orphan
// records — or a verse with two notes would look like two verses and mask a
// real shortfall of one.
func TestOmittedVersesAreCountedOncePerVerseNotPerNote(t *testing.T) {
	decoded, omitted := verseTally(
		map[int][]Verse{1: {{Verse: 1}}},
		map[int][]OrphanFootnote{
			17: {
				{Verse: 36, Text: "first body"},
				{Verse: 36, Text: "second body on the SAME verse"},
			},
		})
	if decoded != 1 {
		t.Errorf("decoded = %d, want 1", decoded)
	}
	if omitted != 1 {
		t.Errorf("omitted = %d, want 1 — two notes on one omitted verse is one "+
			"missing verse, and counting records instead would hide a shortfall", omitted)
	}
}

// A feed that states nothing cannot be reconciled, and saying so once is the
// honest answer. It must not be mistaken for agreement.
func TestABookThatStatesNoTotalIsReportedSeparately(t *testing.T) {
	r := &recorder{}
	a := newVerseCountAudit("")
	a.logf = r.logf
	a.book("Genesis", 0, map[int][]Verse{1: {{Verse: 1}}}, nil)
	a.report()

	got := r.joined()
	if !strings.Contains(got, "state no total") {
		t.Errorf("an unstated total was not reported:\n%s", got)
	}
	if strings.Contains(got, "do not add up") {
		t.Errorf("an unstated total was reported as a mismatch:\n%s", got)
	}
	if !strings.Contains(got, "helloao") {
		t.Errorf("an unnamed edition should fall back to a placeholder:\n%s", got)
	}
}

// The identity the check rests on, against the real feeds. Skips in CI, where
// build/ does not exist — which is why the synthetic cases above carry the
// weight and this one only confirms the premise still holds upstream.
func TestTheFeedsOwnVerseCountsStillReconcile(t *testing.T) {
	for _, tc := range []struct {
		path   string
		decode func([]byte) (*BibleData, error)
	}{
		{"build/biblecache/bsb.json", decodeCanonical66},
		{"build/biblecache/webc.json", decodeHelloAOCatholic},
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
		stated := 0
		for _, b := range doc.Books {
			if b.TotalNumberOfVerses == 0 {
				t.Errorf("%s: book %s states no verse total; the check would be blind here",
					tc.path, b.ID)
			}
			stated += b.TotalNumberOfVerses
		}
		if stated == 0 {
			t.Fatalf("%s: no book stated a total — the field name has drifted and "+
				"this test proves nothing", tc.path)
		}

		bd, err := tc.decode(body)
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		got := 0
		for book, chapters := range bd.Verses {
			d, o := verseTally(chapters, bd.OrphanFootnotes[book])
			got += d + o
		}
		if got != stated {
			t.Errorf("%s: feed states %d verses, decode accounts for %d (%+d)",
				tc.path, stated, got, got-stated)
		}
	}
}
