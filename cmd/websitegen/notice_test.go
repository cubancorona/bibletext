package main

// The notice pages: what they promise a reader who followed a link to words
// this site does not have.
//
// The promise is docs/NKJV_FLOW.md's I1 — every state offers at least one action
// reaching a state where the reader is reading something — plus the two things
// the page must never get wrong: it must not show licensed text (that half lives
// in licensed_exclusion_test.go), and it must not hand anyone a confident link
// to a verse that means something else in the translation it points at.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	bibletext "bibletext"
)

// noticeFixture builds the real site from a canon with several books, chapter
// counts that differ between translations, and the two chapters whose
// versification actually diverges — so the coverage walks below have something
// to be wrong about.
func noticeFixture(t *testing.T) (*siteWriter, []loadedVersion) {
	t.Helper()

	verses := func(book string, ch, n int) []bibletext.Verse {
		out := make([]bibletext.Verse, 0, n)
		for i := 1; i <= n; i++ {
			out = append(out, v(book, ch, i, "fixture verse text"))
		}
		return out
	}
	chapters := func(book string, counts map[int]int) map[int][]bibletext.Verse {
		m := map[int][]bibletext.Verse{}
		for ch, n := range counts {
			m[ch] = verses(book, ch, n)
		}
		return m
	}
	protestant := func() *bibletext.BibleData {
		return &bibletext.BibleData{
			Books: []string{"John", "Acts", "Romans", "Daniel", "Esther"},
			Verses: map[string]map[int][]bibletext.Verse{
				"John":   chapters("John", map[int]int{1: 51, 2: 25, 3: 36}),
				"Acts":   chapters("Acts", map[int]int{8: 40}),
				"Romans": chapters("Romans", map[int]int{14: 26, 15: 33, 16: 24}),
				"Daniel": chapters("Daniel", map[int]int{11: 45, 12: 13}),
				"Esther": chapters("Esther", map[int]int{1: 22}),
			},
		}
	}
	catholic := &bibletext.BibleData{
		Books: []string{"John", "Acts", "Romans", "Daniel", "Esther", "Tobit", "Judith"},
		Verses: map[string]map[int][]bibletext.Verse{
			"John":   chapters("John", map[int]int{1: 51, 2: 25, 3: 36}),
			"Acts":   chapters("Acts", map[int]int{8: 40}),
			"Romans": chapters("Romans", map[int]int{14: 23, 15: 33, 16: 27}),
			"Daniel": chapters("Daniel", map[int]int{11: 45, 12: 13, 13: 64, 14: 42}),
			"Esther": chapters("Esther", map[int]int{1: 22}),
			"Tobit":  chapters("Tobit", map[int]int{1: 22, 2: 14}),
			"Judith": chapters("Judith", map[int]int{1: 16}),
		},
	}

	var loaded []loadedVersion
	for _, pv := range publishedVersions() {
		bible := protestant()
		if pv.ID == "webc" {
			bible = catholic
		}
		loaded = append(loaded, loadedVersion{webVersion: pv, bible: bible})
	}
	site := &siteWriter{root: filepath.Join(t.TempDir(), "site")}
	if err := writeSite(site, loaded); err != nil {
		t.Fatalf("writeSite: %v", err)
	}
	return site, loaded
}

func canonSource(t *testing.T, all []loadedVersion) loadedVersion {
	t.Helper()
	for _, v := range all {
		if v.ID == noticeCanonSourceID {
			return v
		}
	}
	t.Fatalf("no %q among the loaded versions", noticeCanonSourceID)
	return loadedVersion{}
}

// emittedChapters lists every <version>/<slug>/<n>/index.html under one root,
// as "slug/n".
func emittedChapters(t *testing.T, root, versionID string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	dir := filepath.Join(root, versionID)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "index.html" {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) != 3 {
			return nil
		}
		if _, convErr := strconv.Atoi(parts[1]); convErr != nil {
			return nil
		}
		out[parts[0]+"/"+parts[1]] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return out
}

