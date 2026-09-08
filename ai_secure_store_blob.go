package bibletext

import (
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// The storage half of a credential store whose CIPHER lives somewhere else.
//
// An Android Keystore key never leaves the device's secure hardware, so the
// encrypt/decrypt call has to happen in Java. Everything AROUND that call —
// the envelope, the file, the atomic write, and the (found, ok) answer that
// decides whether a reader keeps their key — lives here, deliberately with no
// build tag, so it runs under `go test` on every platform.
//
// That split is the point. TestSecretStoreContract builds a host binary, and
// there is no on-device Go test runner in this project, so anything placed in
// the android-tagged file is code no test ever executes. Only the cipher call
// earns that.

const (
	secretBlobVersion = 0x01
	secretBlobIVLen   = 12
	secretBlobTagLen  = 16
	// version + iv + tag + at least one byte of ciphertext.
	secretBlobMinLen = 1 + secretBlobIVLen + secretBlobTagLen + 1
)

// Unwrap's status, mirroring the Java side's tri-state return.
const (
	unwrapFailed = -1 // the store failed, or could not be asked
	unwrapDead   = 0  // this blob can never be decrypted on this device again
	unwrapOK     = 1
)

// keyWrapper is the platform cipher: the one part that cannot be tested on a
// host, reduced to two methods so everything else can be.
type keyWrapper interface {
	// Wrap returns iv||ciphertext||tag, or nil on any failure.
	Wrap(aad, plaintext []byte) []byte
	// Unwrap returns the plaintext with unwrapOK, or nil with unwrapDead or
	// unwrapFailed. The two failure states are NOT interchangeable: see Read.
	Unwrap(aad, payload []byte) (plaintext []byte, status int)
}

type blobSecretStore struct {
	name string
	dir  string
	w    keyWrapper
	// Two keyStore instances migrate concurrently on a device — the shared one
	// from the background load and the app-state one from the UI thread, each
	// arriving on its own JNI goroutine. Nothing else serialises them.
	mu sync.Mutex
}

func (s *blobSecretStore) Name() string { return s.name }

// frameSecretBlob prefixes the version byte the reader is stored under.
func frameSecretBlob(payload []byte) []byte {
	out := make([]byte, 0, 1+len(payload))
	return append(append(out, secretBlobVersion), payload...)
}

// unframeSecretBlob strips the version byte. An UNKNOWN version is reported as
// unusable, and the caller then treats it as a store error rather than as an
// absent key — otherwise "we changed the format" would read as "delete every
// reader's key".
func unframeSecretBlob(raw []byte) ([]byte, bool) {
	if len(raw) < secretBlobMinLen || raw[0] != secretBlobVersion {
		return nil, false
	}
	return raw[1:], true
}

// validSecretAccount keeps an account id inside its own directory. The ids are
// provider ids and the API.Bible id, all plain words, so this is a whitelist
// rather than an escaping problem.
func validSecretAccount(account string) bool {
	if account == "" || len(account) > 64 {
		return false
	}
	for _, r := range account {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-':
		default:
			return false
		}
	}
	// "." and ".." pass the character test and must not pass this one.
	return account != "." && account != ".."
}

func (s *blobSecretStore) path(account string) (string, bool) {
	if s == nil || s.dir == "" || !validSecretAccount(account) {
		return "", false
	}
	return filepath.Join(s.dir, "ai-"+account+".bin"), true
}

// Read reports found=false with ok=true for EXACTLY ONE condition: the blob
// file does not exist. A missing keystore alias, a failed authentication, an
// unreadable file, an unknown envelope version, a JNI call that could not run —
// every one of those reports ok=false.
//
// That asymmetry is the whole safety argument. ok=false only blocks a migration
// and leaves the reader's existing plaintext copy in place; found=false with
// ok=true is what lets the caller erase it. So no misclassification anywhere
// below this line, in Java or in C, can reach that erase.
func (s *blobSecretStore) Read(account string) (string, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readLocked(account)
}

func (s *blobSecretStore) readLocked(account string) (string, bool, bool) {
	p, ok := s.path(account)
	if !ok {
		return "", false, false
	}
	raw, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, true // the only definitive absence
	}
	if err != nil {
		return "", false, false
	}
	payload, ok := unframeSecretBlob(raw)
	if !ok {
		return "", false, false
	}
	pt, status := s.w.Unwrap([]byte(account), payload)
	if status == unwrapOK && len(pt) > 0 {
		return string(pt), true, true
	}
	if status == unwrapDead {
		// Say so once, but do not delete the file on our own judgement and do
		// not call it absent. The next successful Write replaces it.
		log.Printf("bibletext: the stored key for %q can no longer be decrypted on this device", account)
	}
	return "", false, false
}

// Write reports success only once the bytes are durable AND have been read back
// and decrypted, because this boolean alone is what erases the reader's only
// other copy. A store assembled out of a file and a piece of secure hardware
// cannot afford to answer from the write call alone.
func (s *blobSecretStore) Write(account, value string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.path(account)
	if !ok {
		return false
	}
	if value == "" {
		err := os.Remove(p)
		// Deleting what is not there is not an error: the caller clears a key
		// it may never have written.
		return err == nil || errors.Is(err, fs.ErrNotExist)
	}
	blob := s.w.Wrap([]byte(account), []byte(value))
	if len(blob) == 0 {
		return false
	}
	if !writeSecretBlobFile(p, frameSecretBlob(blob)) {
		return false
	}
	got, found, readOK := s.readLocked(account)
	return readOK && found && got == value
}

// writeSecretBlobFile is the cache's write-fsync-rename, at 0600. The rename is
// atomic against a reader but is not a durability barrier: without the sync the
// name can become visible while the bytes behind it are not, which here would
// mean a key that reads back empty after a power loss.
func writeSecretBlobFile(path string, content []byte) bool {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return false
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return false
	}
	if err := f.Sync(); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return false
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return false
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return false
	}
	return true
}

// secretBlobDirFrom derives the key directory from Fyne's storage root.
//
// Unlike appStorageDir it REFUSES to fall back to the storage root. Falling
// back is the right answer for a re-fetchable Bible cache and the wrong one for
// a credential: the storage root is inside files/, which Android Auto Backup
// uploads to the reader's Google Drive, and putting the ciphertext there would
// separate it from the Keystore key that opens it. On an unrecognised layout
// this returns ("", false) and the caller then declares no store at all, which
// is exactly today's behaviour rather than a new way to lose a key.
func secretBlobDirFrom(root string) (string, bool) {
	if root == "" {
		return "", false
	}
	dir := root
	for i := 0; i < 4; i++ {
		parent := filepath.Dir(dir)
		if parent == dir || parent == string(filepath.Separator) {
			break
		}
		if filepath.Base(dir) == "files" {
			return filepath.Join(parent, "no_backup", "bibletext", "keys"), true
		}
		dir = parent
	}
	return "", false
}
