package bibletext

import (
	"errors"
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

// TestStartupBibleCacheMissWithSavedStateSkipsSeed pins the upgrade-safety
// contract: when a decoder/cache epoch changes, an existing reader's durable
// position and history must never be validated against the Gospel-only seed.
// The complete Bible is loaded before startup state restoration instead.
func TestStartupBibleCacheMissWithSavedStateSkipsSeed(t *testing.T) {
	v, _ := versionByID(defaultVersionID)
	full := fullValidBible()
	seedCalls, fullCalls := 0, 0

	got, mode, seeded, err := loadStartupBible(
		v,
		true,
		func(BibleVersion) (*BibleData, dataMode, error) {
			return nil, modeReal, errors.New("cache epoch miss")
		},
		func() (*BibleData, error) {
			seedCalls++
			return loadSeedGospels()
		},
		func(BibleVersion, *BibleData) (*BibleData, dataMode, error) {
			fullCalls++
			return full, modeReal, nil
		},
	)
	if err != nil {
		t.Fatalf("loadStartupBible: %v", err)
	}
	if got != full || mode != modeReal || seeded {
		t.Fatalf("startup result = data %p mode %v seeded %v, want full data %p / real / false",
			got, mode, seeded, full)
	}
	if seedCalls != 0 || fullCalls != 1 {
		t.Fatalf("loader calls: seed=%d full=%d, want seed=0 full=1", seedCalls, fullCalls)
	}
}

// If an existing reader is offline during the migration, failing safely is
// preferable to opening partial data and destroying their history. Retry can
// recover later; an overwritten preference cannot.
func TestStartupBibleSavedStateOfflineNeverFallsBackToSeed(t *testing.T) {
	v, _ := versionByID(defaultVersionID)
	wantErr := errors.New("offline")
	seedCalls := 0

	got, _, seeded, err := loadStartupBible(
		v,
		true,
		func(BibleVersion) (*BibleData, dataMode, error) {
			return nil, modeReal, errCacheNotFound
		},
		func() (*BibleData, error) {
			seedCalls++
			return loadSeedGospels()
		},
		func(BibleVersion, *BibleData) (*BibleData, dataMode, error) {
			return nil, modeReal, wantErr
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("loadStartupBible error = %v, want %v", err, wantErr)
	}
	if got != nil || seeded || seedCalls != 0 {
		t.Fatalf("offline migration returned data %p seeded=%v seedCalls=%d; want nil/false/0",
			got, seeded, seedCalls)
	}
}

// A genuinely new install has no history to protect, so it should retain the
// instant Gospel-seed startup while the complete Bible downloads in the background.
func TestStartupBibleCacheMissWithoutSavedStateUsesSeed(t *testing.T) {
	v, _ := versionByID(defaultVersionID)
	seed, err := loadSeedGospels()
	if err != nil {
		t.Fatal(err)
	}
	seedCalls, fullCalls := 0, 0

	got, mode, seeded, err := loadStartupBible(
		v,
		false,
		func(BibleVersion) (*BibleData, dataMode, error) {
			return nil, modeReal, errCacheNotFound
		},
		func() (*BibleData, error) {
			seedCalls++
			return seed, nil
		},
		func(BibleVersion, *BibleData) (*BibleData, dataMode, error) {
			fullCalls++
			return fullValidBible(), modeReal, nil
		},
	)
	if err != nil {
		t.Fatalf("loadStartupBible: %v", err)
	}
	if got != seed || mode != modeReal || !seeded {
		t.Fatalf("startup result = data %p mode %v seeded %v, want seed %p / real / true",
			got, mode, seeded, seed)
	}
	if seedCalls != 1 || fullCalls != 0 {
		t.Fatalf("loader calls: seed=%d full=%d, want seed=1 full=0", seedCalls, fullCalls)
	}
}

// A fresh install (no saved reading state) opens on Matthew 1 — the New
// Testament's first page — in every canon that has Matthew (all of ours).
func TestDefaultStartBookIsMatthew(t *testing.T) {
	seed, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if got := defaultStartBook(seed); got != "Matthew" {
		t.Errorf("defaultStartBook(seed) = %q, want Matthew", got)
	}
	// A canon WITHOUT Matthew (defensive fallback) opens on its first book.
	noMatt := &BibleData{
		Books: []string{"Genesis"},
		Verses: map[string]map[int][]Verse{"Genesis": {1: {
			{BookName: "Genesis", Book: "Genesis", Chapter: 1, Verse: 1, Text: "In the beginning"},
		}}},
	}
	if got := defaultStartBook(noMatt); got != "Genesis" {
		t.Errorf("defaultStartBook(no-Matthew canon) = %q, want Genesis", got)
	}
}
