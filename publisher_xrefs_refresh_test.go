package bibletext

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func publisherRefreshFixture() *BibleData {
	return &BibleData{
		Books: []string{"John"},
		Verses: map[string]map[int][]Verse{"John": {1: {
			{BookName: "John", Book: "John", Chapter: 1, Verse: 1, Text: "Invented first passage.", Footnotes: []Footnote{{Text: "A translation note."}}},
			{BookName: "John", Book: "John", Chapter: 1, Verse: 2, Text: "Invented second passage.", Footnotes: []Footnote{{Kind: footnoteKindCrossref, Caller: "+", Text: "Mark 1:1"}}},
		}}},
		Headings: map[string]map[int][]Heading{"John": {1: {
			{Text: "First section", Style: "heading", BeforeVerse: 1, Footnotes: []Footnote{{Text: "A heading note."}}},
			{Text: "Second section1:2 A run-in heading note.", Style: "heading", BeforeVerse: 2},
		}}},
	}
}

func publisherRefreshRoundTrip(t *testing.T, bd *BibleData) *BibleData {
	t.Helper()
	path := filepath.Join(t.TempDir(), "translation.json")
	if err := saveBibleToCache(path, bd, currentUTCTime); err != nil {
		t.Fatal(err)
	}
	got, err := loadBibleFromCache(path)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestPublisherTableVerseCacheRefresh(t *testing.T) {
	bd := publisherRefreshFixture()
	feedNotes := append([]Footnote(nil), bd.Verses["John"][1][1].Footnotes...)
	table := map[string]publisherVerseEntry{
		"John 1:1": {Notes: []publisherVerseNote{{AtEnd: true, Text: "Luke 1:1", Refs: []NoteRef{{ID: "LUK.1.1", Start: 0, End: 8}}}}},
		"John 1:2": {Notes: []publisherVerseNote{{AtEnd: true, Text: "Luke 1:2"}}},
	}
	applyPublisherVerseCrossRefs(bd, table)
	bd = publisherRefreshRoundTrip(t, bd)
	if ns := bd.Verses["John"][1][0].Footnotes; len(ns) != 2 || !ns[1].PublisherTable {
		t.Fatal("cache did not preserve recovered-note provenance")
	}
	table["John 1:1"] = publisherVerseEntry{Notes: []publisherVerseNote{{AtEnd: true, Text: "Luke 1:3", Refs: []NoteRef{{ID: "LUK.1.3", Start: 0, End: 8}}}}}
	applyPublisherVerseCrossRefs(bd, table)
	applyPublisherVerseCrossRefs(bd, table)
	ns := bd.Verses["John"][1][0].Footnotes
	if len(ns) != 2 || ns[0].Text != "A translation note." || ns[1].Text != "Luke 1:3" || len(ns[1].Refs) != 1 || ns[1].Refs[0].ID != "LUK.1.3" {
		t.Fatal("table revision was not applied exactly once")
	}
	if !reflect.DeepEqual(bd.Verses["John"][1][1].Footnotes, feedNotes) {
		t.Fatal("feed cross-reference changed")
	}
	bd = publisherRefreshRoundTrip(t, bd)
	applyPublisherVerseCrossRefs(bd, nil)
	if ns = bd.Verses["John"][1][0].Footnotes; len(ns) != 1 || ns[0].Text != "A translation note." {
		t.Fatal("deleted table entry survived cache refresh")
	}
	if !reflect.DeepEqual(bd.Verses["John"][1][1].Footnotes, feedNotes) {
		t.Fatal("table deletion removed a feed note")
	}
}

func TestPublisherTableHeadingCacheRefresh(t *testing.T) {
	bd := publisherRefreshFixture()
	raw := bd.Headings["John"][1][1].Text
	table := map[string][]publisherHeadingNote{"John 1": {{BeforeVerse: 1, HeadingCount: 1, HeadingHash: redLetterTextHash("First section"), Text: "Luke 1:1"}}}
	applyPublisherHeadingCrossRefs(bd, table)
	if len(bd.Headings["John"][1][0].Footnotes) != 2 {
		t.Fatal("recovered note lost when another heading has a run-in note")
	}
	if bd.Headings["John"][1][1].Text != raw {
		t.Fatal("stored feed heading was rewritten")
	}
	bd = publisherRefreshRoundTrip(t, bd)
	if !bd.Headings["John"][1][0].Footnotes[1].PublisherTable {
		t.Fatal("heading provenance lost in cache")
	}
	table["John 1"][0].Text = "Luke 1:3"
	applyPublisherHeadingCrossRefs(bd, table)
	applyPublisherHeadingCrossRefs(bd, table)
	ns := bd.Headings["John"][1][0].Footnotes
	if len(ns) != 2 || ns[0].Text != "A heading note." || ns[1].Text != "Luke 1:3" {
		t.Fatal("heading table revision accumulated stale references")
	}
	bd = publisherRefreshRoundTrip(t, bd)
	applyPublisherHeadingCrossRefs(bd, nil)
	if ns = bd.Headings["John"][1][0].Footnotes; len(ns) != 1 || ns[0].Text != "A heading note." {
		t.Fatal("deleted heading entry survived cache refresh")
	}
	if bd.Headings["John"][1][1].Text != raw {
		t.Fatal("cache refresh rewrote a stored heading")
	}
}

func TestPublisherHeadingFallbackRequiresOriginalSingleton(t *testing.T) {
	for _, tc := range []struct {
		name          string
		originalCount int
		title         string
		want          bool
	}{
		{"sole renamed heading", 1, "Revised section", true},
		{"one survivor of two renamed headings", 2, "Revised section", false},
		{"unique exact survivor", 2, "Original section", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bd := &BibleData{Headings: map[string]map[int][]Heading{"John": {1: {{Text: tc.title, Style: "heading", BeforeVerse: 1}}}}}
			table := map[string][]publisherHeadingNote{"John 1": {{BeforeVerse: 1, HeadingCount: tc.originalCount, HeadingHash: redLetterTextHash("Original section"), Text: "Luke 1:1"}}}
			applyPublisherHeadingCrossRefs(bd, table)
			if got := len(bd.Headings["John"][1][0].Footnotes) > 0; got != tc.want {
				t.Fatalf("attached=%v want %v", got, tc.want)
			}
		})
	}
}

