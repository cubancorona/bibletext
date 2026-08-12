package bibletext

// Licensed-cache compliance edges, complementing the happy-path gates in
// apibible_test.go: the recency window's exact boundary, malformed and
// clock-skewed saved_at stamps, the §10 startup purge under hostile
// filesystem conditions, and the no-superseded-fallback guarantee licensed
// versions carry (their epoch-migration stale-serve is PD-only by design).
// Nothing here ever fetches — only the cache-only load paths run — so the
// licence env trio holds fixture values and no server is needed.

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// unsetNKJVLicence clears the whole credential trio so the nkjv source is
// definitively unavailable regardless of the ambient environment.
func unsetNKJVLicence(t *testing.T) {
	t.Helper()
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "")
}

// setNKJVLicence configures the trio with fixture values, making the nkjv
// source available() without any provider ever being contacted.
func setNKJVLicence(t *testing.T) {
	t.Helper()
	t.Setenv("BIBLE_API_KEY", "test-key")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "1")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "test-bible")
}

// withNKJVEpoch temporarily bumps nkjv's cacheEpoch in the registry (restored
// on cleanup) so supersededCachePaths has something to return — nkjv ships at
// epoch 0 today, and both cachePathForVersion and supersededCachePaths read
// the registry, so mutating it is the only way to keep the two consistent.
func withNKJVEpoch(t *testing.T, epoch int) {
	t.Helper()
	for i := range registeredVersions {
		if registeredVersions[i].ID == "nkjv" {
			idx, prev := i, registeredVersions[i].cacheEpoch
			registeredVersions[idx].cacheEpoch = epoch
			t.Cleanup(func() { registeredVersions[idx].cacheEpoch = prev })
			return
		}
	}
	t.Fatal("nkjv not registered")
}

// (a) The 30-day window uses a strict "older than" comparison; pin it an hour
// each side of the boundary (stamping exactly AT the boundary would race the
// wall clock between write and check).
func TestLicensedCacheStaleWindowBoundary(t *testing.T) {
	dir := t.TempDir()
	justInside := filepath.Join(dir, "inside.json")
	justPast := filepath.Join(dir, "past.json")
	writeCacheStampedAt(t, justInside, time.Now().UTC().Add(-licensedRecencyWindow+time.Hour))
	writeCacheStampedAt(t, justPast, time.Now().UTC().Add(-licensedRecencyWindow-time.Hour))

	if licensedCacheStale(justInside) {
		t.Error("29d23h-old cache is inside the 30-day window and must still serve")
	}
	if !licensedCacheStale(justPast) {
		t.Error("30d1h-old cache is past the 30-day window and must revalidate (API.Bible §11)")
	}
}

// (b) A cache whose JSON is valid but carries no saved_at stamp. Every cache
// the app writes is stamped (saveBibleToCache), so this is a hand-edited or
// foreign file: cacheSavedAt rejects it (zero-stamp check), and the recency
// gate maps that error to NOT-stale — an unreadable age is treated like an
// absent cache, leaving the full load path (which decodes and validates the
// whole file) to decide its fate.
func TestCacheSavedAtMissingStamp(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "stampless.json")
	if err := os.WriteFile(missing, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	zero := filepath.Join(dir, "zero-stamp.json")
	if err := os.WriteFile(zero, []byte(`{"version":1,"saved_at":"0001-01-01T00:00:00Z"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{missing, zero} {
		at, err := cacheSavedAt(path)
		if err == nil {
			t.Errorf("%s: cacheSavedAt must reject a cache with no usable saved_at stamp", filepath.Base(path))
		}
		if !at.IsZero() {
			t.Errorf("%s: errored cacheSavedAt must return the zero time, got %v", filepath.Base(path), at)
		}
		if licensedCacheStale(path) {
			t.Errorf("%s: a stamp-less cache reads as absent, not stale", filepath.Base(path))
		}
	}
}

// (c) A corrupted/truncated cache file (a torn write): the licensed startup
// fast path must treat it as a miss — no panic, no serve.
func TestLicensedStartupTreatsCorruptCacheAsMiss(t *testing.T) {
	setNKJVLicence(t)
	dir := t.TempDir()
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(dir, "bibletext-cache.json"))
	v, ok := versionByID("nkjv")
	if !ok {
		t.Fatal("nkjv not registered")
	}
	path := cachePathForVersion("nkjv")
	if err := os.WriteFile(path, []byte(`{"version":1,"saved_at":"2026-07-0`), 0o644); err != nil {
		t.Fatal(err)
	}

	if licensedCacheStale(path) {
		t.Error("corrupt cache must read as absent, not stale")
	}
	if _, _, err := loadVersionFromCacheOnly(v); err == nil {
		t.Fatal("corrupt licensed cache must be a startup miss, not served")
	}
}

// (d) saved_at in the FUTURE (device clock skew, or a fetch stamped by a fast
// clock): the strict older-than comparison makes it fresh — never stale, and
// certainly no panic on the negative age.
func TestLicensedCacheFutureSavedAtIsFresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.json")
	future := time.Now().UTC().Add(48 * time.Hour)
	writeCacheStampedAt(t, path, future)

	at, err := cacheSavedAt(path)
	if err != nil {
		t.Fatalf("future stamp must still be readable: %v", err)
	}
	if !at.Equal(future) {
		t.Errorf("stamp round-trip: got %v, want %v", at, future)
	}
	if licensedCacheStale(path) {
		t.Error("clock skew: a future-stamped cache is fresh, not stale")
	}
}

// (e) The §10 purge end to end: with the licence gone the nkjv cache is
// removed and the public-domain web cache survives byte-identical — and a
// second run with nothing left to do is a clean no-op.
func TestPurgeEndToEndAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(dir, "bibletext-cache.json"))
	unsetNKJVLicence(t)
	nkjvPath := cachePathForVersion("nkjv")
	webPath := cachePathForVersion("web")
	writeCacheStampedAt(t, nkjvPath, time.Now().UTC())
	writeCacheStampedAt(t, webPath, time.Now().UTC())
	webBefore, err := os.ReadFile(webPath)
	if err != nil {
		t.Fatal(err)
	}

	purgeUnavailableLicensedCaches()

	if _, err := os.Stat(nkjvPath); !os.IsNotExist(err) {
		t.Error("unlicensed nkjv cache must be removed (API.Bible §10)")
	}
	if _, err := os.Stat(webPath); err != nil {
		t.Fatal("public-domain cache must survive the purge")
	}

	purgeUnavailableLicensedCaches() // nothing left to remove — must be a no-op

	if _, err := os.Stat(nkjvPath); !os.IsNotExist(err) {
		t.Error("second purge: nkjv cache must stay gone")
	}
	webAfter, err := os.ReadFile(webPath)
	if err != nil {
		t.Fatalf("second purge touched the public-domain cache: %v", err)
	}
	if string(webAfter) != string(webBefore) {
		t.Error("second purge altered the public-domain cache bytes")
	}
}

