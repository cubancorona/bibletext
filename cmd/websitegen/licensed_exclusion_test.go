package main

// Licensed-text exclusion. The web reader is published, cached by search
// engines and unfurled into strangers' message threads — it must NEVER emit a
// licensed translation's TEXT. The public-domain three (web/bsb/webc) are the
// only translations this site carries words of, forever, unless a human
// deliberately decides otherwise, and these tests are the tripwire that makes
// that decision loud.
//
// WHAT CHANGED, AND WHY. This file used to walk every emitted file and fail on
// the case-insensitive substring "nkjv" anywhere in it. That was a PROXY for the
// real rule — a licence covers a publisher's TEXT, not the name of their
// translation — and it worked only while the site said nothing about the NKJV at
// all. It stopped working the day /nkjv/ pages had to exist: the app emits
// /nkjv/… share links, and the site answered them with a 404 for every recipient
// without the app and a bare URL in every message thread (B_WEB_404,
// B_UNFURL_NKJV). Those pages must be able to say "New King James Version" out
// loud, because that is the one fact the reader needs.
//
// So the proxy is replaced by the thing it stood for, in four parts:
//
//	1. publishedVersions — the list that carries a DECODER and a fetch URL — is
//	   still pinned to exactly the public-domain three (unchanged).
//	2. A licensed id may be a site ROOT only if it is in noticedVersions, whose
//	   entries carry an id and a display name and nothing that could hold text.
//	3. No scripture may appear under a notice root or on a notice page. Proved
//	   with a sentinel: the fixture's verse text carries a token that must appear
//	   under /web/ and must appear NOWHERE under /nkjv/ or on any canon-gap page.
//	4. No page under /web/, /bsb/ or /webc/ may so much as mention the NKJV. That
//	   is the OLD assertion, kept exactly where it still applies — the published
//	   pages must not advertise a translation they do not carry, and it doubles
//	   as the canary for the byte-identity of those three trees.
//
// Everything here is hermetic: publishedVersions is a static literal and the
// site is built from in-memory fixture data, so no network and no real key are
// ever touched.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

// licensedVersionIDs are the registry ids whose TEXT requires a distribution
// license (versions.go: nrsv/lsb/nkjv). None may ever have words on the web.
var licensedVersionIDs = []string{"nrsv", "lsb", "nkjv"}

// publicDomainWebIDs is the frozen contract from share_link.go: the only
// version ids the web reader publishes TEXT for, in the generator's order.
var publicDomainWebIDs = []string{"web", "bsb", "webc"}

// scriptureSentinel is embedded in every fixture verse. Any file containing it
// contains scripture; any file that must not contain scripture must not contain
// it. Deliberately a token no rendered chrome would ever produce.
const scriptureSentinel = "FIXTUREVERSETOKEN"

// setLicensedEnv configures everything that makes licensed versions live in
// the APP: the full NKJV env trio (which flips nkjv's licensedAPISource to
// available, so it is real, fetchable and selectable) plus the internal QA
// unlock that makes even the unconfigured licensed versions selectable. This
// is the strongest "selectable in the app" state that exists — and the site
// generator must not notice any of it.
func setLicensedEnv(t *testing.T) {
	t.Helper()
	t.Setenv("BIBLE_API_KEY", "test-key-never-sent-anywhere")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "1")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "test-provider-id")
	t.Setenv("BIBLETEXT_ENABLE_TESTING", "1")
}

