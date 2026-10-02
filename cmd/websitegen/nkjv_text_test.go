package main

// The switch (nkjv_text.go) in both of its states, through run() — the same
// path main() takes — with the network replaced by fixtures. The text is
// synthetic throughout (nkjv_fixture_test.go).

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	bibletext "github.com/cubancorona/bibletext"
)

const testSiteKey = "synthetic-site-key-0123456789"

// fetchRecord is what a stand-in fetch saw.
type fetchRecord struct {
	calls     int
	key       string
	envAtCall string
}

// standIn replaces both network paths for one test: the public-domain editions
// load as published, and the licensed fetch is fetch, recorded.
func standIn(t *testing.T, published []loadedVersion, fetch func(id string) (bibletext.LicensedEdition, error)) (*fetchRecord, *int) {
	t.Helper()
	rec := &fetchRecord{}
	loads := 0
	prevFetch, prevLoad := fetchLicensedEdition, loadPublished
	t.Cleanup(func() { fetchLicensedEdition, loadPublished = prevFetch, prevLoad })
	loadPublished = func(string, bool) ([]loadedVersion, error) {
		loads++
		return published, nil
	}
	fetchLicensedEdition = func(id, key string) (bibletext.LicensedEdition, error) {
		rec.calls++
		rec.key = key
		rec.envAtCall = os.Getenv(siteKeyEnv)
		return fetch(id)
	}
	return rec, &loads
}

func referenceOf(t *testing.T, versions []loadedVersion) *bibletext.BibleData {
	t.Helper()
	for _, v := range versions {
		if v.ID == versificationReferenceID {
			return v.bible
		}
	}
	t.Fatal("no reference edition in the fixture")
	return nil
}

// richLicensedEdition is the licensed fixture with the features a real chapter
// draws: supplied words and a small-capital span in John 3:16, a heading over
// John 3, and a title on Psalm 3.
func richLicensedEdition(t *testing.T, ref *bibletext.BibleData) bibletext.LicensedEdition {
	t.Helper()
	bd := syntheticLicensedText(t, ref, fixtureVerseText)
	for i, v := range bd.Verses["John"][3] {
		if v.Verse != 16 {
			continue
		}
		v.Text = "Supplied fixture words, and the Lord in small capitals."
		v.Supplied = []bibletext.TextSpan{{Start: 0, End: 8}}
		lord := utf8.RuneCountInString(v.Text[:strings.Index(v.Text, "Lord")])
		v.SmallCaps = []bibletext.TextSpan{{Start: lord, End: lord + 4}}
		bd.Verses["John"][3][i] = v
	}
	bd.Superscriptions = map[string]map[int]bibletext.Superscription{
		"Psalms": {3: {Text: "A licensed fixture title."}},
	}
	bd.Headings["John"][3] = []bibletext.Heading{{Text: "A Licensed Fixture Heading", Style: "s", BeforeVerse: 1}}
	return bibletext.LicensedEdition{Bible: bd, Requests: 67, CopyrightLines: 1}
}

func fixedNow() time.Time { return fixedRetrieval }

func readSiteFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read /%s: %v", rel, err)
	}
	return string(b)
}

func findAsset(t *testing.T, root, pattern string) string {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join(root, "assets", pattern))
	if len(matches) != 1 {
		t.Fatalf("assets/%s: %d matches", pattern, len(matches))
	}
	return matches[0]
}

// --- off ---------------------------------------------------------------------

