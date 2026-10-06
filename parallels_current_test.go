//go:build !next

package bibletext

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// The Gospel parallels of the shipping build: the synopsis as the last release
// shipped it, and no same saying on another occasion. The next major
// release's are in parallels_next_test.go (docs/NEXT.md).

// gospelSynopsisColumns is how many Gospel passages the synopsis's sets name.
const gospelSynopsisColumns = 469

// gospelVersesInNoSet are the WEB's Gospel verses that belong to no synopsis
// set, in Gospel order.
var gospelVersesInNoSet = []string{
	"Matthew 4:23", "Matthew 4:24", "Matthew 4:25",
	"Luke 6:17", "Luke 6:18", "Luke 6:19", "Luke 6:24", "Luke 6:25", "Luke 6:26",
	"Luke 6:43", "Luke 6:44", "Luke 6:45", "Luke 21:37", "Luke 21:38",
	"John 11:55", "John 11:56", "John 11:57",
	"John 13:31", "John 13:32", "John 13:33", "John 13:34", "John 13:35",
}

// The synopsis in force is gospel_parallels.json, and there is no
// other-occasion data at all.
func TestTheShippingBuildsParallelsAreInForce(t *testing.T) {
	want, err := os.ReadFile("assets/parallels/gospel_parallels.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gospelParallelsJSON, want) {
		t.Error("the synopsis in force is not assets/parallels/gospel_parallels.json")
	}
	if gospelOccasionsJSON != nil {
		t.Errorf("the shipping build holds %d bytes of other-occasion data", len(gospelOccasionsJSON))
	}
	for _, ref := range []struct {
		book  string
		ch, v int
	}{{"Luke", 11, 2}, {"Matthew", 6, 9}, {"Luke", 8, 16}, {"Matthew", 23, 14}} {
		if got := gospelOccasionsForVerse(ref.book, ref.ch, ref.v); len(got) != 0 {
			t.Errorf("%s %d:%d lists %d passages on another occasion", ref.book, ref.ch, ref.v, len(got))
		}
	}
}

// NO ROW IS ANOTHER OCCASION. Luke's Lord's Prayer lists the Treasury's row
// to Matthew's, as the last release did, with nothing hidden and nothing
// above it; and a row marked as another occasion by hand would still be shown
// as the synopsis's, with its tag and not the next release's label.
func TestTheShippingBuildListsNoOtherOccasion(t *testing.T) {
	withCrossRefIndex(t, "Luke.11.2\tMatt.6.9\t40\nLuke.11.2\tMatt.6.9-Matt.6.13\t30\n")
	bd := xrefBible(map[string]map[int]int{"Luke": {11: 54}, "Matthew": {6: 34}})
	st := &AppState{Bible: bd, CurrentBook: "Luke", CurrentChapter: 11, CurrentVersion: "web"}
	var got []string
	for _, c := range crossRefsForSelection(st, "", selSpan{lo: 2, hi: 2}) {
		if c.Parallel || c.OtherOccasion {
			t.Errorf("Luke 11:2 lists %s as a parallel", c.label())
		}
		got = append(got, c.label())
	}
	if strings.Join(got, "; ") != "Matthew 6:9; Matthew 6:9-13" {
		t.Errorf("Luke 11:2 lists %q, want both Treasury rows", got)
	}

	app := test.NewApp()
	defer app.Quit()
	marked := crossRef{Book: "Matthew", Chapter: 6, Verse: 9, EndV: 13, Parallel: true, OtherOccasion: true, Title: "A synthetic saying"}
	if marked.otherOccasion() {
		t.Error("the shipping build reads a row as another occasion")
	}
	lst := buildCrossRefList(st, nil, []crossRef{marked}, nil, lightPalette, func(crossRef) {})
	if lst.Parallels != 1 || lst.OtherOccasions != 0 || len(lst.Objects) != 1 {
		t.Fatalf("list = %d parallels, %d other-occasion rows, %d objects; want 1, 0, 1", lst.Parallels, lst.OtherOccasions, len(lst.Objects))
	}
	row := strings.Join(textsIn(lst.Objects[0]), " | ")
	if !strings.Contains(row, "PARALLEL") || strings.Contains(row, otherOccasionLabel) {
		t.Errorf("the row is not drawn as the synopsis's: %q", row)
	}
}
