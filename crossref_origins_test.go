package bibletext

import (
	"fmt"
	"strings"
	"testing"
)

func TestCrossRefOriginsRetainEachTreasurySourceAndRank(t *testing.T) {
	withCrossRefIndex(t, "Gen.1.1\tIsa.1.1\t5\n"+
		"Gen.1.2\tIsa.1.2\t9\n"+
		"Gen.1.3\tIsa.1.1\t30\n"+
		"Gen.1.3\tIsa.1.3\t40\n")
	bd := xrefBible(map[string]map[int]int{"Genesis": {1: 3}, "Isaiah": {1: 3}})
	st := &AppState{Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1, CurrentVersion: "web"}
	got := crossRefsForSelection(st, "", selSpan{lo: 1, hi: 3})
	want := []struct {
		origin, target, votes int
	}{{1, 1, 5}, {2, 2, 9}, {3, 3, 40}, {3, 1, 30}}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d", len(got), len(want))
	}
	for i, w := range want {
		c := got[i]
		if c.SourceVerse != w.origin || c.Verse != w.target || c.Votes != w.votes ||
			c.Book != "Isaiah" || c.SourceLabel != fmt.Sprintf("Genesis 1:%d", w.origin) || c.SourceGroup != "" {
			t.Errorf("row %d = %+v; want origin %d, target Isaiah 1:%d, votes %d", i, c, w.origin, w.target, w.votes)
		}
	}
}

func TestCrossRefOriginsKeepEverySelectedVersesCappedGroup(t *testing.T) {
	var rows strings.Builder
	for source := 1; source <= 3; source++ {
		for target := 1; target <= 17; target++ {
			fmt.Fprintf(&rows, "Gen.1.%d\tIsa.%d.%d\t%d\n", source, source, target, 100-target)
		}
	}
	withCrossRefIndex(t, rows.String())
	bd := xrefBible(map[string]map[int]int{"Genesis": {1: 3}, "Isaiah": {1: 17, 2: 17, 3: 17}})
	st := &AppState{Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1, CurrentVersion: "web"}
	got := crossRefsForSelection(st, "", selSpan{lo: 1, hi: 3})
	if len(got) != 3*maxCrossRefsPerVerse {
		t.Fatalf("got %d rows, want three complete %d-row groups", len(got), maxCrossRefsPerVerse)
	}
	for i, c := range got {
		source, target := i/maxCrossRefsPerVerse+1, i%maxCrossRefsPerVerse+1
		if c.SourceVerse != source || c.Chapter != source || c.Verse != target {
			t.Errorf("row %d = %+v; want origin %d, target Isaiah %d:%d", i, c, source, source, target)
		}
	}
}

func TestCrossRefOriginsUseDisplayedVerseNumbers(t *testing.T) {
	for _, tc := range []struct {
		version, book  string
		chapter, verse int
		datasetSource  string
		wantRows       int
	}{
		{"web", "Romans", 14, 24, "Rom.16.25", 1},
		{"bsb", "Romans", 16, 25, "Rom.16.25", 1},
		{"webc", "Daniel", 3, 91, "Dan.3.24", 1},
		{"webc", "Daniel", 3, 24, "Dan.3.24", 0},
		{"bsb", "Philippians", 1, 16, "Phil.1.17", 1},
	} {
		t.Run(fmt.Sprintf("%s-%s-%d-%d", tc.version, tc.book, tc.chapter, tc.verse), func(t *testing.T) {
			withCrossRefIndex(t, tc.datasetSource+"\tIsa.1.1\t7\n")
			bd := xrefBible(map[string]map[int]int{tc.book: {tc.chapter: tc.verse}, "Isaiah": {1: 1}})
			st := &AppState{Bible: bd, CurrentBook: tc.book, CurrentChapter: tc.chapter, CurrentVersion: tc.version}
			got := crossRefsForSelection(st, "", selSpan{lo: tc.verse, hi: tc.verse})
			if len(got) != tc.wantRows {
				t.Fatalf("got %d rows, want %d", len(got), tc.wantRows)
			}
			if len(got) > 0 {
				want := fmt.Sprintf("%s %d:%d", tc.book, tc.chapter, tc.verse)
				if got[0].SourceVerse != tc.verse || got[0].SourceLabel != want {
					t.Errorf("origin = %d / %q; want %d / %q", got[0].SourceVerse, got[0].SourceLabel, tc.verse, want)
				}
			}
		})
	}
}

func withOriginPericopes(t *testing.T, pericopes []gPericope) {
	t.Helper()
	gospelOnce.Do(loadGospelParallels)
	old := gospelPericopes
	gospelPericopes = pericopes
	t.Cleanup(func() { gospelPericopes = old })
}

