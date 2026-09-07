package bibletext

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Every entry in the generated NKJV table must be ACCEPTED by the guard that
// reads it, against the text the app really holds.
//
// A refused entry is not inert. The guard refuses a verse whose text no longer
// matches the fingerprint the table was built from, and redLetterRuns then
// paints that verse red from end to end — so in the places where Christ quotes
// the Old Testament, the narrator's "Jesus said to him," is printed as though
// he had said it too. Nothing else can catch this: the licensed text is not in
// this repository, so the table can only be checked against itself, and against
// itself it is always perfectly consistent.
//
// Skips without the licensed cache, which is where every other check of this
// text lives too. Write one with the app, or with the fetch the release
// pipeline uses.
func TestNKJVRedLetterTableIsAcceptedByItsOwnGuard(t *testing.T) {
	path := cachePathForVersion("nkjv")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("no licensed cache at %s — open the NKJV in the app once", path)
	}
	var c struct {
		Data struct {
			Verses map[string]map[string][]Verse `json:"Verses"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("cache is unreadable: %v", err)
	}
	text := map[string]Verse{}
	for book, chs := range c.Data.Verses {
		for _, vs := range chs {
			for _, v := range vs {
				text[verseKeyFor(book, v.Chapter, v.Verse)] = v
			}
		}
	}
	var missing, refused, accepted int
	var refusedKeys []string
	for key := range nkjvRedLetterSpans {
		v, ok := text[key]
		if !ok {
			missing++
			continue
		}
		if _, ok := nkjvRedLetterSpansFor(v.BookName, v.Chapter, v.Verse, v.Text); ok {
			accepted++
			continue
		}
		refused++
		if len(refusedKeys) < 20 {
			refusedKeys = append(refusedKeys, key)
		}
	}
	if accepted == 0 {
		t.Fatal("no entry was checked against real text — the test proved nothing")
	}
	if refused > 0 {
		t.Errorf("%d of %d entries are refused by the guard, so those verses would be painted "+
			"red end to end with the narrator's words among them: %s",
			refused, len(nkjvRedLetterSpans), strings.Join(refusedKeys, ", "))
	}
	if missing > 0 {
		t.Errorf("%d entries name a verse the cache has no text for", missing)
	}
	t.Logf("%d entries, all accepted against the text at %s", accepted, path)
}
