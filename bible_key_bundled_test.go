package bibletext

import (
	"encoding/base64"
	"os"
	"testing"
)

// withBundledKey supplies a release linker value for the duration of a test.
func withBundledKey(t *testing.T, key string) {
	t.Helper()
	prev := bundledBibleKeyEnc
	if key == "" {
		bundledBibleKeyEnc = ""
	} else {
		raw := []byte(key)
		enc := make([]byte, len(raw))
		for i, b := range raw {
			enc[i] = b ^ bundledKeyMask[i%len(bundledKeyMask)]
		}
		bundledBibleKeyEnc = base64.StdEncoding.EncodeToString(enc)
	}
	t.Cleanup(func() { bundledBibleKeyEnc = prev })
}

func TestBundledKeyObfuscationRoundTrip(t *testing.T) {
	const fixtureKey = "fixture-api-key-not-a-real-credential"

	if bundledBibleKeyEnc != "" {
		t.Fatal("the repo must never carry a bundled key — bundledBibleKeyEnc has a value")
	}
	if bundledBibleKey() != "" {
		t.Error("no bundled key means no key")
	}
	withBundledKey(t, fixtureKey)
	if got := bundledBibleKey(); got != fixtureKey {
		t.Errorf("round trip = %q", got)
	}
	// The obfuscated form must not contain the key verbatim (the whole point:
	// `strings` on a release binary should not surface it).
	if bundledBibleKeyEnc == fixtureKey {
		t.Error("key is stored in the clear")
	}
	bundledBibleKeyEnc = "!!!not base64!!!"
	if bundledBibleKey() != "" {
		t.Error("undecodable value must yield no key, not garbage")
	}
}

// TestLinkedBundledKey is activated only by the synthetic release-flow script.
// It proves the real linker assignment reaches the intended package variable
// and that the production decoder returns the supplied fixture at runtime.
func TestLinkedBundledKey(t *testing.T) {
	expected, ok := os.LookupEnv("BIBLETEXT_TEST_LINKED_KEY")
	if !ok {
		t.Skip("synthetic linked-key check is not active")
	}
	if got := bundledBibleKey(); got != expected {
		t.Fatalf("linked release key did not decode to the synthetic fixture")
	}
}

// TestBundledKeyRuntimeFallback covers the release-key lifecycle without ever
// copying the project credential into on-device storage.
func TestBundledKeyRuntimeFallback(t *testing.T) {
	const appKey = "app-bundled-key"

	t.Run("fresh release uses fallback without persisting it", func(t *testing.T) {
		withBundledKey(t, appKey)
		prefs := newFakePrefs()
		k := newKeyStoreWith(prefs)
		if got := k.bibleAPIKey(); got != appKey {
			t.Errorf("effective key = %q, want the bundled fallback", got)
		}
		if !k.usingBundledBibleKey() {
			t.Error("usingBundledBibleKey should report the fallback")
		}
		if got := prefs.String(prefKeyPrefix + bibleKeyID); got != "" {
			t.Errorf("bundled fallback was persisted as %q", got)
		}
		if got := prefs.String(prefBibleKeySeeded); got != "" {
			t.Errorf("fresh fallback wrote obsolete seeded provenance %q", got)
		}
	})

	t.Run("reader key wins and provenance is not byte equality", func(t *testing.T) {
		withBundledKey(t, appKey)
		k := newKeyStoreWith(newFakePrefs())
		k.setBibleAPIKey("readers-own")
		if got := k.bibleAPIKey(); got != "readers-own" {
			t.Errorf("effective key = %q, want the reader's", got)
		}
		if k.usingBundledBibleKey() {
			t.Error("a reader's key must not report as bundled")
		}

		sameBytes := newKeyStoreWith(newFakePrefs())
		sameBytes.setBibleAPIKey(appKey)
		if sameBytes.usingBundledBibleKey() {
			t.Error("a reader-stored key must not become bundled by byte equality")
		}
	})

	t.Run("stays gone once cleared", func(t *testing.T) {
		withBundledKey(t, appKey)
		k := newKeyStoreWith(newFakePrefs())
		k.setBibleAPIKey("")
		k.noteBibleKeyCleared(true)
		if got := k.bibleAPIKey(); got != "" {
			t.Errorf("cleared fallback came back as %q", got)
		}
		if k.usingBundledBibleKey() {
			t.Error("cleared fallback still reports as bundled")
		}
		// Entering a key again cancels the "cleared" memory.
		k.setBibleAPIKey("readers-own")
		k.noteBibleKeyCleared(false)
		if got := k.bibleAPIKey(); got != "readers-own" {
			t.Errorf("reader key after re-enable = %q", got)
		}
	})

	t.Run("rotation is immediate and leaves no stored copy", func(t *testing.T) {
		withBundledKey(t, appKey)
		prefs := newFakePrefs()
		k := newKeyStoreWith(prefs)

		withBundledKey(t, "app-rotated-key") // a later build ships a new key
		if got := k.bibleAPIKey(); got != "app-rotated-key" {
			t.Errorf("rotated fallback = %q", got)
		}
		if got := prefs.String(prefKeyPrefix + bibleKeyID); got != "" {
			t.Errorf("rotation persisted a raw key as %q", got)
		}
	})

	t.Run("no key in this build changes nothing", func(t *testing.T) {
		withBundledKey(t, "")
		k := newKeyStoreWith(newFakePrefs())
		if got := k.bibleAPIKey(); got != "" {
			t.Errorf("keyless build returned %q", got)
		}
		if k.usingBundledBibleKey() {
			t.Error("keyless build must not claim a bundled key")
		}
	})
}

