package bibletext

// The seams cmd/websitegen publishes a licensed translation through. Hermetic:
// API.Bible is a local server answering at the NKJV's real provider path with
// synthetic text in the provider's content shape, the key is a synthetic
// string, and the suite's own cache directory (TestMain) is the only disk the
// fetch could reach.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

const siteFixtureKey = "synthetic-site-key-0123456789"

// licensedSiteFixture serves the whole 66-book canon, one two-verse chapter a
// book, through the PASSAGES endpoint the real walk uses, at the registry's
// own provider id and nowhere else. copyright names the line each book's
// response carries. Every request is counted, and one that carries the key
// anywhere but the api-key header fails the test.
func licensedSiteFixture(t *testing.T, hits *atomic.Int64, copyright func(usfm string) string) {
	t.Helper()
	base := "/bibles/" + nkjvProviderBibleID
	mux := http.NewServeMux()
	auth := func(w http.ResponseWriter, r *http.Request) bool {
		hits.Add(1)
		if strings.Contains(r.URL.String(), siteFixtureKey) {
			t.Errorf("the key travelled in the request URL: %s", r.URL.Path)
		}
		if r.Header.Get("api-key") != siteFixtureKey {
			w.WriteHeader(http.StatusUnauthorized)
			return false
		}
		return true
	}
	mux.HandleFunc(base+"/books", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		type ch struct {
			ID     string `json:"id"`
			Number string `json:"number"`
		}
		type book struct {
			ID       string `json:"id"`
			Chapters []ch   `json:"chapters"`
		}
		var books []book
		for _, usfm := range usfmCanonical66 {
			books = append(books, book{ID: usfm, Chapters: []ch{{ID: usfm + ".1", Number: "1"}}})
		}
		json.NewEncoder(w).Encode(map[string]any{"data": books})
	})
	mux.HandleFunc(base+"/passages/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		rangeID := strings.TrimPrefix(r.URL.Path, base+"/passages/")
		usfm := strings.SplitN(rangeID, ".", 2)[0]
		content := fmt.Sprintf(`[{"name":"para","type":"tag","attrs":{"style":"p"},"items":[
		  {"name":"verse","type":"tag","attrs":{"style":"v","number":"1","sid":"%[1]s 1:1"},"items":[{"type":"text","text":"1"}]},
		  {"type":"text","text":"Synthetic licensed verse one.","attrs":{"verseId":"%[1]s.1.1"}},
		  {"name":"verse","type":"tag","attrs":{"style":"v","number":"2","sid":"%[1]s 1:2"},"items":[{"type":"text","text":"2"}]},
		  {"type":"text","text":"Synthetic licensed verse two.","attrs":{"verseId":"%[1]s.1.2"}}
		]}]`, usfm)
		body, _ := json.Marshal(map[string]any{"data": map[string]any{
			"id": usfm + ".1.1-" + usfm + ".1.2", "verseCount": 2,
			"copyright": copyright(usfm), "content": json.RawMessage(content),
		}})
		w.Write(body)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	prev := apiBibleBaseURL
	apiBibleBaseURL = srv.URL
	t.Cleanup(func() { apiBibleBaseURL = prev })
}

func oneCopyrightLine(string) string { return "Synthetic copyright line." }

// listCacheDir names every file in the suite's translation cache directory.
func listCacheDir(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(cachePathForVersion("nkjv")))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// The site's fetch takes its key and its provider id from nowhere but its
// argument and the registry. The environment here says otherwise on every
// count — a different key, a different provider id, the operator opt-in — and
// the fixture answers only the registry's id with only the argument's key, so
// a fetch that listened to any of them fails.
func TestFetchLicensedEditionUsesOnlyWhatItIsGiven(t *testing.T) {
	var hits atomic.Int64
	licensedSiteFixture(t, &hits, oneCopyrightLine)
	t.Setenv("BIBLE_API_KEY", "synthetic-environment-key-never-used")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "synthetic-other-bible")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "1")
	cacheBefore := listCacheDir(t)

	ed, err := FetchLicensedEdition("nkjv", siteFixtureKey)
	if err != nil {
		t.Fatalf("FetchLicensedEdition: %v", err)
	}
	if ed.Bible == nil || len(ed.Bible.Books) != 66 {
		t.Fatalf("fetched %v books, want the 66-book canon", ed.Bible)
	}
	if got := ed.Bible.Verses["John"][1]; len(got) != 2 || got[0].Text != "Synthetic licensed verse one." {
		t.Errorf("John 1 decoded as %+v", got)
	}
	if ed.Requests != hits.Load() || ed.Requests != 67 {
		t.Errorf("Requests = %d, the server saw %d; want 67 (the canon, then a passage a book)", ed.Requests, hits.Load())
	}
	if ed.CopyrightLines != 1 {
		t.Errorf("CopyrightLines = %d, want 1", ed.CopyrightLines)
	}
	if after := listCacheDir(t); !reflect.DeepEqual(after, cacheBefore) {
		t.Errorf("the fetch wrote to the translation cache: before %v, after %v", cacheBefore, after)
	}
}

// Two different lines across the canon is a provider whose notice the site
// could not print as one — the count is what lets the generator refuse it.
func TestFetchLicensedEditionCountsDistinctCopyrightLines(t *testing.T) {
	var hits atomic.Int64
	licensedSiteFixture(t, &hits, func(usfm string) string {
		if usfm == "REV" {
			return "A second synthetic copyright line."
		}
		return "Synthetic copyright line."
	})
	ed, err := FetchLicensedEdition("nkjv", siteFixtureKey)
	if err != nil {
		t.Fatalf("FetchLicensedEdition: %v", err)
	}
	if ed.CopyrightLines != 2 {
		t.Errorf("CopyrightLines = %d, want 2", ed.CopyrightLines)
	}
	// And a second fetch starts its count afresh rather than inheriting.
	licensedSiteFixture(t, &hits, oneCopyrightLine)
	if ed, err = FetchLicensedEdition("nkjv", siteFixtureKey); err != nil || ed.CopyrightLines != 1 {
		t.Errorf("a later one-line fetch reports %d lines (err %v), want 1", ed.CopyrightLines, err)
	}
}

