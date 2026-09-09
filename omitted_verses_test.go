package bibletext

import (
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"testing"
)

// The table names the verses each translation omits, and it is only worth
// anything if it still matches the feeds it was generated from. These check
// both directions: that the table says what the publishers' feeds say, and that
// it agrees with the versification data, which derives the same fact by an
// entirely different route.

func TestTheOmissionTableIsNotEmpty(t *testing.T) {
	total := 0
	for _, byBook := range omittedVerses {
		for _, byChapter := range byBook {
			for _, vs := range byChapter {
				total += len(vs)
			}
		}
	}
	// 58 across three editions when generated. A table that silently emptied
	// would make every lookup answer "nothing is omitted", which is the failure
	// this whole item exists to prevent.
	if total < 50 {
		t.Fatalf("the omission table holds %d verses; it has emptied or the generator "+
			"has drifted — re-run scripts/gen-omitted-verses.py", total)
	}
	t.Logf("%d omitted verses across %d editions", total, len(omittedVerses))
}

// The verses everyone knows: the classic critical-text omissions. If the table
// lost these it lost its reason to exist.
func TestTheTableKnowsTheClassicOmissions(t *testing.T) {
	for _, tc := range []struct {
		edition, book string
		chapter, v    int
	}{
		{"bsb", "Matthew", 17, 21},
		{"bsb", "Matthew", 18, 11},
		{"bsb", "Mark", 9, 44},
		{"bsb", "Mark", 9, 46},
		{"bsb", "John", 5, 4},
		{"bsb", "Acts", 8, 37},
		{"bsb", "Romans", 16, 24},
		{"web", "Luke", 17, 36},
		{"web", "Acts", 15, 34},
		{"webc", "Luke", 17, 36},
	} {
		if !omitsVerse(tc.edition, tc.book, tc.chapter, tc.v) {
			t.Errorf("%s does not record %s %d:%d as omitted",
				tc.edition, tc.book, tc.chapter, tc.v)
		}
	}
}

// CONTROL: a verse that is PRESENT must not be reported as omitted, and an
// edition with no table must answer nothing rather than guessing.
func TestThePresentVersesAreNotReportedAsOmitted(t *testing.T) {
	for _, tc := range []struct {
		edition, book string
		chapter, v    int
	}{
		{"bsb", "John", 3, 16},
		{"bsb", "Matthew", 17, 20}, // the verse before an omission
		{"bsb", "Matthew", 17, 22}, // and the one after
		{"web", "Luke", 17, 35},
		{"web", "Luke", 17, 37},
		{"nkjv", "Luke", 17, 36}, // the licensed edition has no table at all
		{"nkjv", "John", 3, 16},
	} {
		if omitsVerse(tc.edition, tc.book, tc.chapter, tc.v) {
			t.Errorf("%s wrongly records %s %d:%d as omitted",
				tc.edition, tc.book, tc.chapter, tc.v)
		}
	}
	if got := omittedVersesIn("nkjv", "Luke", 17); got != nil {
		t.Errorf("the licensed edition returned %v; it has no table and must answer nothing", got)
	}
	if got := omittedVersesIn("bsb", "NotABook", 1); got != nil {
		t.Errorf("an unknown book returned %v", got)
	}
}

// Greek Esther's numbering corresponds to nothing, so its gaps are not
// omissions and must never be marked as such.
func TestGreekEsthersGapsAreNotCalledOmissions(t *testing.T) {
	for _, v := range []int{6} {
		if omitsVerse("webc", "Esther", 4, v) {
			t.Errorf("Esther 4:%d is recorded as omitted; Greek Esther is incommensurable "+
				"and a gap in it is not an omission", v)
		}
	}
	if got := omittedVersesIn("webc", "Esther", 9); got != nil {
		t.Errorf("Greek Esther chapter 9 reports omissions %v", got)
	}
}

// The second derivation. versification_data.go records, independently, the
// verses the reference has that an edition lacks; every one must be a hole
// here. Two routes to the same fact, and a disagreement means one is wrong.
func TestTheTableAgreesWithTheVersificationData(t *testing.T) {
	src, err := os.ReadFile("versification_data.go")
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile(`(?s)"(\w+)": \{\s*absent: \[\]verseRef\{(.*?)\},\s*moved`)
	ref := regexp.MustCompile(`\{"([^"]+)", (\d+), (\d+)\}`)
	checked := 0
	for _, m := range block.FindAllStringSubmatch(string(src), -1) {
		edition := m[1]
		for _, r := range ref.FindAllStringSubmatch(m[2], -1) {
			book := r[1]
			ch, err := strconv.Atoi(r[2])
			if err != nil {
				t.Fatal(err)
			}
			v, err := strconv.Atoi(r[3])
			if err != nil {
				t.Fatal(err)
			}
			checked++
			if !omitsVerse(edition, book, ch, v) {
				t.Errorf("versification_data.go says %s lacks %s %d:%d, but the omission "+
					"table does not record it", edition, book, ch, v)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no absent verses were read from versification_data.go; this test proves nothing")
	}
	t.Logf("cross-checked %d verses against the versification data", checked)
}

// And against the feeds themselves, where they are available: the table must
// name every interior hole and no other.
func TestTheTableMatchesTheFeeds(t *testing.T) {
	for _, tc := range []struct{ edition, path string }{
		{"bsb", "build/biblecache/bsb.json"},
		{"web", "build/biblecache/web.json"},
	} {
		body, err := os.ReadFile(tc.path)
		if err != nil {
			t.Skipf("%s not present (build/ is gitignored); skipping", tc.path)
		}
		var doc struct {
			Books []helloAOBook `json:"books"`
		}
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatal(err)
		}
		holes := 0
		for _, b := range doc.Books {
			book := canonicalNameForUSFM(b.Order)
			if book == "" {
				continue
			}
			for _, w := range b.Chapters {
				var printed []int
				for _, node := range w.Chapter.Content {
					var head struct {
						Type    string            `json:"type"`
						Number  int               `json:"number"`
						Content []json.RawMessage `json:"content"`
					}
					if json.Unmarshal(node, &head) != nil || head.Type != "verse" {
						continue
					}
					if text, _, _ := bsbVerseTextMarkedLevels(head.Content); text != "" {
						printed = append(printed, head.Number)
					}
				}
				if len(printed) == 0 {
					continue
				}
				lo, hi := printed[0], printed[0]
				have := map[int]bool{}
				for _, n := range printed {
					have[n] = true
					if n < lo {
						lo = n
					}
					if n > hi {
						hi = n
					}
				}
				for v := lo; v <= hi; v++ {
					if have[v] {
						continue
					}
					holes++
					if !omitsVerse(tc.edition, book, w.Chapter.Number, v) {
						t.Errorf("%s %s %d:%d is a hole in the feed and is not in the table",
							tc.edition, book, w.Chapter.Number, v)
					}
				}
			}
		}
		if holes == 0 {
			t.Fatalf("%s: no holes found in the feed; this test proves nothing", tc.path)
		}
		t.Logf("%s: %d holes in the feed, all in the table", tc.edition, holes)
	}
}