func TestBundledKeyLegacyMigration(t *testing.T) {
	const oldAppKey = "old-app-bundled-key"

	t.Run("deletes only a fingerprint-matched preference copy", func(t *testing.T) {
		withBundledKey(t, "current-app-bundled-key")
		prefs := newFakePrefs()
		prefs.SetString(prefKeyPrefix+bibleKeyID, oldAppKey)
		prefs.SetString(prefBibleKeySeeded, keyFingerprint(oldAppKey))
		k := newKeyStoreWith(prefs)

		k.migrateBundledBibleKey()
		if got := prefs.String(prefKeyPrefix + bibleKeyID); got != "" {
			t.Errorf("legacy raw copy remains as %q", got)
		}
		if got := prefs.String(prefBibleKeySeeded); got != "" {
			t.Errorf("legacy provenance remains as %q", got)
		}
		if got := k.bibleAPIKey(); got != "current-app-bundled-key" {
			t.Errorf("post-migration fallback = %q", got)
		}
	})

	t.Run("keeps a prior clear decision after deleting the old copy", func(t *testing.T) {
		withBundledKey(t, "current-app-bundled-key")
		prefs := newFakePrefs()
		prefs.SetString(prefKeyPrefix+bibleKeyID, oldAppKey)
		prefs.SetString(prefBibleKeySeeded, keyFingerprint(oldAppKey))
		prefs.SetString(prefBibleKeyCleared, "1")
		k := newKeyStoreWith(prefs)

		k.migrateBundledBibleKey()
		if got := k.bibleAPIKey(); got != "" {
			t.Errorf("migration re-enabled a cleared fallback as %q", got)
		}
		if k.usingBundledBibleKey() {
			t.Error("migration re-enabled bundled provenance after Clear")
		}
	})

	t.Run("preserves a reader-owned preference key", func(t *testing.T) {
		withBundledKey(t, "current-app-bundled-key")
		prefs := newFakePrefs()
		prefs.SetString(prefKeyPrefix+bibleKeyID, "readers-own")
		prefs.SetString(prefBibleKeySeeded, keyFingerprint(oldAppKey))
		k := newKeyStoreWith(prefs)

		k.migrateBundledBibleKey()
		if got := k.bibleAPIKey(); got != "readers-own" {
			t.Errorf("migration changed reader key to %q", got)
		}
		if got := prefs.String(prefBibleKeySeeded); got != "" {
			t.Errorf("stale provenance remains as %q", got)
		}
	})

	t.Run("deletes secure app copy without deleting preference reader copy", func(t *testing.T) {
		withBundledKey(t, "current-app-bundled-key")
		prefs := newFakePrefs()
		prefs.SetString(prefKeyPrefix+bibleKeyID, "readers-own")
		prefs.SetString(prefBibleKeySeeded, keyFingerprint(oldAppKey))
		secrets := &fakeSecrets{m: map[string]string{bibleKeyID: oldAppKey}}
		k := &keyStore{prefs: prefs, secrets: secrets}

		k.migrateBundledBibleKey()
		if _, found := secrets.m[bibleKeyID]; found {
			t.Error("legacy app key remains in the credential store")
		}
		if got := k.bibleAPIKey(); got != "readers-own" {
			t.Errorf("migration lost preference reader key; got %q", got)
		}
	})

	t.Run("credential-store read error defers all migration", func(t *testing.T) {
		withBundledKey(t, "current-app-bundled-key")
		prefs := newFakePrefs()
		prefs.SetString(prefKeyPrefix+bibleKeyID, oldAppKey)
		prefs.SetString(prefBibleKeySeeded, keyFingerprint(oldAppKey))
		secrets := &fakeSecrets{m: map[string]string{bibleKeyID: oldAppKey}, failRead: true}
		k := &keyStore{prefs: prefs, secrets: secrets}

		k.migrateBundledBibleKey()
		if got := prefs.String(prefKeyPrefix + bibleKeyID); got != oldAppKey {
			t.Errorf("read error changed preference copy to %q", got)
		}
		if got := prefs.String(prefBibleKeySeeded); got == "" {
			t.Error("read error discarded provenance needed for a retry")
		}
		if got := secrets.m[bibleKeyID]; got != oldAppKey {
			t.Errorf("read error changed secure copy to %q", got)
		}

		if got := k.bibleAPIKey(); got != oldAppKey {
			t.Errorf("effective key during read failure = %q, want the intact old copy", got)
		}
		secrets.failRead = false
		if got := k.bibleAPIKey(); got != "current-app-bundled-key" {
			t.Errorf("effective key after recovery = %q, want the current fallback", got)
		}
		if _, found := secrets.m[bibleKeyID]; found {
			t.Error("secure app copy remains after read recovery")
		}
		if got := prefs.String(prefKeyPrefix + bibleKeyID); got != "" {
			t.Errorf("preference app copy remains after read recovery as %q", got)
		}
	})

	t.Run("credential-store delete error retains provenance for retry", func(t *testing.T) {
		withBundledKey(t, "current-app-bundled-key")
		prefs := newFakePrefs()
		prefs.SetString(prefBibleKeySeeded, keyFingerprint(oldAppKey))
		secrets := &fakeSecrets{m: map[string]string{bibleKeyID: oldAppKey}, failWrite: true}
		k := &keyStore{prefs: prefs, secrets: secrets}

		k.migrateBundledBibleKey()
		if got := secrets.m[bibleKeyID]; got != oldAppKey {
			t.Errorf("failed delete changed secure copy to %q", got)
		}
		if got := prefs.String(prefBibleKeySeeded); got == "" {
			t.Error("failed delete discarded provenance needed for a retry")
		}
		if !k.usingBundledBibleKey() {
			t.Error("unmigrated fingerprint-matched copy lost bundled provenance")
		}
		secrets.failWrite = false
		if got := k.bibleAPIKey(); got != "current-app-bundled-key" {
			t.Errorf("effective key after delete recovery = %q, want the current fallback", got)
		}
		if _, found := secrets.m[bibleKeyID]; found {
			t.Error("secure app copy remains after delete recovery")
		}
	})

	t.Run("reader write fallback retains provenance until secure cleanup", func(t *testing.T) {
		prefs := newFakePrefs()
		prefs.SetString(prefBibleKeySeeded, keyFingerprint(oldAppKey))
		secrets := &fakeSecrets{m: map[string]string{bibleKeyID: oldAppKey}, failWrite: true}
		k := &keyStore{prefs: prefs, secrets: secrets}

		if !k.setBibleAPIKey("readers-own") {
			t.Fatal("reader fallback write failed")
		}
		if got := prefs.String(prefBibleKeySeeded); got == "" {
			t.Fatal("fallback write discarded provenance for the secure app copy")
		}
		secrets.failWrite = false
		k.migrateBundledBibleKey()
		if _, found := secrets.m[bibleKeyID]; found {
			t.Error("secure app copy remains after retry")
		}
		if got := k.bibleAPIKey(); got != "readers-own" {
			t.Errorf("cleanup lost reader fallback key; got %q", got)
		}
	})
}

// TestBundledKeyUnlocksNKJV: the runtime fallback must unlock the NKJV exactly
// as a reader-entered key does, without a seeding write.
func TestBundledKeyUnlocksNKJV(t *testing.T) {
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "")
	withBundledKey(t, "app-bundled-key")
	fake := withFakeSharedKeys(t)

	nkjv, ok := versionByID("nkjv")
	if !ok {
		t.Fatal("nkjv not registered")
	}
	if !nkjv.canSelect() {
		t.Error("the bundled key must unlock the NKJV")
	}
	if got := fake.prefs.String(prefKeyPrefix + bibleKeyID); got != "" {
		t.Errorf("unlock persisted bundled key as %q", got)
	}
}
