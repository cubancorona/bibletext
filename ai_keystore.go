package bibletext

// On-device storage for the user's AI keys and provider choice. Non-secret
// settings use Fyne Preferences (per-app, persisted across launches). API keys
// use the platform credential store where one is available — the Apple Keychain
// on iOS ONLY (macOS release builds are ad-hoc signed, so a keychain ACL would
// break on every update; see ai_secure_store_darwin.go) — with one-time
// migration from the legacy preference value.
// Wrapped behind small interfaces so tests can substitute in-memory stores.

import (
	"strings"

	"fyne.io/fyne/v2"
)

// prefStore is the slice of fyne.Preferences this package needs (also satisfied
// by a test fake).
type prefStore interface {
	String(key string) string
	StringWithFallback(key, fallback string) string
	SetString(key, value string)
}

// secretStore is implemented by the Apple Keychain adapter. Other platforms
// return nil from newPlatformSecretStore and retain the existing Preferences
// storage until they gain an equivalent native credential-store adapter.
type secretStore interface {
	// Read returns (value, found, ok). ok=false means the credential store
	// itself failed (transient Keychain error) — CALLERS MUST NOT treat that
	// as "no key": the legacy fallback below keeps working and the migration
	// erase is skipped, so nothing user-entered is ever stranded or lost.
	Read(account string) (value string, found, ok bool)
	// Write with an empty value deletes the item.
	Write(account, value string) bool
}

type keyStore struct {
	prefs   prefStore
	secrets secretStore
}

const (
	prefActiveProvider = "ai.activeProvider"
	// prefAIEnabled holds the Settings → Assistant choice of "None": "off" hides
	// every AI feature (the Study-with-AI selection menu, the Find passage search,
	// the key/model fields); "" or anything else means AI is available (the
	// default). Stored keys are kept while off, so re-choosing a provider restores
	// the previous setup untouched.
	prefAIEnabled = "ai.enabled"
	prefKeyPrefix = "ai.key."
	// prefModelOverridePrefix holds a user-pinned model per provider (blank =
	// use the recommended/self-healed model). prefModelResolvedPrefix caches the
	// model that self-heal discovered when the default was retired, so the swap is
	// remembered across launches. Both are per-provider (suffix = provider id).
	prefModelOverridePrefix = "ai.model.override."
	prefModelResolvedPrefix = "ai.model.resolved."
)

// newKeyStore binds to the running app's Preferences. Returns an inert store
// (no app yet / unit tests) whose getters yield defaults and setters no-op.
func newKeyStore() *keyStore {
	if app := fyne.CurrentApp(); app != nil {
		k := &keyStore{prefs: app.Preferences(), secrets: newPlatformSecretStore()}
		k.migrateAllKeys()
		return k
	}
	return &keyStore{}
}

// keyInSecureStore reports whether this provider's key is CURRENTLY held by the
// platform credential store — a definitive found-and-readable answer, not
// "this platform has a keychain". The Settings status uses it so the saved
// label describes where the key actually is: a keychain write can fail (an
// ad-hoc-signed build has no keychain entitlement, and a device store can be
// temporarily unreadable), in which case the key is still safely in
// Preferences and the UI must not claim otherwise.
func (k *keyStore) keyInSecureStore(id string) bool {
	if k == nil || k.secrets == nil {
		return false
	}
	_, found, ok := k.secrets.Read(id)
	return ok && found
}

// migrateAllKeys sweeps EVERY provider's pre-1.1.6 Preferences key into the
// credential store up front (the implementation requirement: the lazy per-provider migration
// left non-selected providers' keys in plaintext Preferences indefinitely).
// apiKey carries the same logic per read; this just runs it for each provider
// once at startup. Errors leave the legacy copy in place for the next try.
func (k *keyStore) migrateAllKeys() {
	if k == nil || k.prefs == nil || k.secrets == nil {
		return
	}
	for _, p := range aiProviders() {
		k.apiKey(p.ID)
	}
}

func newKeyStoreWith(p prefStore) *keyStore { return &keyStore{prefs: p} }

func (k *keyStore) activeProvider() string {
	if k == nil || k.prefs == nil {
		return defaultProviderID
	}
	id := strings.TrimSpace(k.prefs.StringWithFallback(prefActiveProvider, defaultProviderID))
	if _, ok := providerByID(id); !ok {
		return defaultProviderID
	}
	return id
}

