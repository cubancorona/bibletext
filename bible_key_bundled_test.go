package bibletext

import (
	"encoding/base64"
	"testing"
)

// withBundledKey compiles in a bundled key for the duration of a test, the
// way the release build's generated file does.
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
	if bundledBibleKeyEnc != "" {
		t.Fatal("the repo must never carry a bundled key — bundledBibleKeyEnc has a value")
	}
	if bundledBibleKey() != "" {
		t.Error("no bundled key means no key")
	}
	withBundledKey(t, "[redacted-repository-secret]")
	if got := bundledBibleKey(); got != "[redacted-repository-secret]" {
		t.Errorf("round trip = %q", got)
	}
	// The obfuscated form must not contain the key verbatim (the whole point:
	// `strings` on a release binary should not surface it).
	if bundledBibleKeyEnc == "[redacted-repository-secret]" {
		t.Error("key is stored in the clear")
	}
	bundledBibleKeyEnc = "!!!not base64!!!"
	if bundledBibleKey() != "" {
		t.Error("undecodable value must yield no key, not garbage")
	}
}

// TestBundledKeySeeding covers the whole life cycle the operator relies on:
// seed on first run, never overwrite the reader's own key, never come back
// after the reader clears it, and rotate in place when a new build ships a
// different key.
func TestBundledKeySeeding(t *testing.T) {
	const appKey = "app-bundled-key"

	t.Run("seeds on first run", func(t *testing.T) {
		withBundledKey(t, appKey)
		k := newKeyStoreWith(newFakePrefs())
		k.seedBundledBibleKey()
		if got := k.bibleAPIKey(); got != appKey {
			t.Errorf("stored key = %q, want the bundled key", got)
		}
		if !k.usingBundledBibleKey() {
			t.Error("usingBundledBibleKey should report the bundled key")
		}
		// Seeding twice changes nothing.
		k.seedBundledBibleKey()
		if got := k.bibleAPIKey(); got != appKey {
			t.Errorf("second seed changed the key to %q", got)
		}
	})

	t.Run("never overwrites the reader's key", func(t *testing.T) {
		withBundledKey(t, appKey)
		k := newKeyStoreWith(newFakePrefs())
		k.setBibleAPIKey("readers-own")
		k.seedBundledBibleKey()
		if got := k.bibleAPIKey(); got != "readers-own" {
			t.Errorf("stored key = %q, want the reader's", got)
		}
		if k.usingBundledBibleKey() {
			t.Error("a reader's key must not report as bundled")
		}
	})

	t.Run("stays gone once cleared", func(t *testing.T) {
		withBundledKey(t, appKey)
		k := newKeyStoreWith(newFakePrefs())
		k.seedBundledBibleKey()
		// The reader empties the field in Settings.
		k.setBibleAPIKey("")
		k.noteBibleKeyCleared(true)
		k.seedBundledBibleKey()
		if got := k.bibleAPIKey(); got != "" {
			t.Errorf("cleared key came back as %q", got)
		}
		// Entering a key again cancels the "cleared" memory.
		k.setBibleAPIKey("readers-own")
		k.noteBibleKeyCleared(false)
		k.setBibleAPIKey("")
		k.noteBibleKeyCleared(true)
		k.seedBundledBibleKey()
		if got := k.bibleAPIKey(); got != "" {
			t.Errorf("cleared again but got %q", got)
		}
	})

	t.Run("rotates its own key but not the reader's", func(t *testing.T) {
		withBundledKey(t, appKey)
		k := newKeyStoreWith(newFakePrefs())
		k.seedBundledBibleKey()

		withBundledKey(t, "app-rotated-key") // a later build ships a new key
		k.seedBundledBibleKey()
		if got := k.bibleAPIKey(); got != "app-rotated-key" {
			t.Errorf("rotation left %q in place", got)
		}

		// A reader's own key survives rotation untouched.
		k2 := newKeyStoreWith(newFakePrefs())
		k2.setBibleAPIKey("readers-own")
		k2.seedBundledBibleKey()
		if got := k2.bibleAPIKey(); got != "readers-own" {
			t.Errorf("rotation clobbered the reader's key with %q", got)
		}
	})

	t.Run("no key in this build changes nothing", func(t *testing.T) {
		withBundledKey(t, "")
		k := newKeyStoreWith(newFakePrefs())
		k.seedBundledBibleKey()
		if got := k.bibleAPIKey(); got != "" {
			t.Errorf("keyless build seeded %q", got)
		}
		if k.usingBundledBibleKey() {
			t.Error("keyless build must not claim a bundled key")
		}
	})
}

// TestBundledKeyUnlocksNKJV: a seeded key must unlock the NKJV exactly as a
// reader-entered one does — that is the whole point of shipping it.
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
	if nkjv.canSelect() {
		t.Fatal("nkjv unlocked before seeding")
	}
	fake.seedBundledBibleKey()
	if !nkjv.canSelect() {
		t.Error("the bundled key must unlock the NKJV")
	}
}