// TestPublishedVersionsExcludeLicensedEvenWhenConfigured pins the generator's
// version list to exactly the public-domain three. The env setup makes NKJV
// fully configured (and every licensed version selectable in-app), so a pass
// here proves the site's list is not derived from availability or canSelect —
// publishing is a separate, deliberate decision. Adding a licensed id to
// publishedVersions turns this red.
func TestPublishedVersionsExcludeLicensedEvenWhenConfigured(t *testing.T) {
	setLicensedEnv(t)

	got := publishedVersions()
	var ids []string
	for _, pv := range got {
		ids = append(ids, pv.ID)
	}
	if len(ids) != len(publicDomainWebIDs) {
		t.Fatalf("publishedVersions = %v, want exactly the public-domain ids %v — "+
			"the web reader must never grow a version without a deliberate licensing decision",
			ids, publicDomainWebIDs)
	}
	for i, want := range publicDomainWebIDs {
		if ids[i] != want {
			t.Errorf("publishedVersions[%d] = %q, want %q (list pinned: %v)", i, ids[i], want, publicDomainWebIDs)
		}
	}
	for _, pv := range got {
		for _, lic := range licensedVersionIDs {
			if pv.ID == lic {
				t.Errorf("publishedVersions contains licensed id %q — licensed text must NEVER reach the web reader", lic)
			}
		}
	}
	// The fallback root every licensed share-link degrades to must itself be
	// published, or the fallback would 404.
	found := false
	for _, id := range ids {
		if id == defaultVersionID {
			found = true
		}
	}
	if !found {
		t.Errorf("defaultVersionID %q is not among the published versions %v", defaultVersionID, ids)
	}
}

// TestOnlyNoticedLicensedIDsMayBeRoots is the URL-namespace half. A licensed id
// reaches the root in one of three ways: publishedVersions (pinned above),
// noticedVersions (allowed, and text-free by construction — noticeVersion has
// no decoder and no data), or a book slug colliding with it under a future canon
// change (never allowed). This pins which licensed ids are noticed, so adding
// /nrsv/ or /lsb/ signposts is a deliberate edit here and not a side effect.
func TestOnlyNoticedLicensedIDsMayBeRoots(t *testing.T) {
	noticed := map[string]bool{}
	for _, nv := range noticedVersions() {
		noticed[nv.ID] = true
	}
	if len(noticed) != 1 || !noticed["nkjv"] {
		t.Errorf("noticedVersions = %v, want exactly {nkjv} — a new signpost root must be a deliberate edit here", noticed)
	}
	// A noticed version carries a name and NOTHING that could hold scripture.
	// This is the structural guarantee behind the sentinel walk below.
	for _, nv := range noticedVersions() {
		if nv.Name == "" {
			t.Errorf("noticed version %q has no display name — the page could not say what it is", nv.ID)
		}
	}
	for _, id := range licensedVersionIDs {
		if book, ok := bibletext.BookFromSlug(id); ok {
			t.Errorf("book %q owns the slug %q — a licensed version id must never double as a book path segment", book, id)
		}
		if noticed[id] {
			continue
		}
		for reserved := range reservedRootNames {
			if strings.Contains(strings.ToLower(reserved), id) {
				t.Errorf("reservedRootNames entry %q mentions licensed id %q — no root may be staked for it", reserved, id)
			}
		}
	}
	// And the reverse guard: the published ids are root directories, so no book
	// slug may ever collide with one (the site would nest a version under a
	// version). Holds by inspection today; keep it held.
	for _, id := range publicDomainWebIDs {
		if book, ok := bibletext.BookFromSlug(id); ok {
			t.Errorf("book %q owns the slug %q, which is a published version's root", book, id)
		}
	}
}