// TestEveryLinkableNKJVChapterHasAPage — the coverage promise, checked over the
// WHOLE canon rather than a sample. Every chapter the app can build a /nkjv/
// share link for must resolve, and nothing else may be written there: an extra
// page is a URL the app can never produce and a reader can only reach by
// mistyping.
//
// The fixture's canon stands in for the real one here. The real count — 1,189,
// the same canon the WEB has — is held by an EQUALITY guard in
// scripts/publish-site.sh, which refuses to publish a tree with any other
// number, so the two halves together cover it: this proves the rule, the guard
// proves the arithmetic on the real data.
func TestEveryLinkableNKJVChapterHasAPage(t *testing.T) {
	site, all := noticeFixture(t)
	canon := canonSource(t, all)

	want := map[string]bool{}
	for _, book := range canon.bible.Books {
		slug, ok := bibletext.BookSlug(book)
		if !ok {
			t.Fatalf("%s has no slug", book)
		}
		for _, ch := range sortedChapters(canon.bible, book) {
			want[slug+"/"+strconv.Itoa(ch)] = true
		}
	}
	if len(want) == 0 {
		t.Fatal("the fixture canon is empty; this test proved nothing")
	}

	got := emittedChapters(t, site.root, "nkjv")
	for ref := range want {
		if !got[ref] {
			t.Errorf("/nkjv/%s/ was not written — the app can share that link and it would 404", ref)
		}
	}
	for ref := range got {
		if !want[ref] {
			t.Errorf("/nkjv/%s/ was written but is not in the canon — no share link can name it", ref)
		}
	}
	// The indexes above the chapters, so the crumbs land somewhere.
	for _, rel := range []string{"nkjv/index.html", "nkjv/john/index.html"} {
		if _, err := os.Stat(filepath.Join(site.root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s missing: %v", rel, err)
		}
	}
}

// TestEveryCanonGapHasAPage is the other placement: for every (version, book,
// chapter) that ANY published translation carries and this one does not, a page
// must exist. This is the walk that would have caught /web/daniel/13/, the gap
// nobody had noticed because it hides inside a book the translation does have.
func TestEveryCanonGapHasAPage(t *testing.T) {
	site, all := noticeFixture(t)
	union := unionCanon(all)

	gaps := 0
	for _, v := range all {
		emitted := emittedChapters(t, site.root, v.ID)
		for _, book := range union.books {
			slug, ok := bibletext.BookSlug(book)
			if !ok {
				t.Fatalf("%s has no slug", book)
			}
			if len(v.bible.Verses[book]) == 0 {
				if _, err := os.Stat(filepath.Join(site.root, v.ID, slug, "index.html")); err != nil {
					t.Errorf("/%s/%s/ missing: the book index for a book this canon lacks", v.ID, slug)
				}
			}
			for _, ch := range union.chapters[book] {
				if hasChapter(v, book, ch) {
					continue
				}
				gaps++
				if !emitted[slug+"/"+strconv.Itoa(ch)] {
					t.Errorf("/%s/%s/%d/ missing — another translation on this site serves that path",
						v.ID, slug, ch)
				}
			}
		}
	}
	if gaps == 0 {
		t.Fatal("the fixture has no canon gaps; this test proved nothing")
	}
}

// TestNoticePagesAlwaysOfferARouteOut is I1, asserted over every notice page in
// the tree. A page that says "not here" and stops is the dead end this whole
// change exists to remove, so the assertion is on the LINKS: at least one must
// point into a translation this site actually serves.
func TestNoticePagesAlwaysOfferARouteOut(t *testing.T) {
	site, all := noticeFixture(t)

	published := map[string]bool{}
	for _, v := range all {
		published[v.ID] = true
	}
	href := regexp.MustCompile(`href="([^"]+)"`)
	checked := 0
	walkSite(t, site, func(rel, body string) {
		if !strings.HasSuffix(rel, "index.html") || !strings.Contains(body, `class="lede"`) {
			return
		}
		checked++
		out := 0
		for _, m := range href.FindAllStringSubmatch(body, -1) {
			// Resolve the relative link against the page's own directory and
			// ask whether it lands under a published translation.
			target := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(rel), m[1])))
			if published[strings.SplitN(target, "/", 2)[0]] {
				out++
			}
		}
		if out == 0 {
			t.Errorf("%s offers no link into a published translation — it is a dead end (I1)", rel)
		}
	})
	if checked == 0 {
		t.Fatal("no notice page was found; this test proved nothing")
	}
}

