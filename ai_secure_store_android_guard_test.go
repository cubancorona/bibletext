package bibletext

import (
	"os"
	"strings"
	"testing"
)

// The Android credential store is the one piece of this change that no test
// executes: ai_secure_store_android.go compiles only for Android, and there is
// no on-device Go test runner here. The behaviour that matters is therefore
// pinned in the one place checked on every platform — the source itself.
//
// Two of these decisions can cost a reader their API key if they are quietly
// undone, so a change that drops them has to delete this test to land.
func TestAndroidKeysStayOutOfBackedUpStorage(t *testing.T) {
	src, err := os.ReadFile("ai_secure_store_android.go")
	if err != nil {
		t.Fatalf("read ai_secure_store_android.go: %v", err)
	}
	body := string(src)

	for _, want := range []string{
		// The strict derivation, which refuses an unrecognised layout rather
		// than falling back into files/ the way the Bible cache does.
		"secretBlobDirFrom(fyneStorageRoot())",
		// The store is declared absent when the platform structurally cannot
		// do Keystore AES, rather than pretending and failing later.
		"btKeysProbe",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("ai_secure_store_android.go no longer contains %q — the reader's API "+
				"keys could return to backed-up storage", want)
		}
	}

	// appStorageDir falls back to the storage ROOT on an unrecognised layout,
	// which is inside files/ and therefore inside Auto Backup. Right for a
	// re-fetchable Bible cache, wrong for a credential.
	if strings.Contains(body, "appStorageDir(") {
		t.Error("the key directory is derived with appStorageDir, whose fallback is the " +
			"backed-up storage root; use secretBlobDirFrom, which refuses instead")
	}

	// The sentinel that decides what ships when the bridge is missing. A zero
	// here would read as "definitively absent" and invite the caller to erase
	// the reader's only other copy.
	if strings.Count(body, "status := C.int(-1)") < 2 {
		t.Error("an unwrap/wrap status sentinel is no longer initialised to -1; a failed " +
			"or absent bridge would be reported as a definitive absence")
	}

	// A key must never reach a log line. Checked per line so that a parameter
	// named plaintext in a signature is not mistaken for one being printed.
	for i, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || !strings.Contains(trimmed, "log.") {
			continue
		}
		for _, secret := range []string{"plaintext", "payload", "value", "key)"} {
			if strings.Contains(trimmed, secret) {
				t.Errorf("ai_secure_store_android.go:%d logs something named %q: %s",
					i+1, secret, trimmed)
			}
		}
	}

	// CONTROL: the sweep must be able to fail.
	if strings.Contains(body, "this string is not in the file") {
		t.Fatal("the control string matched; this test proves nothing")
	}
}

// The API-23 gate is STRUCTURAL: every android.security.keystore type is
// referenced only from BtKeysV23, which is reached only from inside an SDK_INT
// check, so ART never resolves it on API 21-22. An `if` inside the same class
// would not be enough — verification happens per class.
func TestBtKeysGatesOnAPI23(t *testing.T) {
	src, err := os.ReadFile("android/BtKeys.java")
	if err != nil {
		t.Fatalf("read android/BtKeys.java: %v", err)
	}
	body := string(src)

	// Split the file at the second class, so the public entry points can be
	// checked separately from the implementation that uses the API-23 types.
	split := strings.Index(body, "final class BtKeysV23")
	if split < 0 {
		t.Fatal("BtKeysV23 is gone — the API-23 types are no longer isolated in their own " +
			"class, so ART could try to resolve them on API 21-22")
	}
	entry, impl := body[:split], body[split:]

	if n := strings.Count(entry, "Build.VERSION.SDK_INT < 23"); n < 3 {
		t.Errorf("only %d SDK_INT gates in BtKeys' public methods; available, wrap and "+
			"unwrap must each refuse on API 21-22", n)
	}
	for _, api23 := range []string{"KeyGenParameterSpec", "KeyProperties", "KeyPermanentlyInvalidatedException"} {
		// The import block sits above the split, so look only at the class body.
		entryBody := entry[strings.Index(entry, "public final class BtKeys"):]
		if strings.Contains(entryBody, api23) {
			t.Errorf("BtKeys references the API-23 type %q outside BtKeysV23; on API 21-22 "+
				"resolving the class would throw before any gate ran", api23)
		}
	}
	if !strings.Contains(impl, "setUserAuthenticationRequired(false)") {
		t.Error("the key is no longer explicitly non-auth-bound; an auth-bound key makes " +
			"KeyPermanentlyInvalidatedException an ordinary event, and that exception is " +
			"what classifies a reader's stored key as permanently dead")
	}
	if !strings.Contains(impl, "setRandomizedEncryptionRequired(true)") {
		t.Error("randomized encryption is no longer required, so a caller-supplied IV " +
			"would be accepted and IV reuse becomes possible")
	}
	if strings.Contains(impl, "deleteEntry") {
		t.Error("BtKeys deletes a keystore entry; removing the shared alias would make " +
			"every stored key undecryptable at once")
	}

	// CONTROL.
	if strings.Contains(body, "this string is not in the file") {
		t.Fatal("the control string matched; this test proves nothing")
	}
}
