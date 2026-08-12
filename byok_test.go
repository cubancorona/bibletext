package bibletext

import (
	"testing"
)

// withFakeSharedKeys swaps the registry's key store for a fake-pref-backed
// one for the duration of a test.
func withFakeSharedKeys(t *testing.T) *keyStore {
	t.Helper()
	fake := newKeyStoreWith(newFakePrefs())
	prev := sharedKeys
	sharedKeys = func() *keyStore { return fake }
	t.Cleanup(func() { sharedKeys = prev })
	return fake
}

func TestBibleKeyStoreRoundTrip(t *testing.T) {
	store := newKeyStoreWith(newFakePrefs())
	if store.bibleAPIKey() != "" {
		t.Fatal("fresh store must have no bible key")
	}
	if !store.setBibleAPIKey("  abc123  ") {
		t.Fatal("save failed")
	}
	if got := store.bibleAPIKey(); got != "abc123" {
		t.Errorf("bibleAPIKey = %q, want trimmed abc123", got)
	}
	if !store.setBibleAPIKey("") {
		t.Fatal("clear failed")
	}
	if store.bibleAPIKey() != "" {
		t.Error("cleared key still present")
	}
}

// TestBYOKUnlocksNKJV pins the whole BYOK contract: a reader's stored
// API.Bible key alone unlocks the NKJV (no environment configuration), the
// built-in provider id is used, the key's absence locks it again, and
// non-BYOK licensed versions (NRSV/LSB) are untouched by the reader's key.
func TestBYOKUnlocksNKJV(t *testing.T) {
	// Ensure no env leaks in from the developer's shell.
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "")
	fake := withFakeSharedKeys(t)

	nkjv, ok := versionByID("nkjv")
	if !ok {
		t.Fatal("nkjv not registered")
	}
	if !byokCapable(nkjv) {
		t.Fatal("nkjv must be BYOK-capable")
	}
	if nkjv.canSelect() {
		t.Fatal("nkjv must be locked with no key anywhere")
	}

	fake.setBibleAPIKey("readers-own-key")
	if !nkjv.canSelect() {
		t.Fatal("stored key must unlock nkjv")
	}
	src := nkjv.source.(*licensedAPISource)
	if got := src.apiKey(); got != "readers-own-key" {
		t.Errorf("apiKey = %q, want the stored key", got)
	}
	if got := src.providerVersionID(); got != nkjvProviderBibleID {
		t.Errorf("providerVersionID = %q, want the built-in NKJV id", got)
	}

	// The reader's key never unlocks non-BYOK licensed versions.
	for _, id := range []string{"nrsv", "lsb"} {
		v, ok := versionByID(id)
		if !ok {
			t.Fatalf("%s not registered", id)
		}
		if byokCapable(v) {
			t.Errorf("%s must not be BYOK-capable", id)
		}
		if v.canSelect() {
			t.Errorf("%s must stay locked despite the reader's key", id)
		}
	}

	// Env still overrides BYOK when both exist.
	t.Setenv("BIBLE_API_KEY", "operator-key")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "operator-bible-id")
	if got := src.apiKey(); got != "operator-key" {
		t.Errorf("env key must win, got %q", got)
	}
	if got := src.providerVersionID(); got != "operator-bible-id" {
		t.Errorf("env provider id must win, got %q", got)
	}

	// Clearing the key locks nkjv again (§10 purge relies on this flip).
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "")
	fake.setBibleAPIKey("")
	if nkjv.canSelect() {
		t.Error("cleared key must lock nkjv again")
	}
}

// TestBYOKPickerPlacement: with only the reader's stored key, NKJV leads the
// picker and appears in the BYOK footer note when locked.
func TestBYOKPickerPlacement(t *testing.T) {
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "")
	fake := withFakeSharedKeys(t)

	// Locked: named in the BYOK note, not the evaluation note.
	foundBYOK := false
	for _, n := range lockedVersionNames(true) {
		if n == "NKJV" {
			foundBYOK = true
		}
	}
	if !foundBYOK {
		t.Error("locked NKJV missing from the BYOK footer note")
	}
	for _, n := range lockedVersionNames(false) {
		if n == "NKJV" {
			t.Error("NKJV must not appear in the evaluation footer note")
		}
	}

	// Key stored → leads the picker.
	fake.setBibleAPIKey("readers-own-key")
	if got := versionPickerOrder(); got[0].ID != "nkjv" {
		t.Errorf("BYOK-unlocked NKJV should lead the picker, got %q", got[0].ID)
	}
}
