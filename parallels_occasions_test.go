package bibletext

import (
	"fmt"
	"strings"
	"testing"
)

// The second kind of Gospel parallel, the same saying on another occasion
// (parallels.go), on synthetic data: its source identities and the Treasury
// relationships retained alongside overlapping Gospel destinations. The code
// is shared by both states of the next switch; only the next major release
// has the data (parallels_next.go), so the tests on the data itself are in
// parallels_next_test.go.

// THE LOADER READS WHAT IT CAN AND DROPS THE REST: a passage that does not
// parse, a pair naming a passage the group does not list, a group with no
// pair left.
func TestParseGospelOccasionsDropsWhatItCannotRead(t *testing.T) {
	got := parseGospelOccasions([]byte(`{"groups":[
		{"id":"A","title":"A synthetic saying","passages":["Luke 8:16","Luke 11:33","Acts 1:1","Luke x"],
		 "pairs":[["Luke 8:16","Luke 11:33"],["Luke 8:16","Acts 1:1"],["Luke 8:16","Luke 9:9"],["Luke 8:16","Luke 8:16"]]},
		{"id":"B","title":"Nothing left","passages":["Mark 1:1"],"pairs":[["Mark 1:1","Mark 2:2"]]}]}`))
	if len(got) != 1 {
		t.Fatalf("got %d groups, want the one with a readable pair", len(got))
	}
	g := got[0]
	if len(g.passages) != 2 || len(g.pairs) != 1 || g.title != "A synthetic saying" {
		t.Fatalf("got %+v; want two passages (Acts is not a Gospel, \"Luke x\" not a reference) and one pair", g)
	}
	if g.id != "A" || strings.Join(g.refs, "; ") != "8:16; 11:33" {
		t.Errorf("source identity/notation lost: id=%q refs=%q", g.id, g.refs)
	}
	if a, b := g.passages[g.pairs[0][0]], g.passages[g.pairs[0][1]]; a.book != "Luke" || b.book != "Luke" {
		t.Errorf("a pair within one Gospel was not kept: %+v %+v", a, b)
	}
	if parseGospelOccasions([]byte("not json")) != nil {
		t.Error("a file that does not parse gave groups")
	}
}

// Sources retain their own relationships even when destination ranges are
// equal, contained by one another, or overlap a Gospel row.
func TestTreasuryRowsRemainAlongsideOverlappingGospelGroups(t *testing.T) {
	withOriginPericopes(t, []gPericope{{
		id: 801, title: "Synthetic event",
		spans: map[string][]gSpan{"Matthew": {{1, 1, 1, 1}}, "Mark": {{3, 20, 3, 30}}},
		refs:  map[string]string{"Matthew": "1:1", "Mark": "3:20-30"},
	}})
	occasionOnce.Do(loadGospelOccasions)
	old := occasionGroups
	occasionGroups = parseGospelOccasions([]byte(`{"groups":[
		{"id":"SYNTHETIC","title":"Synthetic saying","passages":["Matthew 1:1","Luke 11:2-4"],"pairs":[["Matthew 1:1","Luke 11:2-4"]]}
	]}`))
	t.Cleanup(func() { occasionGroups = old })
	var rows strings.Builder
	for i, target := range []string{
		"Luke.11.2", "Luke.11.2-Luke.11.4", "Luke.11.1-Luke.11.4",
		"Luke.11.4-Luke.12.1", "Mark.3.24", "Mark.3.20-Mark.3.30", "Luke.24.53-John.1.1",
	} {
		fmt.Fprintf(&rows, "Matt.1.1\t%s\t%d\n", target, 100-i)
	}
	for v := 1; v <= maxCrossRefsPerVerse; v++ {
		fmt.Fprintf(&rows, "Matt.1.1\tPs.119.%d\t%d\n", v, 40-v)
	}
	withCrossRefIndex(t, rows.String())
	bd := xrefBible(map[string]map[int]int{
		"Matthew": {1: 1}, "Mark": {3: 35}, "Luke": {11: 54, 12: 59, 24: 53},
		"John": {1: 1}, "Psalms": {119: 176},
	})
	st := &AppState{Bible: bd, CurrentBook: "Matthew", CurrentChapter: 1, CurrentVersion: "web"}
	var treasury []string
	synopsis, occasions := 0, 0
	for _, c := range crossRefsForSelection(st, "", selSpan{lo: 1, hi: 1}) {
		switch {
		case c.OtherOccasion:
			occasions++
			if c.label() != "Luke 11:2–4" || c.SourceLabel != "Matthew 1:1" {
				t.Errorf("occasion changed: %+v", c)
			}
		case c.Parallel:
			synopsis++
			if c.label() != "Mark 3:20–30" || c.SourceLabel != "Matthew 1:1" {
				t.Errorf("synopsis changed: %+v", c)
			}
		default:
			treasury = append(treasury, c.label())
			if c.SourceVerse != 1 || c.SourceLabel != "Matthew 1:1" {
				t.Errorf("Treasury origin changed: %+v", c)
			}
		}
	}
	wantOccasions := 0
	if nextRelease {
		wantOccasions = 1
	}
	if synopsis != 1 || occasions != wantOccasions {
		t.Fatalf("control: got %d synopsis and %d occasion rows; want 1 and %d", synopsis, occasions, wantOccasions)
	}
	if len(treasury) != maxCrossRefsPerVerse {
		t.Fatalf("got %d Treasury rows, want %d", len(treasury), maxCrossRefsPerVerse)
	}
	want := "Luke 11:2; Luke 11:2–4; Luke 11:1–4; Luke 11:4–12:1; Mark 3:24; Mark 3:20–30; Luke 24:53–John 1:1"
	if got := strings.Join(treasury[:7], "; "); got != want {
		t.Errorf("Treasury relationships = %q; want %q", got, want)
	}
	if last := treasury[len(treasury)-1]; last != "Psalms 119:9" {
		t.Errorf("Treasury last row = %q, want Psalms 119:9 without duplicate-driven backfill", last)
	}
}
