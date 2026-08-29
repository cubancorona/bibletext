package bibletext

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
	cacheFileName      = "bibletext-cache.json"
)

var errCacheNotFound = errors.New("cache not found")

type bibleCache struct {
	Version int        `json:"version"`
	SavedAt time.Time  `json:"saved_at"`
	Data    *BibleData `json:"data"`
}

func defaultCachePath() string {
	if custom := os.Getenv("BIBLETEXT_CACHE_PATH"); custom != "" {
		return custom
	}

	// On Android os.UserCacheDir() has no valid target ($HOME/.cache is read-only),
	// so without this the cache would fall back to an unwritable CWD-relative path
	// and the whole Bible would re-download every launch (and never work offline).
	// appStorageDir returns Fyne's per-app writable storage there, and "" elsewhere
	// (iOS + desktop already get a proper writable dir from os.UserCacheDir).
	if dir := appStorageDir(); dir != "" {
		return filepath.Join(dir, cacheFileName)
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		return cacheFileName
	}

	return filepath.Join(cacheDir, "bibletext", cacheFileName)
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

// cacheSavedAt reads only the envelope's SavedAt stamp — the licensed-content
// recency gate (licensedCacheStale) needs the age of a cache without paying
// for a full decode + validation of ~31k verses.
func cacheSavedAt(path string) (time.Time, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return time.Time{}, errCacheNotFound
		}
		return time.Time{}, err
	}
	var envelope struct {
		SavedAt time.Time `json:"saved_at"`
	}
	if err := json.Unmarshal(content, &envelope); err != nil {
		return time.Time{}, err
	}
	if envelope.SavedAt.IsZero() {
		return time.Time{}, errors.New("cache has no saved_at stamp")
	}
	return envelope.SavedAt, nil
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

	// WRITE, FSYNC, THEN RENAME. os.Rename is atomic against a concurrent
	// READER, but it is not a durability barrier: after a power loss or an
	// iOS jetsam kill the rename can be visible while the megabytes behind it
	// are not, leaving a zero-length or truncated file at the current-epoch
	// path. That file is a complete cache as far as anything that only stats
	// it is concerned — the input to V1 in docs/VERSION_STATES.md. The sync
	// is what makes the temp file's contents durable BEFORE it becomes the
	// name the app reads, so the cache on disk is always either the previous
	// one or a whole new one, never a half of either.
	tmpPath := path + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("write cache temp file: %w", err)
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write cache temp file: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("sync cache temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close cache temp file: %w", err)
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

	// A CACHE THAT CANNOT BE WRITTEN MUST NOT COST THE READER THEIR BIBLE.
	// This used to discard the text it had just downloaded, which made an
	// unwritable cache directory indistinguishable from being offline at
	// every call site — including the retry loop, which would then retry
	// forever at ten-minute intervals with no possibility of success, on a
	// device where the app could never open a version at all. The download
	// succeeded; the reader gets it for this session, and the next launch
	// tries the cache again. See D6 in docs/VERSION_STATES.md.
	if err := saveBibleToCache(cachePath, apiData, nowFn); err != nil {
		fmt.Fprintln(os.Stderr, "BibleText: fetched the Bible but could not cache it, serving it anyway:", err)
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
