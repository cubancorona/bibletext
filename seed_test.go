package bibletext

import (
	"strings"
	"testing"
)

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