func (k *keyStore) setActiveProvider(id string) {
	if k == nil || k.prefs == nil {
		return
	}
	if _, ok := providerByID(id); !ok {
		return
	}
	k.prefs.SetString(prefActiveProvider, id)
}

// aiEnabled reports whether AI features are available at all — false when the
// reader chose "None" in Settings → Assistant. Defaults to true.
func (k *keyStore) aiEnabled() bool {
	if k == nil || k.prefs == nil {
		return true
	}
	return k.prefs.StringWithFallback(prefAIEnabled, "") != "off"
}

func (k *keyStore) setAIEnabled(on bool) {
	if k == nil || k.prefs == nil {
		return
	}
	v := ""
	if !on {
		v = "off"
	}
	k.prefs.SetString(prefAIEnabled, v)
}

func (k *keyStore) apiKey(id string) string {
	if k == nil || k.prefs == nil {
		return ""
	}
	if k.secrets != nil {
		key, found, ok := k.secrets.Read(id)
		if ok && found {
			return strings.TrimSpace(key)
		}
		// Builds before 1.1.6 stored keys in Preferences. Move that value into
		// Keychain on first access, then erase the unencrypted legacy copy —
		// but ONLY on a definitive not-found (ok && !found): after a store
		// ERROR the item may exist and be temporarily unreadable, so neither
		// migrate nor erase; serve the legacy copy and try again next call.
		legacy := strings.TrimSpace(k.prefs.String(prefKeyPrefix + id))
		if ok && legacy != "" && k.secrets.Write(id, legacy) {
			k.prefs.SetString(prefKeyPrefix+id, "")
		}
		return legacy
	}
	return strings.TrimSpace(k.prefs.String(prefKeyPrefix + id))
}

// setAPIKey reports whether the value was saved. Callers that do not present a
// storage status may ignore the result; the Settings sheet uses it to avoid
// claiming that a failed Keychain write succeeded.
func (k *keyStore) setAPIKey(id, key string) bool {
	if k == nil || k.prefs == nil {
		return false
	}
	key = strings.TrimSpace(key)
	if k.secrets != nil {
		if k.secrets.Write(id, key) {
			// Remove any pre-1.1.6 preference copy after a successful secure write.
			k.prefs.SetString(prefKeyPrefix+id, "")
			return true
		}
		if key == "" {
			// A failed DELETE must be reported: the key is still stored, and
			// telling the reader it is gone would be a false assurance.
			return false
		}
		// A failed WRITE falls back to Preferences — the same place builds
		// before 1.1.6 kept keys. Refusing to store anything would leave a
		// reader whose credential store is unavailable (no keychain
		// entitlement, a device store that is broken or restricted) unable to
		// use their own key at all. migrateAllKeys retries the secure write on
		// every later launch, and the Settings status reports the REAL
		// location (keyInSecureStore), so this never claims to be a keychain.
		k.prefs.SetString(prefKeyPrefix+id, key)
		return true
	}
	k.prefs.SetString(prefKeyPrefix+id, key)
	return true
}

// overrideModel is the user-pinned model for a provider (blank = none).
func (k *keyStore) overrideModel(id string) string {
	if k == nil || k.prefs == nil {
		return ""
	}
	return strings.TrimSpace(k.prefs.String(prefModelOverridePrefix + id))
}

func (k *keyStore) setOverrideModel(id, model string) {
	if k == nil || k.prefs == nil {
		return
	}
	k.prefs.SetString(prefModelOverridePrefix+id, strings.TrimSpace(model))
}

// resolvedModel is the model self-heal discovered for a provider when the default
// was retired (blank = none discovered yet).
func (k *keyStore) resolvedModel(id string) string {
	if k == nil || k.prefs == nil {
		return ""
	}
	return strings.TrimSpace(k.prefs.String(prefModelResolvedPrefix + id))
}

func (k *keyStore) setResolvedModel(id, model string) {
	if k == nil || k.prefs == nil {
		return
	}
	k.prefs.SetString(prefModelResolvedPrefix+id, strings.TrimSpace(model))
}
