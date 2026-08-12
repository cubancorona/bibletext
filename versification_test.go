package bibletext

// Where our translations DISAGREE about verse numbers.
//
// applyShareTarget opens a shared link in whatever translation the reader is
// already using, on the grounds that the passage is the same either way. That is
// true almost everywhere and false in two specific chapters, and the difference
// matters to anything keyed to (book, chapter, verse) across translations —
// shared links and stored notes above all.
//
// This test pins the disagreement to the data rather than to anyone's memory, so
// the claim in applyShareTarget's comment cannot quietly rot: refresh either
// translation and, if the versification moves, this fails and says exactly how.
// The per-verse read-along timing tables are the only whole-canon per-verse
// structure we ship for two translations at once, which is why they are the
// source here.

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"
)

// verseShape maps "Book C" → the sorted verse numbers that chapter contains.
func verseShape(t *testing.T, raw []byte) map[string][]int {
	t.Helper()
	var doc map[string]map[string][][]float64
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("timing table did not parse: %v", err)
	}
	out := make(map[string][]int, 1200)
	for book, chapters := range doc {
		for chapter, verses := range chapters {
			nums := make([]int, 0, len(verses))
			for _, v := range verses {
				if len(v) > 0 {
					nums = append(nums, int(v[0]))
				}
			}
			sort.Ints(nums)
			out[book+" "+chapter] = nums
		}
	}
	return out
}

func TestWebAndBSBVersificationDiverges(t *testing.T) {
	web := verseShape(t, webTimingsJSON)
	bsb := verseShape(t, bsbTimingsJSON)

	// Books and chapters must align exactly — a shared link names a chapter, and
	// if the two disagreed about which chapters EXIST the link contract itself
	// would be unsound.
	for ref := range web {
		if _, ok := bsb[ref]; !ok {
			t.Errorf("%s exists in WEB but not BSB — chapter sets have diverged", ref)
		}
	}
	for ref := range bsb {
		if _, ok := web[ref]; !ok {
			t.Errorf("%s exists in BSB but not WEB — chapter sets have diverged", ref)
		}
	}

	// The verses one translation has and the other lacks, per chapter.
	type gap struct{ webOnly, bsbOnly []int }
	diverged := map[string]gap{}
	for ref, w := range web {
		b, ok := bsb[ref]
		if !ok {
			continue
		}
		inB := make(map[int]bool, len(b))
		for _, n := range b {
			inB[n] = true
		}
		inW := make(map[int]bool, len(w))
		for _, n := range w {
			inW[n] = true
		}
		var g gap
		for _, n := range w {
			if !inB[n] {
				g.webOnly = append(g.webOnly, n)
			}
		}
		for _, n := range b {
			if !inW[n] {
				g.bsbOnly = append(g.bsbOnly, n)
			}
		}
		if len(g.webOnly) > 0 || len(g.bsbOnly) > 0 {
			diverged[ref] = g
		}
	}

	// Ten chapters where the BSB omits a verse WEB carries (textual-critical
	// omissions). Numbering is otherwise intact, so only a reference landing ON
	// the omitted verse finds nothing.
	omissions := map[string][]int{
		"Matthew 17": {21},
		"Matthew 18": {11},
		"Matthew 23": {14},
		"Mark 7":     {16},
		"Mark 9":     {44, 46},
		"Mark 11":    {26},
		"Mark 15":    {28},
		"Luke 23":    {17},
		"John 5":     {4},
		"Acts 28":    {29},
	}
	// And the two where the numbering genuinely SHIFTS: the Romans doxology sits
	// at 14:24-26 in WEB and at 16:25-27 in the BSB. A reference here resolves to
	// different text in the two translations rather than to nothing — the one
	// case a cross-translation note must not silently trust.
	renumbered := map[string]gap{
		"Romans 14": {webOnly: []int{24, 25, 26}},
		"Romans 16": {webOnly: []int{24}, bsbOnly: []int{25, 26, 27}},
	}

	want := map[string]gap{}
	for ref, only := range omissions {
		want[ref] = gap{webOnly: only}
	}
	for ref, g := range renumbered {
		want[ref] = g
	}

	show := func(g gap) string {
		return fmt.Sprintf("WEB-only=%v BSB-only=%v", g.webOnly, g.bsbOnly)
	}
	for ref, expect := range want {
		got, ok := diverged[ref]
		if !ok {
			t.Errorf("%s no longer diverges (expected %s) — the data changed; re-check applyShareTarget's comment", ref, show(expect))
			continue
		}
		if fmt.Sprint(got.webOnly) != fmt.Sprint(expect.webOnly) ||
			fmt.Sprint(got.bsbOnly) != fmt.Sprint(expect.bsbOnly) {
			t.Errorf("%s diverges differently now: got %s, want %s", ref, show(got), show(expect))
		}
	}
	for ref, got := range diverged {
		if _, known := want[ref]; !known {
			t.Errorf("NEW versification divergence at %s (%s) — anything keyed to (book, chapter, verse) across translations must account for it, and applyShareTarget's comment needs updating", ref, show(got))
		}
	}
}