// Off is the site as it was: no fetch, no key read (the variable is still
// there afterwards, untouched), no clock read, no new file — and the very tree
// the golden pins.
func TestNKJVTextOffNeverFetchesOrReadsTheKey(t *testing.T) {
	rec, _ := standIn(t, goldenFixtureVersions(), func(string) (bibletext.LicensedEdition, error) {
		return bibletext.LicensedEdition{}, errors.New("the fetch was reached with the switch off")
	})
	t.Setenv(siteKeyEnv, testSiteKey)
	out := filepath.Join(t.TempDir(), "site")
	err := run(runOptions{out: out, cache: t.TempDir(), nkjvText: false, now: func() time.Time {
		t.Error("the retrieval clock was read with the switch off")
		return time.Time{}
	}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if rec.calls != 0 {
		t.Errorf("API.Bible was asked %d times with the switch off", rec.calls)
	}
	if got := os.Getenv(siteKeyEnv); got != testSiteKey {
		t.Errorf("the key variable was touched with the switch off")
	}
	walk := &siteWriter{root: out}
	walkSite(t, walk, func(rel, body string) {
		for _, mark := range []string{`class="lic"`, `class="retrieved"`, "api.bible", "nkjv."} {
			if strings.Contains(body, mark) {
				t.Errorf("%s carries %s with the switch off", rel, mark)
			}
		}
	})
	for _, pattern := range []string{"nkjv.*.css", "Junicode-SmallCaps.*", "Junicode-BoldSmallCaps.*", "Junicode-ItalicSmallCaps.*"} {
		if m, _ := filepath.Glob(filepath.Join(out, "assets", pattern)); len(m) > 0 {
			t.Errorf("assets/%s was written with the switch off", pattern)
		}
	}
	if strings.Contains(readSiteFile(t, out, "404.html"), "/nkjv/") {
		t.Error("the 404 offers /nkjv/ with the switch off")
	}
	if strings.Contains(readSiteFile(t, out, strings.TrimPrefix(findAsset(t, out, "reader.*.js"), out+"/")), `"nkjv"`) {
		t.Error("reader.js has an nkjv column with the switch off")
	}
	// The same tree, file for file, as before the switch existed.
	got, want := siteDigests(t, out), readGolden(t)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("run() with the switch off does not build the pinned tree (%d files against %d); "+
			"see TestNKJVTextOffSiteIsByteIdenticalToTheBase for which", len(got), len(want))
	}
}

// --- on: refusals ------------------------------------------------------------

func TestNKJVTextOnRefusesOffline(t *testing.T) {
	rec, loads := standIn(t, goldenFixtureVersions(), func(string) (bibletext.LicensedEdition, error) {
		return bibletext.LicensedEdition{}, errors.New("unreachable")
	})
	t.Setenv(siteKeyEnv, testSiteKey)
	err := run(runOptions{out: filepath.Join(t.TempDir(), "site"), cache: t.TempDir(), offline: true, nkjvText: true, now: fixedNow})
	if err == nil || !strings.Contains(err.Error(), "-offline") {
		t.Fatalf("an offline build with the switch on gave %v", err)
	}
	if rec.calls != 0 || *loads != 0 {
		t.Errorf("an offline refusal still fetched (%d) or loaded feeds (%d)", rec.calls, *loads)
	}
}

func TestNKJVTextOnRefusesMissingKey(t *testing.T) {
	rec, loads := standIn(t, goldenFixtureVersions(), func(string) (bibletext.LicensedEdition, error) {
		return bibletext.LicensedEdition{}, errors.New("unreachable")
	})
	err := run(runOptions{out: filepath.Join(t.TempDir(), "site"), cache: t.TempDir(), nkjvText: true, now: fixedNow})
	if err == nil || !strings.Contains(err.Error(), siteKeyEnv) {
		t.Fatalf("a build with the switch on and no key gave %v", err)
	}
	if rec.calls != 0 || *loads != 0 {
		t.Errorf("a keyless refusal still fetched (%d) or loaded feeds (%d) — it must come first", rec.calls, *loads)
	}
}

func TestNKJVTextOnRefusesMalformedKey(t *testing.T) {
	for _, bad := range []string{"short-key", "synthetic key with spaces 0123", strings.Repeat("k", 513)} {
		rec, _ := standIn(t, goldenFixtureVersions(), func(string) (bibletext.LicensedEdition, error) {
			return bibletext.LicensedEdition{}, errors.New("unreachable")
		})
		t.Setenv(siteKeyEnv, bad)
		err := run(runOptions{out: filepath.Join(t.TempDir(), "site"), cache: t.TempDir(), nkjvText: true, now: fixedNow})
		if err == nil {
			t.Errorf("a %d-character malformed key was accepted", len(bad))
			continue
		}
		if strings.Contains(err.Error(), bad) {
			t.Errorf("the refusal carries the value: %v", err)
		}
		if rec.calls != 0 {
			t.Errorf("a malformed key was sent to API.Bible")
		}
	}
}

// A fetch that fails fails the build: no notice pages in its place, and the
// output directory untouched — writeSite, which clears it first, never runs.
func TestNKJVTextOnAbortsOnFetchError(t *testing.T) {
	standIn(t, goldenFixtureVersions(), func(string) (bibletext.LicensedEdition, error) {
		return bibletext.LicensedEdition{}, errors.New("synthetic outage")
	})
	t.Setenv(siteKeyEnv, testSiteKey)
	out := filepath.Join(t.TempDir(), "site")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(out, "previous-build.html")
	if err := os.WriteFile(sentinel, []byte("the last good build"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := run(runOptions{out: out, cache: t.TempDir(), nkjvText: true, now: fixedNow})
	if err == nil || !strings.Contains(err.Error(), "synthetic outage") {
		t.Fatalf("a failed fetch gave %v", err)
	}
	if _, statErr := os.Stat(sentinel); statErr != nil {
		t.Error("the output directory was cleared by a build that could not finish")
	}
	if _, statErr := os.Stat(filepath.Join(out, "nkjv")); statErr == nil {
		t.Error("a failed fetch still wrote /nkjv/ — a fallback to the notice pages")
	}
}

// A write refused part-way leaves no tree at all. The writer refuses the page
// that carries the key only after the public-domain editions are on disk, and
// half a site is not kept: with the text on, its NKJV pages would be licensed
// text that no finished build stands behind.
func TestNKJVTextOnRefusedWriteLeavesNoTree(t *testing.T) {
	published := goldenFixtureVersions()
	standIn(t, published, func(string) (bibletext.LicensedEdition, error) {
		ed := richLicensedEdition(t, referenceOf(t, published))
		ed.Bible.Verses["John"][3][4].Text = "A fixture verse that carries " + testSiteKey + "."
		return ed, nil
	})
	t.Setenv(siteKeyEnv, testSiteKey)
	out := filepath.Join(t.TempDir(), "site")
	err := run(runOptions{out: out, cache: t.TempDir(), nkjvText: true, now: fixedNow})
	if err == nil || !strings.Contains(err.Error(), "contains the API.Bible key") || strings.Contains(err.Error(), testSiteKey) {
		t.Fatalf("a page carrying the key gave %v", err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Errorf("a refused write left %s behind (%v)", out, statErr)
	}
}

// The key is read, taken out of the environment before the fetch runs, handed
// to the fetch, and written into no file.
func TestNKJVTextOnTakesTheKeyOutOfTheEnvironment(t *testing.T) {
	published := goldenFixtureVersions()
	rec, _ := standIn(t, published, func(string) (bibletext.LicensedEdition, error) {
		return richLicensedEdition(t, referenceOf(t, published)), nil
	})
	t.Setenv(siteKeyEnv, testSiteKey)
	out := filepath.Join(t.TempDir(), "site")
	if err := run(runOptions{out: out, cache: t.TempDir(), nkjvText: true, now: fixedNow}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if rec.calls != 1 || rec.key != testSiteKey {
		t.Errorf("the fetch ran %d times with the right key: %v", rec.calls, rec.key == testSiteKey)
	}
	if rec.envAtCall != "" || os.Getenv(siteKeyEnv) != "" {
		t.Error("the key was still in the environment when the fetch ran, or after")
	}
	err := filepath.WalkDir(out, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(b), testSiteKey) {
			t.Errorf("%s contains the key", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// --- on: the checks on the fetched text ---------------------------------------

func TestCheckLicensedComplete(t *testing.T) {
	ref := referenceOf(t, goldenFixtureVersions())
	whole := func() *bibletext.BibleData { return syntheticLicensedText(t, ref, fixtureVerseText) }
	if err := checkLicensedComplete("nkjv", whole(), ref); err != nil {
		t.Fatalf("a whole edition was refused: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(bd *bibletext.BibleData)
		want   []string
	}{
		{"a verse dropped", func(bd *bibletext.BibleData) {
			bd.Verses["John"][3] = append(bd.Verses["John"][3][:15:15], bd.Verses["John"][3][16:]...)
		}, []string{"John 3 lacks verse 16"}},
		{"a chapter dropped", func(bd *bibletext.BibleData) { delete(bd.Verses["John"], 1) },
			[]string{"John 1 is missing"}},
		{"a stray chapter", func(bd *bibletext.BibleData) {
			bd.Verses["John"][99] = []bibletext.Verse{{BookName: "John", Chapter: 99, Verse: 1, Text: "stray"}}
		}, []string{"John 99 is not in the reference canon"}},
		{"a book dropped", func(bd *bibletext.BibleData) {
			bd.Books = bd.Books[:len(bd.Books)-1]
			delete(bd.Verses, "Esther")
		}, []string{"Esther is missing"}},
		{"a stray book", func(bd *bibletext.BibleData) {
			bd.Books = append(bd.Books, "Tobit")
			bd.Verses["Tobit"] = map[int][]bibletext.Verse{1: {{BookName: "Tobit", Chapter: 1, Verse: 1, Text: "stray"}}}
		}, []string{"Tobit is not in the reference canon"}},
		{"the doxology left where the reference numbers it", func(bd *bibletext.BibleData) {
			bd.Verses["Romans"] = map[int][]bibletext.Verse{14: ref.Verses["Romans"][14], 16: ref.Verses["Romans"][16]}
		}, []string{"Romans 14 has unexpected verse 24, 25, 26", "Romans 16 lacks verse 25, 26, 27"}},
	} {
		bd := whole()
		tc.mutate(bd)
		err := checkLicensedComplete("nkjv", bd, ref)
		if err == nil {
			t.Errorf("%s: accepted", tc.name)
			continue
		}
		for _, w := range tc.want {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("%s: %q does not say %q", tc.name, err, w)
			}
		}
		if strings.Contains(err.Error(), "fixture verse") {
			t.Errorf("%s: the refusal quotes text: %v", tc.name, err)
		}
	}
	// A wholly wrong fetch is summarised, not listed.
	bd := whole()
	for _, book := range []string{"Psalms", "John", "Acts", "Romans", "Daniel", "Esther"} {
		for ch := range bd.Verses[book] {
			bd.Verses[book][ch] = bd.Verses[book][ch][:1]
		}
	}
	if err := checkLicensedComplete("nkjv", bd, ref); err == nil || !regexp.MustCompile(`and \d+ more$`).MatchString(err.Error()) {
		t.Errorf("a fetch wrong everywhere gave %v", err)
	}
}

func TestLoadLicensedRefusesWhatCannotBePublished(t *testing.T) {
	ref := referenceOf(t, goldenFixtureVersions())
	good := func() bibletext.LicensedEdition { return richLicensedEdition(t, ref) }
	for _, tc := range []struct {
		name string
		ed   func() bibletext.LicensedEdition
		want string
	}{
		{"two copyright lines", func() bibletext.LicensedEdition { e := good(); e.CopyrightLines = 2; return e }, "2 different copyright lines"},
		{"no copyright line", func() bibletext.LicensedEdition { e := good(); e.CopyrightLines = 0; return e }, "0 different copyright lines"},
		{"no headings", func() bibletext.LicensedEdition { e := good(); e.Bible.Headings = nil; return e }, "no publisher headings"},
		{"no text", func() bibletext.LicensedEdition { e := good(); e.Bible = nil; return e }, "no text"},
		{"a short edition", func() bibletext.LicensedEdition { e := good(); delete(e.Bible.Verses["John"], 3); return e }, "John 3 is missing"},
	} {
		prev := fetchLicensedEdition
		fetchLicensedEdition = func(string, string) (bibletext.LicensedEdition, error) { return tc.ed(), nil }
		_, err := loadLicensed(true, false, testSiteKey, ref, fixedNow)
		fetchLicensedEdition = prev
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want an error saying %q", tc.name, err, tc.want)
		}
	}
	// Off reaches none of it, whatever it is handed.
	prev := fetchLicensedEdition
	defer func() { fetchLicensedEdition = prev }()
	fetchLicensedEdition = func(string, string) (bibletext.LicensedEdition, error) {
		t.Error("the fetch ran with the switch off")
		return good(), nil
	}
	if got, err := loadLicensed(false, true, "", nil, nil); got != nil || err != nil {
		t.Errorf("off returned %v, %v", got, err)
	}
}

// A registry notice the site cannot print stops the build before the fetch,
// so a change to its wording costs none of the fetch's requests.
func TestLoadLicensedReadsTheNoticeBeforeTheFetch(t *testing.T) {
	prevFetch, prevNotice := fetchLicensedEdition, licenceNotice
	t.Cleanup(func() { fetchLicensedEdition, licenceNotice = prevFetch, prevNotice })
	fetches := 0
	fetchLicensedEdition = func(string, string) (bibletext.LicensedEdition, error) {
		fetches++
		return bibletext.LicensedEdition{}, errors.New("the fixture fetch failed")
	}
	for _, tc := range []struct{ notice, want string }{
		{"", "no licence notice"},
		{"A fixture notice. All rights reserved.", "does not end with"},
	} {
		licenceNotice = func(string) string { return tc.notice }
		if _, err := loadLicensed(true, false, testSiteKey, nil, fixedNow); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("notice %q: %v, want an error saying %q", tc.notice, err, tc.want)
		}
	}
	if fetches != 0 {
		t.Errorf("the fetch ran %d times for a notice the site cannot print", fetches)
	}
	// The registry's own notice goes on to the fetch.
	licenceNotice = prevNotice
	if _, err := loadLicensed(true, false, testSiteKey, nil, fixedNow); err == nil ||
		!strings.Contains(err.Error(), "the fixture fetch failed") || fetches != 1 {
		t.Errorf("with the registry's notice: %v after %d fetches, want the fetch's own error after one", err, fetches)
	}
}

// --- on: the pages ---------------------------------------------------------------

// The text on, end to end through run(): what every kind of page carries.
func TestNKJVTextOnPages(t *testing.T) {
	published := goldenFixtureVersions()
	standIn(t, published, func(string) (bibletext.LicensedEdition, error) {
		return richLicensedEdition(t, referenceOf(t, published)), nil
	})
	t.Setenv(siteKeyEnv, testSiteKey)
	out := filepath.Join(t.TempDir(), "site")
	if err := run(runOptions{out: out, cache: t.TempDir(), nkjvText: true, now: fixedNow}); err != nil {
		t.Fatalf("run: %v", err)
	}
	cssRel := strings.TrimPrefix(filepath.ToSlash(findAsset(t, out, "nkjv.*.css")), filepath.ToSlash(out)+"/")
	const lic = `<p class="lic">Scripture taken from the New King James Version`
	const retrieved = `<p class="retrieved">Text provided by API.Bible (<a href="https://api.bible">api.bible</a>), ` +
		`retrieved <time datetime="2026-10-02">2 October 2026</time>.</p>`

	john := readSiteFile(t, out, "nkjv/john/3/index.html")
	for _, want := range []string{
		`class="v" id="v16"`, `<i>Supplied</i>`, "Lᴏʀᴅ", `<h2 class="sec">`, lic, retrieved,
		`href="../../../` + cssRel + `"`, `<a class="vpick on" title="New King James Version"`,
	} {
		if !strings.Contains(john, want) {
			t.Errorf("/nkjv/john/3/ lacks %s", want)
		}
	}
	for _, unwanted := range []string{`id="openapp"`, noticeCSSName, noticeJSName, "apple-itunes-app"} {
		if strings.Contains(john, unwanted) {
			t.Errorf("/nkjv/john/3/ carries %s, which belongs to a notice page", unwanted)
		}
	}
	// The notice sits at the very foot, after the app link and the platforms,
	// and the page ends with the rights holder's notice and the one line: the
	// notice stops before the registry's own API.Bible credit, which the line
	// replaces (siteNotice).
	if strings.Index(john, `id="getapp"`) > strings.Index(john, lic) || !strings.Contains(john, retrieved+`</footer>`) {
		t.Error("the notice is not at the foot of the footer")
	}
	if want := `<p class="lic">` + strings.TrimSuffix(bibletext.VersionLicenseNotice("nkjv"), registryCredit) + `</p>` +
		retrieved + `</footer>`; !strings.Contains(john, want) {
		t.Errorf("the page does not end with the notice and the one line:\n want %s", want)
	}
	if !strings.Contains(readSiteFile(t, out, "nkjv/psalms/3/index.html"), `<p class="pst">A licensed fixture title.</p>`) {
		t.Error("/nkjv/psalms/3/ lacks its title")
	}
	for _, rel := range []string{"nkjv/index.html", "nkjv/john/index.html"} {
		if page := readSiteFile(t, out, rel); !strings.Contains(page, lic) || !strings.Contains(page, retrieved) {
			t.Errorf("/%s lacks the notice or the retrieval line", rel)
		}
	}
	// A chapter the NKJV does not have: the canon-gap page, under the NKJV's
	// footer and with its stylesheet as well as the notice's.
	gap := readSiteFile(t, out, "nkjv/tobit/1/index.html")
	for _, want := range []string{lic, retrieved, noticeCSSName, cssRel, `href="../../../webc/tobit/1/"`} {
		if !strings.Contains(gap, want) {
			t.Errorf("/nkjv/tobit/1/ lacks %s", want)
		}
	}
	// Every /nkjv/ page, and no other; and no page credits the provider twice.
	walkSite(t, &siteWriter{root: out}, func(rel, body string) {
		if !strings.HasSuffix(rel, ".html") {
			return
		}
		under := strings.HasPrefix(rel, "nkjv/")
		if has := strings.Contains(body, retrieved); has != under {
			t.Errorf("%s: retrieval line present %v, under /nkjv/ %v", rel, has, under)
		}
		if strings.Contains(body, "provided via API.Bible") {
			t.Errorf("%s carries the registry's API.Bible credit as well as the retrieval line", rel)
		}
	})
	web := readSiteFile(t, out, "web/john/3/index.html")
	if !strings.Contains(web, `href="../../../nkjv/john/3/">NKJV</a>`) {
		t.Error("/web/john/3/ offers no NKJV pill")
	}
	if strings.Contains(web, cssRel) || strings.Contains(web, `class="lic"`) {
		t.Error("/web/john/3/ carries the NKJV's stylesheet or notice")
	}
	js := readSiteFile(t, out, strings.TrimPrefix(filepath.ToSlash(findAsset(t, out, "reader.*.js")), filepath.ToSlash(out)+"/"))
	if !strings.Contains(js, `"nkjv":`) {
		t.Error("reader.js has no nkjv column, so the Go to picker on /nkjv/ opens empty")
	}
	if !strings.Contains(readSiteFile(t, out, "404.html"), `<li><a href="/nkjv/">New King James Version</a></li>`) {
		t.Error("the 404 does not offer the NKJV")
	}
	// Every face the stylesheet names was written beside it.
	css := readSiteFile(t, out, cssRel)
	for _, m := range regexp.MustCompile(`url\(([^)]+)\)`).FindAllStringSubmatch(css, -1) {
		if _, err := os.Stat(filepath.Join(out, "assets", m[1])); err != nil {
			t.Errorf("nkjv.css names %s, which was not written", m[1])
		}
	}
}

func TestRetrievedLineIsTheLondonCivilDate(t *testing.T) {
	for _, tc := range []struct {
		at         time.Time
		iso, words string
	}{
		{time.Date(2026, 10, 1, 23, 30, 0, 0, time.UTC), "2026-10-02", "2 October 2026"},    // BST: already the 2nd
		{time.Date(2026, 12, 31, 23, 30, 0, 0, time.UTC), "2026-12-31", "31 December 2026"}, // GMT: still the 31st
		{time.Date(2026, 3, 29, 0, 30, 0, 0, time.UTC), "2026-03-29", "29 March 2026"},      // the spring change
	} {
		foot := licenceFoot(&webLicence{Notice: "N", Retrieved: londonDate(tc.at)})
		want := `<p class="retrieved">Text provided by API.Bible (<a href="https://api.bible">api.bible</a>), retrieved ` +
			`<time datetime="` + tc.iso + `">` + tc.words + `</time>.</p>`
		if !strings.Contains(foot, want) {
			t.Errorf("%s renders as %s, want %s", tc.at, foot, want)
		}
	}
	if licenceFoot(nil) != "" {
		t.Error("a page with no licence gained a footer line")
	}
	// The notice is escaped, not trusted.
	if foot := licenceFoot(&webLicence{Notice: "a <b> & c", Retrieved: londonDate(fixedRetrieval)}); !strings.Contains(foot,
		`<p class="lic">a &lt;b&gt; &amp; c</p>`) {
		t.Errorf("the notice is not escaped: %s", foot)
	}
}

// THE SITE'S FOOTER CREDITS API.BIBLE ONCE. The registry's notice ends with the
// app's API.Bible credit, and the app keeps printing it; the site prints the
// notice without that sentence, because its retrieval line names the provider
// (and the date) instead. A notice that does not end with the credit is refused
// rather than printed whole.
func TestTheSiteNoticeLeavesTheCreditToTheRetrievalLine(t *testing.T) {
	registry := bibletext.VersionLicenseNotice("nkjv")
	if !strings.HasSuffix(registry, "All rights reserved. Text provided via API.Bible (api.bible).") {
		t.Fatalf("the registry's notice, which the app prints, no longer ends with its API.Bible credit: %q", registry)
	}
	got, err := siteNotice("nkjv", registry)
	if err != nil {
		t.Fatalf("siteNotice: %v", err)
	}
	if want := "Scripture taken from the New King James Version®. Copyright © 1982 by Thomas Nelson. " +
		"Used by permission. All rights reserved."; got != want {
		t.Errorf("the site prints the notice as %q, want %q", got, want)
	}
	for _, notice := range []string{
		"A fixture notice. All rights reserved.",                     // no credit to drop
		"A fixture notice. Text provided via API.Bible (api.bible)",  // the credit without its full stop
		" Text provided via API.Bible (api.bible).",                  // the credit alone
		"Text provided via API.Bible (api.bible). A fixture notice.", // the credit, not last
	} {
		if got, err := siteNotice("nkjv", notice); err == nil {
			t.Errorf("%q was printed as %q, not refused", notice, got)
		}
	}
	if line := fmt.Sprintf(retrievedLineFormat, "D"); !strings.HasPrefix(line, "Text provided by API.Bible (") {
		t.Errorf("the retrieval line no longer names the provider: %s", line)
	}
}

// declaredFace is one @font-face as the browser reads it for matching.
type declaredFace struct {
	family, style, weight, src string
	ranges                     map[rune]bool // nil: the whole face
}

// declaredFaces reads every @font-face out of a stylesheet, in order.
func declaredFaces(t *testing.T, css string) []declaredFace {
	t.Helper()
	field := func(body, name string) string {
		m := regexp.MustCompile(name + `:\s*([^;]+);`).FindStringSubmatch(body)
		if m == nil {
			return ""
		}
		return strings.TrimSpace(m[1])
	}
	var faces []declaredFace
	for _, m := range regexp.MustCompile(`@font-face\{([^}]*)\}`).FindAllStringSubmatch(css, -1) {
		f := declaredFace{family: field(m[1], "font-family"), style: field(m[1], "font-style"),
			weight: field(m[1], "font-weight")}
		if src := regexp.MustCompile(`url\(([^)]+)\)`).FindStringSubmatch(m[1]); src != nil {
			f.src = src[1]
		}
		if r := field(m[1], "unicode-range"); r != "" {
			f.ranges = map[rune]bool{}
			for _, p := range strings.Split(r, ",") {
				n, err := strconv.ParseInt(strings.TrimPrefix(strings.TrimSpace(p), "U+"), 16, 32)
				if err != nil {
					t.Fatalf("unparseable range entry %q", p)
				}
				f.ranges[rune(n)] = true
			}
		}
		faces = append(faces, f)
	}
	return faces
}

// wantFace is a face a stylesheet must declare, at its place in the order.
type wantFace struct {
	src, style, weight string
	ranges             []rune // nil: the whole face
}

// checkFaces holds a stylesheet's faces to want, in order. A supplement joins
// the composite of a run only when its weight and style are the run's exactly,
// so each is pinned; its unicode-range must be exactly the app's table (one
// code point missing is a letter set in a system face, one extra a download
// for a page that does not need it); and a supplement must come AFTER the face
// it supplements, or the browser asks the base face first.
func checkFaces(t *testing.T, name, css string, want []wantFace) {
	t.Helper()
	faces := declaredFaces(t, css)
	if len(faces) != len(want) {
		t.Fatalf("%s declares %d faces, want %d:\n%s", name, len(faces), len(want), css)
	}
	for i, w := range want {
		f := faces[i]
		if f.family != `"Junicode"` && !strings.HasPrefix(w.src, "ui") {
			t.Errorf("%s: %s joins family %s, not the scripture stack's Junicode", name, f.src, f.family)
		}
		if f.src != w.src || f.style != w.style || f.weight != w.weight {
			t.Errorf("%s: face %d is %s %s %s, want %s %s %s", name, i, f.src, f.style, f.weight, w.src, w.style, w.weight)
		}
		if w.ranges == nil && f.ranges != nil {
			t.Errorf("%s: %s is limited to a unicode-range; it is a whole cut", name, f.src)
		}
		if w.ranges != nil {
			ok := len(f.ranges) == len(w.ranges)
			for _, r := range w.ranges {
				ok = ok && f.ranges[r]
			}
			if !ok {
				t.Errorf("%s: %s's unicode-range has %d code points and is not the app's %d", name, f.src, len(f.ranges), len(w.ranges))
			}
		}
	}
}

// The NKJV pages' stylesheet: the small capitals in each cut the divine name is
// set in, each after the base face reader.css declares for that cut.
func TestNKJVCSSDeclaresEveryFaceTheNKJVPagesDraw(t *testing.T) {
	css := nkjvCSS(nkjvFonts{smallCaps: "sc.woff2", boldSmallCaps: "bsc.woff2", italicSmallCaps: "isc.woff2"})
	sc := bibletext.WebSmallCapitalRunes()
	checkFaces(t, "nkjv.css", css, []wantFace{
		{"sc.woff2", "normal", "400", sc},
		{"bsc.woff2", "normal", "700", sc},
		{"isc.woff2", "italic", "400", sc},
	})
	if !strings.Contains(css, "@media print{.foot{display:block}") {
		t.Error("the notice would not print")
	}
}

// The reader's stylesheet, which every page loads: the regular and bold cuts,
// the true italic for the psalm titles, the notes' Greek, and the Hebrew at
// both weights it is set in — the notes' at the regular, the NKJV's stanza
// letters in a bold heading — so neither is drawn from a system face nor
// thickened.
func TestReaderCSSDeclaresTheScriptureFacesAndTheirSupplements(t *testing.T) {
	css := readerCSS(webFonts{uiRegular: "ui-r.woff2", uiBold: "ui-b.woff2", scriptureRegular: "r.woff2",
		scriptureBold: "b.woff2", scriptureItalic: "i.woff2", scriptureGreek: "gr.woff2", hebrew: "heb.woff2"})
	heb := bibletext.WebHebrewRunes()
	checkFaces(t, "reader.css", css, []wantFace{
		{"ui-r.woff2", "normal", "400", nil},
		{"ui-b.woff2", "normal", "700", nil},
		{"r.woff2", "normal", "400", nil},
		{"b.woff2", "normal", "700", nil},
		{"i.woff2", "italic", "400", nil},
		{"gr.woff2", "normal", "400", bibletext.WebGreekRunes()},
		{"heb.woff2", "normal", "400", heb},
		{"heb.woff2", "normal", "700", heb},
	})
}

func TestSiteWriterRefusesTheKey(t *testing.T) {
	site := &siteWriter{root: t.TempDir(), secret: testSiteKey}
	err := site.write("nkjv/leak.html", "<p>"+testSiteKey+"</p>")
	if err == nil || strings.Contains(err.Error(), testSiteKey) {
		t.Fatalf("writing the key gave %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(site.root, "nkjv", "leak.html")); statErr == nil {
		t.Error("the file was written anyway")
	}
	if err := site.write("nkjv/fine.html", "<p>no key here</p>"); err != nil {
		t.Errorf("a clean file was refused: %v", err)
	}
}

func TestNKJVTextStateNames(t *testing.T) {
	// scripts/publish-site.sh matches these two words exactly.
	if nkjvTextState(true) != "on" || nkjvTextState(false) != "off" {
		t.Errorf("states print as %q / %q", nkjvTextState(true), nkjvTextState(false))
	}
}

// ONE READER OF THE SWITCH, AND NO OVERRIDE. The constant is used exactly once
// outside its declaration — in main(), which hands it down — and the only
// environment variable the program reads is the key. A second read, or an
// os.Getenv that could flip the state at publish time, fails here.
func TestTheNKJVSwitchIsReadOnce(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	uses, inMain, flags := 0, 0, []string{}
	visit := func(fn string) func(n ast.Node) bool {
		return func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.ValueSpec:
				for _, name := range x.Names {
					if name.Name == "nkjvSiteText" {
						return false // the declaration itself
					}
				}
			case *ast.Ident:
				if x.Name == "nkjvSiteText" {
					uses++
					if fn == "main" {
						inMain++
					}
				}
			case *ast.CallExpr:
				sel, ok := x.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkgIdent, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}
				switch {
				case pkgIdent.Name == "os" && (sel.Sel.Name == "Getenv" || sel.Sel.Name == "LookupEnv" || sel.Sel.Name == "Environ"):
					readsKey := false
					if len(x.Args) == 1 {
						id, ok := x.Args[0].(*ast.Ident)
						readsKey = ok && id.Name == "siteKeyEnv"
					}
					if !readsKey {
						t.Errorf("%s: os.%s reads something other than the key", fset.Position(x.Pos()), sel.Sel.Name)
					}
				case pkgIdent.Name == "flag" && len(x.Args) > 0:
					if lit, ok := x.Args[0].(*ast.BasicLit); ok {
						flags = append(flags, lit.Value)
					}
				}
			}
			return true
		}
	}
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			for _, decl := range f.Decls {
				fn := ""
				if fd, ok := decl.(*ast.FuncDecl); ok {
					fn = fd.Name.Name
				}
				ast.Inspect(decl, visit(fn))
			}
		}
	}
	if uses != 1 || inMain != 1 {
		t.Errorf("nkjvSiteText is read %d times (%d in main); want once, in main", uses, inMain)
	}
	if got := strings.Join(flags, " "); got != `"out" "cache" "offline" "print-nkjv-text"` {
		t.Errorf("the flags are %s; a new one must not be able to override the switch", got)
	}
}

// Neither state can write the other's tree over its own: a translation loaded
// as text and listed as noticed is refused before anything is written.
func TestWriteSiteRefusesTwoTreesForOneRoot(t *testing.T) {
	published := goldenFixtureVersions()
	loaded := append(published, licensedFixtureVersion(syntheticLicensedText(t, referenceOf(t, published), fixtureVerseText)))
	site := &siteWriter{root: filepath.Join(t.TempDir(), "site")}
	if err := writeSite(site, loaded, noticedVersionsFor(false)); err == nil {
		t.Fatal("the NKJV was written as text and as notices in one build")
	}
	if site.files != 0 {
		t.Errorf("%d files were written before the refusal", site.files)
	}
}