// fixtureSite builds the real site — real writeSite, real publishedVersions,
// real renderers — from in-memory scripture, with the NKJV environment fully
// configured. WEB Catholic gets a book and a chapter the other two lack, so the
// canon-gap pages are exercised too.
func fixtureSite(t *testing.T) *siteWriter {
	t.Helper()
	setLicensedEnv(t)

	verse := func(book string, ch, num int) bibletext.Verse {
		return v(book, ch, num, "For God so loved the world "+scriptureSentinel+".")
	}
	protestant := &bibletext.BibleData{
		Books: []string{"John", "Daniel"},
		Verses: map[string]map[int][]bibletext.Verse{
			"John":   {3: {verse("John", 3, 16), verse("John", 3, 17)}},
			"Daniel": {12: {verse("Daniel", 12, 1)}},
		},
	}
	catholic := &bibletext.BibleData{
		Books: []string{"John", "Daniel", "Tobit"},
		Verses: map[string]map[int][]bibletext.Verse{
			"John":   {3: {verse("John", 3, 16), verse("John", 3, 17)}},
			"Daniel": {12: {verse("Daniel", 12, 1)}, 13: {verse("Daniel", 13, 1)}},
			"Tobit":  {1: {verse("Tobit", 1, 1)}},
		},
	}
	var loaded []loadedVersion
	for _, pv := range publishedVersions() {
		bible := protestant
		if pv.ID == "webc" {
			bible = catholic
		}
		loaded = append(loaded, loadedVersion{webVersion: pv, bible: bible})
	}

	site := &siteWriter{root: filepath.Join(t.TempDir(), "site")}
	if err := writeSite(site, loaded); err != nil {
		t.Fatalf("writeSite: %v", err)
	}
	return site
}

// walkSite calls fn for every emitted text file, with its slash-separated path
// relative to the site root.
func walkSite(t *testing.T, site *siteWriter, fn func(rel string, body string)) {
	t.Helper()
	err := filepath.WalkDir(site.root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		switch filepath.Ext(path) {
		case ".html", ".js", ".css", ".txt":
		default:
			return nil
		}
		rel, relErr := filepath.Rel(site.root, path)
		if relErr != nil {
			return relErr
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		fn(filepath.ToSlash(rel), string(body))
		return nil
	})
	if err != nil {
		t.Fatalf("walk site tree: %v", err)
	}
}

// TestGeneratedSiteRootsAreOnlyPublishedOrNoticed: the root namespace is
// frozen to the three published translations, the noticed signposts, assets/
// and 404.html — nothing else, and nothing missing (so no later walk can pass
// vacuously on a truncated build).
func TestGeneratedSiteRootsAreOnlyPublishedOrNoticed(t *testing.T) {
	site := fixtureSite(t)

	allowed := map[string]bool{"assets": true, "404.html": true}
	for _, id := range publicDomainWebIDs {
		allowed[id] = true
	}
	for _, nv := range noticedVersions() {
		allowed[nv.ID] = true
	}
	entries, err := os.ReadDir(site.root)
	if err != nil {
		t.Fatalf("read site root: %v", err)
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if !allowed[e.Name()] {
			t.Errorf("unexpected site root entry %q — the root namespace is frozen to %v", e.Name(), allowed)
		}
		seen[e.Name()] = true
	}
	for want := range allowed {
		if !seen[want] {
			t.Errorf("site root is missing %q — the build is incomplete, so the other walks prove nothing", want)
		}
	}
}

// TestNoLicensedRootCarriesScripture is the tripwire proper, and the one that
// replaced the "nkjv" substring check. The sentinel rides in every fixture
// verse: it MUST appear under the published roots (or this test is measuring
// nothing) and MUST NOT appear anywhere under a noticed root.
//
// This goes red the moment anyone hands renderNotice real verse text — which is
// exactly what it is for, since the structural defence (noticeSpec has no field
// that can hold scripture) is one refactor away from being untrue.
func TestNoLicensedRootCarriesScripture(t *testing.T) {
	site := fixtureSite(t)

	noticed := map[string]bool{}
	for _, nv := range noticedVersions() {
		noticed[nv.ID] = true
	}
	published, sentinelSeen := 0, 0
	walkSite(t, site, func(rel, body string) {
		root := strings.SplitN(rel, "/", 2)[0]
		has := strings.Contains(body, scriptureSentinel)
		if noticed[root] {
			published++
			if has {
				t.Errorf("%s carries scripture — a noticed translation's pages must publish no text at all", rel)
			}
			return
		}
		if has {
			sentinelSeen++
		}
	})
	if published == 0 {
		t.Fatal("no pages were written under a noticed root — this test proved nothing")
	}
	if sentinelSeen == 0 {
		t.Fatal("the sentinel appears nowhere in the whole site — the fixture is not reaching the pages, " +
			"so the assertion above is vacuous")
	}
}

