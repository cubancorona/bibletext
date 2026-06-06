package holybible

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionRegistry(t *testing.T) {
	want := map[string]struct {
		abbrev       string
		publicDomain bool
		testing      bool // expected isTesting() with no license env set
	}{
		"web":  {"WEB", true, false},
		"nrsv": {"NRSV", false, true},
		"lsb":  {"LSB", false, true},
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
	legacy := filepath.Join(dir, "holy-bible-cache.json")
	t.Setenv("HOLY_BIBLE_CACHE_PATH", legacy)

	if got := cachePathForVersion("web"); got != legacy {
		t.Errorf("web cache = %q, want legacy %q", got, legacy)
	}
	wantNRSV := filepath.Join(dir, "holy-bible-nrsv.json")
	if got := cachePathForVersion("nrsv"); got != wantNRSV {
		t.Errorf("nrsv cache = %q, want %q", got, wantNRSV)
	}
}

func TestLicensedSourceAvailability(t *testing.T) {
	s := newLicensedSource("nrsv")

	// Nothing configured -> not available.
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("HOLY_BIBLE_LICENSE_NRSV", "")
	t.Setenv("HOLY_BIBLE_PROVIDER_ID_NRSV", "")
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
	t.Setenv("HOLY_BIBLE_LICENSE_NRSV", "1")
	t.Setenv("HOLY_BIBLE_PROVIDER_ID_NRSV", "abc123")
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

func TestPlaceholderMirrorsStructure(t *testing.T) {
	base := baseSampleBible()
	nrsv, _ := versionByID("nrsv")

	data, mode, err := loadVersionData(nrsv, base)
	if err != nil || mode != modeTesting {
		t.Fatalf("loadVersionData(nrsv) = mode %v err %v", mode, err)
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
	if !strings.Contains(got, "NRSV") || !strings.Contains(got, "John 1:1") {
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
	t.Setenv("HOLY_BIBLE_CACHE_PATH", filepath.Join(dir, "holy-bible-cache.json"))
	if err := saveBibleToCache(cachePathForVersion("web"), fullValidBible(), currentUTCTime); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	web, _ := versionByID("web")
	data, mode, err := loadVersionData(web, nil)
	if err != nil || mode != modeReal || data == nil {
		t.Fatalf("loadVersionData(web) = mode %v err %v", mode, err)
	}
}

func TestSwitchVersionUpdatesState(t *testing.T) {
	base := baseSampleBible()
	state := &AppState{
		Bible:          base,
		CurrentVersion: "web",
		currentMode:    modeReal,
		loadedVersions: map[string]*BibleData{"web": base},
		CurrentBook:    "John",
		CurrentChapter: 1,
	}

	// No window in tests, so rebuildWindow is a no-op; the state still updates.
	switchVersion(state, "lsb")
	if state.CurrentVersion != "lsb" || state.currentMode != modeTesting {
		t.Fatalf("after switch: version=%q mode=%v", state.CurrentVersion, state.currentMode)
	}
	if state.Bible == base {
		t.Error("Bible should have swapped to the LSB placeholder")
	}
	if state.currentVersion().Abbrev != "LSB" {
		t.Errorf("currentVersion abbrev = %q", state.currentVersion().Abbrev)
	}
	// Switching back to the cached base is instant and restores it.
	switchVersion(state, "web")
	if state.CurrentVersion != "web" || state.Bible != base {
		t.Error("switching back to web should restore the base data")
	}
}
