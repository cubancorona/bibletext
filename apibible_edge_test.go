package bibletext

// Edge-case tests for the API.Bible decoder and fetch pipeline, extending
// apibible_test.go: ranged and gapped verse numbers, out-of-order content,
// Psalm superscriptions, marker/attr degradation, and the two fetch paths the
// happy-path fixture never exercises (the per-book /chapters fallback and a
// chapter that keeps 500ing through the retry).

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// --- Decoder edge cases -------------------------------------------------------

func TestDecodeAPIBibleEdgeRangedMarker(t *testing.T) {
	// A ranged verse marker ("17-18") keys under the FIRST number at the
	// decoder level: the following text lands on verse 17 and no verse 18
	// entry is invented.
	content := `[
	  {"name":"para","type":"tag","attrs":{"style":"p"},"items":[
	    {"name":"verse","type":"tag","attrs":{"style":"v","number":"17-18"},"items":[{"type":"text","text":"17-18"}]},
	    {"type":"text","text":"Do not think that I came to destroy the Law."}
	  ]}
	]`
	vs, _, err := decodeAPIBibleChapter(json.RawMessage(content), "Matthew", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 {
		t.Fatalf("got %d verses, want 1: %+v", len(vs), vs)
	}
	if vs[0].Verse != 17 {
		t.Errorf("ranged marker keyed verse %d, want 17", vs[0].Verse)
	}
	if vs[0].Text != "Do not think that I came to destroy the Law." {
		t.Errorf("ranged verse text wrong: %q", vs[0].Text)
	}
}

func TestDecodeAPIBibleEdgeVerseGap(t *testing.T) {
	// Some manuscripts omit verses (Acts 8:37 and friends): marker 36 is
	// followed by marker 38. Both decode and no verse 37 is invented.
	content := `[
	  {"name":"para","type":"tag","attrs":{"style":"p"},"items":[
	    {"name":"verse","type":"tag","attrs":{"style":"v","number":"36"},"items":[{"type":"text","text":"36"}]},
	    {"type":"text","text":"Verse thirty-six text."},
	    {"name":"verse","type":"tag","attrs":{"style":"v","number":"38"},"items":[{"type":"text","text":"38"}]},
	    {"type":"text","text":"Verse thirty-eight text."}
	  ]}
	]`
	vs, _, err := decodeAPIBibleChapter(json.RawMessage(content), "Acts", 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 2 {
		t.Fatalf("got %d verses, want 2: %+v", len(vs), vs)
	}
	if vs[0].Verse != 36 || vs[0].Text != "Verse thirty-six text." {
		t.Errorf("verse 36 wrong: %+v", vs[0])
	}
	if vs[1].Verse != 38 || vs[1].Text != "Verse thirty-eight text." {
		t.Errorf("verse 38 wrong: %+v", vs[1])
	}
	for _, v := range vs {
		if v.Verse == 37 {
			t.Errorf("verse 37 invented across the gap: %+v", v)
		}
	}
}

func TestDecodeAPIBibleEdgeOutOfOrder(t *testing.T) {
	// Verses arriving out of order in the JSON still come back sorted — the
	// decoder keys by number and sorts, it does not trust wire order.
	content := `[
	  {"name":"para","type":"tag","attrs":{"style":"p"},"items":[
	    {"name":"verse","type":"tag","attrs":{"style":"v","number":"3"},"items":[{"type":"text","text":"3"}]},
	    {"type":"text","text":"Third verse text."},
	    {"name":"verse","type":"tag","attrs":{"style":"v","number":"1"},"items":[{"type":"text","text":"1"}]},
	    {"type":"text","text":"First verse text."},
	    {"name":"verse","type":"tag","attrs":{"style":"v","number":"2"},"items":[{"type":"text","text":"2"}]},
	    {"type":"text","text":"Second verse text."}
	  ]}
	]`
	vs, _, err := decodeAPIBibleChapter(json.RawMessage(content), "John", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 3 {
		t.Fatalf("got %d verses, want 3: %+v", len(vs), vs)
	}
	wantTexts := []string{"First verse text.", "Second verse text.", "Third verse text."}
	for i, want := range wantTexts {
		if vs[i].Verse != i+1 {
			t.Errorf("position %d holds verse %d, want %d", i, vs[i].Verse, i+1)
		}
		if vs[i].Text != want {
			t.Errorf("verse %d text %q, want %q", i+1, vs[i].Text, want)
		}
	}
}

func TestDecodeAPIBibleEdgeSuperscriptionNoLeak(t *testing.T) {
	// Psalm superscription: a style="d" title paragraph whose text carries NO
	// verseId, before verse 1's marker. It arrives while current==0, and
	// appendText drops verse-0 text — so the title must NOT leak into verse 1.
	content := `[
	  {"name":"para","type":"tag","attrs":{"style":"d"},"items":[
	    {"type":"text","text":"A Psalm of David."}
	  ]},
	  {"name":"para","type":"tag","attrs":{"style":"q1"},"items":[
	    {"name":"verse","type":"tag","attrs":{"style":"v","number":"1","sid":"PSA 23:1"},"items":[{"type":"text","text":"1"}]},
	    {"type":"text","text":"The LORD is my shepherd;","attrs":{"verseId":"PSA.23.1"}}
	  ]}
	]`
	vs, _, err := decodeAPIBibleChapter(json.RawMessage(content), "Psalms", 23)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 {
		t.Fatalf("got %d verses, want 1: %+v", len(vs), vs)
	}
	if vs[0].Verse != 1 {
		t.Errorf("verse keyed as %d, want 1", vs[0].Verse)
	}
	if vs[0].Text != "The LORD is my shepherd;" {
		t.Errorf("verse 1 text wrong: %q", vs[0].Text)
	}
	if strings.Contains(vs[0].Text, "Psalm of David") {
		t.Errorf("superscription leaked into verse 1: %q", vs[0].Text)
	}
}

func TestDecodeAPIBibleEdgeTitleOnlyChapter(t *testing.T) {
	// A chapter that is ONLY a title paragraph (no verse markers, no keyed
	// text) must fail loudly, never succeed as an empty chapter.
	content := `[
	  {"name":"para","type":"tag","attrs":{"style":"d"},"items":[
	    {"type":"text","text":"A Psalm of David."}
	  ]}
	]`
	_, _, err := decodeAPIBibleChapter(json.RawMessage(content), "Psalms", 23)
	if err == nil {
		t.Fatal("title-only chapter must be an error, not success")
	}
	if !strings.Contains(err.Error(), "no verse text") {
		t.Errorf("want the no-verse-text error, got: %v", err)
	}
}

func TestDecodeAPIBibleEdgeDeepNestedChars(t *testing.T) {
	// nd (divine name) and it (italic) char spans nested INSIDE a wj span:
	// all inner text is kept in document order; nd renders UPPERCASE (the
	// plain-text realization of the small-caps divine name, preserving the
	// LORD/Lord distinction), while it (supplied words) stays as-is.
	content := `[
	  {"name":"para","type":"tag","attrs":{"style":"p"},"items":[
	    {"name":"verse","type":"tag","attrs":{"style":"v","number":"5"},"items":[{"type":"text","text":"5"}]},
	    {"name":"char","type":"tag","attrs":{"style":"wj"},"items":[
	      {"type":"text","text":"Truly I tell you, ","attrs":{"verseId":"MAT.5.5"}},
	      {"name":"char","type":"tag","attrs":{"style":"nd"},"items":[
	        {"type":"text","text":"the Lord","attrs":{"verseId":"MAT.5.5"}}
	      ]},
	      {"type":"text","text":" honors ","attrs":{"verseId":"MAT.5.5"}},
	      {"name":"char","type":"tag","attrs":{"style":"it"},"items":[
	        {"type":"text","text":"the meek","attrs":{"verseId":"MAT.5.5"}}
	      ]},
	      {"type":"text","text":" today.","attrs":{"verseId":"MAT.5.5"}}
	    ]}
	  ]}
	]`
	vs, _, err := decodeAPIBibleChapter(json.RawMessage(content), "Matthew", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 {
		t.Fatalf("got %d verses, want 1: %+v", len(vs), vs)
	}
	want := "Truly I tell you, THE LORD honors the meek today."
	if vs[0].Text != want {
		t.Errorf("nested char spans:\n got  %q\n want %q", vs[0].Text, want)
	}
}

func TestDecodeAPIBibleEdgeIgnoresEmptyAndUnknown(t *testing.T) {
	// Empty items arrays, unknown node types, whitespace-only text, and empty
	// trailing paragraphs are all ignored without panicking.
	content := `[
	  {"name":"para","type":"tag","attrs":{"style":"p"},"items":[
	    {"name":"verse","type":"tag","attrs":{"style":"v","number":"1"},"items":[]},
	    {"name":"note","type":"tag","attrs":{"style":"f"},"items":[]},
	    {"type":"milestone","name":"zaln","attrs":{}},
	    {"type":"text","text":"   "},
	    {"type":"text","text":"Real verse text."},
	    {"name":"unknown-widget","type":"tag"}
	  ]},
	  {"name":"para","type":"tag","attrs":{"style":"p"},"items":[]}
	]`
	vs, _, err := decodeAPIBibleChapter(json.RawMessage(content), "John", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 {
		t.Fatalf("got %d verses, want 1: %+v", len(vs), vs)
	}
	if vs[0].Verse != 1 || vs[0].Text != "Real verse text." {
		t.Errorf("verse wrong after junk nodes: %+v", vs[0])
	}
}

func TestDecodeAPIBibleEdgeMarkerWithoutNumber(t *testing.T) {
	// A verse marker with NEITHER number nor sid resolves to 0 and must leave
	// current unchanged: following text keeps accruing to the previous verse
	// (and the marker's own "*" glyph never leaks — its subtree isn't walked).
	content := `[
	  {"name":"para","type":"tag","attrs":{"style":"p"},"items":[
	    {"name":"verse","type":"tag","attrs":{"style":"v","number":"7"},"items":[{"type":"text","text":"7"}]},
	    {"type":"text","text":"Ask, and it will be given you. "},
	    {"name":"verse","type":"tag","attrs":{"style":"v"},"items":[{"type":"text","text":"*"}]},
	    {"type":"text","text":"Seek, and you will find."}
	  ]}
	]`
	vs, _, err := decodeAPIBibleChapter(json.RawMessage(content), "Matthew", 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 {
		t.Fatalf("got %d verses, want 1: %+v", len(vs), vs)
	}
	want := "Ask, and it will be given you. Seek, and you will find."
	if vs[0].Verse != 7 || vs[0].Text != want {
		t.Errorf("numberless marker broke accrual:\n got  %d %q\n want 7 %q", vs[0].Verse, vs[0].Text, want)
	}
}

func TestDecodeAPIBibleEdgeHugeVerseNumber(t *testing.T) {
	// leadingInt discards strconv.Atoi's error, and Atoi SATURATES on range
	// overflow (math.MaxInt + ErrRange) — it never wraps negative and never
	// panics. Pin that: an absurd attrs.number must not panic the decoder and
	// must never key a verse under a non-positive number.
	if got := leadingInt("999999999999999999999"); got <= 0 {
		t.Fatalf("leadingInt overflow must saturate positive, got %d", got)
	}
	content := `[
	  {"name":"para","type":"tag","attrs":{"style":"p"},"items":[
	    {"name":"verse","type":"tag","attrs":{"style":"v","number":"999999999999999999999"},"items":[]},
	    {"type":"text","text":"Overflow-keyed text."}
	  ]}
	]`
	// An absurd marker number is IGNORED (it would overflow the decoder's
	// packed chapter/verse keys): its text cannot key a verse, and since no
	// sane verse precedes it here the chapter correctly fails loudly rather
	// than keying garbage or wrapping negative.
	vs, _, err := decodeAPIBibleChapter(json.RawMessage(content), "John", 1)
	if err == nil {
		t.Fatalf("overflowed marker must not key a verse, got %+v", vs)
	}
	for _, v := range vs {
		if v.Verse <= 0 {
			t.Errorf("overflowed marker keyed verse %d — wrapped negative", v.Verse)
		}
	}
}

// --- Fetch edge cases against fixture servers ---------------------------------

// apiBibleFallbackFixture serves the 66-book canon WITHOUT piggybacked chapter
// lists, forcing fetchAPIBible down the per-book /chapters fallback for every
// book. fallbackHits counts those fallback listings.
func apiBibleFallbackFixture(t *testing.T, fallbackHits *atomic.Int32) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	auth := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("api-key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return false
		}
		return true
	}
	mux.HandleFunc("/bibles/test-bible/books", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		type book struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		var books []book
		for _, usfm := range usfmCanonical66 {
			books = append(books, book{ID: usfm, Name: apiBibleBookName(usfm)})
		}
		json.NewEncoder(w).Encode(map[string]any{"data": books})
	})
	mux.HandleFunc("/bibles/test-bible/books/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, "/bibles/test-bible/books/")
		bookID, ok := strings.CutSuffix(rest, "/chapters")
		if !ok || bookID == "" || strings.Contains(bookID, "/") {
			t.Errorf("unexpected books subpath %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fallbackHits.Add(1)
		fmt.Fprintf(w, `{"data":[{"id":%q,"number":"intro"},{"id":%q,"number":"1"}]}`,
			bookID+".intro", bookID+".1")
	})
	mux.HandleFunc("/bibles/test-bible/chapters/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/bibles/test-bible/chapters/")
		if strings.HasSuffix(id, ".intro") {
			t.Errorf("intro pseudo-chapter %q must not be fetched via the fallback path", id)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		fmt.Fprintf(w, `{"data":{"id":%q,"content":%s}}`, id, proseChapterContent)
	})
	return httptest.NewServer(mux)
}

func TestFetchAPIBibleEdgeChapterListFallback(t *testing.T) {
	var fallbackHits atomic.Int32
	srv := apiBibleFallbackFixture(t, &fallbackHits)
	defer srv.Close()
	prev := apiBibleBaseURL
	apiBibleBaseURL = srv.URL
	t.Cleanup(func() { apiBibleBaseURL = prev })

	data, err := fetchAPIBible("NKJV", "test-bible", "test-key")
	if err != nil {
		t.Fatal(err)
	}
	if got := fallbackHits.Load(); got != 66 {
		t.Errorf("per-book /chapters fallback hit %d times, want 66", got)
	}
	if len(data.Books) != 66 {
		t.Fatalf("got %d books, want 66", len(data.Books))
	}
	vs := data.Verses["Genesis"][1]
	if len(vs) != 2 || vs[0].Verse != 16 || vs[1].Verse != 17 {
		t.Errorf("Genesis 1 wrong through the fallback path: %+v", vs)
	}
}

// apiBibleFlaky500Fixture is the happy-path fixture with one poisoned well:
// Genesis's chapter always answers 500, so both the initial request and the
// retry fail. genesisHits counts the attempts.
func apiBibleFlaky500Fixture(t *testing.T, genesisHits *atomic.Int32) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	auth := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("api-key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return false
		}
		return true
	}
	mux.HandleFunc("/bibles/test-bible/books", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		type ch struct {
			ID     string `json:"id"`
			Number string `json:"number"`
		}
		type book struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Chapters []ch   `json:"chapters"`
		}
		var books []book
		for _, usfm := range usfmCanonical66 {
			books = append(books, book{
				ID:       usfm,
				Name:     apiBibleBookName(usfm),
				Chapters: []ch{{ID: usfm + ".1", Number: "1"}},
			})
		}
		json.NewEncoder(w).Encode(map[string]any{"data": books})
	})
	mux.HandleFunc("/bibles/test-bible/chapters/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/bibles/test-bible/chapters/")
		if strings.HasPrefix(id, "GEN.") {
			genesisHits.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, `{"data":{"id":%q,"content":%s}}`, id, proseChapterContent)
	})
	return httptest.NewServer(mux)
}

