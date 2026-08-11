package main

// Licensed-text exclusion. The web reader is published, cached by search
// engines and unfurled into strangers' message threads — it must NEVER emit a
// licensed translation's text, or link to a root where one could live. The
// public-domain three (web/bsb/webc) are the whole site, forever, unless a
// human deliberately decides otherwise — and these tests are the tripwire that
// makes that decision loud.
//
// The generator's version list is HARDCODED (publishedVersions in main.go), so
// no filter function exists to probe with a hypothetical licensed-but-
// configured version. These tests therefore PIN the hardcoding — adding "nkjv"
// (or any licensed id) to publishedVersions fails below — and prove that the
// app-side switches that make a licensed version real and selectable (the
// BIBLE_API_KEY / BIBLETEXT_LICENSE_NKJV / BIBLETEXT_PROVIDER_ID_NKJV trio,
// plus BIBLETEXT_ENABLE_TESTING) have no influence on what the site publishes.
//
// Everything here is hermetic: publishedVersions is a static literal and the
// site is built from in-memory fixture data, so no network and no real key are
// ever touched.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	bibletext "bibletext"
)

// licensedVersionIDs are the registry ids whose text requires a distribution
// license (versions.go: nrsv/lsb/nkjv). None may ever surface on the web.
var licensedVersionIDs = []string{"nrsv", "lsb", "nkjv"}

// publicDomainWebIDs is the frozen contract from share_link.go: the only
// version ids the web reader publishes, in the generator's order.
var publicDomainWebIDs = []string{"web", "bsb", "webc"}

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

// TestLicensedVersionIDsAreNotGeneratableRoots covers the URL namespace: the
// generator's root path segments are exactly the published version ids plus
// assets/ and 404.html, so a licensed id could only become a root via (1) the
// version list — pinned above — or (2) a book slug colliding with it under a
// future canon change. The slug table (bookslugs.go) must therefore never map
// any book to a licensed version id, and reservedRootNames must not quietly
// pre-stake a licensed root either (it exists solely to protect the four
// hand-written files — a licensed entry appearing there would mean someone had
// started carving out an nkjv page at the root).
func TestLicensedVersionIDsAreNotGeneratableRoots(t *testing.T) {
	for _, id := range licensedVersionIDs {
		if book, ok := bibletext.BookFromSlug(id); ok {
			t.Errorf("book %q owns the slug %q — a licensed version id must never double as a book path segment", book, id)
		}
		for reserved := range reservedRootNames {
			if strings.Contains(strings.ToLower(reserved), id) {
				t.Errorf("reservedRootNames entry %q mentions licensed id %q — no root may be staked for a licensed version", reserved, id)
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

// TestGeneratedSiteTreeHasOnlyPublicDomainRoots is the end-to-end proof: build
// the real site (fixture scripture, real writeSite/render pipeline, real
// publishedVersions) with the NKJV environment fully configured, then walk
// every file it wrote. No path may contain a licensed version id as a segment,
// the root may contain only the published ids plus assets/ and 404.html, and
// no emitted page or asset may so much as mention NKJV — the chrome (version
// switcher, 404, reader.js book table) must not advertise it either.
func TestGeneratedSiteTreeHasOnlyPublicDomainRoots(t *testing.T) {
	setLicensedEnv(t)

	verses := map[string]map[int][]bibletext.Verse{
		"John": {3: {
			v("John", 3, 16, "For God so loved the world."),
			v("John", 3, 17, "For God didn't send his Son to judge the world."),
		}},
	}
	var loaded []loadedVersion
	for _, pv := range publishedVersions() {
		loaded = append(loaded, loadedVersion{
			webVersion: pv,
			bible:      &bibletext.BibleData{Books: []string{"John"}, Verses: verses},
		})
	}

	site := &siteWriter{root: filepath.Join(t.TempDir(), "site")}
	if err := writeSite(site, loaded); err != nil {
		t.Fatalf("writeSite: %v", err)
	}

	allowedRoots := map[string]bool{"assets": true, "404.html": true}
	for _, id := range publicDomainWebIDs {
		allowedRoots[id] = true
	}
	entries, err := os.ReadDir(site.root)
	if err != nil {
		t.Fatalf("read site root: %v", err)
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if !allowedRoots[e.Name()] {
			t.Errorf("unexpected site root entry %q — the root namespace is frozen to %v", e.Name(), allowedRoots)
		}
		seen[e.Name()] = true
	}
	// The inverse, so this test can never pass vacuously on an empty build:
	// every published version really was written.
	for want := range allowedRoots {
		if !seen[want] {
			t.Errorf("site root is missing %q — the build is incomplete, so the walk below proves nothing", want)
		}
	}

	err = filepath.WalkDir(site.root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(site.root, path)
		if relErr != nil {
			return relErr
		}
		for _, seg := range strings.Split(filepath.ToSlash(rel), "/") {
			for _, lic := range licensedVersionIDs {
				if strings.EqualFold(seg, lic) {
					t.Errorf("generated path %q contains licensed version id %q as a segment", rel, lic)
				}
			}
		}
		if d.IsDir() {
			return nil
		}
		switch filepath.Ext(path) {
		case ".html", ".js", ".css", ".txt":
			body, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if strings.Contains(strings.ToLower(string(body)), "nkjv") {
				t.Errorf("generated file %q mentions NKJV — the site must neither serve nor advertise licensed text", rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk site tree: %v", err)
	}
}
