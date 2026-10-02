package bibletext

import (
	"archive/zip"
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"
)

func TestParseOSISTarget(t *testing.T) {
	single, ok := parseOSISTarget("Ps.90.2")
	if !ok || single.Book != "Psalms" || single.Chapter != 90 || single.Verse != 2 || single.EndV != 0 {
		t.Errorf("single: %+v ok=%v", single, ok)
	}
	if single.label() != "Psalms 90:2" {
		t.Errorf("single label = %q", single.label())
	}

	rng, ok := parseOSISTarget("Rom.1.19-Rom.1.20")
	if !ok || rng.Book != "Romans" || rng.Verse != 19 || rng.EndV != 20 {
		t.Errorf("range: %+v ok=%v", rng, ok)
	}
	if rng.label() != "Romans 1:19-20" {
		t.Errorf("range label = %q", rng.label())
	}

	if _, ok := parseOSISTarget("Zzz.1.1"); ok {
		t.Error("unknown book should not parse")
	}
}

func makeCrossRefZip(t *testing.T, rows string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("cross_references.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("From Verse\tTo Verse\tVotes\n" + rows)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParseCrossRefZipAndRank(t *testing.T) {
	rows := "Gen.1.1\tHeb.1.2\t64\n" +
		"Gen.1.1\tJohn.1.1-John.1.3\t369\n" +
		"Gen.1.1\tPs.90.2\t61\n"
	idx, err := parseCrossRefZip(makeCrossRefZip(t, rows))
	if err != nil {
		t.Fatal(err)
	}
	got := idx[crossRefKey("Genesis", 1, 1)]
	if len(got) != 3 {
		t.Fatalf("want 3 refs, got %d", len(got))
	}
	// Highest votes first: John 1:1-3 (369).
	if got[0].Book != "John" || got[0].Votes != 369 {
		t.Errorf("top ref = %+v, want John 1:1-3 (369)", got[0])
	}
}

// The cross-reference dataset (OpenBible's TSK) numbers the Romans doxology
// 16:25-27, as the BSB and the ESV do. The app's reference versification is
// the WEB, which numbers it 14:24-26. Nothing normalised between the two, so
// both halves of the feature failed on that passage:
//
//   - a reader on ANY translation selecting the doxology looked the dataset up
//     under the reference number and found none of its 92 rows;
//   - a row POINTING at the doxology kept its dataset number, which in the WEB
//     names a verse that does not exist — a labelled row with a blank preview
//     and a tap that goes nowhere.
//
// Both are fixed by normalising the dataset into the reference numbering as it
// is parsed, so everything downstream sees one numbering.
func TestCrossRefDatasetNumberingIsNormalised(t *testing.T) {
	// One TSK row in the dataset's own numbering: Romans 16:25 -> Eph 3:20.
	const tsv = "From Verse\tTo Verse\tVotes\n" +
		"Rom.16.25\tEph.3.20\t30\n" +
		"Eph.3.20\tRom.16.25-Rom.16.27\t70\n"

	idx, err := parseCrossRefRows(strings.NewReader(tsv))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// CONTROL: the parser must produce something at all, or the absences below
	// prove nothing.
	if len(idx) == 0 {
		t.Fatal("control: the parser produced no rows; the assertions below are vacuous")
	}

	// (1) The SOURCE key is the reference number, not the dataset's.
	if got := idx[crossRefKey("Romans", 14, 24)]; len(got) != 1 {
		t.Errorf("the doxology's row must be keyed at the reference number "+
			"Romans 14:24, got %d rows there", len(got))
	}
	if got := idx[crossRefKey("Romans", 16, 25)]; len(got) != 0 {
		t.Errorf("nothing may remain keyed at the dataset's Romans 16:25: %+v", got)
	}

	// (2) The TARGET is rewritten too, span end included, so the panel can
	// preview and navigate to text that exists.
	rows := idx[crossRefKey("Ephesians", 3, 20)]
	if len(rows) != 1 {
		t.Fatalf("expected one row from Ephesians 3:20, got %+v", rows)
	}
	tgt := rows[0]
	if tgt.Book != "Romans" || tgt.Chapter != 14 || tgt.Verse != 24 {
		t.Errorf("target start must be the reference number Romans 14:24, got %s %d:%d",
			tgt.Book, tgt.Chapter, tgt.Verse)
	}
	if tgt.EndV != 26 || (tgt.EndCh != 0 && tgt.EndCh != 14) {
		t.Errorf("target span end must map to 14:26, got EndCh=%d EndV=%d", tgt.EndCh, tgt.EndV)
	}
}

// The dataset's 3 John runs to verse 15, as the ESV's does; every shipped
// text ends the letter at 14, with the closing greeting inside it. The
// dataset's one row from 3 John 1:15 — the friends greeted "by name", to John
// 10:3 — was keyed to a verse no translation has, so no reader ever saw it.
func TestTheDatasetsThirdJohnFifteenIsTheReferencesFourteen(t *testing.T) {
	const tsv = "From Verse\tTo Verse\tVotes\n" +
		"3John.1.15\tJohn.10.3\t1\n" +
		"John.10.3\t3John.1.15\t1\n"
	idx, err := parseCrossRefRows(strings.NewReader(tsv))
	if err != nil {
		t.Fatal(err)
	}
	if len(idx) == 0 {
		t.Fatal("control: the parser produced no rows; the assertions below are vacuous")
	}
	if got := idx[crossRefKey("3 John", 1, 14)]; len(got) != 1 || got[0].Book != "John" {
		t.Errorf("3 John 1:15's row must be keyed at 3 John 1:14, got %+v", got)
	}
	if got := idx[crossRefKey("3 John", 1, 15)]; len(got) != 0 {
		t.Errorf("nothing may stay keyed at a 3 John 1:15 no translation has: %+v", got)
	}
	if got := idx[crossRefKey("John", 10, 3)]; len(got) != 1 || got[0].label() != "3 John 1:14" {
		t.Errorf("a row pointing at 3 John 1:15 must point at 1:14, got %+v", got)
	}
}

// withCrossRefIndex installs a Treasury index parsed from a synthetic TSV for
// the test's duration, in place of the downloaded one.
func withCrossRefIndex(t *testing.T, rows string) {
	t.Helper()
	idx, err := parseCrossRefRows(strings.NewReader("From Verse\tTo Verse\tVotes\n" + rows))
	if err != nil {
		t.Fatal(err)
	}
	crossRefMu.Lock()
	was := crossRefIndex
	crossRefIndex = idx
	crossRefMu.Unlock()
	t.Cleanup(func() {
		crossRefMu.Lock()
		crossRefIndex = was
		crossRefMu.Unlock()
	})
}

// xrefBible is a synthetic text holding the named chapters, each with verses
// 1..n, so a panel can be built without a downloaded translation.
func xrefBible(chapters map[string]map[int]int) *BibleData {
	bd := &BibleData{Verses: map[string]map[int][]Verse{}}
	var books []string
	for book := range chapters {
		books = append(books, book)
	}
	sort.Strings(books)
	for _, book := range books {
		chs := chapters[book]
		bd.Books = append(bd.Books, book)
		bd.Verses[book] = map[int][]Verse{}
		for ch, n := range chs {
			for v := 1; v <= n; v++ {
				bd.Verses[book][ch] = append(bd.Verses[book][ch], Verse{
					BookName: book, Book: book, Chapter: ch, Verse: v,
					Text: fmt.Sprintf("Synthetic text of %s %d:%d.", book, ch, v),
				})
			}
		}
	}
	return bd
}

// A RANGE THAT RUNS INTO THE NEXT BOOK. Eighteen Treasury rows end in a
// different book from the one they start in, and the end's book was thrown
// away, so the end was read as a verse of the START book: "Leviticus
// 27:34-1:1" ran backwards, and "2 John 1:1-15" named a verse 2 John does not
// have — the dataset means 2 John 1:1 to 3 John 1:15, the greeting every
// shipped text prints at 3 John 1:14.
func TestACrossBookRangeKeepsItsEndBook(t *testing.T) {
	r, ok := parseOSISTarget("Lev.27.34-Num.1.1")
	if !ok || r.Book != "Leviticus" || r.EndBook != "Numbers" || r.EndCh != 1 || r.EndV != 1 {
		t.Fatalf("parsed %+v (ok=%v)", r, ok)
	}
	if got := r.label(); got != "Leviticus 27:34-Numbers 1:1" {
		t.Errorf("label %q, want \"Leviticus 27:34-Numbers 1:1\"", got)
	}

	withCrossRefIndex(t, "Num.3.1\tLev.27.34-Num.1.1\t5\n"+
		"Acts.11.30\t2John.1.1-3John.1.15\t4\n")
	bd := xrefBible(map[string]map[int]int{
		"Leviticus": {27: 34}, "Numbers": {1: 3, 3: 1},
		"Acts": {11: 30}, "2 John": {1: 13}, "3 John": {1: 14},
	})
	for _, tc := range []struct {
		book    string
		ch, v   int
		want    string
		version string
	}{
		{"Numbers", 3, 1, "Leviticus 27:34-Numbers 1:1", "web"},
		{"Numbers", 3, 1, "Leviticus 27:34-Numbers 1:1", "bsb"},
		{"Acts", 11, 30, "2 John 1:1-3 John 1:14", "web"},
		{"Acts", 11, 30, "2 John 1:1-3 John 1:14", "nkjv"},
	} {
		st := &AppState{Bible: bd, CurrentBook: tc.book, CurrentChapter: tc.ch, CurrentVersion: tc.version}
		refs := crossRefsForSelection(st, "", selSpan{lo: tc.v, hi: tc.v})
		var labels []string
		for _, c := range refs {
			labels = append(labels, c.label())
		}
		// CONTROL: the panel was built at all, or a missing label proves nothing.
		if len(refs) == 0 {
			t.Fatalf("%s %s %d:%d: the panel is empty", tc.version, tc.book, tc.ch, tc.v)
		}
		if labels[0] != tc.want {
			t.Errorf("%s %s %d:%d lists %q, want %q first", tc.version, tc.book, tc.ch, tc.v, labels, tc.want)
		}
	}
}