func TestFetchLicensedEditionRefusesBeforeAnyRequest(t *testing.T) {
	var hits atomic.Int64
	licensedSiteFixture(t, &hits, oneCopyrightLine)
	t.Setenv("BIBLE_API_KEY", "synthetic-environment-key-never-used")
	for _, tc := range []struct{ id, key, why string }{
		{"nkjv", "", "an empty key must not fall back to any other credential"},
		{"web", siteFixtureKey, "a public-domain edition has no licensed source"},
		{"no-such-version", siteFixtureKey, "an unknown id"},
	} {
		if _, err := FetchLicensedEdition(tc.id, tc.key); err == nil {
			t.Errorf("FetchLicensedEdition(%q) succeeded: %s", tc.id, tc.why)
		} else if strings.Contains(err.Error(), "synthetic-environment-key") {
			t.Errorf("an error carries a key: %v", err)
		}
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("%d requests were sent for fetches that should have been refused outright", n)
	}
}

func TestFetchLicensedEditionReportsARejectedKey(t *testing.T) {
	var hits atomic.Int64
	licensedSiteFixture(t, &hits, oneCopyrightLine)
	_, err := FetchLicensedEdition("nkjv", "synthetic-wrong-key-0123456789")
	if err == nil || !strings.Contains(err.Error(), "rejected the key") {
		t.Fatalf("a rejected key gave %v", err)
	}
	if strings.Contains(err.Error(), "synthetic-wrong-key") {
		t.Errorf("the error carries the key: %v", err)
	}
}

// The NKJV's measured delta, applied to a synthetic reference: the Romans
// doxology leaves chapter 14 and closes chapter 16, and Acts 8:37 — which the
// reference lacks — is expected.
func TestExpectedVerseNumbersFollowTheDelta(t *testing.T) {
	numbered := func(book string, ch int, nums ...int) []Verse {
		var out []Verse
		for _, n := range nums {
			out = append(out, Verse{BookName: book, Chapter: ch, Verse: n, Text: "x"})
		}
		return out
	}
	span := func(lo, hi int) []int {
		var out []int
		for n := lo; n <= hi; n++ {
			out = append(out, n)
		}
		return out
	}
	ref := &BibleData{
		Books: []string{"John", "Acts", "Romans"},
		Verses: map[string]map[int][]Verse{
			"John":   {3: numbered("John", 3, span(1, 36)...)},
			"Acts":   {8: numbered("Acts", 8, append(span(1, 36), 38, 39, 40)...)},
			"Romans": {14: numbered("Romans", 14, span(1, 26)...), 16: numbered("Romans", 16, span(1, 24)...)},
		},
	}
	got, err := ExpectedVerseNumbers("nkjv", ref)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		book string
		ch   int
		want []int
	}{
		{"John", 3, span(1, 36)},
		{"Acts", 8, span(1, 40)},
		{"Romans", 14, span(1, 23)},
		{"Romans", 16, span(1, 27)},
	} {
		if !reflect.DeepEqual(got[tc.book][tc.ch], tc.want) {
			t.Errorf("%s %d = %v, want %v", tc.book, tc.ch, got[tc.book][tc.ch], tc.want)
		}
	}
	// Luke is not in the reference, so its extra verse promises nothing.
	if _, ok := got["Luke"]; ok {
		t.Error("a book the reference lacks gained an expected chapter from the delta's extras")
	}

	// The reference numbers like itself.
	self, err := ExpectedVerseNumbers("web", ref)
	if err != nil || !reflect.DeepEqual(self["Acts"][8], append(span(1, 36), 38, 39, 40)) {
		t.Errorf("the reference's own numbers came back as %v (%v)", self["Acts"][8], err)
	}
	// No delta, no promise.
	if _, err := ExpectedVerseNumbers("no-such-version", ref); err == nil {
		t.Error("an id with no delta was assumed to number like the reference")
	}
	// A book whose numbering does not correspond at all cannot be checked.
	ref.Books = append(ref.Books, "Esther")
	ref.Verses["Esther"] = map[int][]Verse{1: numbered("Esther", 1, span(1, 22)...)}
	if _, err := ExpectedVerseNumbers("webc", ref); err == nil {
		t.Error("WEBC's Greek Esther was given expected verse numbers")
	}
}

func TestVersionLicenseNoticeIsTheRegistrys(t *testing.T) {
	if got := VersionLicenseNotice("nkjv"); !strings.Contains(got, "New King James Version") ||
		!strings.Contains(got, "Thomas Nelson") {
		t.Errorf("the NKJV's notice is %q", got)
	}
	for _, id := range []string{"web", "bsb", "webc", "no-such-version"} {
		if got := VersionLicenseNotice(id); got != "" {
			t.Errorf("%s carries a licence notice: %q", id, got)
		}
	}
}

func TestWebSmallCapitalRunesAreTheDrawnOnes(t *testing.T) {
	got := WebSmallCapitalRunes()
	if len(got) != len(smallCapitals) || len(got) != 25 {
		t.Fatalf("%d runes, want the table's 25", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Fatalf("not sorted at %d: %U then %U", i, got[i-1], got[i])
		}
	}
	for letter, sc := range smallCapitals {
		found := false
		for _, r := range got {
			found = found || r == sc
		}
		if !found {
			t.Errorf("the small capital for %q (%U) is missing", letter, sc)
		}
	}
}
