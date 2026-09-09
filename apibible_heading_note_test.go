package bibletext

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// S9 of docs/SCRIPTURE_WORKLIST.md: do the NKJV's section headings carry notes,
// and how many?
//
// The design question this was originally asked for is settled — a note inside
// a heading block is kept on the heading itself (Heading.Footnotes, set in
// apibible.go), rather than landing on whichever verse happened to be current.
// What was never known is the COUNT, and nothing on disk can say: the cached
// chapters are New Testament only, and a heading's notes are not written into
// the cache in a form that can be counted after the fact.
//
// So this is a probe, not a guard. Five passages, chosen where a heading is
// most likely to carry a note: the parallel-passage reference lines in the
// Gospels and Chronicles, and the acrostic headings of Psalm 119. It costs
// about five API calls against the NKJV quota and skips without a key, so it
// never runs in CI and never runs by accident.
//
//	BIBLE_API_KEY=… go test -run TestLiveNKJVHeadingNotes -v .
func TestLiveNKJVHeadingNotes(t *testing.T) {
	key := os.Getenv("BIBLE_API_KEY")
	if key == "" {
		t.Skip("BIBLE_API_KEY not set — the NKJV heading-note probe costs quota and is skipped")
	}
	bibleID := os.Getenv("BIBLETEXT_PROVIDER_ID_NKJV")
	if bibleID == "" {
		bibleID = "63097d2a0a2f7db3-01"
	}

	// Where a heading most plausibly carries a note. Parallel-passage lines
	// (the "r" style) are references by nature, so they are the best chance of
	// finding one; Psalm 119 is the densest run of headings in the canon.
	probes := []struct {
		name string
		book string
		rng  string
		why  string
	}{
		{"Matthew 3", "Matthew", "MAT.3.1-MAT.3.17", "parallel-passage lines in a Gospel"},
		{"Mark 1", "Mark", "MRK.1.1-MRK.1.45", "the same, in the most heavily cross-referenced Gospel"},
		{"2 Chronicles 1", "2 Chronicles", "2CH.1.1-2CH.1.17", "parallels the Kings narrative"},
		{"Psalm 119", "Psalms", "PSA.119.1-PSA.119.48", "twenty-two acrostic headings"},
		{"Hebrews 1", "Hebrews", "HEB.1.1-HEB.1.14", "dense Old Testament citation"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), apiBibleRequestTimeout)
	defer cancel()
	client := newHTTPClient()
	client.Timeout = apiBibleRequestTimeout

	totalHeadings, withNotes, totalNotes := 0, 0, 0
	byStyle := map[string]int{}
	var examples []string

	for _, p := range probes {
		path := "/bibles/" + bibleID + "/passages/" + p.rng + "?" + apiBibleContentQuery
		var pr struct {
			Data struct {
				Content json.RawMessage `json:"content"`
			} `json:"data"`
		}
		if err := apiBibleGet(ctx, client, key, path, &pr); err != nil {
			t.Errorf("%s: %v", p.name, err)
			continue
		}
		_, _, _, heads, err := decodeAPIBiblePassage(pr.Data.Content, p.book, 1)
		if err != nil {
			t.Errorf("%s: %v", p.name, err)
			continue
		}
		for ch, hs := range heads {
			for _, h := range hs {
				totalHeadings++
				byStyle[h.Style]++
				if len(h.Footnotes) == 0 {
					continue
				}
				withNotes++
				totalNotes += len(h.Footnotes)
				if len(examples) < 6 {
					examples = append(examples, fmt.Sprintf("%s %d %q [%s] → %q",
						p.book, ch, h.Text, h.Style,
						strings.TrimSpace(h.Footnotes[0].Text)))
				}
			}
		}
	}

	if totalHeadings == 0 {
		t.Fatal("no headings were decoded from any probe; the probe is measuring nothing")
	}

	t.Logf("NKJV heading-note probe: %d headings across 5 passages, %d carry notes (%d notes total)",
		totalHeadings, withNotes, totalNotes)
	var styles []string
	for s, n := range byStyle {
		styles = append(styles, fmt.Sprintf("%s×%d", s, n))
	}
	t.Logf("  heading styles seen: %s", strings.Join(styles, ", "))
	for _, e := range examples {
		t.Logf("  %s", e)
	}
	if withNotes == 0 {
		t.Log("  ANSWER: no heading in these five passages carries a note. Record this in " +
			"docs/SOURCE_FIELDS.md so the question is not asked a third time.")
	}
}
