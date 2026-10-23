package bibletext

// The record behind decision 13's line: a mark in the settings that a key was
// saved, which backs up with them, beside a key that does not on a store a
// restore can leave behind (keysLeftByRestore). The line is said only where
// all of it holds (keyLeftOnOldDevice); these hold the mark to the key and
// the answer to each of its conditions.

import "testing"

// TestTheSavedMarkFollowsTheReadersKey: saving a key marks the account,
// clearing it unmarks it, a clear the store refused leaves the mark with the
// key, a key saved before the mark existed is marked by the next launch's
// sweep, and the API.Bible key an older release copied on-device, which was
// never the reader's, is not.
func TestTheSavedMarkFollowsTheReadersKey(t *testing.T) {
	mark := func(k *keyStore, id string) string { return k.prefs.String(prefKeySavedPrefix + id) }

	k := &keyStore{prefs: newFakePrefs(), secrets: newDeviceBoundTestStore()}
	k.setAPIKey(defaultProviderID, "a-key")
	if mark(k, defaultProviderID) != "1" {
		t.Fatal("a saved key left its account unmarked")
	}
	k.setAPIKey(defaultProviderID, "")
	if mark(k, defaultProviderID) != "" {
		t.Fatal("a cleared key left its account marked")
	}

	refusing := &keyStore{prefs: newFakePrefs(), secrets: &fakeSecrets{m: map[string]string{}}}
	refusing.setAPIKey(defaultProviderID, "a-key")
	refusing.secrets.(*fakeSecrets).failWrite = true
	if refusing.setAPIKey(defaultProviderID, "") {
		t.Fatal("control: a delete the store refused must report failure")
	}
	if mark(refusing, defaultProviderID) != "1" {
		t.Fatal("a clear the store refused unmarked a key that is still saved")
	}

	// Saved by a release before the mark: in the store, no mark. The launch's
	// sweep marks it, and a restore from a backup taken after says so.
	store := newDeviceBoundTestStore()
	store.m[defaultProviderID] = "an-older-key"
	older := &keyStore{prefs: newFakePrefs(), secrets: store}
	older.migrateAllKeys()
	if mark(older, defaultProviderID) != "1" {
		t.Fatal("the launch's sweep did not mark a key saved before the mark existed")
	}
	if mark(older, bibleKeyID) != "" {
		t.Fatal("the sweep marked an account that holds no key")
	}
	restored := &keyStore{prefs: older.prefs, secrets: newDeviceBoundTestStore()}
	if !restored.keyLeftOnOldDevice(defaultProviderID) {
		t.Fatal("settings backed up after the sweep, restored onto a phone without the key, do not say it stayed behind")
	}

	// The project key an older release copied on-device, its provenance not
	// yet resolved because the store failed to answer: not the reader's key.
	seeded := newDeviceBoundTestStore()
	seeded.m[bibleKeyID] = "the-project-key"
	prefs := newFakePrefs()
	prefs.SetString(prefBibleKeySeeded, keyFingerprint("the-project-key"))
	app := &keyStore{prefs: prefs, secrets: seeded}
	app.markKeySaved(bibleKeyID)
	if mark(app, bibleKeyID) != "" {
		t.Fatal("the project key an older release copied was marked as the reader's")
	}

	// No credential store: the key is in the settings themselves, which travel
	// with it, so there is nothing to mark.
	plain := newKeyStoreWith(newFakePrefs())
	plain.setAPIKey(defaultProviderID, "a-key")
	if mark(plain, defaultProviderID) != "" {
		t.Fatal("a key kept in the settings themselves was marked")
	}
}

// TestAKeyIsSaidToBeOnTheOldDeviceOnlyWhenEveryConditionHolds walks the four
// conditions of keyLeftOnOldDevice one at a time: the store is device-bound,
// the account is marked, the store answered that it holds none, and no copy
// is left in the settings. Each alone false must make it false.
func TestAKeyIsSaidToBeOnTheOldDeviceOnlyWhenEveryConditionHolds(t *testing.T) {
	const id = defaultProviderID
	build := func(store secretStore, marked bool, legacy string) *keyStore {
		p := newFakePrefs()
		if marked {
			p.SetString(prefKeySavedPrefix+id, "1")
		}
		p.SetString(prefKeyPrefix+id, legacy)
		return &keyStore{prefs: p, secrets: store}
	}
	holding := newDeviceBoundTestStore()
	holding.m[id] = "a-key"
	for _, tc := range []struct {
		name  string
		k     *keyStore
		wants bool
	}{
		{"every condition", build(newDeviceBoundTestStore(), true, ""), true},
		{"a store whose keys travel", build(emptySecretStore{}, true, ""), false},
		{"no store at all", &keyStore{prefs: build(nil, true, "").prefs}, false},
		{"not marked", build(newDeviceBoundTestStore(), false, ""), false},
		{"the store holds it", build(holding, true, ""), false},
		{"the store cannot answer", build(unreadableDeviceBoundStore{}, true, ""), false},
		{"a copy in the settings", build(newDeviceBoundTestStore(), true, "a-legacy-key"), false},
	} {
		if got := tc.k.keyLeftOnOldDevice(id); got != tc.wants {
			t.Errorf("%s: keyLeftOnOldDevice = %v, want %v", tc.name, got, tc.wants)
		}
	}
}