func TestCrossRefOriginsKeepEventsAndSourcesSeparate(t *testing.T) {
	makeEvent := func(id int) gPericope {
		return gPericope{
			id: id, title: "Synthetic event",
			spans: map[string][]gSpan{"Matthew": {{1, 1, 1, 2}}, "Luke": {{1, 1, 1, 1}}},
			refs:  map[string]string{"Matthew": "1:1-2", "Luke": "1:1"},
		}
	}
	withOriginPericopes(t, []gPericope{makeEvent(701), makeEvent(702)})
	withCrossRefIndex(t, "Matt.1.1\tLuke.1.1\t7\n")
	bd := xrefBible(map[string]map[int]int{"Matthew": {1: 2}, "Luke": {1: 1}})
	st := &AppState{Bible: bd, CurrentBook: "Matthew", CurrentChapter: 1, CurrentVersion: "web"}
	got := crossRefsForSelection(st, "", selSpan{lo: 1, hi: 2})
	if len(got) != 3 {
		t.Fatalf("got %d rows; want one per event plus the Treasury relationship", len(got))
	}
	if !got[0].Parallel || !got[1].Parallel || got[2].Parallel || got[2].SourceVerse != 1 {
		t.Fatalf("rows do not retain their sources: %+v", got)
	}
	if got[0].SourceGroup == "" || got[0].SourceGroup == got[1].SourceGroup {
		t.Errorf("events with identical titles and targets share a group: %q / %q", got[0].SourceGroup, got[1].SourceGroup)
	}
	for _, c := range got[:2] {
		if c.SourceLabel != "Matthew 1:1–2" || c.label() != "Luke 1:1" {
			t.Errorf("event origin/target = %q / %q", c.SourceLabel, c.label())
		}
	}
}

func TestGospelSourceLabelsMapSeparateSpansAndRetainUnmovedParts(t *testing.T) {
	for _, tc := range []struct{ book, raw, version, want string }{
		{"Luke", "6:27-28,32-36", "web", "Luke 6:27–28, 32–36"},
		{"Luke", "22:54a,63-71", "web", "Luke 22:54a, 63–71"},
		{"Matthew", "23:14a", "bsb", "Matthew 23:13"},
		{"Matthew", "17:14-21", "bsb", "Matthew 17:14–20"},
		{"Daniel", "3:19-30", "webc", "Daniel 3:19–23, 3:91–97"},
	} {
		if got := gospelSourceLabel(tc.book, parseGospelRef(tc.raw), tc.raw, tc.version); got != tc.want {
			t.Errorf("%s %s in %s = %q, want %q", tc.book, tc.raw, tc.version, got, tc.want)
		}
	}
}

func TestGospelSourceMetadataFollowsTheSelectedEditionsNumbering(t *testing.T) {
	withOriginPericopes(t, []gPericope{{
		id: 703, title: "Synthetic event",
		spans: map[string][]gSpan{"Matthew": {{23, 14, 23, 14}}, "Luke": {{1, 1, 1, 1}}},
		refs:  map[string]string{"Matthew": "23:14a", "Luke": "1:1"},
	}})
	bd := xrefBible(map[string]map[int]int{"Matthew": {23: 39}, "Luke": {1: 1}})
	st := &AppState{Bible: bd, CurrentBook: "Matthew", CurrentChapter: 23, CurrentVersion: "bsb"}
	got := localCrossRefsForSelection(st, "", selSpan{lo: 13, hi: 13})
	var synopsis []crossRef
	for _, c := range got {
		if !c.OtherOccasion {
			synopsis = append(synopsis, c)
		}
	}
	if len(synopsis) != 1 || synopsis[0].SourceLabel != "Matthew 23:13" || synopsis[0].label() != "Luke 1:1" {
		t.Fatalf("mapped source group = %+v", synopsis)
	}
}

func TestOtherOccasionGroupsRetainDistinctSourcePassagesAndOverlaps(t *testing.T) {
	withOriginPericopes(t, nil)
	occasionOnce.Do(loadGospelOccasions)
	old := occasionGroups
	occasionGroups = parseGospelOccasions([]byte(`{"groups":[
		{"id":"SYNTHETIC-A","title":"Synthetic saying","passages":["Luke 1:1-2","Luke 1:3-4","Matthew 1:1-2"],"pairs":[["Luke 1:1-2","Matthew 1:1-2"],["Luke 1:3-4","Matthew 1:1-2"]]},
		{"id":"SYNTHETIC-B","title":"Synthetic saying","passages":["Luke 1:1-2","Matthew 1:1-3"],"pairs":[["Luke 1:1-2","Matthew 1:1-3"]]}
	]}`))
	t.Cleanup(func() { occasionGroups = old })
	bd := xrefBible(map[string]map[int]int{"Luke": {1: 4}, "Matthew": {1: 3}})
	st := &AppState{Bible: bd, CurrentBook: "Luke", CurrentChapter: 1, CurrentVersion: "web"}
	got := localCrossRefsForSelection(st, "", selSpan{lo: 1, hi: 4})
	if !nextRelease {
		if len(got) != 0 {
			t.Fatalf("shipping lookup exposed %d other-occasion rows", len(got))
		}
		return
	}
	if len(got) != 3 {
		t.Fatalf("got %d rows; want three source relations without merging their target ranges", len(got))
	}
	groups := map[string]crossRef{}
	counts := map[string]int{}
	for _, c := range got {
		if !c.OtherOccasion || c.SourceGroup == "" {
			t.Fatalf("missing occasion metadata: %+v", c)
		}
		groups[c.SourceGroup] = c
		counts[c.SourceLabel+" -> "+c.label()]++
	}
	if len(groups) != 3 || counts["Luke 1:1–2 -> Matthew 1:1–2"] != 1 ||
		counts["Luke 1:1–2 -> Matthew 1:1–3"] != 1 || counts["Luke 1:3–4 -> Matthew 1:1–2"] != 1 {
		t.Errorf("source relations = %v", counts)
	}
}