func TestPublicPublisherBSBCanonAllAttachments(t *testing.T) {
	if os.Getenv("BIBLETEXT_XREF_REALDATA") == "" {
		t.Skip("set BIBLETEXT_XREF_REALDATA=1 for local public-canon check")
	}
	bd, err := loadBibleFromCache(realCachePath(cachePathForVersion("bsb")))
	if err != nil {
		t.Skipf("public cache unavailable: %v", err)
	}
	// Remove prior experimental notes in memory so every association must be
	// established afresh, including caches made before provenance was recorded.
	for _, chapters := range bd.Headings {
		for _, hs := range chapters {
			for i := range hs {
				var kept []Footnote
				for _, n := range hs[i].Footnotes {
					if n.Kind != footnoteKindCrossref {
						kept = append(kept, n)
					}
				}
				hs[i].Footnotes = kept
			}
		}
	}
	applyPublisherCrossRefs("bsb", bd)
	count := 0
	for key, entries := range bsbPublisherCrossRefs {
		book, ch, ok := splitPublisherChapterKey(key)
		if !ok {
			t.Fatal("invalid chapter key")
		}
		for _, entry := range entries {
			matches := 0
			for _, h := range bd.Headings[book][ch] {
				if h.BeforeVerse != entry.BeforeVerse || redLetterTextHash(h.Text) != entry.HeadingHash {
					continue
				}
				for _, n := range h.Footnotes {
					if n.PublisherTable && n.Kind == footnoteKindCrossref && n.Text == entry.Text && reflect.DeepEqual(n.Refs, entry.Refs) {
						matches++
					}
				}
			}
			if matches != 1 {
				t.Fatalf("%s before verse %d: attached %d times", key, entry.BeforeVerse, matches)
			}
			count++
		}
	}
	if count != 1325 {
		t.Fatalf("attached %d references, want 1325", count)
	}
}
