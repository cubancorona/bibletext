package bibletext

import (
	"archive/zip"
	"bytes"
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
