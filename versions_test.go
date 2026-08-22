package bibletext

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	fynetest "fyne.io/fyne/v2/test"
)

func TestVersionRegistry(t *testing.T) {
	want := map[string]struct {
		abbrev       string
		publicDomain bool
		testing      bool // expected isTesting() with no license env set
	}{
		// What a default build actually offers. The NRSV and the LSB are both
		// behind build tags now (versions_nrsv.go / versions_lsb.go), so there
		// is no unlicensed EVALUATION version here to assert about — the only
		// locked one left is the NKJV, locked for a different reason: it is
		// bring-your-own-key and no key is configured in a test.
		"web":  {"WEB", true, false},
		"bsb":  {"BSB", true, false},
		"webc": {"WEBC", true, false},
		"nkjv": {"NKJV", false, true},
	}

	for id, exp := range want {
		v, ok := versionByID(id)
		if !ok {
			t.Fatalf("version %q missing from registry", id)
		}
		if v.Abbrev != exp.abbrev || v.Name == "" {
			t.Errorf("%s: abbrev=%q name=%q", id, v.Abbrev, v.Name)
		}
		if v.PublicDomain != exp.publicDomain {
			t.Errorf("%s: PublicDomain=%v want %v", id, v.PublicDomain, exp.publicDomain)
		}
		if got := v.isTesting(); got != exp.testing {
			t.Errorf("%s: isTesting()=%v want %v (no license env)", id, got, exp.testing)
		}
	}

	if _, ok := versionByID("nope"); ok {
		t.Error("unknown version should not resolve")
	}
	if defaultVersionID != "web" {
		t.Errorf("default version = %q, want web", defaultVersionID)
	}
}

func TestCachePathForVersion(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "bibletext-cache.json")
	t.Setenv("BIBLETEXT_CACHE_PATH", legacy)

	wantWEB := filepath.Join(dir, "bibletext-web-v2.json")
	if got := cachePathForVersion("web"); got != wantWEB {
		t.Errorf("web cache = %q, want %q", got, wantWEB)
	}
	wantLSB := filepath.Join(dir, "bibletext-lsb.json")
	if got := cachePathForVersion("lsb"); got != wantLSB {
		t.Errorf("lsb cache = %q, want %q", got, wantLSB)
	}

	// BSB carries a cacheEpoch (its decoder has changed three times), so its
	// cache path is versioned and stale pre-epoch caches are bypassed.
	wantBSB := filepath.Join(dir, "bibletext-bsb-v3.json")
	if got := cachePathForVersion("bsb"); got != wantBSB {
		t.Errorf("bsb cache = %q, want %q", got, wantBSB)
	}
}

// TestPurgeSupersededCaches verifies a cacheEpoch bump cleans up the stale
// pre-epoch cache file while leaving the current epoch's file and other versions'
// caches untouched.
func TestPurgeSupersededCaches(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "bibletext-cache.json")
	t.Setenv("BIBLETEXT_CACHE_PATH", legacy)

	stale := filepath.Join(dir, "bibletext-bsb.json")      // v0 (superseded)
	staleV1 := filepath.Join(dir, "bibletext-bsb-v1.json") // v1 (superseded)
	staleV2 := filepath.Join(dir, "bibletext-bsb-v2.json") // v2 (superseded)
	current := filepath.Join(dir, "bibletext-bsb-v3.json") // v3 (active)
	web := legacy                                          // web cache (other version)
	for _, p := range []string{stale, staleV1, staleV2, current, web} {
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	bsb, _ := versionByID("bsb")
	purgeSupersededCaches(bsb)

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale v0 cache should have been removed (err=%v)", err)
	}
	if _, err := os.Stat(staleV1); !os.IsNotExist(err) {
		t.Errorf("stale v1 cache should have been removed (err=%v)", err)
	}
	if _, err := os.Stat(staleV2); !os.IsNotExist(err) {
		t.Errorf("stale v2 cache should have been removed (err=%v)", err)
	}
	if _, err := os.Stat(current); err != nil {
		t.Errorf("current-epoch cache must be kept: %v", err)
	}
	if _, err := os.Stat(web); err != nil {
		t.Errorf("other version's (web) cache must be untouched: %v", err)
	}
}

