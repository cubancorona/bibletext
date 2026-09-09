package bibletext

import (
	"os"
	"reflect"
	"strconv"
	"testing"
)

// TestBundledTitleRows is the acceptance gate for the recorded tables built
// with the Psalm titles in the transcript (S18): in both human narrations every
// titled psalm's chapter leads with a verse-0 row that ends at or before verse
// 1, the same 116 chapters in each, and no other chapter anywhere carries one.
// Built to FAIL on a table from before the titles — a table with none is still
// a valid table (readalong.go), just not the one this repository ships.
func TestBundledTitleRows(t *testing.T) {
	loadTimings()
	var titledSets []map[int]bool
	for _, rec := range []string{"bsb-hays", "web-williams"} {
		books := allTimings[rec]
		if len(books) == 0 {
			t.Fatalf("%s: no table loaded", rec)
		}
		total := 0
		for _, chs := range books {
			total += len(chs)
		}
		if total != 1189 {
			t.Errorf("%s: %d chapters, want 1189 (66 books)", rec, total)
		}
		titled := map[int]bool{}
		for book, chs := range books {
			for ch, rows := range chs {
				for i, r := range rows {
					if r.verse != readAlongTitle {
						continue
					}
					if book != "Psalms" {
						t.Errorf("%s: %s %s carries a verse-0 row; only the Psalms have titles", rec, book, ch)
					}
					if i != 0 {
						t.Errorf("%s: Psalm %s has a verse-0 row at index %d; the title leads", rec, ch, i)
					}
				}
				if book != "Psalms" || len(rows) == 0 || rows[0].verse != readAlongTitle {
					continue
				}
				n, _ := strconv.Atoi(ch)
				titled[n] = true
				if len(rows) < 2 || rows[1].verse != 1 || !(rows[0].start > 0 && rows[0].start < rows[0].end && rows[0].end <= rows[1].start) {
					t.Errorf("%s: Psalm %s title row %+v does not sit before verse 1 %+v", rec, ch, rows[0], rows[1:2])
				}
			}
		}
		if len(titled) != 116 {
			t.Errorf("%s: %d psalms carry a title row, want 116", rec, len(titled))
		}
		// CONTROLS: the untitled psalms, including the one whose ALEPH is an
		// acrostic letter and not a title.
		for _, ch := range []int{1, 2, 10, 119, 150} {
			if titled[ch] {
				t.Errorf("%s: Psalm %d has a title row but no title", rec, ch)
			}
		}
		if !titled[3] || !titled[18] || !titled[51] || !titled[145] {
			t.Errorf("%s: a titled psalm (3, 18, 51, 145) lacks its row: %v %v %v %v", rec, titled[3], titled[18], titled[51], titled[145])
		}
		titledSets = append(titledSets, titled)
	}
	if !reflect.DeepEqual(titledSets[0], titledSets[1]) {
		t.Error("the two narrations disagree about which psalms are titled")
	}
	// The synthetic Greek-book narration has no Psalms and no title rows.
	for book, chs := range allTimings[webbeRecordingID] {
		for ch, rows := range chs {
			for _, r := range rows {
				if r.verse == readAlongTitle {
					t.Errorf("webbe: %s %s carries a verse-0 row", book, ch)
				}
			}
		}
	}

	// Cross-check against the feed itself where it is present: the set of
	// psalms the decoder gives a superscription is the set the tables title.
	body, err := os.ReadFile("build/biblecache/bsb.json")
	if err != nil {
		t.Skip("build/biblecache/bsb.json not present; the feed cross-check is skipped")
	}
	bd, err := decodeCanonical66(body)
	if err != nil {
		t.Fatal(err)
	}
	fromFeed := map[int]bool{}
	for ch := range bd.Superscriptions["Psalms"] {
		fromFeed[ch] = true
	}
	if len(fromFeed) == 0 {
		t.Fatal("the feed decoded no psalm titles; this cross-check proves nothing")
	}
	if !reflect.DeepEqual(fromFeed, titledSets[0]) {
		t.Errorf("the feed titles %d psalms, the table %d, and they differ", len(fromFeed), len(titledSets[0]))
	}
}
