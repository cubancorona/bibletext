package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	cacheSchemaVersion = 1
	cacheFileName      = "holy-bible-cache.json"
)

var errCacheNotFound = errors.New("cache not found")

type bibleCache struct {
	Version int        `json:"version"`
	SavedAt time.Time  `json:"saved_at"`
	Data    *BibleData `json:"data"`
}

func defaultCachePath() string {
	if custom := os.Getenv("HOLY_BIBLE_CACHE_PATH"); custom != "" {
		return custom
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		return cacheFileName
	}

	return filepath.Join(cacheDir, "holy-bible", cacheFileName)
}

func loadBibleFromCache(path string) (*BibleData, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errCacheNotFound
		}
		return nil, fmt.Errorf("read cache: %w", err)
	}

	var cached bibleCache
	if err := json.Unmarshal(content, &cached); err != nil {
		return nil, fmt.Errorf("decode cache: %w", err)
	}

	if cached.Version != cacheSchemaVersion {
		return nil, fmt.Errorf("cache version mismatch: got %d, want %d", cached.Version, cacheSchemaVersion)
	}
	if cached.Data == nil {
		return nil, errors.New("cache missing bible data")
	}
	if err := validateBibleData(cached.Data); err != nil {
		return nil, fmt.Errorf("cache validation failed: %w", err)
	}

	return cached.Data, nil
}

func saveBibleToCache(path string, data *BibleData, nowFn func() time.Time) error {
	if err := validateBibleData(data); err != nil {
		return fmt.Errorf("cannot cache invalid bible data: %w", err)
	}

	cached := bibleCache{
		Version: cacheSchemaVersion,
		SavedAt: nowFn().UTC(),
		Data:    data,
	}

	content, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("encode cache: %w", err)
	}

	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create cache dir: %w", err)
		}
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0o644); err != nil {
		return fmt.Errorf("write cache temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("activate cache file: %w", err)
	}

	return nil
}

func loadBibleData(fetchFn func() (*BibleData, error), cachePath string, nowFn func() time.Time) (*BibleData, string, error) {
	cachedData, cacheErr := loadBibleFromCache(cachePath)
	if cacheErr == nil {
		cachedData.PrepareSearchIndex()
		return cachedData, "cache", nil
	}

	apiData, apiErr := fetchFn()
	if apiErr != nil {
		if errors.Is(cacheErr, errCacheNotFound) {
			return nil, "", fmt.Errorf("api fetch failed and cache does not exist: %w", apiErr)
		}
		return nil, "", fmt.Errorf("api fetch failed after cache load error (%v): %w", cacheErr, apiErr)
	}

	if err := saveBibleToCache(cachePath, apiData, nowFn); err != nil {
		return nil, "", fmt.Errorf("fetched bible data but failed to save cache: %w", err)
	}

	apiData.PrepareSearchIndex()
	return apiData, "api", nil
}

func validateBibleData(data *BibleData) error {
	if data == nil {
		return errors.New("nil bible data")
	}
	if len(data.Books) == 0 {
		return errors.New("no books available")
	}
	if len(data.Verses) == 0 {
		return errors.New("no verses available")
	}

	for _, book := range data.Books {
		chapters, ok := data.Verses[book]
		if !ok {
			return fmt.Errorf("missing verses for book %q", book)
		}
		if len(chapters) == 0 {
			return fmt.Errorf("book %q has no chapters", book)
		}

		hasVerse := false
		for _, verses := range chapters {
			if len(verses) > 0 {
				hasVerse = true
				break
			}
		}
		if !hasVerse {
			return fmt.Errorf("book %q has no verses", book)
		}
	}

	return nil
}