func TestLicensedSourceAvailability(t *testing.T) {
	s := newLicensedSource("lsb")

	// Nothing configured -> not available.
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("BIBLETEXT_LICENSE_LSB", "")
	t.Setenv("BIBLETEXT_PROVIDER_ID_LSB", "")
	if s.available() {
		t.Fatal("licensed source should be unavailable with no env")
	}
	if _, err := s.fetch(); err == nil {
		t.Error("fetch should error when unavailable")
	}

	// Key alone is not enough (must also opt in to the license + provider id).
	t.Setenv("BIBLE_API_KEY", "k")
	if s.available() {
		t.Error("key alone should not make it available")
	}

	// All three -> available (real fetch is still a scaffold, but the gate opens).
	t.Setenv("BIBLETEXT_LICENSE_LSB", "1")
	t.Setenv("BIBLETEXT_PROVIDER_ID_LSB", "abc123")
	if !s.available() {
		t.Error("key + license opt-in + provider id should be available")
	}
}

func TestWebSourceAlwaysAvailable(t *testing.T) {
	if !(webSource{}).available() {
		t.Error("web source must always be available (public domain)")
	}
}

func baseSampleBible() *BibleData {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	bd.PrepareSearchIndex()
	return bd
}

// fullValidBible builds a minimal Bible with one verse in every book so it
// passes validateBibleData (used for cache round-trips).
func fullValidBible() *BibleData {
	bd := NewBibleData()
	for _, book := range bd.Books {
		bd.Verses[book] = map[int][]Verse{
			1: {{BookName: book, Book: book, Chapter: 1, Verse: 1, Text: book + " 1:1 sample."}},
		}
	}
	bd.PrepareSearchIndex()
	return bd
}

// evaluationVersion is a licensed source with no credentials — the shape the
// NRSV and the LSB had before both moved behind build tags.
//
// BUILT, NOT LOOKED UP. These tests ask "how does an unconfigured licensed
// source behave", which is a different question from "which translations does
// the app offer". Tying them together is what made a catalogue change break six
// tests at once; separating them means the behaviour stays covered in a default
// build even though no such version ships in one.
func evaluationVersion() BibleVersion {
	return BibleVersion{
		ID: "lsb", Name: "Legacy Standard Bible", Abbrev: "LSB",
		Publisher: "© The Lockman Foundation — license required",
		source:    newLicensedSource("lsb"),
	}
}

func TestPlaceholderMirrorsStructure(t *testing.T) {
	base := baseSampleBible()
	lsb := evaluationVersion()

	data, mode, err := loadVersionData(lsb, base)
	if err != nil || mode != modeTesting {
		t.Fatalf("loadVersionData(lsb) = mode %v err %v", mode, err)
	}

	// Same books and per-chapter verse counts as the base.
	if len(data.Books) != len(base.Books) {
		t.Fatalf("books = %d, want %d", len(data.Books), len(base.Books))
	}
	for _, book := range base.Books {
		for _, ch := range base.GetChapterNumbersForBook(book) {
			if got, want := len(data.GetChapter(book, ch)), len(base.GetChapter(book, ch)); got != want {
				t.Fatalf("%s %d: %d verses, want %d", book, ch, got, want)
			}
		}
	}

	// Placeholder text is clearly labeled, references the verse, and differs
	// from the real base text.
	pv := data.GetChapter("John", 1)
	if len(pv) == 0 {
		t.Fatal("expected placeholder verses for John 1")
	}
	got := pv[0].Text
	if !strings.Contains(got, "LSB") || !strings.Contains(got, "John 1:1") {
		t.Errorf("placeholder text not labeled/referenced: %q", got)
	}
	if got == base.GetChapter("John", 1)[0].Text {
		t.Error("placeholder text should differ from the base translation")
	}
}

