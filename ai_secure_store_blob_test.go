package bibletext

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeWrapper is real AES-GCM in process, standing in for the Android Keystore.
// Real crypto rather than a stub, so the round trip, the AAD binding and the
// authentication failure are all genuine — only the key's residence differs.
type fakeWrapper struct {
	key []byte
	// forced statuses, for the failure rows the real cipher will not produce.
	failWrap   bool
	forceDead  bool
	forceError bool
}

func newFakeWrapper(t *testing.T) *fakeWrapper {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return &fakeWrapper{key: k}
}

func (f *fakeWrapper) aead(t *testing.T) cipher.AEAD {
	t.Helper()
	block, err := aes.NewCipher(f.key)
	if err != nil {
		t.Fatalf("aes: %v", err)
	}
	a, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("gcm: %v", err)
	}
	return a
}

func (f *fakeWrapper) Wrap(aad, plaintext []byte) []byte {
	if f.failWrap {
		return nil
	}
	block, err := aes.NewCipher(f.key)
	if err != nil {
		return nil
	}
	a, err := cipher.NewGCM(block)
	if err != nil {
		return nil
	}
	iv := make([]byte, a.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		return nil
	}
	return append(iv, a.Seal(nil, iv, plaintext, aad)...)
}

func (f *fakeWrapper) Unwrap(aad, payload []byte) ([]byte, int) {
	if f.forceError {
		return nil, unwrapFailed
	}
	if f.forceDead {
		return nil, unwrapDead
	}
	block, err := aes.NewCipher(f.key)
	if err != nil {
		return nil, unwrapFailed
	}
	a, err := cipher.NewGCM(block)
	if err != nil {
		return nil, unwrapFailed
	}
	if len(payload) < a.NonceSize() {
		return nil, unwrapDead
	}
	pt, err := a.Open(nil, payload[:a.NonceSize()], payload[a.NonceSize():], aad)
	if err != nil {
		// Authentication failure: wrong key, wrong account, tampered bytes.
		// There is no future in which this succeeds.
		return nil, unwrapDead
	}
	return pt, unwrapOK
}

func newTestBlobStore(t *testing.T) (*blobSecretStore, *fakeWrapper) {
	t.Helper()
	w := newFakeWrapper(t)
	return &blobSecretStore{name: "Test Store", dir: t.TempDir(), w: w}, w
}

func TestSecretBlobFraming(t *testing.T) {
	payload := make([]byte, secretBlobMinLen) // comfortably long enough framed
	for i := range payload {
		payload[i] = byte(i + 1)
	}
	framed := frameSecretBlob(payload)
	if framed[0] != secretBlobVersion {
		t.Fatalf("version byte = %#x, want %#x", framed[0], secretBlobVersion)
	}
	got, ok := unframeSecretBlob(framed)
	if !ok || string(got) != string(payload) {
		t.Fatalf("round trip failed: ok=%v", ok)
	}

	if _, ok := unframeSecretBlob(framed[:secretBlobMinLen-1]); ok {
		t.Error("a blob shorter than the minimum was accepted")
	}
	// An unknown version must be rejected as UNUSABLE, which Read turns into a
	// store error. If a future format made this look like an absent key, the
	// upgrade that introduced it would erase every reader's key.
	future := append([]byte{}, framed...)
	future[0] = 0x02
	if _, ok := unframeSecretBlob(future); ok {
		t.Error("an unknown envelope version was accepted")
	}
}

// TestSecretStoreClassification is the table that decides key loss.
func TestSecretStoreClassification(t *testing.T) {
	const account = "gemini"

	t.Run("missing file is the only definitive absence", func(t *testing.T) {
		s, _ := newTestBlobStore(t)
		v, found, ok := s.Read(account)
		if v != "" || found || !ok {
			t.Fatalf("got (%q, found=%v, ok=%v), want (\"\", false, true)", v, found, ok)
		}
	})

	t.Run("undecryptable blob is a store error, not an absence", func(t *testing.T) {
		s, w := newTestBlobStore(t)
		if !s.Write(account, "secret") {
			t.Fatal("write refused")
		}
		w.forceDead = true
		v, found, ok := s.Read(account)
		if ok || found || v != "" {
			t.Fatalf("got (%q, found=%v, ok=%v), want (\"\", false, false) — reporting a dead "+
				"blob as absent lets the caller erase the reader's only other copy", v, found, ok)
		}
	})

	t.Run("transient failure is a store error", func(t *testing.T) {
		s, w := newTestBlobStore(t)
		if !s.Write(account, "secret") {
			t.Fatal("write refused")
		}
		w.forceError = true
		if _, _, ok := s.Read(account); ok {
			t.Fatal("a transient store failure was reported as success")
		}
	})

	t.Run("malformed envelope is a store error", func(t *testing.T) {
		s, _ := newTestBlobStore(t)
		p, _ := s.path(account)
		if err := os.WriteFile(p, []byte{0x02, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13,
			14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29}, 0o600); err != nil {
			t.Fatal(err)
		}
		v, found, ok := s.Read(account)
		if ok || found || v != "" {
			t.Fatalf("got (%q, found=%v, ok=%v), want a store error", v, found, ok)
		}
	})

	t.Run("a blob cannot be read out of another account's slot", func(t *testing.T) {
		s, _ := newTestBlobStore(t)
		if !s.Write("gemini", "gemini-secret") {
			t.Fatal("write refused")
		}
		// Move the file into another provider's slot: the account id is the
		// AAD, so authentication must fail rather than yield the wrong key.
		from, _ := s.path("gemini")
		to, _ := s.path("openai")
		if err := os.Rename(from, to); err != nil {
			t.Fatal(err)
		}
		if v, _, ok := s.Read("openai"); ok || v != "" {
			t.Fatalf("a moved blob decrypted as %q for the wrong account", v)
		}
	})
}

