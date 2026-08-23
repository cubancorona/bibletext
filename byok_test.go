package bibletext

import (
	"strings"
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
// non-BYOK licensed versions (the LSB) are untouched by the reader's key.
func TestBYOKUnlocksNKJV(t *testing.T) {
	// Ensure no env leaks in from the surrounding shell.
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
	// Built rather than looked up: the LSB is behind the `lsb` build tag now, so
	// a default build registers no non-BYOK licensed version. The BEHAVIOUR
	// still needs covering — a reader's API.Bible key must not unlock a source
	// that was never wired to accept one.
	for _, v := range []BibleVersion{evaluationVersion()} {
		if byokCapable(v) {
			t.Errorf("%s must not be BYOK-capable", v.ID)
		}
		if v.canSelect() {
			t.Errorf("%s must stay locked despite the reader's key", v.ID)
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

// TestProviderKeyLabels pins the Settings key-row labels: every provider has
// a short name, and the row reads "<Product> key" — never a bare "Key", and
// never the vendor parenthetical that would crowd the field beside it.
func TestProviderKeyLabels(t *testing.T) {
	want := map[string]string{
		"gemini":    "Gemini key",
		"openai":    "ChatGPT key",
		"anthropic": "Claude key",
		"grok":      "Grok key",
	}
	for _, p := range aiProviders() {
		if p.ShortName == "" {
			t.Errorf("provider %q has no ShortName", p.ID)
		}
		if strings.Contains(p.ShortName, "(") {
			t.Errorf("provider %q ShortName %q keeps the vendor parenthetical", p.ID, p.ShortName)
		}
		got := providerKeyLabel(p)
		if w, ok := want[p.ID]; ok && got != w {
			t.Errorf("provider %q key label = %q, want %q", p.ID, got, w)
		}
	}
	// A provider without a short name still names itself rather than
	// falling back to a bare "key".
	if got := providerKeyLabel(providerInfo{Name: "Some Provider"}); got != "Some Provider key" {
		t.Errorf("fallback label = %q", got)
	}
}
