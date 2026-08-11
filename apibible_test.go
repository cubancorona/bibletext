package bibletext

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// --- Decoder ------------------------------------------------------------------

// chapterJSON builds the content-type=json block tree the decoder consumes.
const psalmChapterContent = `[
  {"name":"para","type":"tag","attrs":{"style":"q1"},"items":[
    {"name":"verse","type":"tag","attrs":{"style":"v","number":"1","sid":"PSA 46:1"},"items":[{"type":"text","text":"1"}]},
    {"type":"text","text":"God is our refuge and strength,","attrs":{"verseId":"PSA.46.1"}}
  ]},
  {"name":"para","type":"tag","attrs":{"style":"q2"},"items":[
    {"type":"text","text":"a very present help in trouble.","attrs":{"verseId":"PSA.46.1"}}
  ]},
  {"name":"para","type":"tag","attrs":{"style":"q1"},"items":[
    {"name":"verse","type":"tag","attrs":{"style":"v","number":"2","sid":"PSA 46:2"},"items":[{"type":"text","text":"2"}]},
    {"type":"text","text":"Therefore we won't be afraid,","attrs":{"verseId":"PSA.46.2"}}
  ]}
]`

// proseChapterContent exercises nested char spans (wj = words of Jesus) and a
// verse continuing across two text nodes in one paragraph.
const proseChapterContent = `[
  {"name":"para","type":"tag","attrs":{"style":"p"},"items":[
    {"name":"verse","type":"tag","attrs":{"style":"v","number":"16"},"items":[{"type":"text","text":"16"}]},
    {"name":"char","type":"tag","attrs":{"style":"wj"},"items":[
      {"type":"text","text":"For God so loved the world, ","attrs":{"verseId":"JHN.3.16"}}
    ]},
    {"type":"text","text":"that he gave his one and only Son.","attrs":{"verseId":"JHN.3.16"}},
    {"name":"verse","type":"tag","attrs":{"style":"v","number":"17"},"items":[{"type":"text","text":"17"}]},
    {"type":"text","text":"For God didn't send his Son to judge.","attrs":{"verseId":"JHN.3.17"}}
  ]}
]`

func TestDecodeAPIBibleChapterPoetryLines(t *testing.T) {
	vs, err := decodeAPIBibleChapter(json.RawMessage(psalmChapterContent), "Psalms", 46)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 2 {
		t.Fatalf("got %d verses, want 2: %+v", len(vs), vs)
	}
	// Verse 1 spans two q-paragraphs: the boundary must be an authored "\n"
	// poem line, the convention every rendering surface shares.
	want := "God is our refuge and strength,\na very present help in trouble."
	if vs[0].Text != want {
		t.Errorf("verse 1:\n got  %q\n want %q", vs[0].Text, want)
	}
	if vs[0].Verse != 1 || vs[0].BookName != "Psalms" || vs[0].Chapter != 46 {
		t.Errorf("verse 1 keys wrong: %+v", vs[0])
	}
	if vs[1].Text != "Therefore we won't be afraid," || vs[1].Verse != 2 {
		t.Errorf("verse 2 wrong: %+v", vs[1])
	}
}

