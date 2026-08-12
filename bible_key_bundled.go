package bibletext

// A bundled API.Bible key — the project's own key, compiled into RELEASE
// builds so the NKJV works out of the box, with the reader still able to
// replace or remove it in Settings.
//
// WHY IT LIVES IN THE NORMAL KEY STORE. The bundled key is not a hidden
// fallback: on first run it is written into the same on-device store as every
// other key (Keychain on iOS, Preferences elsewhere), so Settings →
// Translations shows exactly what is in use, "Clear" really removes it, and
// pasting a personal key really replaces it. A reader who clears it is never
// re-seeded (prefBibleKeyCleared), so the removal sticks.
//
// WHAT IT IS NOT. A key inside a shipped binary is EXTRACTABLE — the
// obfuscation below defeats `strings`, not a determined reader, and nothing
// can. Two consequences the operator accepted deliberately:
//
//   - Quota. API.Bible's free Starter plan is 5,000 calls per MONTH for the
//     whole account. A first NKJV download is ~197 calls and each device
//     revalidates every 30 days (§11), so the bundled key supports on the
//     order of TWENTY-FIVE active devices before the month runs dry — and
//     when it does, every reader on the bundled key (including the operator)
//     gets fetch failures until the reset. This is a launch convenience with
//     a kill switch, not a scaling plan.
//   - The kill switch. Ship a build without the key (omit the embed step —
//     see scripts/embed-bible-key.sh) and new installs are BYOK again;
//     existing installs keep the copy already in their key store until they
//     clear it. Rotating the key instead — a new key in a new build —
//     replaces the old bundled copy in place, but never a reader's own key
//     and never one they cleared.
//
// Keys are never committed: bundledBibleKeyEnc is empty in the repo and in
// every developer build, and the release scripts generate the value from
// .env.local into a gitignored file for the duration of one build.

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

// bundledBibleKeyEnc is the obfuscated bundled key, set by the generated
// file the release scripts write (see scripts/embed-bible-key.sh). Empty
// here — and therefore in every build that does not run that step.
var bundledBibleKeyEnc string

// bundledKeyMask is the XOR mask used to keep the key out of `strings`
// output. Obfuscation, explicitly not encryption: the mask sits in the same
// binary. It exists so a release asset does not carry a harvestable
// plaintext credential, nothing more.
const bundledKeyMask = "bibletext-nkjv"

// bundledBibleKey returns the compiled-in API.Bible key, or "" when this
// build has none (all developer builds, and any release built without the
// embed step).
func bundledBibleKey() string {
	if bundledBibleKeyEnc == "" {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(bundledBibleKeyEnc)
	if err != nil {
		return ""
	}
	out := make([]byte, len(raw))
	for i, b := range raw {
		out[i] = b ^ bundledKeyMask[i%len(bundledKeyMask)]
	}
	return strings.TrimSpace(string(out))
}

// keyFingerprint identifies a key without storing it — used to tell "the
// copy of OUR bundled key that we seeded" apart from "a key the reader
// entered", so a rotated bundled key can replace the former and never the
// latter.
func keyFingerprint(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:8])
}

// seedBundledBibleKey installs the bundled key on first run, and updates it
// in place when a later build ships a rotated one. It never touches a key
// the reader entered, and never returns after the reader clears it.
func (k *keyStore) seedBundledBibleKey() {
	if k == nil || k.prefs == nil {
		return
	}
	key := bundledBibleKey()
	if key == "" {
		return
	}
	fp := keyFingerprint(key)
	seededFP := strings.TrimSpace(k.prefs.String(prefBibleKeySeeded))
	existing := k.bibleAPIKey()

	switch {
	case existing == "":
		// Nothing stored. Seed unless the reader removed it themselves.
		if strings.TrimSpace(k.prefs.String(prefBibleKeyCleared)) != "" {
			return
		}
	case seededFP != "" && keyFingerprint(existing) == seededFP && seededFP != fp:
		// What is stored is the app's OWN previously bundled key, untouched
		// by the reader, and the app now ships a different one — rotate it.
	default:
		return // the reader's own key, or the current bundled key already in place
	}

	if k.setBibleAPIKey(key) {
		k.prefs.SetString(prefBibleKeySeeded, fp)
	}
}

// usingBundledBibleKey reports whether the key currently in the store is the
// one that shipped with the app — so Settings can say so plainly rather than
// implying the reader supplied it.
func (k *keyStore) usingBundledBibleKey() bool {
	key := bundledBibleKey()
	if key == "" || k == nil {
		return false
	}
	return k.bibleAPIKey() == key
}

// noteBibleKeyCleared records that the reader emptied the field, so the
// bundled key is not re-seeded on the next launch. Re-entering any key
// cancels it: that is a reader who wants a key again.
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