// TestNoticeChapterPageSaysWhatItMustSay pins the four things the owner asked
// for on one page: the passage is named, the translation is named, there is an
// explicit open-in-app affordance worded together with the explanation, and the
// parallel passage is offered.
func TestNoticeChapterPageSaysWhatItMustSay(t *testing.T) {
	site, _ := noticeFixture(t)
	body, err := os.ReadFile(filepath.Join(site.root, "nkjv", "john", "3", "index.html"))
	if err != nil {
		t.Fatalf("read /nkjv/john/3/: %v", err)
	}
	page := string(body)

	for _, want := range []string{
		`<span class="passage">John 3</span>`,         // the passage, upgradeable to John 3:16
		"New King James Version",                      // the translation, by name
		"Not available here &mdash; but you can open", // the explanation and the offer, together
		`id="openapp"`,                                // the explicit affordance
		"https://bibletext.co.uk/",                    // the download, correct with JS off
		`data-pkg="uk.co.bibletext"`,                  // the Android handoff
		`data-ios="https://apps.apple.com/app/id6784567351"`,
		`<meta name="apple-itunes-app"`, // the iOS route that needs no app change
		`href="../../../web/john/3/"`,   // the parallel passage
		`data-frag="verse"`,             // …carrying the shared verse
		`<meta property="og:title" content="John 3 (New King James Version)">`,
		`<meta name="robots" content="noindex,follow">`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("/nkjv/john/3/ is missing %s", want)
		}
	}
	// The unfurl must name the passage and carry no scripture. B_UNFURL_NKJV was
	// a bare URL in the message thread; a preview quoting the NKJV would be
	// worse than that.
	desc := regexp.MustCompile(`og:description" content="([^"]*)"`).FindStringSubmatch(page)
	if desc == nil {
		t.Fatal("no og:description")
	}
	if !strings.Contains(desc[1], "John 3") {
		t.Errorf("the unfurl does not name the passage: %q", desc[1])
	}
	if strings.Contains(desc[1], "fixture verse text") {
		t.Errorf("the unfurl quotes scripture: %q", desc[1])
	}
}

// TestParallelLinksCarryTheVerseOnlyWhereNumberingAgrees is the one that
// protects a reader from being shown different words from the ones they were
// sent. John 3 maps straight across; Romans 16 does not (the NKJV numbers the
// doxology there and the WEB numbers it in chapter 14), and Acts 8 does not (the
// NKJV has 8:37 and none of the published three do).
func TestParallelLinksCarryTheVerseOnlyWhereNumberingAgrees(t *testing.T) {
	site, _ := noticeFixture(t)

	read := func(path string) string {
		b, err := os.ReadFile(filepath.Join(site.root, filepath.FromSlash(path), "index.html"))
		if err != nil {
			t.Fatalf("read /%s/: %v", path, err)
		}
		return string(b)
	}
	link := func(page, href string) string {
		m := regexp.MustCompile(`<a([^>]*)href="` + regexp.QuoteMeta(href) + `"`).FindStringSubmatch(page)
		if m == nil {
			t.Fatalf("no link to %s on the page", href)
		}
		return m[1]
	}

	agree := read("nkjv/john/3")
	if !strings.Contains(link(agree, "../../../web/john/3/"), `data-frag="verse"`) {
		t.Error("John 3 maps verse-for-verse into the WEB; the link should carry the verse")
	}
	if strings.Contains(agree, `class="vnote"`) {
		t.Error("John 3 needs no versification caveat, but the page has one")
	}

	for _, tc := range []struct{ path, href, why string }{
		{"nkjv/romans/16", "../../../web/romans/16/", "the doxology is numbered 14:24-26 in the WEB"},
		{"nkjv/acts/8", "../../../web/acts/8/", "the NKJV has Acts 8:37 and the WEB does not"},
	} {
		page := read(tc.path)
		if strings.Contains(link(page, tc.href), `data-frag="verse"`) {
			t.Errorf("/%s/ offers to carry the verse, but %s", tc.path, tc.why)
		}
		if !strings.Contains(link(page, tc.href), `data-frag="note"`) {
			t.Errorf("/%s/ drops the sender's note as well as the verse", tc.path)
		}
		if !strings.Contains(page, `class="vnote"`) {
			t.Errorf("/%s/ silently opens the chapter instead of the verse and never says why", tc.path)
		}
	}
}

