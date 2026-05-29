package holybible

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadBibleDataUsesCacheWhenAvailable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.json")

	seed := testBibleData()
	if err := saveBibleToCache(cachePath, seed, func() time.Time { return time.Unix(10, 0) }); err != nil {
		t.Fatalf("save cache: %v", err)
	}

	fetchCalls := 0
	fetchFn := func() (*BibleData, error) {
		fetchCalls++
		return nil, errors.New("should not be called")
	}

	data, source, err := loadBibleData(fetchFn, cachePath, time.Now)
	if err != nil {
		t.Fatalf("loadBibleData error: %v", err)
	}
	if source != "cache" {
		t.Fatalf("expected source cache, got %q", source)
	}
	if fetchCalls != 0 {
		t.Fatalf("expected no fetch calls, got %d", fetchCalls)
	}
	if got := data.GetVerse("Genesis", 1, 1); got == nil || got.Text == "" {
		t.Fatalf("expected cached verse content, got %#v", got)
	}
}

func TestLoadBibleDataFetchesAndCachesWhenCacheMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.json")

	fetchCalls := 0
	fetchFn := func() (*BibleData, error) {
		fetchCalls++
		return testBibleData(), nil
	}

	data, source, err := loadBibleData(fetchFn, cachePath, func() time.Time { return time.Unix(20, 0) })
	if err != nil {
		t.Fatalf("loadBibleData error: %v", err)
	}
	if source != "api" {
		t.Fatalf("expected source api, got %q", source)
	}
	if fetchCalls != 1 {
		t.Fatalf("expected 1 fetch call, got %d", fetchCalls)
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("expected cache file to be created: %v", err)
	}
	if got := data.GetVerse("Genesis", 1, 1); got == nil || got.Text == "" {
		t.Fatalf("expected fetched verse content, got %#v", got)
	}
}

func TestLoadBibleDataRecoversFromCorruptCache(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.json")
	if err := os.WriteFile(cachePath, []byte("{bad json"), 0o644); err != nil {
		t.Fatalf("seed corrupt cache: %v", err)
	}

	fetchCalls := 0
	fetchFn := func() (*BibleData, error) {
		fetchCalls++
		return testBibleData(), nil
	}

	_, source, err := loadBibleData(fetchFn, cachePath, time.Now)
	if err != nil {
		t.Fatalf("loadBibleData error: %v", err)
	}
	if source != "api" {
		t.Fatalf("expected api source, got %q", source)
	}
	if fetchCalls != 1 {
		t.Fatalf("expected 1 fetch call, got %d", fetchCalls)
	}

	reloaded, err := loadBibleFromCache(cachePath)
	if err != nil {
		t.Fatalf("expected repaired cache to load, got: %v", err)
	}
	if reloaded.GetVerse("Genesis", 1, 1) == nil {
		t.Fatal("expected repaired cache to include Genesis 1:1")
	}
}

func TestLoadBibleDataReturnsErrorWhenCacheInvalidAndFetchFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.json")
	if err := os.WriteFile(cachePath, []byte("{bad json"), 0o644); err != nil {
		t.Fatalf("seed corrupt cache: %v", err)
	}

	fetchFn := func() (*BibleData, error) {
		return nil, errors.New("network down")
	}

	_, _, err := loadBibleData(fetchFn, cachePath, time.Now)
	if err == nil {
		t.Fatal("expected combined cache+api error, got nil")
	}
	if !strings.Contains(err.Error(), "cache load error") {
		t.Fatalf("expected cache context in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "network down") {
		t.Fatalf("expected api error context, got: %v", err)
	}
}

func TestValidateBibleDataRejectsMissingBookVerses(t *testing.T) {
	t.Parallel()

	d := testBibleData()
	d.Books = append(d.Books, "Exodus")
	if err := validateBibleData(d); err == nil {
		t.Fatal("expected validation error for missing Exodus")
	}
}

func testBibleData() *BibleData {
	return &BibleData{
		Books: []string{"Genesis"},
		Verses: map[string]map[int][]Verse{
			"Genesis": {
				1: {
					{BookName: "Genesis", Book: "Genesis", Chapter: 1, Verse: 1, Text: "In the beginning"},
				},
			},
		},
	}
}
