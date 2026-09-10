package bibletext

// The rotation against the SHIPPED EDITIONS, offline.
//
// The editions are downloaded, not committed — the licensed one through the
// owner's key — so CI cannot load them; and a test that loads them when present
// and skips otherwise proves nothing on the machine that matters. scripts/gen-votd-snapshot.py records,
// for every entry and every edition, whether the passage is present and how
// its text begins and ends. These tests read that record. When the list
// changes, regenerate the snapshot in the same change — the first test here
// is what says so.

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

type votdSnapshotEntry struct {
	First   string `json:"first"`
	Last    string `json:"last"`
	Via     string `json:"via"`
	Missing string `json:"missing"`
}

func loadVOTDSnapshot(t *testing.T) map[string]map[string]votdSnapshotEntry {
	t.Helper()
	raw, err := os.ReadFile("testdata/verse_of_day_snapshot.json")
	if err != nil {
		t.Fatalf("no snapshot: %v (run scripts/gen-votd-snapshot.py)", err)
	}
	var doc struct {
		Editions map[string]map[string]votdSnapshotEntry `json:"editions"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("snapshot does not parse: %v", err)
	}
	return doc.Editions
}

// Every entry is present in every edition — itself, or through its alternate
// where the edition lacks the book — and the snapshot matches the list.
func TestEveryRotationEntryResolvesInEveryShippedEdition(t *testing.T) {
	eds := loadVOTDSnapshot(t)
	for _, want := range []string{"web", "bsb", "webc", "nkjv"} {
		if _, ok := eds[want]; !ok {
			t.Errorf("snapshot has no %s edition", want)
		}
	}
	keys := map[string]dayPassage{}
	for _, p := range verseOfDayRefs {
		keys[p.key()] = p
	}
	for ed, table := range eds {
		for k := range keys {
			e, ok := table[k]
			if !ok {
				t.Errorf("%s: %s is not in the snapshot — regenerate it: scripts/gen-votd-snapshot.py", ed, k)
				continue
			}
			if e.Missing != "" {
				t.Errorf("%s: %s is missing (%s) in the shipped edition", ed, k, e.Missing)
			}
			_, deutero := verseOfDayAlternates[keys[k]]
			switch {
			case e.Via != "" && ed == "webc":
				t.Errorf("webc: %s shows its alternate, but the Catholic edition carries the book", k)
			case e.Via != "" && !deutero:
				t.Errorf("%s: %s shows an alternate, but only the Catholic edition's own books have one", ed, k)
			case e.Via == "" && deutero && ed != "webc":
				t.Errorf("%s: %s shows itself, but this edition has no such book", ed, k)
			}
		}
		for k := range table {
			if _, ok := keys[k]; !ok {
				t.Errorf("%s: snapshot carries %s, which is no longer an entry — regenerate it", ed, k)
			}
		}
	}
}

// THE FRAGMENTS ARE KNOWN. A passage that begins or ends mid-sentence is
// framed with an ellipsis on the card; that is acceptable for the few below,
// each of which was widened as far as its sentence allowed. A new one must be
// added here deliberately, with the same consideration — the guard is against
// a clause slipping into the rotation unnoticed, as sixty once did.
func TestRotationFragmentsAreTheKnownOnes(t *testing.T) {
	beginsMid := map[string]bool{
		"2 Chronicles 20:15": true, "2 Corinthians 5:7": true, "2 Peter 1:3-4": true,
		"Ephesians 2:8-10": true, "Hebrews 10:23-25": true, "Isaiah 30:21": true,
		"Isaiah 58:11": true, "Philippians 1:6": true, "Philippians 2:3-4": true,
		"Romans 10:9-10": true, "Romans 12:12": true, "Romans 3:23-24": true,
	}
	runsOn := map[string]bool{
		"1 Peter 3:15": true, "1 Timothy 2:5-6": true, "Amos 5:4": true,
		"Deuteronomy 7:9": true, "Ephesians 1:7": true, "Ephesians 2:4-5": true,
		"Exodus 34:6": true, "Isaiah 61:1": true, "Romans 12:12": true, "Romans 3:23-24": true,
	}
	web, ok := loadVOTDSnapshot(t)["web"]
	if !ok {
		t.Fatal("no WEB in the snapshot")
	}
	var gotBegin, gotRun []string
	for k, e := range web {
		if r, _ := utf8.DecodeRuneInString(e.First); unicode.IsLower(r) {
			gotBegin = append(gotBegin, k)
		}
		if r, _ := utf8.DecodeRuneInString(e.Last); unicode.IsLetter(r) || strings.ContainsRune(",;:—–", r) {
			gotRun = append(gotRun, k)
		}
	}
	sort.Strings(gotBegin)
	sort.Strings(gotRun)
	diff := func(name string, got []string, known map[string]bool) {
		seen := map[string]bool{}
		for _, k := range got {
			seen[k] = true
			if !known[k] {
				t.Errorf("%s: %s is new — widen it to its sentence, or add it here on purpose", name, k)
			}
		}
		for k := range known {
			if !seen[k] {
				t.Errorf("%s: %s is listed as a fragment but no longer is — remove it here", name, k)
			}
		}
	}
	diff("begins mid-sentence", gotBegin, beginsMid)
	diff("runs on", gotRun, runsOn)
	if len(gotBegin) == 0 || len(gotRun) == 0 {
		t.Error("the snapshot shows no fragments at all; either the rotation is perfect or the " +
			"snapshot records no text — this test cannot tell, so it fails")
	}
}
