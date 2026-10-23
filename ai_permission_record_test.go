package bibletext

// What a provider's permission record holds (aiAllowedRecord,
// ai_permission.go): a fingerprint of the key it was given with, never the
// key.

import (
	"strings"
	"testing"
)

// No preference holds the key's text after Allow, and the record for one key
// is not the record for another, or for the same key with another provider.
func TestThePermissionRecordHoldsNoKey(t *testing.T) {
	const key = "synthetic-test-key-0123456789"
	prefs := newFakePrefs()
	ks := &keyStore{prefs: prefs, secrets: &fakeSecrets{m: map[string]string{}}}
	ks.setAPIKey(providerOpenAI, key)
	grantAIPermission(ks, providerOpenAI)
	rec := prefs.String(prefAIAllowedPrefix + providerOpenAI)
	if rec == "" {
		t.Fatal("Allow recorded nothing")
	}
	for k, v := range prefs.m {
		if strings.Contains(v, key) || strings.Contains(v, key[:12]) {
			t.Errorf("preference %q holds the key's text", k)
		}
	}
	if aiAllowedRecord(providerOpenAI, key) == aiAllowedRecord(providerOpenAI, key+"x") {
		t.Error("two keys share a record")
	}
	if aiAllowedRecord(providerOpenAI, key) == aiAllowedRecord(providerAnthropic, key) {
		t.Error("two providers share a record for one key")
	}
	if aiAllowedRecord(providerOpenAI, "  ") != "" || ks.aiAllowed(providerOpenAI, "") {
		t.Error("an empty key has a record or a permission")
	}
	if rec != aiAllowedRecord(providerOpenAI, key) || aiAllowedRecord(providerOpenAI, " "+key+" ") != rec {
		t.Error("the record is not the fingerprint of the trimmed key")
	}
	// A record in the form of an earlier build is no permission.
	prefs.SetString(prefAIAllowedPrefix+providerOpenAI, "1")
	if ks.aiAllowed(providerOpenAI, key) {
		t.Error("a record of another form holds a permission")
	}
}
