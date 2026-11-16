package bibletext

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const publisherEntryVerseText = "A synthetic passage for attachment checks."
const publisherEntryHeadingText = "Synthetic section."

type publisherEntryCase struct {
	versionID string
	book      string
	bookID    string
	bookOrder int
	chapter   int
	verse     int
	citation  string
	target    string
	heading   bool
}

func publisherEntryCases() []publisherEntryCase {
	return []publisherEntryCase{
		{versionID: "web", book: "Matthew", bookID: "MAT", bookOrder: 40, chapter: 5, verse: 43,
			citation: "Leviticus 19:18", target: "LEV.19.18"},
		{versionID: "webc", book: "Tobit", bookID: "TOB", chapter: 2, verse: 6,
			citation: "Amos 8:10", target: "AMO.8.10"},
		{versionID: "bsb", book: "1 Chronicles", bookID: "1CH", bookOrder: 13, chapter: 16, verse: 7,
			citation: "Psalm 105:1–15", target: "PSA.105.1-PSA.105.15", heading: true},
	}
}

// publisherEntryBible deliberately contains no recovered notes. Loader tests
// must prove their own attachment step, independently of the source decoder.
func publisherEntryBible(tc publisherEntryCase) *BibleData {
	bd := &BibleData{
		Books: []string{tc.book},
		Verses: map[string]map[int][]Verse{
			tc.book: {tc.chapter: {{BookName: tc.book, Book: tc.book, Chapter: tc.chapter, Verse: tc.verse, Text: publisherEntryVerseText}}},
		},
	}
	if tc.heading {
		bd.Headings = map[string]map[int][]Heading{
			tc.book: {tc.chapter: {{Text: publisherEntryHeadingText, Style: "heading", BeforeVerse: tc.verse}}},
		}
	}
	return bd
}

func publisherEntryBody(t *testing.T, tc publisherEntryCase) []byte {
	t.Helper()
	content := []any{}
	if tc.heading {
		content = append(content, map[string]any{"type": "heading", "content": []string{publisherEntryHeadingText}})
	}
	content = append(content, map[string]any{"type": "verse", "number": tc.verse, "content": []string{publisherEntryVerseText}})
	body, err := json.Marshal(map[string]any{
		"translation": map[string]any{"shortName": "SYNTHETIC"},
		"books": []any{map[string]any{
			"id": tc.bookID, "order": tc.bookOrder, "totalNumberOfVerses": 1,
			"chapters": []any{map[string]any{
				"numberOfVerses": 1,
				"chapter":        map[string]any{"number": tc.chapter, "content": content},
			}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func assertPublisherEntryNotes(t *testing.T, tc publisherEntryCase, bd *BibleData) {
	t.Helper()
	if bd == nil {
		t.Fatal("no decoded Bible")
	}
	v := bd.GetVerse(tc.book, tc.chapter, tc.verse)
	if v == nil {
		t.Fatal("synthetic host verse was lost")
	}
	if v.Text != publisherEntryVerseText {
		t.Error("attachment changed or replaced the synthetic verse text")
	}
	notes := v.Footnotes
	if tc.heading {
		if len(notes) != 0 {
			t.Errorf("heading note appeared on verse: %d notes", len(notes))
		}
		heads := bd.Headings[tc.book][tc.chapter]
		if len(heads) != 1 {
			t.Fatalf("got %d headings, want the one synthetic heading", len(heads))
		}
		if heads[0].Text != publisherEntryHeadingText {
			t.Error("attachment changed or replaced the synthetic heading text")
		}
		notes = heads[0].Footnotes
	}
	if len(notes) != 1 {
		t.Fatalf("got %d recovered notes, want 1", len(notes))
	}
	note := notes[0]
	if note.Kind != footnoteKindCrossref || note.Text != tc.citation {
		t.Error("recovered note does not match the edition's citation")
	}
	if len(note.Refs) != 1 || note.Refs[0].ID != tc.target {
		t.Error("recovered note does not retain its expected target")
	}
}

func testPublisherEntryDecoder(t *testing.T, versionID string, decode func([]byte) (*BibleData, error)) {
	t.Helper()
	for _, tc := range publisherEntryCases() {
		if tc.versionID != versionID {
			continue
		}
		bd, err := decode(publisherEntryBody(t, tc))
		if err != nil {
			t.Fatal(err)
		}
		assertPublisherEntryNotes(t, tc, bd)
		return
	}
	t.Fatal("missing decoder fixture")
}

func TestPublisherCrossRefsDecodeWEB(t *testing.T) {
	testPublisherEntryDecoder(t, "web", decodeWEB)
}

func TestPublisherCrossRefsDecodeBSB(t *testing.T) {
	testPublisherEntryDecoder(t, "bsb", decodeBSB)
}

func TestPublisherCrossRefsDecodeHelloAOCatholic(t *testing.T) {
	testPublisherEntryDecoder(t, "webc", decodeHelloAOCatholic)
}

func TestPublisherCrossRefsCacheOnlyCurrent(t *testing.T) {
	testPublisherEntryLoader(t, "cache-only-current")
}

func TestPublisherCrossRefsCacheOnlySuperseded(t *testing.T) {
	testPublisherEntryLoader(t, "cache-only-superseded")
}

func TestPublisherCrossRefsVersionDataCache(t *testing.T) {
	testPublisherEntryLoader(t, "version-data-cache")
}

func TestPublisherCrossRefsVersionDataFetch(t *testing.T) {
	testPublisherEntryLoader(t, "version-data-fetch")
}

func testPublisherEntryLoader(t *testing.T, route string) {
	t.Helper()
	for _, tc := range publisherEntryCases() {
		t.Run(tc.versionID, func(t *testing.T) {
			t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(t.TempDir(), "cache.json"))
			v, ok := versionByID(tc.versionID)
			if !ok {
				t.Fatal("public edition is not registered")
			}
			calls := 0
			v.source = &fakeVersionSource{avail: true, data: publisherEntryBible(tc), calls: &calls}
			currentPath := cachePathForVersion(v.ID)
			wantCalls := 0
			switch route {
			case "cache-only-current", "version-data-cache":
				mustCache(t, currentPath, publisherEntryBible(tc))
			case "cache-only-superseded":
				older := supersededCachePaths(v)
				if len(older) == 0 {
					t.Fatal("control: edition needs a previous cache epoch")
				}
				mustCache(t, older[0], publisherEntryBible(tc))
				if _, err := os.Stat(currentPath); !os.IsNotExist(err) {
					t.Fatal("control: current cache must be absent")
				}
			case "version-data-fetch":
				wantCalls = 1
			default:
				t.Fatalf("unknown loader route %q", route)
			}
			var bd *BibleData
			var mode dataMode
			var err error
			if route == "cache-only-current" || route == "cache-only-superseded" {
				bd, mode, err = loadVersionFromCacheOnly(v)
			} else {
				bd, mode, err = loadVersionData(v, nil)
			}
			if err != nil {
				t.Fatal(err)
			}
			if mode != modeReal {
				t.Errorf("got mode %v, want real data", mode)
			}
			if calls != wantCalls {
				t.Errorf("source fetched %d times, want %d", calls, wantCalls)
			}
			assertPublisherEntryNotes(t, tc, bd)
		})
	}
}
