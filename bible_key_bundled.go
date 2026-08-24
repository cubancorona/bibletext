package bibletext

// Release-key support. Normal source and developer builds are keyless; release
// scripts inject an obfuscated value directly at link time without generating
// a credential-bearing source file in the repository.
//
// The release key is decoded only when needed and is never copied into
// Preferences or the platform credential store. A reader-owned stored key takes
// precedence. Settings can suppress the release fallback permanently with
// prefBibleKeyCleared, without persisting the project credential itself.
//
// A key inside a shipped binary is extractable. The obfuscation below only
// prevents an accidental plaintext `strings` match; it is not a security
// boundary. Releases that predate the runtime fallback may already have copied
// the project key into on-device storage. migrateBundledBibleKey removes only a
// value whose saved fingerprint proves that it was app-seeded; reader-owned
// values survive.
//
// bundledBibleKeyEnc remains empty in source. Release tooling obtains the key
// from a dedicated external source and supplies this variable with a linker
// `-X` flag; no credential-bearing source file is written under the repo.

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

// bundledBibleKeyEnc is set only by the final release link command. It remains
// empty in source and in builds outside the keyed release pipeline.
var bundledBibleKeyEnc string

// bundledKeyMask is the XOR mask used to keep the key out of `strings`
// output. Obfuscation, explicitly not encryption: the mask sits in the same
// binary. It exists so a release asset does not carry a harvestable
// plaintext credential, nothing more.
const bundledKeyMask = "bibletext-nkjv"

// bundledBibleKey returns the compiled-in API.Bible key, or "" when this
// build has none.
func bundledBibleKey() string {
	if bundledBibleKeyEnc == "" {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(bundledBibleKeyEnc)
	if err != nil {
		return ""
	}
	defer clear(raw)
	out := make([]byte, len(raw))
	for i, b := range raw {
		out[i] = b ^ bundledKeyMask[i%len(bundledKeyMask)]
	}
	key := strings.TrimSpace(string(out))
	clear(out)
	return key
}

// keyFingerprint identifies a key without storing it. Older releases saved this
// short digest beside a raw app-seeded copy; migration uses it to distinguish
// that legacy copy from a reader-owned credential.
func keyFingerprint(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:8])
}

// migrateBundledBibleKey removes the raw project key written to on-device
// storage by older releases. prefBibleKeySeeded is provenance from those
// releases: deletion is allowed only when a stored value has that fingerprint.
// A credential-store read error is not absence; in that case all state is left
// intact so a later effective read or launch can retry without misclassifying
// or losing a key.
func (k *keyStore) migrateBundledBibleKey() {
	if k == nil || k.prefs == nil {
		return
	}
	seededFP := strings.TrimSpace(k.prefs.String(prefBibleKeySeeded))
	if seededFP == "" {
		return
	}

	if k.secrets != nil {
		stored, found, ok := k.secrets.Read(bibleKeyID)
		if !ok {
			return
		}
		if found && keyFingerprint(stored) == seededFP {
			if !k.secrets.Write(bibleKeyID, "") {
				return
			}
		}
	}

	legacyPref := strings.TrimSpace(k.prefs.String(prefKeyPrefix + bibleKeyID))
	if legacyPref != "" && keyFingerprint(legacyPref) == seededFP {
		k.prefs.SetString(prefKeyPrefix+bibleKeyID, "")
	}

	// Any remaining value did not match the app-seeded fingerprint and therefore
	// belongs to the reader. The old provenance no longer describes stored state.
	k.prefs.SetString(prefBibleKeySeeded, "")
}

// usingBundledBibleKey reports provenance, not byte equality. A reader may
// deliberately store the same bytes as a bundled value; without legacy seeded
// provenance that remains a reader-owned key. Before migration completes, a
// fingerprint-matched old stored copy still correctly reports as app-bundled.
func (k *keyStore) usingBundledBibleKey() bool {
	if k == nil || k.prefs == nil {
		return false
	}
	k.migrateBundledBibleKey()
	if stored := k.apiKey(bibleKeyID); stored != "" {
		seededFP := strings.TrimSpace(k.prefs.String(prefBibleKeySeeded))
		return seededFP != "" && keyFingerprint(stored) == seededFP
	}
	return !k.bundledBibleKeyCleared() && bundledBibleKey() != ""
}

func (k *keyStore) bundledBibleKeyCleared() bool {
	return k == nil || k.prefs == nil || strings.TrimSpace(k.prefs.String(prefBibleKeyCleared)) != ""
}

// noteBibleKeyCleared records that the reader emptied the field, so the bundled
// fallback stays disabled on later launches. Re-entering any key cancels it:
// that is a reader who wants a key again.
func (k *keyStore) noteBibleKeyCleared(cleared bool) {
	if k == nil || k.prefs == nil {
		return
	}
	if cleared {
		k.prefs.SetString(prefBibleKeyCleared, "1")
		return
	}
	k.prefs.SetString(prefBibleKeyCleared, "")
}
