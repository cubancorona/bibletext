package bibletext

import (
	"strings"
	"testing"
)

// The second kind of Gospel parallel, the same saying on another occasion
// (parallels.go), on synthetic data: the reader of its file, the rule that
// lists each passage once, and the Treasury rows such a row hides. The code
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
	if a, b := g.passages[g.pairs[0][0]], g.passages[g.pairs[0][1]]; a.book != "Luke" || b.book != "Luke" {
		t.Errorf("a pair within one Gospel was not kept: %+v %+v", a, b)
	}
	if parseGospelOccasions([]byte("not json")) != nil {
		t.Error("a file that does not parse gave groups")
	}
}

// ONE PASSAGE, ONE ROW. A row a same-occasion row covers is left out, as is a
// row an earlier one covers or repeats; a row that covers earlier ones takes
// the place of the first of them (TestOtherOccasionRowsAreDeduplicatedByTheVersesTheyCover
// shows it on the data).
func TestAddOtherOccasionRowListsEachPassageOnceAtItsWidest(t *testing.T) {
	row := func(book string, ch, v, endV int) crossRef {
		return crossRef{Book: book, Chapter: ch, Verse: v, EndV: endV, Parallel: true, OtherOccasion: true}
	}
	parallels := []crossRef{{Book: "Mark", Chapter: 3, Verse: 20, EndV: 30, Parallel: true}}
	var rows []crossRef
	for _, c := range []crossRef{
		row("Mark", 3, 22, 0),      // inside the same-occasion row: out
		row("Luke", 11, 15, 0),     // in
		row("Luke", 11, 15, 0),     // the same again: out
		row("Matthew", 9, 32, 34),  // in
		row("Luke", 11, 14, 15),    // covers Luke 11:15: takes its place
		row("Matthew", 9, 33, 0),   // inside Matthew 9:32-34: out
		row("Mark", 3, 31, 0),      // outside the same-occasion row: in
		row("Matthew", 12, 22, 24), // in
	} {
		rows = addOtherOccasionRow(rows, parallels, c)
	}
	var labels []string
	for _, r := range rows {
		labels = append(labels, r.label())
	}
	want := "Luke 11:14-15; Matthew 9:32-34; Mark 3:31; Matthew 12:22-24"
	if strings.Join(labels, "; ") != want {
		t.Errorf("rows = %q, want %q", strings.Join(labels, "; "), want)
	}
}

// HIDING BY THE VERSES COVERED. A same-occasion row hides a Treasury row with
// its own label and no other; an other-occasion row hides every Treasury row
// inside its passage, and none that runs past it or into another book. The
// cap is counted after the hiding, so a hidden row hands its place on.
func TestAnOtherOccasionRowHidesTheTreasuryRowsInsideIt(t *testing.T) {
	var rows []tskRow
	for _, c := range []crossRef{
		{Book: "Luke", Chapter: 11, Verse: 2},                                      // inside Luke 11:2-4: hidden
		{Book: "Luke", Chapter: 11, Verse: 2, EndV: 4},                             // the passage itself: hidden
		{Book: "Luke", Chapter: 11, Verse: 1, EndV: 4},                             // runs past its start: shown
		{Book: "Luke", Chapter: 11, Verse: 4, EndCh: 12, EndV: 1},                  // runs past its end: shown
		{Book: "Mark", Chapter: 3, Verse: 24},                                      // inside the pericope, not its label: shown
		{Book: "Mark", Chapter: 3, Verse: 20, EndV: 30},                            // the pericope's label: hidden
		{Book: "Luke", Chapter: 24, Verse: 53, EndBook: "John", EndCh: 1, EndV: 1}, // into another book: shown
	} {
		r, ok := packTSKRow(c)
		if !ok {
			t.Fatalf("cannot pack %s", c.label())
		}
		rows = append(rows, r)
	}
	for v := 1; v <= maxCrossRefsPerVerse; v++ {
		r, _ := packTSKRow(crossRef{Book: "Psalms", Chapter: 1, Verse: v})
		rows = append(rows, r)
	}
	resolve := func(c crossRef) []crossRef { return []crossRef{c} }
	hidden := crossRefHidden{
		labels: map[string]bool{"Mark 3:20-30": true},
		covers: []crossRef{{Book: "Luke", Chapter: 11, Verse: 2, EndV: 4, Parallel: true, OtherOccasion: true}},
	}
	mine, read := treasuryRowsFor(rows, resolve, hidden)
	var got []string
	for _, c := range mine[:4] {
		got = append(got, c.label())
	}
	want := "Luke 11:1-4; Luke 11:4-12:1; Mark 3:24; Luke 24:53-John 1:1"
	if strings.Join(got, "; ") != want {
		t.Errorf("the first rows shown are %q, want %q", strings.Join(got, "; "), want)
	}
	// Three hidden rows hand their places on: sixteen shown, read to the 19th.
	if len(mine) != maxCrossRefsPerVerse || read != maxCrossRefsPerVerse+3 {
		t.Errorf("%d rows shown, read to row %d; want %d, read to row %d", len(mine), read, maxCrossRefsPerVerse, maxCrossRefsPerVerse+3)
	}
	// CONTROL: hidden by label alone, as the shipping build hides, the
	// passage's rows show.
	if mine, _ := treasuryRowsFor(rows, resolve, crossRefHidden{labels: hidden.labels}); mine[0].label() != "Luke 11:2" {
		t.Errorf("by label alone the first row is %s, want Luke 11:2", mine[0].label())
	}
}
