package bibletext

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseGospelRef(t *testing.T) {
	cases := []struct {
		in   string
		want []gSpan
	}{
		{"1:1-4", []gSpan{{1, 1, 1, 4}}},
		{"5:13-16", []gSpan{{5, 13, 5, 16}}},
		{"1:14a", []gSpan{{1, 14, 1, 14}}},                         // verse-part letter ignored
		{"1:14b-15", []gSpan{{1, 14, 1, 15}}},                      // letter on start of range
		{"6:1-6a", []gSpan{{6, 1, 6, 6}}},                          // letter on end of range
		{"8:34-9:1", []gSpan{{8, 34, 9, 1}}},                       // cross-chapter
		{"7:53-8:11", []gSpan{{7, 53, 8, 11}}},                     // cross-chapter
		{"15:18-16:4", []gSpan{{15, 18, 16, 4}}},                   // cross-chapter
		{"6:27-28,32-36", []gSpan{{6, 27, 6, 28}, {6, 32, 6, 36}}}, // 2nd inherits chapter
		{"22:54a,63-71", []gSpan{{22, 54, 22, 54}, {22, 63, 22, 71}}},
		{"11:12-14,20-26", []gSpan{{11, 12, 11, 14}, {11, 20, 11, 26}}},
		{"18:15-18,25-27", []gSpan{{18, 15, 18, 18}, {18, 25, 18, 27}}},
		{"2:21", []gSpan{{2, 21, 2, 21}}}, // single verse
	}
	for _, tc := range cases {
		got := parseGospelRef(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("%q: got %d spans %v, want %d %v", tc.in, len(got), got, len(tc.want), tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%q span %d: got %v, want %v", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestGospelParallelsLoadAndLookup(t *testing.T) {
	gospelOnce.Do(loadGospelParallels)
	if len(gospelPericopes) < 200 {
		t.Fatalf("expected the synopsis to load (~267 pericopes), got %d", len(gospelPericopes))
	}

	// Every non-null Gospel ref in the dataset must have parsed to at least one span
	// (a silent parse failure would drop a column). The dataset has
	// gospelSynopsisColumns such refs, which the next major release's synopsis
	// raises (parallels_current_test.go, parallels_next_test.go).
	cols := 0
	for _, p := range gospelPericopes {
		for _, spans := range p.spans {
			if len(spans) == 0 {
				t.Errorf("pericope %q has an empty span list", p.title)
			}
			cols++
		}
	}
	if cols != gospelSynopsisColumns {
		t.Errorf("expected %d parsed Gospel columns, got %d (a ref failed to parse)", gospelSynopsisColumns, cols)
	}

	// The Beatitudes: Matthew 5:1-12 ‖ Luke 6:20-23. A verse inside the Matthew
	// passage should surface the Luke parallel (and no Matthew self-reference).
	got := gospelParallelsForVerse("Matthew", 5, 3)
	var sawLuke, sawSelf bool
	for _, c := range got {
		if c.Book == "Matthew" {
			sawSelf = true
		}
		if c.Book == "Luke" && c.Chapter == 6 && c.Verse == 20 && c.EndV == 23 {
			sawLuke = true
		}
		if !c.Parallel {
			t.Errorf("parallel ref not tagged Parallel: %+v", c)
		}
	}
	if !sawLuke {
		t.Errorf("Matthew 5:3 should yield the Luke 6:20-23 parallel; got %v", got)
	}
	if sawSelf {
		t.Errorf("a verse's parallels must exclude its own Gospel; got %v", got)
	}

	// Cross-chapter span containment: Mark 8:34-9:1 (Taking up the cross). Both a
	// verse in ch8 and the boundary verse 9:1 must resolve to the same pericope's
	// parallels (Matthew + Luke present).
	for _, ref := range []struct{ ch, v int }{{8, 35}, {9, 1}} {
		p := gospelParallelsForVerse("Mark", ref.ch, ref.v)
		if len(p) == 0 {
			t.Errorf("Mark %d:%d should fall inside the cross-chapter pericope", ref.ch, ref.v)
		}
	}

	// A non-Gospel verse has no parallels.
	if p := gospelParallelsForVerse("Genesis", 1, 1); p != nil {
		t.Errorf("non-Gospel verse should have no parallels, got %v", p)
	}
}

// webGospelChapterEnds is the last verse of each chapter of the four Gospels
// in the WEB, the numbering the parallels data is written in. The WEB has
// no Luke 17:36 (webGospelGaps).
var webGospelChapterEnds = map[string][]int{
	"Matthew": {25, 23, 17, 25, 48, 34, 29, 34, 38, 42, 30, 50, 58, 36, 39, 28, 27, 35, 30, 34, 46, 46, 39, 51, 46, 75, 66, 20},
	"Mark":    {45, 28, 35, 41, 43, 56, 37, 38, 50, 52, 33, 44, 37, 72, 47, 20},
	"Luke":    {80, 52, 38, 44, 39, 49, 50, 56, 62, 42, 54, 59, 35, 35, 32, 31, 37, 43, 48, 47, 38, 71, 56, 53},
	"John":    {51, 25, 36, 54, 47, 71, 53, 59, 41, 42, 57, 50, 38, 31, 27, 33, 26, 40, 42, 31, 25},
}

var webGospelGaps = map[verseRef]bool{{"Luke", 17, 36}: true}

// eachWEBGospelVerse calls f for every verse of the WEB's four Gospels.
func eachWEBGospelVerse(f func(book string, ch, v int)) {
	for _, g := range gospelColumns {
		for i, last := range webGospelChapterEnds[g.book] {
			for v := 1; v <= last; v++ {
				if !webGospelGaps[verseRef{g.book, i + 1, v}] {
					f(g.book, i+1, v)
				}
			}
		}
	}
}

// THE TWENTY-TWO VERSES. Twenty-two verses of the WEB's four Gospels belong
// to no synopsis set in the shipping build: Matthew 4:23-25, Luke 6:17-19,
// 6:24-26, 6:43-45 and 21:37-38, John 11:55-57 and 13:31-35. The next major
// release places them (gospel_parallels_next.json), and then every verse
// belongs to one (gospelVersesInNoSet).
func TestTheGospelVersesInNoSynopsisSet(t *testing.T) {
	gospelOnce.Do(loadGospelParallels)
	total := 0
	var uncovered []string
	eachWEBGospelVerse(func(book string, ch, v int) {
		total++
		for _, p := range gospelPericopes {
			if spansContain(p.spans[book], ch, v) {
				return
			}
		}
		uncovered = append(uncovered, fmt.Sprintf("%s %d:%d", book, ch, v))
	})
	// CONTROL: the table is the WEB's whole four Gospels.
	if total != 1071+678+1150+879 {
		t.Fatalf("the verse table holds %d verses, want the WEB's 3,778", total)
	}
	if got, want := strings.Join(uncovered, ", "), strings.Join(gospelVersesInNoSet, ", "); got != want {
		t.Errorf("the Gospel verses in no synopsis set are\n  %s\nwant\n  %s", got, want)
	}
}