func TestDecodeAPIBibleChapterNestedSpans(t *testing.T) {
	vs, err := decodeAPIBibleChapter(json.RawMessage(proseChapterContent), "John", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 2 {
		t.Fatalf("got %d verses, want 2", len(vs))
	}
	// The wj char span nests its text; both nodes belong to verse 16 and join
	// with a single space (prose, no poem break).
	want := "For God so loved the world, that he gave his one and only Son."
	if vs[0].Text != want {
		t.Errorf("verse 16:\n got  %q\n want %q", vs[0].Text, want)
	}
	if vs[1].Verse != 17 {
		t.Errorf("verse 17 not keyed: %+v", vs[1])
	}
}

func TestDecodeAPIBibleChapterRejectsNonJSONContent(t *testing.T) {
	// content-type=html/text answers with a string — must fail loudly, never
	// produce an empty chapter.
	if _, err := decodeAPIBibleChapter(json.RawMessage(`"<p>html soup</p>"`), "John", 3); err == nil {
		t.Fatal("string content must be rejected")
	}
	if _, err := decodeAPIBibleChapter(json.RawMessage(`[]`), "John", 3); err == nil {
		t.Fatal("empty block list must be rejected")
	}
}

func TestVerseNumberFallbacks(t *testing.T) {
	// Ranged verse markers ("17-18") key under the first number, and sid /
	// verseId parsing survives book codes with dots and spaces.
	if got := leadingInt("17-18"); got != 17 {
		t.Errorf("leadingInt range = %d", got)
	}
	if got := verseNumFromID("PSA.46.10"); got != 10 {
		t.Errorf("verseNumFromID = %d", got)
	}
	n := apiBibleNode{}
	n.Attrs.SID = "PSA 46:11"
	if got := verseNumFromMarker(n); got != 11 {
		t.Errorf("verseNumFromMarker sid = %d", got)
	}
}

// --- Full fetch against a fixture server -------------------------------------

// apiBibleFixture serves a tiny 66-book bible: every canonical book exists,
// with one chapter each (so validateBibleData passes), and Psalms carries the
// poetry fixture. It also asserts the api-key header on every request.
func apiBibleFixture(t *testing.T) *httptest.Server {
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
				ID:   usfm,
				Name: usfmToCatholicName[usfm],
				Chapters: []ch{
					{ID: usfm + ".intro", Number: "intro"}, // must be skipped
					{ID: usfm + ".1", Number: "1"},
				},
			})
		}
		json.NewEncoder(w).Encode(map[string]any{"data": books})
	})
	mux.HandleFunc("/bibles/test-bible/chapters/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/bibles/test-bible/chapters/")
		if strings.HasSuffix(id, ".intro") {
			t.Errorf("intro pseudo-chapter %q must not be fetched", id)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.URL.Query().Get("content-type") != "json" {
			t.Errorf("chapter fetched without content-type=json: %s", r.URL.RawQuery)
		}
		content := proseChapterContent
		if strings.HasPrefix(id, "PSA.") {
			content = psalmChapterContent
		}
		fmt.Fprintf(w, `{"data":{"id":%q,"content":%s}}`, id, content)
	})
	return httptest.NewServer(mux)
}

func TestAPIBibleBookNamesComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, usfm := range usfmCanonical66 {
		name := apiBibleBookName(usfm)
		if name == "" {
			t.Errorf("USFM id %s has no app book name", usfm)
		}
		if seen[name] {
			t.Errorf("USFM id %s duplicates book name %q", usfm, name)
		}
		seen[name] = true
	}
	if len(seen) != 66 {
		t.Errorf("got %d unique book names, want 66", len(seen))
	}
}

func TestFetchAPIBibleAssemblesFullCanon(t *testing.T) {
	srv := apiBibleFixture(t)
	defer srv.Close()
	prev := apiBibleBaseURL
	apiBibleBaseURL = srv.URL
	t.Cleanup(func() { apiBibleBaseURL = prev })

	data, err := fetchAPIBible("NKJV", "test-bible", "test-key")
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Books) != 66 {
		t.Fatalf("got %d books, want 66", len(data.Books))
	}
	if data.Books[0] != "Genesis" || data.Books[18] != "Psalms" || data.Books[65] != "Revelation" {
		t.Errorf("canonical order broken: %v ... %v", data.Books[:3], data.Books[63:])
	}
	// Every book must resolve to a real, unique name with verses under it.
	// (The Catholic USFM map knows only the GREEK Esther/Daniel — ESG/DAG —
	// so plain EST/DAN once silently became "": Daniel nameless, Esther
	// overwritten and lost. This pins the whole canon by name.)
	seen := map[string]bool{}
	for _, name := range data.Books {
		if name == "" {
			t.Fatal("empty book name in assembled canon")
		}
		if seen[name] {
			t.Fatalf("duplicate book name %q in assembled canon", name)
		}
		seen[name] = true
		if len(data.Verses[name]) == 0 {
			t.Errorf("book %q assembled with no chapters", name)
		}
	}
	for _, must := range []string{"Esther", "Daniel"} {
		if !seen[must] {
			t.Errorf("book %q missing from assembled canon", must)
		}
	}
	ps := data.Verses["Psalms"][1]
	if len(ps) != 2 || !strings.Contains(ps[0].Text, "\n") {
		t.Errorf("Psalms poetry not preserved through the full fetch: %+v", ps)
	}
	if err := validateBibleData(data); err != nil {
		t.Errorf("assembled data invalid: %v", err)
	}
}

