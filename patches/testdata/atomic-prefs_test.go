package app

// Proof for patches/fyne-2.7.4-atomic-prefs.patch. It lives in patches/ and is
// COPIED into third_party/fyne/app/ by scripts/setup-fyne-patch.sh, for the same
// reason the emoji font is copied: third_party/fyne is regenerated from scratch
// on every run, so anything written directly into it is destroyed on the next
// build. (Learned by losing this file once.)
//
// Run it with:  go test ./app/ -run TestBT -v   (inside third_party/fyne)

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// blob approximates a full BibleText store: ~200 shared notes plus the reading
// state, which is the size at which the save window is widest.
func blob() []byte {
	values := map[string]any{}
	for i := 0; i < 200; i++ {
		values[fmt.Sprintf("shared.note.%d", i)] = strings.Repeat("a note somebody sent. ", 8)
	}
	values["reading.state"] = `{"b":"John","c":3,"v":16}`
	out, err := json.Marshal(values)
	if err != nil {
		panic(err)
	}
	return append(out, '\n')
}

// readerLoop reads the store the way a relaunch after a kill would, and counts
// everything that is not a complete, parseable blob.
func readerLoop(path string, stop <-chan struct{}, empty, invalid, reads *int64) {
	for {
		select {
		case <-stop:
			return
		default:
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue // not created yet, or briefly unopenable
		}
		atomic.AddInt64(reads, 1)
		if len(data) == 0 {
			atomic.AddInt64(empty, 1)
			continue
		}
		var v map[string]any
		if json.Unmarshal(data, &v) != nil || len(v) != 201 {
			atomic.AddInt64(invalid, 1)
		}
	}
}

// hammer runs two writers (the app has exactly two: the navigation save and the
// lifecycle flush) against four readers.
func hammer(t *testing.T, path string, write func(path string, data []byte)) (empty, invalid, reads int64) {
	t.Helper()
	data := blob()
	stop := make(chan struct{})
	var readersDone sync.WaitGroup
	for i := 0; i < 4; i++ {
		readersDone.Add(1)
		go func() {
			defer readersDone.Done()
			readerLoop(path, stop, &empty, &invalid, &reads)
		}()
	}
	var writersDone sync.WaitGroup
	for i := 0; i < 2; i++ {
		writersDone.Add(1)
		go func() {
			defer writersDone.Done()
			for n := 0; n < 400; n++ {
				write(path, data)
				time.Sleep(time.Microsecond * 50)
			}
		}()
	}
	writersDone.Wait()
	close(stop)
	readersDone.Wait()
	return atomic.LoadInt64(&empty), atomic.LoadInt64(&invalid), atomic.LoadInt64(&reads)
}

// TestBTStockWriteIsTorn is the control. It reproduces the defect with stock
// Fyne's os.Create + one Write, so that the zero below means something.
func TestBTStockWriteIsTorn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	empty, invalid, reads := hammer(t, path, func(path string, data []byte) {
		f, err := os.Create(path)
		if err != nil {
			t.Error(err)
			return
		}
		_, _ = f.Write(data)
		_ = f.Sync()
		_ = f.Close()
	})
	t.Logf("stock os.Create: %d reads, %d EMPTY (every note lost), %d partial/invalid", reads, empty, invalid)
	if empty+invalid == 0 {
		t.Skip("stock happened not to tear on this run; the patched test is the one that matters")
	}
}

// TestBTAtomicWriteNeverTorn is the fix: every observation is a complete blob,
// whenever the reader looks.
func TestBTAtomicWriteNeverTorn(t *testing.T) {
	p := &preferences{}
	path := filepath.Join(t.TempDir(), "preferences.json")
	empty, invalid, reads := hammer(t, path, func(path string, data []byte) {
		w, err := p.storageWriterForPath(path)
		if err != nil {
			t.Error(err)
			return
		}
		if _, err := w.Write(data); err != nil {
			t.Error(err)
		}
		if err := w.Sync(); err != nil {
			t.Error(err)
		}
		if err := w.Close(); err != nil {
			t.Error(err)
		}
	})
	t.Logf("patched atomic write: %d reads, %d empty, %d partial/invalid", reads, empty, invalid)
	if empty != 0 || invalid != 0 {
		t.Fatalf("torn read observed: %d empty, %d invalid", empty, invalid)
	}

	// And no temp files are left lying beside the store.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "preferences.json" {
			t.Errorf("leftover file in storage dir: %s", e.Name())
		}
	}
}