// The two kinds of difference must not be described with one sentence: "the
// numbering differs" and "that verse isn't in this translation" are things a
// reader would act on differently (I5, applied to the web).
func TestVersificationCaveatsSayWhichKindOfDifference(t *testing.T) {
	site, _ := noticeFixture(t)
	read := func(path string) string {
		b, err := os.ReadFile(filepath.Join(site.root, filepath.FromSlash(path), "index.html"))
		if err != nil {
			t.Fatalf("read /%s/: %v", path, err)
		}
		return string(b)
	}
	if !strings.Contains(read("nkjv/acts/8"), "carry every verse") {
		t.Error("Acts 8 differs because a verse is MISSING there; the page calls it a renumbering")
	}
	if !strings.Contains(read("nkjv/romans/16"), "number this chapter differently") {
		t.Error("Romans 16 differs because the doxology MOVED; the page does not say so")
	}
	if !strings.Contains(read("nkjv/esther/1"), "different text whose verses don&#39;t correspond") {
		t.Error("WEBC's Esther is a different book, not a renumbering; the page does not say so")
	}
}

// A canon-gap page is not about a licence, so it must not offer the app: the app
// has no WEB Tobit either, and "open it in the app" would simply be false. The
// translation switch is the whole answer, which is why it is never conditional.
func TestCanonGapPageOffersTheTranslationAndNotTheApp(t *testing.T) {
	site, _ := noticeFixture(t)
	b, err := os.ReadFile(filepath.Join(site.root, "web", "tobit", "1", "index.html"))
	if err != nil {
		t.Fatalf("read /web/tobit/1/: %v", err)
	}
	page := string(b)
	if strings.Contains(page, `id="openapp"`) {
		t.Error("/web/tobit/1/ offers to open Tobit in the app, which does not have it either")
	}
	if !strings.Contains(page, `href="../../../webc/tobit/1/"`) {
		t.Error("/web/tobit/1/ does not offer the translation that carries Tobit")
	}
	if !strings.Contains(page, "deuterocanonical") {
		t.Error("/web/tobit/1/ does not say why Tobit is missing")
	}
	// The footer's "Get the BibleText app" is still there — it is on every page
	// on the site — so the reader is not cut off from the app, just not told a
	// falsehood about it.
	if !strings.Contains(page, `id="getapp"`) {
		t.Error("the shared footer is missing")
	}
}

// The absent-CHAPTER copy says something different from the absent-BOOK copy,
// because the reader's question is different: they have this book open.
func TestAbsentChapterPageDoesTheArithmetic(t *testing.T) {
	site, _ := noticeFixture(t)
	b, err := os.ReadFile(filepath.Join(site.root, "web", "daniel", "13", "index.html"))
	if err != nil {
		t.Fatalf("read /web/daniel/13/: %v", err)
	}
	page := string(b)
	if !strings.Contains(page, "Daniel ends at chapter 12") {
		t.Errorf("/web/daniel/13/ does not say where the WEB's Daniel stops:\n%s", page)
	}
	if strings.Contains(page, "deuterocanonical books, which this edition") {
		t.Error("/web/daniel/13/ claims the whole book is missing; the WEB has Daniel 1-12")
	}
	// Its arrows reach a real chapter page on one side and another notice on the
	// other, so the reader can walk out of the gap in either direction.
	for _, want := range []string{`href="../12/"`, `href="../14/"`} {
		if !strings.Contains(page, want) {
			t.Errorf("/web/daniel/13/ is missing the neighbour link %s", want)
		}
	}
}

// The notice pages load the reader's own script, and that is deliberate:
// reader.js is the only thing on the site that decodes the sender's note, and a
// page where the verse cannot be shown is the page where the note matters most
// (I3). The empty <article class="text"> is where reader.js puts it.
func TestNoticePagesLoadTheReaderSoTheNoteStillRenders(t *testing.T) {
	site, _ := noticeFixture(t)
	b, err := os.ReadFile(filepath.Join(site.root, "nkjv", "john", "3", "index.html"))
	if err != nil {
		t.Fatalf("read /nkjv/john/3/: %v", err)
	}
	page := string(b)
	for _, want := range []string{jsName, noticeJSName, cssName, noticeCSSName, `<article class="text"></article>`} {
		if !strings.Contains(page, want) {
			t.Errorf("/nkjv/john/3/ does not carry %s", want)
		}
	}
	// Order matters: deferred scripts run in document order, and notice.js
	// assumes the note is already on the page.
	if strings.Index(page, jsName) > strings.Index(page, noticeJSName) {
		t.Error("notice.js is linked before reader.js; the note would not be on the page yet")
	}
}

