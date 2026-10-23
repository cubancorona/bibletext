//go:build darwin

package bibletext

import "testing"

// macMissingKeyTestStore uses the actual Apple capability answer with
// in-memory reads and writes, so the regression never touches a Keychain.
type macMissingKeyTestStore struct{ *fakeSecrets }

func (*macMissingKeyTestStore) restoreCanLeaveKeys() bool {
	return (appleKeychainStore{}).restoreCanLeaveKeys()
}

func (s *macMissingKeyTestStore) writeThisDeviceOnly(account, value string) bool {
	return s.Write(account, value)
}

// A signed Mac uses the same saved-mark and definitive absence checks as
// iOS. A missing key can follow a container-only restore or a reset
// keychain; neither cause needs to be inferred to show the neutral line.
func TestTheMacKeychainReportsDefinitivelyMissingSavedKeys(t *testing.T) {
	for _, id := range []string{bibleKeyID, defaultProviderID} {
		for _, tc := range []struct {
			name, saved, key, legacy string
			readError, want          bool
		}{
			{name: "saved key missing", saved: "1", want: true},
			{name: "key still present", saved: "1", key: "synthetic-key"},
			{name: "store error", saved: "1", readError: true},
			{name: "legacy copy", saved: "1", legacy: "synthetic-legacy-key"},
			{name: "no saved mark"},
		} {
			t.Run(id+"/"+tc.name, func(t *testing.T) {
				secrets := &macMissingKeyTestStore{&fakeSecrets{
					m: map[string]string{}, failRead: tc.readError,
				}}
				if tc.key != "" {
					secrets.m[id] = tc.key
				}
				ks := &keyStore{prefs: newFakePrefs(), secrets: secrets}
				ks.prefs.SetString(prefKeySavedPrefix+id, tc.saved)
				ks.prefs.SetString(prefKeyPrefix+id, tc.legacy)
				if got := ks.keyNotRestoredShown(id); got != tc.want {
					t.Fatalf("missing-key line shown = %v, want %v", got, tc.want)
				}
			})
		}
	}
}
