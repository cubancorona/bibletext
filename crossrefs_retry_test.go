package bibletext

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// resetCrossRefLoader empties the Treasury loader's state for the test and
// restores it after: the index, the loaded flag and the URL the zip is
// fetched from.
func resetCrossRefLoader(t *testing.T) {
	t.Helper()
	crossRefMu.Lock()
	wasLoaded, wasIndex, wasURL := crossRefLoaded, crossRefIndex, crossRefURL
	crossRefLoaded, crossRefIndex = false, nil
	crossRefMu.Unlock()
	t.Cleanup(func() {
		crossRefMu.Lock()
		crossRefLoaded, crossRefIndex, crossRefURL = wasLoaded, wasIndex, wasURL
		crossRefMu.Unlock()
	})
}

// treasuryServer serves the Treasury zip while serve is set and refuses
// otherwise, in place of the dataset's host.
func treasuryServer(t *testing.T, zip []byte, serve *atomic.Bool) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !serve.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write(zip)
	}))
	t.Cleanup(srv.Close)
	crossRefURL = srv.URL
}

// A FAILED LOAD IS NOT REMEMBERED. The loader used to set its loaded flag
// before fetching, so a reader who opened the panel offline was told to
// check the connection and try again for the rest of the run, and no retry
// could change the answer. Now the next attempt fetches afresh, and only a
// built index stops the asking.
func TestEnsureCrossRefsRetriesAfterAFailure(t *testing.T) {
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(t.TempDir(), cacheFileName))
	resetCrossRefLoader(t)
	var serve atomic.Bool
	treasuryServer(t, makeCrossRefZip(t, "John.3.16\tRom.5.8\t10\n"), &serve)

	if err := ensureCrossRefs(); err == nil {
		t.Fatal("control: a refused fetch must fail the load")
	}
	crossRefMu.Lock()
	loaded := crossRefLoaded
	crossRefMu.Unlock()
	if loaded {
		t.Fatal("a failed load set the loaded flag")
	}
	serve.Store(true)
	if err := ensureCrossRefs(); err != nil {
		t.Fatalf("the retry after a failure failed too: %v", err)
	}
	crossRefMu.Lock()
	rows := crossRefIndex[crossRefKey("John", 3, 16)]
	loaded = crossRefLoaded
	crossRefMu.Unlock()
	if !loaded || len(rows) != 1 || rows[0].crossRef().label() != "Romans 5:8" {
		t.Errorf("after the retry: loaded=%v, John 3:16 rows %+v; want the index built from the served zip", loaded, rows)
	}
	// CONTROL: once built, the index is kept and the host is not asked again.
	serve.Store(false)
	if err := ensureCrossRefs(); err != nil {
		t.Errorf("a built index asked the host again: %v", err)
	}
}

// A CACHED ZIP THAT WILL NOT PARSE IS REPLACED, not failed on for ever: the
// retry would otherwise read the same bytes and fail the same way, and the
// panel's Try again would be a verb that does nothing.
func TestEnsureCrossRefsReplacesACorruptCache(t *testing.T) {
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(t.TempDir(), cacheFileName))
	resetCrossRefLoader(t)
	var serve atomic.Bool
	serve.Store(true)
	good := makeCrossRefZip(t, "John.3.16\tRom.5.8\t10\n")
	treasuryServer(t, good, &serve)
	if err := os.WriteFile(crossRefCachePath(), []byte("not a zip at all"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ensureCrossRefs(); err == nil {
		t.Fatal("control: the corrupt cache must fail the first load")
	}
	if _, err := os.Stat(crossRefCachePath()); !os.IsNotExist(err) {
		t.Fatalf("the corrupt cache was kept (stat: %v)", err)
	}
	if err := ensureCrossRefs(); err != nil {
		t.Fatalf("the retry did not fetch past the corrupt cache: %v", err)
	}
	cached, err := os.ReadFile(crossRefCachePath())
	if err != nil || string(cached) != string(good) {
		t.Errorf("the fetched zip was not cached in the corrupt one's place (err %v, %d bytes)", err, len(cached))
	}
	crossRefMu.Lock()
	rows := crossRefIndex[crossRefKey("John", 3, 16)]
	crossRefMu.Unlock()
	if len(rows) != 1 {
		t.Errorf("the index was not built from the fetched zip: %+v", rows)
	}
}