// TestCanonGapPagesCarryNoScripture is the same rule for the OTHER placement of
// the notice page. /web/tobit/1/ lives under a published root, so the walk above
// deliberately lets it through; it still must not show WEB Catholic's Tobit.
func TestCanonGapPagesCarryNoScripture(t *testing.T) {
	site := fixtureSite(t)

	checked := 0
	for _, gap := range []string{"web/tobit/1", "bsb/tobit/1", "web/tobit", "web/daniel/13"} {
		body, err := os.ReadFile(filepath.Join(site.root, filepath.FromSlash(gap), "index.html"))
		if err != nil {
			t.Errorf("%s: %v — the canon-gap page is missing, so the gap is still a 404", gap, err)
			continue
		}
		checked++
		if strings.Contains(string(body), scriptureSentinel) {
			t.Errorf("/%s/ carries scripture — a canon-gap page names the passage and does not show it", gap)
		}
	}
	if checked == 0 {
		t.Fatal("no canon-gap page was found; this test proved nothing")
	}
}

// TestPublishedPagesNeverMentionALicensedTranslation is the OLD assertion, kept
// exactly where it still holds. The three published trees carry scripture and
// nothing else may creep into them: no NKJV pill in their nav, no NKJV entry in
// reader.js's book table, no NKJV line in reader.css. It is also the canary for
// the byte-identity those three trees are required to keep — anything that
// changed one of those files would almost certainly trip this first.
func TestPublishedPagesNeverMentionALicensedTranslation(t *testing.T) {
	site := fixtureSite(t)

	published := map[string]bool{}
	for _, id := range publicDomainWebIDs {
		published[id] = true
	}
	// The shared assets belong to the published pages too: they are linked from
	// every one of them, so a mention there is a mention on all 3,906.
	shared := map[string]bool{}
	for _, name := range []string{cssName, jsName} {
		shared[name] = true
	}

	names := []string{"nkjv", "new king james", "nrsv", "new revised standard", "lsb", "legacy standard"}
	scanned := 0
	walkSite(t, site, func(rel, body string) {
		root := strings.SplitN(rel, "/", 2)[0]
		if !published[root] && !shared[rel] {
			return
		}
		scanned++
		low := strings.ToLower(body)
		for _, n := range names {
			if strings.Contains(low, n) {
				t.Errorf("%s mentions %q — a page or shared asset that carries scripture must not "+
					"advertise a translation this site does not publish", rel, n)
			}
		}
	})
	if scanned == 0 {
		t.Fatal("nothing under a published root was scanned; this test proved nothing")
	}
}

// TestNoticePagesCarryNoVerseMarkup is the structural half of the sentinel
// check, and it survives a change of fixture text. paragraphBody is the only
// thing on this site that writes a verse anchor or a words-of-Christ span; if
// either appears on a page that is supposed to carry no scripture, scripture is
// what got onto it.
func TestNoticePagesCarryNoVerseMarkup(t *testing.T) {
	site := fixtureSite(t)

	noticed := map[string]bool{}
	for _, nv := range noticedVersions() {
		noticed[nv.ID] = true
	}
	gaps := map[string]bool{
		"web/tobit/index.html": true, "web/tobit/1/index.html": true,
		"bsb/tobit/1/index.html": true, "web/daniel/13/index.html": true,
	}
	checked := 0
	walkSite(t, site, func(rel, body string) {
		root := strings.SplitN(rel, "/", 2)[0]
		if !noticed[root] && !gaps[rel] {
			return
		}
		checked++
		for _, markup := range []string{`class="v" id="v`, `class="wj"`, `<sup class="n"`} {
			if strings.Contains(body, markup) {
				t.Errorf("%s contains %s — that markup is only ever written around scripture", rel, markup)
			}
		}
	})
	if checked == 0 {
		t.Fatal("no notice page was scanned; this test proved nothing")
	}
}