// TestBTCloseTwice — saveToStorage defers Close, and a caller may close again.
func TestBTCloseTwice(t *testing.T) {
	p := &preferences{}
	path := filepath.Join(t.TempDir(), "preferences.json")
	w, err := p.storageWriterForPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("second Close returned %v, want nil", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "{}\n" {
		t.Fatalf("file = %q, %v", got, err)
	}
}

// TestBTAbandonedWriteLeavesStoreIntact — a save that dies part-way must leave
// what is already stored untouched. This is the whole point of the patch.
func TestBTAbandonedWriteLeavesStoreIntact(t *testing.T) {
	p := &preferences{}
	path := filepath.Join(t.TempDir(), "preferences.json")
	if err := os.WriteFile(path, []byte(`{"shared.notes":"six notes"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	w, err := p.storageWriterForPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(`{"partial":`)); err != nil {
		t.Fatal(err)
	}
	// Stand in for the process being killed mid-save: abandon the writer without
	// closing, then clear the temp file the way a reboot's tmp sweep would.
	aw := w.(*atomicWriter)
	_ = aw.file.Close()
	_ = os.Remove(aw.tmp)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"shared.notes":"six notes"}` {
		t.Fatalf("existing store was damaged: %q", got)
	}
}

// TestBTSweepsStaleTemps — a process killed between CreateTemp and the rename
// leaves a whole copy of the store behind. Old ones are swept; anything recent
// may still belong to a live writer and must be left alone.
func TestBTSweepsStaleTemps(t *testing.T) {
	p := &preferences{}
	dir := t.TempDir()
	path := filepath.Join(dir, "preferences.json")

	stale := filepath.Join(dir, "preferences.json.tmp-000000")
	fresh := filepath.Join(dir, "preferences.json.tmp-999999")
	other := filepath.Join(dir, "something-else.json")
	for _, f := range []string{stale, fresh, other} {
		if err := os.WriteFile(f, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(other, old, old); err != nil {
		t.Fatal(err)
	}

	sweepStale = sync.Once{} // other tests in this package may have spent it
	w, err := p.storageWriterForPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale temp file survived the sweep")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("recent temp file was swept: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("unrelated file was swept: %v", err)
	}
}

// The three raw benchmarks below exist because this Mac is far too noisy to
// compare numbers ACROSS `go test` runs — stock measured 4.3ms in one run and
// 9.0ms in the next, unpatched both times. Measuring every variant in one
// process is the only way to read what the patch actually costs.

func BenchmarkBTRawStock(b *testing.B) {
	path := filepath.Join(b.TempDir(), "preferences.json")
	data := blob()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		f, _ := os.Create(path)
		_, _ = f.Write(data)
		_ = f.Sync()
		_ = f.Close()
	}
}

func BenchmarkBTRawTempRename(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "preferences.json")
	data := blob()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		f, _ := os.CreateTemp(dir, "preferences.json.tmp-*")
		_, _ = f.Write(data)
		_ = f.Sync()
		_ = f.Close()
		_ = os.Rename(f.Name(), path)
	}
}

func BenchmarkBTRawTempRenameDirSync(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "preferences.json")
	data := blob()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		f, _ := os.CreateTemp(dir, "preferences.json.tmp-*")
		_, _ = f.Write(data)
		_ = f.Sync()
		_ = f.Close()
		_ = os.Rename(f.Name(), path)
		if d, err := os.Open(dir); err == nil {
			_ = d.Sync()
			_ = d.Close()
		}
	}
}

func BenchmarkBTAtomicSave(b *testing.B) {
	p := &preferences{}
	path := filepath.Join(b.TempDir(), "preferences.json")
	data := blob()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w, _ := p.storageWriterForPath(path)
		_, _ = w.Write(data)
		_ = w.Sync()
		_ = w.Close()
	}
}
