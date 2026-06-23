package bibletext

import "testing"

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
	// (a silent parse failure would drop a column). The dataset has 469 such refs.
	cols := 0
	for _, p := range gospelPericopes {
		for _, spans := range p.spans {
			if len(spans) == 0 {
				t.Errorf("pericope %q has an empty span list", p.title)
			}
			cols++
		}
	}
	if cols != 469 {
		t.Errorf("expected 469 parsed Gospel columns, got %d (a ref failed to parse)", cols)
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