func TestFetchAPIBibleEdgeChapter500TwiceFails(t *testing.T) {
	var genesisHits atomic.Int32
	srv := apiBibleFlaky500Fixture(t, &genesisHits)
	defer srv.Close()
	prev := apiBibleBaseURL
	apiBibleBaseURL = srv.URL
	t.Cleanup(func() { apiBibleBaseURL = prev })

	_, err := fetchAPIBible("NKJV", "test-bible", "test-key")
	if err == nil {
		t.Fatal("a chapter that 500s through the retry must fail the whole fetch")
	}
	if !strings.Contains(err.Error(), "Genesis 1") {
		t.Errorf("error must name the failing chapter: %v", err)
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("error must surface the HTTP status: %v", err)
	}
	if got := genesisHits.Load(); got != 2 {
		t.Errorf("failing chapter tried %d times, want 2 (initial + one retry)", got)
	}
}

// A POPULATED note node must never leak the translators' words into verse
// text — the app requests include-notes=true and captures bodies into the
// side-band, so the decoder must keep them out of Text regardless of what
// the server sends. (The empty-note case above
// proved nothing: an empty subtree has nothing to leak.)
func TestDecodeAPIBibleEdgeSkipsPopulatedNoteNodes(t *testing.T) {
	content := `[
	  {"name":"para","type":"tag","attrs":{"style":"p"},"items":[
	    {"name":"verse","type":"tag","attrs":{"style":"v","number":"13","sid":"JHN 3:13"},"items":[{"type":"text","text":"13"}]},
	    {"type":"text","text":"No one has ascended to heaven","attrs":{"verseId":"JHN.3.13"}},
	    {"name":"note","type":"tag","attrs":{"style":"f"},"items":[
	      {"name":"char","type":"tag","attrs":{"style":"fr"},"items":[{"type":"text","text":"3:13 "}]},
	      {"name":"char","type":"tag","attrs":{"style":"ft"},"items":[{"type":"text","text":"Alpha-Text omits the fixture clause."}]}
	    ]},
	    {"type":"text","text":" but the fixture clause continues here.","attrs":{"verseId":"JHN.3.13"}}
	  ]}
	]`
	vs, _, err := decodeAPIBibleChapter(json.RawMessage(content), "John", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 {
		t.Fatalf("got %d verses, want 1", len(vs))
	}
	want := "No one has ascended to heaven but the fixture clause continues here."
	if vs[0].Text != want {
		t.Errorf("footnote leaked into Scripture:\n got  %q\n want %q", vs[0].Text, want)
	}
}