// THE ASSET-HASH INVARIANT. The published pages link reader.css and reader.js by
// content hash, so touching either rewrites all 3,906 of them. The notice pages
// therefore carry their OWN pair, and no published page may ever request it.
func TestPublishedPagesDoNotLinkTheNoticeAssets(t *testing.T) {
	site, all := noticeFixture(t)
	published := map[string]bool{}
	for _, v := range all {
		published[v.ID] = true
	}
	gap := regexp.MustCompile(`^(web|bsb|webc)/(tobit|judith)/`)
	scanned := 0
	walkSite(t, site, func(rel, body string) {
		root := strings.SplitN(rel, "/", 2)[0]
		if !published[root] || !strings.HasSuffix(rel, "index.html") {
			return
		}
		if gap.MatchString(rel) || strings.Contains(body, `class="lede"`) {
			return // a notice page living under a published root
		}
		scanned++
		for _, asset := range []string{noticeCSSName, noticeJSName} {
			if strings.Contains(body, asset) {
				t.Errorf("%s links %s — the notice assets must never touch a page that carries scripture", rel, asset)
			}
		}
	})
	if scanned == 0 {
		t.Fatal("no published page was scanned; this test proved nothing")
	}
}

// TestNoticeURLsMatchTheAppsLinks is the join between the two halves of the
// feature, the same one TestSiteURLsMatchTheAppsLinks makes for the published
// three: the path the generator writes must be exactly the path the app's share
// link points at, or every /nkjv/ link sent from the app 404s again.
func TestNoticeURLsMatchTheAppsLinks(t *testing.T) {
	site, all := noticeFixture(t)
	canon := canonSource(t, all)

	tried := 0
	for _, book := range canon.bible.Books {
		for _, ch := range sortedChapters(canon.bible, book) {
			shared := bibletext.ShareLinkURL("nkjv", book, ch, 1, 0)
			if shared == "" {
				t.Fatalf("the app cannot build a link for %s %d", book, ch)
			}
			rel := strings.TrimSuffix(strings.TrimPrefix(shared, "https://bibletext.co.uk/"), "#v1")
			path := filepath.Join(site.root, filepath.FromSlash(rel), "index.html")
			if _, err := os.Stat(path); err != nil {
				t.Errorf("the app links %s and the generator wrote no page there", shared)
			}
			tried++
		}
	}
	if tried == 0 {
		t.Fatal("no share link was checked; this test proved nothing")
	}
}

// englishList is the join used in every versification caveat; a bad join reads
// as a typo on ~1,500 pages at once.
func TestEnglishListReadsAsASentence(t *testing.T) {
	for _, tc := range []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"World English Bible"}, "The World English Bible"},
		{[]string{"A", "B"}, "The A and the B"},
		{[]string{"A", "B", "C"}, "The A, the B and the C"},
	} {
		if got := englishList(tc.in); got != tc.want {
			t.Errorf("englishList(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// Sanity on the union: it must be the real union, not one version's canon, or
// the gap walk above would silently check nothing.
func TestUnionCanonIsEveryChapterAnyTranslationCarries(t *testing.T) {
	_, all := noticeFixture(t)
	union := unionCanon(all)

	got := map[string][]int{}
	for b, chs := range union.chapters {
		got[b] = append([]int(nil), chs...)
		sort.Ints(got[b])
	}
	for _, want := range []struct {
		book string
		list []int
	}{
		{"Daniel", []int{11, 12, 13, 14}},
		{"Tobit", []int{1, 2}},
		{"John", []int{1, 2, 3}},
	} {
		if fmt.Sprint(got[want.book]) != fmt.Sprint(want.list) {
			t.Errorf("union chapters for %s = %v, want %v", want.book, got[want.book], want.list)
		}
	}
}