func TestFetchAPIBibleRejectsBadKey(t *testing.T) {
	srv := apiBibleFixture(t)
	defer srv.Close()
	prev := apiBibleBaseURL
	apiBibleBaseURL = srv.URL
	t.Cleanup(func() { apiBibleBaseURL = prev })

	_, err := fetchAPIBible("NKJV", "test-bible", "wrong-key")
	if err == nil || !strings.Contains(err.Error(), "rejected the key") {
		t.Fatalf("want key-rejected error, got %v", err)
	}
}

// --- Licensed-cache compliance gates -----------------------------------------

// writeCacheStampedAt writes a minimal valid cache whose SavedAt is `at`.
func writeCacheStampedAt(t *testing.T, path string, at time.Time) {
	t.Helper()
	data := &BibleData{
		Books: []string{"John"},
		Verses: map[string]map[int][]Verse{
			"John": {3: {{BookName: "John", Book: "John", Chapter: 3, Verse: 16, Text: "For God so loved the world."}}},
		},
	}
	if err := saveBibleToCache(path, data, func() time.Time { return at }); err != nil {
		t.Fatal(err)
	}
}

func TestLicensedCacheStale(t *testing.T) {
	dir := t.TempDir()
	fresh := filepath.Join(dir, "fresh.json")
	stale := filepath.Join(dir, "stale.json")
	writeCacheStampedAt(t, fresh, time.Now().UTC().Add(-24*time.Hour))
	writeCacheStampedAt(t, stale, time.Now().UTC().Add(-licensedRecencyWindow-time.Hour))

	if licensedCacheStale(fresh) {
		t.Error("day-old cache must not be stale")
	}
	if !licensedCacheStale(stale) {
		t.Error("31-day-old cache must be stale — API.Bible §11 requires a 30-day recency check")
	}
	if licensedCacheStale(filepath.Join(dir, "missing.json")) {
		t.Error("a missing cache is absent, not stale")
	}
}

// TestStaleLicensedCacheIsRevalidatedNotServed pins the compliance behaviour
// end to end: a stale licensed cache triggers a refetch, and if the refetch
// fails the app reports an error rather than quietly serving the stale copy —
// the opposite of the public-domain versions' keep-serving-forever fallback.
func TestStaleLicensedCacheIsRevalidatedNotServed(t *testing.T) {
	srv := apiBibleFixture(t)
	defer srv.Close()
	prevURL := apiBibleBaseURL
	apiBibleBaseURL = srv.URL
	t.Cleanup(func() { apiBibleBaseURL = prevURL })

	t.Setenv("BIBLE_API_KEY", "test-key")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "1")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "test-bible")

	dir := t.TempDir()
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(dir, "bibletext-cache.json"))
	v, ok := versionByID("nkjv")
	if !ok {
		t.Fatal("nkjv not registered")
	}
	path := cachePathForVersion("nkjv")
	writeCacheStampedAt(t, path, time.Now().UTC().Add(-licensedRecencyWindow-time.Hour))

	// The fast startup path must MISS on the stale licensed cache.
	if _, _, err := loadVersionFromCacheOnly(v); err == nil {
		t.Fatal("stale licensed cache must not be served from the cache-only path")
	}

	// The full path revalidates: fetches fresh, replaces the cache.
	data, _, err := loadVersionData(v, nil)
	if err != nil {
		t.Fatalf("revalidating load failed: %v", err)
	}
	if len(data.Books) != 66 {
		t.Errorf("revalidated data wrong shape: %d books", len(data.Books))
	}
	if licensedCacheStale(path) {
		t.Error("cache not refreshed by revalidation")
	}

	// Now make it stale again AND kill the network: the load must ERROR, not
	// fall back to the stale copy.
	writeCacheStampedAt(t, path, time.Now().UTC().Add(-licensedRecencyWindow-time.Hour))
	srv.Close()
	if _, _, err := loadVersionData(v, nil); err == nil {
		t.Fatal("stale licensed cache with no network must error, not serve stale")
	}
}