// (f) The purge is best-effort by contract: a missing cache directory and a
// directory it cannot delete from must both be survived silently.
func TestPurgeSurvivesBadCacheDir(t *testing.T) {
	unsetNKJVLicence(t)

	t.Run("missing directory", func(t *testing.T) {
		t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(t.TempDir(), "never", "made", "bibletext-cache.json"))
		purgeUnavailableLicensedCaches() // must not panic
	})

	t.Run("unwritable directory", func(t *testing.T) {
		sealed := filepath.Join(t.TempDir(), "sealed")
		if err := os.MkdirAll(sealed, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(sealed, "bibletext-cache.json"))
		nkjvPath := cachePathForVersion("nkjv")
		writeCacheStampedAt(t, nkjvPath, time.Now().UTC())
		if err := os.Chmod(sealed, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(sealed, 0o755) })

		purgeUnavailableLicensedCaches() // failed removes are swallowed, not panics

		// Whether the remove actually FAILS is the platform's business, and on
		// two platforms it legitimately succeeds: Windows does not treat a
		// read-only directory as a bar to deleting the files inside it (that
		// is an ACL, not a mode bit — this test went red only on the Windows
		// runner), and root ignores directory permissions everywhere. There
		// the guarantee under test is just the one already proven above: the
		// purge returns instead of panicking.
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			return
		}
		if _, err := os.Stat(nkjvPath); err != nil {
			t.Errorf("remove through a read-only dir should fail silently, leaving the file: %v", err)
		}
	})
}

// (g) The epoch-migration fallback (serve a superseded-epoch cache when the
// current one is missing) is a deliberate stale-serve — right for public
// domain, forbidden for licensed text. Give nkjv an epoch so a superseded
// path exists, plant a fresh, fully valid cache there, and pin that the
// licensed cache-only path still reports a miss rather than serving it.
func TestLicensedCacheOnlyIgnoresSupersededEpochs(t *testing.T) {
	setNKJVLicence(t)
	dir := t.TempDir()
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(dir, "bibletext-cache.json"))
	withNKJVEpoch(t, 1)
	v, ok := versionByID("nkjv")
	if !ok || v.cacheEpoch != 1 {
		t.Fatalf("nkjv epoch bump not visible via registry: %+v", v)
	}
	prev := supersededCachePaths(v)
	if len(prev) == 0 {
		t.Fatal("epoch 1 must yield the epoch-0 superseded path")
	}
	if prev[0] == cachePathForVersion("nkjv") {
		t.Fatalf("superseded path %q must differ from the current-epoch path", prev[0])
	}
	// Fresh and valid — exactly the file the PD fallback would happily serve.
	writeCacheStampedAt(t, prev[0], time.Now().UTC())
	if _, err := loadBibleFromCache(prev[0]); err != nil {
		t.Fatalf("fixture must be servable on its own merits: %v", err)
	}

	if _, _, err := loadVersionFromCacheOnly(v); err == nil {
		t.Fatal("licensed version must never serve a superseded-epoch cache")
	}
	if _, err := os.Stat(prev[0]); err != nil {
		t.Errorf("the cache-only path is read-only — it must not delete the superseded file: %v", err)
	}
}

// (h) A cache that is BOTH stale and unlicensed: the startup purge deletes it,
// and neither the cache-only load that may run before the purge goroutine nor
// the one after it serves or resurrects the file.
func TestStartupPurgeDeletesStaleUnlicensedCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(dir, "bibletext-cache.json"))
	unsetNKJVLicence(t)
	v, ok := versionByID("nkjv")
	if !ok {
		t.Fatal("nkjv not registered")
	}
	path := cachePathForVersion("nkjv")
	writeCacheStampedAt(t, path, time.Now().UTC().Add(-licensedRecencyWindow-time.Hour))

	// Fast paint racing ahead of the purge: unavailable source → miss, and
	// deleting is the purge's job, so the file is left untouched.
	if _, _, err := loadVersionFromCacheOnly(v); err == nil {
		t.Fatal("unlicensed version must never load, stale cache or not")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cache-only load must not touch the file: %v", err)
	}

	purgeUnavailableLicensedCaches()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("stale+unlicensed cache must be purged (§10 removal obligation)")
	}

	// And afterwards: still a miss, and nothing brings the file back.
	if _, _, err := loadVersionFromCacheOnly(v); err == nil {
		t.Fatal("purged version must stay a miss")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("no load path may resurrect a purged licensed cache")
	}
}
