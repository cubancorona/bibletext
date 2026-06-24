package bibletext

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadVersionFromCacheOnlyMiss verifies the cache-only loader returns an error
// (without any network fetch) on a cache miss — the trigger for the Gospels seed-instant
// path in loadStateData.
func TestLoadVersionFromCacheOnlyMiss(t *testing.T) {
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(t.TempDir(), "no-such-cache.json"))
	v, _ := versionByID(defaultVersionID)
	if _, _, err := loadVersionFromCacheOnly(v); err == nil {
		t.Fatal("expected a cache-miss error (and no fetch), got nil")
	}
}

// TestLoadSeedGospels verifies the embedded offline-fallback Gospels decode into a
// usable, search-indexed BibleData (Matthew–John with real WEB text).
func TestLoadSeedGospels(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("loadSeedGospels: %v", err)
	}
	want := []string{"Matthew", "Mark", "Luke", "John"}
	if len(bd.Books) != len(want) {
		t.Fatalf("expected %d books, got %d: %v", len(want), len(bd.Books), bd.Books)
	}
	for i, b := range want {
		if bd.Books[i] != b {
			t.Errorf("book %d = %q, want %q", i, bd.Books[i], b)
		}
	}
	v := bd.GetVerse("John", 3, 16)
	if v == nil {
		t.Fatal("John 3:16 missing from the seed")
	}
	if !strings.Contains(v.Text, "loved the world") {
		t.Errorf("John 3:16 text unexpected: %q", v.Text)
	}
	// PrepareSearchIndex must have run so search/nav work offline (Ref is built there).
	if v.Ref == "" {
		t.Error("seed not search-indexed — Verse.Ref is empty")
	}
}
