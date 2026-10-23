package bibletext

import (
	"regexp"
	"strings"
	"testing"
)

// readmeKeyStorage is README.md's paragraph on where an AI key is kept, white
// space collapsed.
func readmeKeyStorage(t *testing.T) string {
	t.Helper()
	readme := readRepoFile(t, "README.md")
	start := strings.Index(readme, "You supply your own AI-provider key per provider.")
	if start < 0 {
		t.Fatal("README.md no longer has its paragraph on where an AI key is kept")
	}
	para := readme[start:]
	if end := strings.Index(para, "\n\n"); end >= 0 {
		para = para[:end]
	}
	return strings.Join(strings.Fields(para), " ")
}

// README.md says where each platform keeps an AI key as the code keeps it:
// the Keychain on iPhone and iPad and in the Mac App Store edition, the one
// macOS build with a team identity (ai_secure_store_darwin.go); on Android a
// copy wrapped under an Android Keystore key in no_backup/, which Android's
// backup leaves out (ai_secure_store_android.go); and the preferences on
// Windows, Linux and the ad-hoc-signed direct download, which have no store
// (ai_secure_store_other.go, and the darwin gate). It also says what the
// preferences keep of an allowed key on every platform, not only on those
// that keep the key there: a fingerprint, never the key, that ends with the
// key or with None (aiAllowedRecord, keyStore.setAPIKey,
// keyStore.setAIEnabled). Mutation: Android or an Apple platform put back in
// the preferences' list, either store or the record left out, or the record
// read as kept only where the key is.
func TestTheREADMESaysWhereEachPlatformKeepsAnAIKey(t *testing.T) {
	para := readmeKeyStorage(t)
	for _, want := range []string{
		"in the Apple Keychain on iPhone and iPad and in the Mac App Store edition",
		"on Android encrypted under a key held in the Android Keystore, in a folder Android's backup leaves out",
		"in the app's local preferences store on the directly downloaded Mac build, Windows and Linux",
		"the app's preferences, on every platform, also keep a fingerprint of the key it was allowed with, never the key",
	} {
		if !strings.Contains(para, want) {
			t.Errorf("README.md's paragraph on AI keys does not say %q:\n%s", want, para)
		}
	}
	prefs := regexp.MustCompile(`preferences store on ([^—]*)—`).FindStringSubmatch(para)
	if prefs == nil {
		t.Fatalf("README.md's paragraph on AI keys names no platform that keeps them in the preferences:\n%s", para)
	}
	for _, store := range []string{"Android", "iPhone", "iPad", "iOS", "App Store"} {
		if strings.Contains(prefs[1], store) {
			t.Errorf("README.md puts %s's AI key in the preferences store (%q), where it keeps it in a credential store",
				store, strings.TrimSpace(prefs[1]))
		}
	}

	// CONTROL: the code still has the stores the paragraph names, so a change
	// there brings this test, and the paragraph, back to be read.
	if !strings.HasPrefix(readRepoFile(t, "ai_secure_store_other.go"), "//go:build !darwin && !android\n") {
		t.Error("ai_secure_store_other.go's build line changed: read README.md's paragraph on AI keys against it")
	}
	if !strings.Contains(readRepoFile(t, "ai_secure_store_android.go"), "no_backup/") {
		t.Error("ai_secure_store_android.go no longer keeps the key in no_backup/: read README.md's paragraph on AI keys against it")
	}
	if !strings.Contains(readRepoFile(t, "ai_secure_store_darwin.go"), "btAIKeychainUsable") {
		t.Error("ai_secure_store_darwin.go no longer gates the Mac's Keychain on the signature: read README.md's paragraph on AI keys against it")
	}
}

// The Android restore examples include a reinstall or factory reset of the
// same phone, not only a move to another phone. The field describes the
// definitive absence without assuming which restore or other event caused
// it; Apple can reach the same state with different backup behavior.
func TestTheVersionStatesSayARestoreWithoutTheKeyIsAReinstallToo(t *testing.T) {
	doc := readRepoFile(t, "docs/VERSION_STATES.md")
	rows := map[string]string{}
	for _, line := range strings.Split(doc, "\n") {
		switch {
		case strings.HasPrefix(line, "| the `ai.keysaved.<account>` preference |"):
			rows["the state variables' row for `ai.keysaved.<account>`"] = line
		case strings.HasPrefix(line, "| **M5 × M6 × M7** launch |"):
			rows["the launch walk's row"] = line
		}
	}
	if len(rows) != 2 {
		t.Fatalf("docs/VERSION_STATES.md: found %d of the two rows this test reads", len(rows))
	}
	for name, row := range rows {
		if !strings.Contains(row, "onto the same one after a reinstall or a factory reset") {
			t.Errorf("%s does not say a restore without the key is also a reinstall or a factory reset of the same device", name)
		}
	}
	for _, untrue := range []string{"the key stayed on the device it was added on",
		"its key left on the phone it was added on", "the key on the old phone"} {
		if strings.Contains(doc, untrue) {
			t.Errorf("docs/VERSION_STATES.md says %q, untrue of a reinstall", untrue)
		}
	}
	if keyNotRestoredLine != "Your saved key isn't available on this device. Paste it again here." {
		t.Error("the recovery field must describe the missing saved key without assigning a cause")
	}
}