func TestPurgeUnavailableLicensedCaches(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(dir, "bibletext-cache.json"))
	// No licence envs set → nkjv source unavailable.
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "")
	nkjvPath := cachePathForVersion("nkjv")
	writeCacheStampedAt(t, nkjvPath, time.Now().UTC())
	// A public-domain cache must survive the purge untouched.
	webPath := cachePathForVersion("web")
	writeCacheStampedAt(t, webPath, time.Now().UTC())

	purgeUnavailableLicensedCaches()

	if _, err := os.Stat(nkjvPath); !os.IsNotExist(err) {
		t.Error("unlicensed nkjv cache must be removed (API.Bible §10 removal obligation)")
	}
	if _, err := os.Stat(webPath); err != nil {
		t.Error("public-domain cache must never be purged")
	}
}

// --- Passages range walk ------------------------------------------------------

func TestChapterVerseFromRef(t *testing.T) {
	cases := []struct {
		in    string
		ch, v int
	}{
		{"GEN 8:17", 8, 17},
		{"GEN.8.17", 8, 17},
		{"PSA 46:11", 46, 11},
		{"GEN.17.2", 17, 2},
		{"17", 0, 0}, // a bare number is not a reference
		{"", 0, 0},
	}
	for _, c := range cases {
		ch, v := chapterVerseFromRef(c.in)
		if ch != c.ch || v != c.v {
			t.Errorf("chapterVerseFromRef(%q) = (%d,%d), want (%d,%d)", c.in, ch, v, c.ch, c.v)
		}
	}
	if got := passageEndRef("GEN.8.17-GEN.17.2"); got != "GEN.17.2" {
		t.Errorf("passageEndRef = %q", got)
	}
	if got := passageEndRef("GEN.50.26"); got != "GEN.50.26" {
		t.Errorf("passageEndRef single = %q", got)
	}
}