func TestLoadVersionDataWebDoesNotNeedBase(t *testing.T) {
	// The public-domain default must not require a base placeholder; with a temp
	// cache present it loads from cache (no network).
	dir := t.TempDir()
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(dir, "bibletext-cache.json"))
	if err := saveBibleToCache(cachePathForVersion("web"), fullValidBible(), currentUTCTime); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	web, _ := versionByID("web")
	data, mode, err := loadVersionData(web, nil)
	if err != nil || mode != modeReal || data == nil {
		t.Fatalf("loadVersionData(web) = mode %v err %v", mode, err)
	}
}

func TestVersionSelectionGating(t *testing.T) {
	web, _ := versionByID("web")
	lsb := evaluationVersion()

	// Public domain is always selectable; an unlicensed copyrighted version is
	// not selectable by default (it shows as "evaluation in progress").
	t.Setenv("BIBLETEXT_ENABLE_TESTING", "")
	if !web.canSelect() {
		t.Error("web (public domain) must be selectable")
	}
	if lsb.canSelect() {
		t.Error("lsb must not be selectable without a license or the testing flag")
	}

	// The internal testing flag unlocks it for QA.
	t.Setenv("BIBLETEXT_ENABLE_TESTING", "1")
	if !lsb.canSelect() {
		t.Error("lsb should be selectable when BIBLETEXT_ENABLE_TESTING=1")
	}
}

// TestSwitchVersionRefusesUnselectable is the backstop guaranteeing users can't
// reach a not-yet-licensed version's placeholder, even if some code path calls
// switchVersion directly.
func TestSwitchVersionRefusesUnselectable(t *testing.T) {
	t.Setenv("BIBLETEXT_ENABLE_TESTING", "")

	base := baseSampleBible()
	state := &AppState{
		Bible:          base,
		CurrentVersion: "web",
		currentMode:    modeReal,
		loadedVersions: map[string]*BibleData{"web": base},
		CurrentBook:    "John",
		CurrentChapter: 1,
	}

	switchVersion(state, "lsb")
	if state.CurrentVersion != "web" || state.Bible != base {
		t.Errorf("switch to an unlicensed version should be a no-op; got version=%q", state.CurrentVersion)
	}
}

func TestVersionCacheIsCurrent(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "bibletext-cache.json")
	t.Setenv("BIBLETEXT_CACHE_PATH", legacy)

	web, _ := versionByID("web")
	if versionCacheIsCurrent(web) {
		t.Error("no cache files at all → not current")
	}
	if err := os.WriteFile(legacy, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if versionCacheIsCurrent(web) {
		t.Error("the legacy (epoch-0) file must NOT count as web's current epoch")
	}
	if err := os.WriteFile(filepath.Join(dir, "bibletext-web-v2.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !versionCacheIsCurrent(web) {
		t.Error("the v2 file is web's current epoch")
	}
}

// TestStartupStaleEpochSchedulesRefresh pins the OTHER half of the epoch
// migration: booting from a superseded cache must not only keep the reader's
// text and history (the fallback) — it must also mark fullPending so
// triggerFullDownload upgrades the stored text to the current decoder.
// Without it an epoch bump is inert for every existing reader.
func TestStartupStaleEpochSchedulesRefresh(t *testing.T) {
	app := fynetest.NewApp()
	defer app.Quit()

	dir := t.TempDir()
	legacy := filepath.Join(dir, "bibletext-cache.json")
	t.Setenv("BIBLETEXT_CACHE_PATH", legacy)

	seed, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, _, err := loadBibleData(func() (*BibleData, error) { return seed, nil }, legacy, currentUTCTime); err != nil {
		t.Fatalf("writing the legacy cache: %v", err)
	}

	state, err := loadStateData()
	if err != nil {
		t.Fatalf("loadStateData: %v", err)
	}
	if state.Bible == nil || state.Bible.Verses["John"] == nil {
		t.Fatal("boot must serve the fallback cache's text")
	}
	if !state.fullPending {
		t.Error("a superseded-epoch boot must schedule the background refresh (fullPending)")
	}
}
