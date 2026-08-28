package bibletext

import (
	"archive/zip"
	"bytes"
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

// The cross-reference dataset (OpenBible's TSK) is numbered as the KJV is:
// the Romans doxology sits at 16:25-27. The app's reference versification is
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
