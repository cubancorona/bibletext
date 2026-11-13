package bibletext

import (
	"encoding/json"
	"fmt"
	"slices"
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
		// Malformed segments, malformed range ends, non-positive numbers, and
		// inverted ranges reject the whole reference rather than silently
		// truncating or dropping a span.
		{"", nil},
		{"5:1-bad", nil},
		{"5:1-", nil},
		{"5:1–12", nil},
		{"6:27-28,bad", nil},
		{"6:27-28,", nil},
		{"6:28-27", nil},
		{"9:1-8:34", nil},
		{"0:1", nil},
		{"1:0", nil},
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

// EVERY SPAN IN THE EMBEDDED PARALLEL FILES NAMES REAL, IN-ORDER VERSES.
func TestEveryGospelParallelSpanNamesRealVerses(t *testing.T) {
	gospelOnce.Do(loadGospelParallels)
	checkSpan := func(where, book string, s gSpan) {
		ends := webGospelChapterEnds[book]
		if len(ends) == 0 {
			t.Errorf("%s: unknown Gospel %q", where, book)
			return
		}
		if s.ch1 < 1 || s.ch1 > len(ends) || s.ch2 < 1 || s.ch2 > len(ends) {
			t.Errorf("%s: %s span %+v out of chapter bounds (1..%d)", where, book, s, len(ends))
			return
		}
		if s.ch2 < s.ch1 || (s.ch2 == s.ch1 && s.v2 < s.v1) {
			t.Errorf("%s: %s span %+v is inverted", where, book, s)
			return
		}
		if s.v1 < 1 || s.v1 > ends[s.ch1-1] || s.v2 < 1 || s.v2 > ends[s.ch2-1] {
			t.Errorf("%s: %s span %+v out of verse bounds (%d:%d..%d:%d)", where, book, s, s.ch1, ends[s.ch1-1], s.ch2, ends[s.ch2-1])
			return
		}
		for _, endpoint := range [2]verseRef{{book, s.ch1, s.v1}, {book, s.ch2, s.v2}} {
			if webGospelGaps[endpoint] {
				t.Errorf("%s: %s span %+v starts or ends on an omitted WEB verse %+v", where, book, s, endpoint)
			}
		}
	}
	spanCount := 0
	for i, p := range gospelPericopes {
		if p.title == "" {
			t.Errorf("pericope %d has an empty title", i)
		}
		for book, spans := range p.spans {
			for _, s := range spans {
				spanCount++
				checkSpan(fmt.Sprintf("pericope %q", p.title), book, s)
			}
		}
	}
	if spanCount != gospelSynopsisSpans {
		t.Errorf("checked %d synopsis spans, want %d", spanCount, gospelSynopsisSpans)
	}
	if len(gospelOccasionsJSON) > 0 {
		occasionOnce.Do(loadGospelOccasions)
		occSpans := 0
		for _, g := range occasionGroups {
			for _, p := range g.passages {
				for _, s := range p.spans {
					occSpans++
					checkSpan(fmt.Sprintf("occasion %q", g.title), p.book, s)
				}
			}
		}
		if occSpans < len(occasionGroups) {
			t.Errorf("checked %d occasion spans across %d groups", occSpans, len(occasionGroups))
		}
	}
}

// gospelEditionBible builds a four-Gospel BibleData whose verse set matches
// versionID's exact versification and omissions (versification_data.go,
// omitted_verses_data.go), needing no external cache file.
func gospelEditionBible(versionID string) *BibleData {
	bd := &BibleData{
		Books:  []string{"Matthew", "Mark", "Luke", "John"},
		Verses: map[string]map[int][]Verse{},
	}
	present := map[string]map[int]map[int]bool{}
	add := func(book string, ch, v int) {
		if present[book] == nil {
			present[book] = map[int]map[int]bool{}
		}
		if present[book][ch] == nil {
			present[book][ch] = map[int]bool{}
		}
		present[book][ch][v] = true
	}
	eachWEBGospelVerse(func(book string, ch, v int) {
		if outCh, outV, res := crossRefPrintedIn(versionID, book, ch, v); res != verseMapAbsent && res != verseMapIncommensurable {
			add(book, outCh, outV)
		}
	})
	for _, e := range versificationDeltas[versionID].extra {
		if isGospelBook(e.Book) && !omitsVerse(versionID, e.Book, e.Chapter, e.Verse) {
			add(e.Book, e.Chapter, e.Verse)
		}
	}
	for _, book := range bd.Books {
		bd.Verses[book] = map[int][]Verse{}
		for ch := 1; ch <= len(webGospelChapterEnds[book]); ch++ {
			vmap := present[book][ch]
			nums := make([]int, 0, len(vmap))
			for v := range vmap {
				nums = append(nums, v)
			}
			slices.Sort(nums)
			verses := make([]Verse, 0, len(nums))
			for _, v := range nums {
				verses = append(verses, Verse{BookName: book, Book: book, Chapter: ch, Verse: v, Text: "Synthetic verse."})
			}
			bd.Verses[book][ch] = verses
		}
	}
	return bd
}

// EVERY GOSPEL PANEL MATCHES THE EMBEDDED SYNOPSIS ACROSS ALL FOUR EDITIONS.
// Checks the raw synopsis structure (pericope count, 17 contiguous section runs,
// single-Gospel pericopes, zero dropped columns) and holds all 15,103 single-verse
// Gospel panels across WEB, WEB Catholic, BSB and NKJV to the synopsis model:
// zero lost rows, zero lost pericope titles, zero self-Gospel leaks, every
// emitted endpoint present in the edition, and BSB range-end clamping on
// Matthew 17:14–20 and Mark 11:20–25.
func TestGospelParallelsPanelCensusAcrossEditions(t *testing.T) {
	gospelOnce.Do(loadGospelParallels)

	var raw []struct {
		Section string `json:"section"`
		Title   string `json:"title"`
		Refs    struct {
			Matthew *string `json:"matthew"`
			Mark    *string `json:"mark"`
			Luke    *string `json:"luke"`
			John    *string `json:"john"`
		} `json:"refs"`
	}
	if err := json.Unmarshal(gospelParallelsJSON, &raw); err != nil {
		t.Fatalf("unmarshal gospelParallelsJSON: %v", err)
	}
	if len(raw) != gospelSynopsisPericopes || len(gospelPericopes) != gospelSynopsisPericopes {
		t.Fatalf("raw=%d loaded=%d pericopes, want %d", len(raw), len(gospelPericopes), gospelSynopsisPericopes)
	}
	sectionRuns, prevSection, singleGospel := 0, "", 0
	for i, r := range raw {
		sec := strings.TrimSpace(r.Section)
		if sec == "" {
			t.Errorf("pericope %d (%q) has an empty section", i, r.Title)
		} else if sec != prevSection {
			sectionRuns++
			prevSection = sec
		}
		cols := 0
		for _, cell := range []*string{r.Refs.Matthew, r.Refs.Mark, r.Refs.Luke, r.Refs.John} {
			if cell != nil {
				cols++
				if len(parseGospelRef(*cell)) == 0 {
					t.Errorf("pericope %d (%q) dropped ref %q", i, r.Title, *cell)
				}
			}
		}
		if cols == 1 {
			singleGospel++
		}
	}
	if sectionRuns != gospelSynopsisSections {
		t.Errorf("counted %d contiguous section runs, want %d", sectionRuns, gospelSynopsisSections)
	}
	if singleGospel != gospelSynopsisSingleGospel {
		t.Errorf("counted %d single-Gospel pericopes, want %d", singleGospel, gospelSynopsisSingleGospel)
	}

	totalPanels := 0
	wantPanelsByEdition := map[string]int{
		"web":  3778,
		"webc": 3778,
		"bsb":  3768,
		"nkjv": 3779,
	}
	sawBSBClampMatt, sawBSBClampMark := false, false
	for _, id := range []string{"web", "webc", "bsb", "nkjv"} {
		bd := gospelEditionBible(id)
		resolve := crossRefResolver(bd.Books, id)
		panels := 0
		for _, book := range bd.Books {
			for _, ch := range bd.GetChapterNumbersForBook(book) {
				st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: ch, CurrentVersion: id}
				for _, v := range bd.GetChapter(book, ch) {
					panels++
					totalPanels++
					var got []crossRef
					for _, c := range localCrossRefsForSelection(st, "", selSpan{lo: v.Verse, hi: v.Verse}) {
						if !c.otherOccasion() {
							got = append(got, c)
						}
					}
					var want []crossRef
					if srcCh, srcV, ok := crossRefSourceRef(id, v); ok {
						seen := map[string]bool{}
						for i := range gospelPericopes {
							p := &gospelPericopes[i]
							if !spansContain(p.spans[book], srcCh, srcV) {
								continue
							}
							for _, g := range gospelColumns {
								if g.book == book {
									continue
								}
								for _, s := range p.spans[g.book] {
									for _, m := range resolve(spanToCrossRef(g.book, s, p.title)) {
										lbl := m.label()
										if !seen[lbl] {
											seen[lbl] = true
											want = append(want, m)
										}
									}
								}
							}
						}
					}
					if !slices.Equal(got, want) {
						t.Fatalf("%s %s %d:%d parallels = %+v, want %+v", id, book, ch, v.Verse, got, want)
					}
					for _, c := range got {
						if !c.Parallel || strings.TrimSpace(c.Title) == "" || c.Book == book {
							t.Fatalf("%s %s %d:%d malformed parallel %+v", id, book, ch, v.Verse, c)
						}
						if bd.GetVerse(c.Book, c.Chapter, c.Verse) == nil {
							t.Fatalf("%s %s %d:%d parallel start missing: %s", id, book, ch, v.Verse, c.label())
						}
						if c.EndV != 0 {
							endCh := c.EndCh
							if endCh == 0 {
								endCh = c.Chapter
							}
							if bd.GetVerse(c.Book, endCh, c.EndV) == nil {
								t.Fatalf("%s %s %d:%d parallel end missing: %s", id, book, ch, v.Verse, c.label())
							}
						}
						if id == "bsb" && c.label() == "Matthew 17:14–20" {
							sawBSBClampMatt = true
						}
						if id == "bsb" && c.label() == "Mark 11:20–25" {
							sawBSBClampMark = true
						}
					}
				}
			}
		}
		if panels != wantPanelsByEdition[id] {
			t.Errorf("%s built %d Gospel panels, want %d", id, panels, wantPanelsByEdition[id])
		}
	}
	if totalPanels != 15103 {
		t.Errorf("walked %d Gospel panels across four editions, want 15,103", totalPanels)
	}
	if !sawBSBClampMatt || !sawBSBClampMark {
		t.Errorf("BSB clamping seen: Matthew 17:14–20=%v, Mark 11:20–25=%v; want both", sawBSBClampMatt, sawBSBClampMark)
	}
}