func TestDecodeAPIBiblePassageMultiChapter(t *testing.T) {
	// Two chapters in one content array, the chapter carried by the marker
	// sids exactly as the passages endpoint serves them; the chunk "resumes"
	// mid-book so defaultChapter anchors nothing here.
	content := `[
	  {"name":"para","type":"tag","attrs":{"style":"p"},"items":[
	    {"name":"verse","type":"tag","attrs":{"style":"v","number":"33","sid":"GEN 7:33"},"items":[{"type":"text","text":"33"}]},
	    {"type":"text","text":"Verse of the seventh.","attrs":{"verseId":"GEN.7.33"}},
	    {"name":"verse","type":"tag","attrs":{"style":"v","number":"1","sid":"GEN 8:1"},"items":[{"type":"text","text":"1"}]},
	    {"type":"text","text":"First of the eighth.","attrs":{"verseId":"GEN.8.1"}}
	  ]}
	]`
	m, err := decodeAPIBiblePassage(json.RawMessage(content), "Genesis", 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 || len(m[7]) != 1 || len(m[8]) != 1 {
		t.Fatalf("chapter split wrong: %+v", m)
	}
	if m[7][0].Verse != 33 || m[7][0].Text != "Verse of the seventh." || m[7][0].Chapter != 7 {
		t.Errorf("ch7 verse wrong: %+v", m[7][0])
	}
	if m[8][0].Verse != 1 || m[8][0].Chapter != 8 {
		t.Errorf("ch8 verse wrong: %+v", m[8][0])
	}
}

// apiBiblePassageFixture serves a canon through the PASSAGES endpoint: every
// book one chapter of two verses, except Genesis — three 100-verse chapters,
// so the walk must chunk (cap 200), hit the exactly-at-cap chapter boundary,
// take the 404-then-advance-a-chapter path, and merge chunks.
func apiBiblePassageFixture(t *testing.T) *httptest.Server {
	t.Helper()
	genChapters := 3
	genVerses := 100 // per chapter
	mux := http.NewServeMux()
	mux.HandleFunc("/bibles/pass-bible/books", func(w http.ResponseWriter, r *http.Request) {
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
			b := book{ID: usfm, Name: apiBibleBookName(usfm)}
			n := 1
			if usfm == "GEN" {
				n = genChapters
			}
			for i := 1; i <= n; i++ {
				b.Chapters = append(b.Chapters, ch{ID: fmt.Sprintf("%s.%d", usfm, i), Number: strconv.Itoa(i)})
			}
			books = append(books, b)
		}
		json.NewEncoder(w).Encode(map[string]any{"data": books})
	})
	mux.HandleFunc("/bibles/pass-bible/passages/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("api-key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		rangeID := strings.TrimPrefix(r.URL.Path, "/bibles/pass-bible/passages/")
		parts := strings.SplitN(rangeID, "-", 2)
		seg := strings.Split(parts[0], ".")
		if len(seg) < 3 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		usfm := seg[0]
		startCh, _ := strconv.Atoi(seg[1])
		startV, _ := strconv.Atoi(seg[2])
		chapters := 1
		versesPer := 2
		if usfm == "GEN" {
			chapters, versesPer = genChapters, genVerses
		}
		if startCh > chapters || startV > versesPer {
			w.WriteHeader(http.StatusNotFound) // past the book's real end / invalid verse
			return
		}
		// Serve verses from (startCh, startV) forward, capped.
		var blocks []string
		served := 0
		endCh, endV := 0, 0
		for ch := startCh; ch <= chapters && served < apiBiblePassageCap; ch++ {
			v0 := 1
			if ch == startCh {
				v0 = startV
			}
			var items []string
			for v := v0; v <= versesPer && served < apiBiblePassageCap; v++ {
				items = append(items, fmt.Sprintf(
					`{"name":"verse","type":"tag","attrs":{"style":"v","number":"%d","sid":"%s %d:%d"},"items":[{"type":"text","text":"%d"}]},{"type":"text","text":"Verse %d of chapter %d.","attrs":{"verseId":"%s.%d.%d"}}`,
					v, usfm, ch, v, v, v, ch, usfm, ch, v))
				served++
				endCh, endV = ch, v
			}
			blocks = append(blocks, `{"name":"para","type":"tag","attrs":{"style":"p"},"items":[`+strings.Join(items, ",")+`]}`)
		}
		id := fmt.Sprintf("%s.%d.%d-%s.%d.%d", usfm, startCh, startV, usfm, endCh, endV)
		fmt.Fprintf(w, `{"data":{"id":%q,"verseCount":%d,"content":[%s]}}`, id, served, strings.Join(blocks, ","))
	})
	return httptest.NewServer(mux)
}

func TestFetchAPIBibleByPassages(t *testing.T) {
	srv := apiBiblePassageFixture(t)
	defer srv.Close()
	prev := apiBibleBaseURL
	apiBibleBaseURL = srv.URL
	t.Cleanup(func() { apiBibleBaseURL = prev })

	before := apiBibleCallCount.Load()
	data, err := fetchAPIBible("NKJV", "pass-bible", "test-key")
	if err != nil {
		t.Fatal(err)
	}
	calls := apiBibleCallCount.Load() - before
	if len(data.Books) != 66 {
		t.Fatalf("got %d books, want 66", len(data.Books))
	}
	gen := data.Verses["Genesis"]
	if len(gen) != 3 {
		t.Fatalf("Genesis chapters = %d, want 3: have %v", len(gen), len(gen))
	}
	total := 0
	for ch := 1; ch <= 3; ch++ {
		if len(gen[ch]) != 100 {
			t.Errorf("Genesis %d has %d verses, want 100", ch, len(gen[ch]))
		}
		total += len(gen[ch])
	}
	if gen[2][99].Text != "Verse 100 of chapter 2." || gen[3][0].Text != "Verse 1 of chapter 3." {
		t.Errorf("chunk seam texts wrong: %q / %q", gen[2][99].Text, gen[3][0].Text)
	}
	if err := validateBibleData(data); err != nil {
		t.Errorf("assembled data invalid: %v", err)
	}
	// The whole point: books(1) + 65 single-chunk books + Genesis's walk
	// (chunk, 404 continuation, advanced-chapter chunk) — about 70 calls,
	// not one per chapter.
	if calls > 75 {
		t.Errorf("passage fetch used %d calls — the range walk is not batching", calls)
	}
	t.Logf("passage fetch calls: %d", calls)
}