// TestBlobSecretStoreContract runs the whole of the adapter contract against
// the storage half, which is the closest this project can get to executing it
// for Android: the real contract test builds a host binary and Android has no
// on-device Go test runner.
func TestBlobSecretStoreContract(t *testing.T) {
	s, _ := newTestBlobStore(t)

	if s.Name() == "" {
		t.Error("a store must name itself: the Settings sheet says where the key went")
	}
	// 1. Absent is reported as definitively absent, never as an error.
	if v, found, ok := s.Read(contractAccount); !ok || found || v != "" {
		t.Fatalf("absent item: got (%q, found=%v, ok=%v), want (\"\", false, true)", v, found, ok)
	}
	// 2. Write, then read it straight back.
	if !s.Write(contractAccount, "first-value") {
		t.Fatal("write refused")
	}
	if v, found, ok := s.Read(contractAccount); !ok || !found || v != "first-value" {
		t.Fatalf("after write: got (%q, found=%v, ok=%v)", v, found, ok)
	}
	// 3. Overwrite replaces rather than duplicating or appending.
	if !s.Write(contractAccount, "second-value") {
		t.Fatal("overwrite refused")
	}
	if v, _, _ := s.Read(contractAccount); v != "second-value" {
		t.Fatalf("after overwrite: got %q", v)
	}
	// 4. Empty value deletes, and the item is then definitively absent.
	if !s.Write(contractAccount, "") {
		t.Fatal("delete refused")
	}
	if v, found, ok := s.Read(contractAccount); !ok || found || v != "" {
		t.Fatalf("after delete: got (%q, found=%v, ok=%v)", v, found, ok)
	}
	// 5. Deleting what is not there is not an error.
	if !s.Write(contractAccount, "") {
		t.Error("deleting an absent item must succeed")
	}
}

// TestBlobWriteVerifiesBeforeReportingSuccess: a wrapper that encrypts happily
// and cannot decrypt must make Write return false. A true here is what erases
// the reader's plaintext copy.
func TestBlobWriteVerifiesBeforeReportingSuccess(t *testing.T) {
	s, w := newTestBlobStore(t)
	w.forceDead = true // encrypts fine, never reads back
	if s.Write("gemini", "value-that-cannot-be-read-back") {
		t.Fatal("Write reported success without a verified read-back; the caller would " +
			"now erase the reader's only other copy of their key")
	}
}

func TestBlobWriteIsAtomicAndPrivate(t *testing.T) {
	s, _ := newTestBlobStore(t)
	if !s.Write("gemini", "value") {
		t.Fatal("write refused")
	}
	p, _ := s.path("gemini")
	if _, err := os.Stat(p + ".tmp"); !errors.Is(err, fs.ErrNotExist) {
		t.Error("the temp file survived the write")
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("blob mode = %v, want 0600", info.Mode().Perm())
	}
	// The plaintext must not be sitting in the file.
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "value") {
		t.Error("the stored blob contains the plaintext")
	}
}

func TestDeleteAbsentSucceeds(t *testing.T) {
	s, _ := newTestBlobStore(t)
	// os.Remove on a missing file returns an error; contract item 5 requires
	// true. Easy to omit, and it surfaces as a false "couldn't remove the
	// stored key" in Settings.
	if !s.Write("never-written", "") {
		t.Fatal("deleting an item that was never written must succeed")
	}
}

func TestSecretBlobDirRefusesBackedUpStorage(t *testing.T) {
	root := filepath.Join("/data", "user", "0", "uk.co.bibletext", "files", "fyne")
	dir, ok := secretBlobDirFrom(root)
	if !ok {
		t.Fatalf("the real Android layout was rejected: %q", root)
	}
	if strings.Contains(dir, string(filepath.Separator)+"files"+string(filepath.Separator)) {
		t.Errorf("keys would live under files/, which Auto Backup uploads: %q", dir)
	}
	if !strings.Contains(dir, "no_backup") {
		t.Errorf("keys are not under no_backup: %q", dir)
	}

	// An unrecognised layout must refuse rather than fall back to the storage
	// root the way the Bible cache does — falling back would put the ciphertext
	// back inside the backed-up directory.
	for _, bad := range []string{"", "/", "/tmp/somewhere/else"} {
		if d, ok := secretBlobDirFrom(bad); ok {
			t.Errorf("secretBlobDirFrom(%q) = %q, true; want a refusal", bad, d)
		}
	}
}

func TestSecretAccountIDsCannotEscapeTheDirectory(t *testing.T) {
	s, _ := newTestBlobStore(t)
	for _, bad := range []string{"", ".", "..", "../evil", "a/b", "a\\b", strings.Repeat("x", 65)} {
		if p, ok := s.path(bad); ok {
			t.Errorf("path(%q) = %q, true; want a refusal", bad, p)
		}
	}
	for _, good := range []string{"gemini", "openai", "x.ai", "api-bible", "a_b"} {
		if _, ok := s.path(good); !ok {
			t.Errorf("path(%q) was refused", good)
		}
	}
}
