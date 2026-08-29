package bibletext

import "testing"

// The contract every credential-store adapter must satisfy, written once so a
// Windows or Linux implementation is held to the same behaviour the Apple one
// is — and written BEFORE those exist, so they arrive with their test rather
// than after it.
//
// The distinction the keystore actually branches on is (found, ok):
//
//	found=false, ok=true   definitively absent — safe to migrate a legacy copy
//	                       in and erase it afterwards
//	ok=false               the store failed — the legacy copy must be KEPT
//
// An adapter that reports a failure as "absent" would let a transient error
// erase a reader's only copy of their key, which is the one outcome none of
// this may ever produce.
const contractAccount = "zz-secret-store-contract" // never a real provider id

func TestSecretStoreContract(t *testing.T) {
	s := newPlatformSecretStore()
	if s == nil {
		// Not a skip. A build with no store must still keep keys working
		// through Preferences and must not claim a store it does not have —
		// that is the common case on Linux and on every unsigned build, so it
		// is asserted rather than passed over.
		t.Log("no platform store in this build — asserting the fallback contract instead")
		k := &keyStore{prefs: &fakePrefs{m: map[string]string{}}, secrets: nil}
		if got := k.secureStoreName(); got != "" {
			t.Errorf("no store, but the sheet would name %q", got)
		}
		if k.keyInSecureStore(providerGemini) {
			t.Error("no store, yet a key is claimed to be in one")
		}
		if !k.setAPIKey(providerGemini, "fallback-value") {
			t.Fatal("with no store, a key must still save to Preferences")
		}
		if got := k.apiKey(providerGemini); got != "fallback-value" {
			t.Fatalf("fallback round-trip = %q", got)
		}
		return
	}

	if s.Name() == "" {
		t.Error("a store must name itself: the Settings sheet says where the key went")
	}
	t.Cleanup(func() { s.Write(contractAccount, "") })

	// 1. Absent must be reported as definitively absent, never as an error.
	if v, found, ok := s.Read(contractAccount); !ok || found || v != "" {
		t.Fatalf("absent item: got (%q, found=%v, ok=%v), want (\"\", false, true) — "+
			"reporting absence as a store error blocks migration forever; reporting "+
			"an error as absence erases the reader's only copy", v, found, ok)
	}
	// 2. Write, then read it straight back.
	if !s.Write(contractAccount, "first-value") {
		t.Fatal("write refused")
	}
	if v, found, ok := s.Read(contractAccount); !ok || !found || v != "first-value" {
		t.Fatalf("after write: got (%q, found=%v, ok=%v), want (\"first-value\", true, true)", v, found, ok)
	}
	// 3. Overwrite replaces rather than duplicating or appending.
	if !s.Write(contractAccount, "second-value") {
		t.Fatal("overwrite refused")
	}
	if v, _, _ := s.Read(contractAccount); v != "second-value" {
		t.Fatalf("after overwrite: got %q, want \"second-value\"", v)
	}
	// 4. Empty value deletes, and the item is then definitively absent.
	if !s.Write(contractAccount, "") {
		t.Fatal("delete refused")
	}
	if v, found, ok := s.Read(contractAccount); !ok || found || v != "" {
		t.Fatalf("after delete: got (%q, found=%v, ok=%v), want (\"\", false, true)", v, found, ok)
	}
	// 5. Deleting what is not there is not an error: the keystore clears a key
	//    it may never have written.
	if !s.Write(contractAccount, "") {
		t.Error("deleting an absent item must succeed")
	}
}
