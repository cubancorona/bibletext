package bibletext

// On-device storage for the user's AI keys and provider choice. Non-secret
// settings use Fyne Preferences (per-app, persisted across launches). API keys
// use the platform credential store where one is available, including the
// Apple Keychain on iOS and stably signed Mac builds, with one-time migration
// from the legacy preference value. The ad-hoc-signed Mac direct download
// retains Preferences storage (ai_secure_store_darwin.go).
// Wrapped behind small interfaces so tests can substitute in-memory stores.

import (
	"crypto/sha256"
	"encoding/hex"
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
	// Name is what this store is called where a reader can see it. The Settings
	// sheet tells them where their key actually went, and "Keychain" is only
	// the right word on Apple platforms — a Windows Credential Manager entry
	// described as a Keychain is a small lie in the one place the app is
	// explaining where a credential lives.
	Name() string
	// Read returns (value, found, ok). ok=false means the credential store
	// itself failed (transient Keychain error) — CALLERS MUST NOT treat that
	// as "no key": the legacy fallback below keeps working and the migration
	// erase is skipped, so nothing user-entered is ever stranded or lost.
	Read(account string) (value string, found, ok bool)
	// Write with an empty value deletes the item.
	Write(account, value string) bool
}

// keysLeftByRestore identifies a credential store whose keys can be absent
// while a saved mark remains in Preferences. This can follow a restore, but
// the app does not detect one: the missing-key line states only the absence
// established by keyLeftOnOldDevice. Android and the Apple Keychain qualify.
// Windows, Linux and the Mac's direct download keep keys in Preferences,
// alongside the settings, and have no separate store to check.
type keysLeftByRestore interface {
	// restoreCanLeaveKeys reports that this store is one.
	restoreCanLeaveKeys() bool
	// writeThisDeviceOnly keeps value under account where no backup or
	// transfer carries it to another device; an empty value deletes it. It is
	// for a record that holds no part of a key and must not outlive a restore
	// onto another device (dismissKeyNotRestored).
	writeThisDeviceOnly(account, value string) bool
}

type keyStore struct {
	prefs   prefStore
	secrets secretStore
	// notRestoredDismissed holds, per account, a ✕ on keyNotRestoredLine
	// tapped in this run, so the line stays dismissed for the rest of it
	// where the store could not keep the record (dismissKeyNotRestored).
	// Touched only from the UI thread, by the Settings sheet.
	notRestoredDismissed map[string]bool
}

const (
	prefActiveProvider = "ai.activeProvider"
	// prefAIEnabled holds the Settings → Assistant choice of "None": "off" hides
	// every AI feature (the Study-with-AI selection menu, the Find passage search,
	// the key/model fields); "" or anything else means AI is available (the
	// default). Stored keys are kept while off, so re-choosing a provider restores
	// the previous setup untouched.
	prefAIEnabled = "ai.enabled"
	// prefAIAllowedPrefix records, per provider (suffix = provider id), that
	// the reader allowed Study with AI to send their text to that provider
	// with the key it had then (ai_permission.go). It holds a fingerprint of
	// that key, never the key (aiAllowedRecord). Any change of the
	// provider's key, or choosing "None", ends it, and the app asks again
	// before the next request.
	prefAIAllowedPrefix = "ai.allowed."
	prefKeyPrefix       = "ai.key."
	// prefModelOverridePrefix holds a user-pinned model per provider (blank =
	// use the recommended/self-healed model). prefModelResolvedPrefix caches the
	// model that self-heal discovered when the default was retired, so the swap is
	// remembered across launches. Both are per-provider (suffix = provider id).
	prefModelOverridePrefix = "ai.model.override."
	prefModelResolvedPrefix = "ai.model.resolved."
	// prefModelResolvedForPrefix records WHICH default a cached self-heal was
	// standing in for. When the app ships a new recommended model the old cache
	// is no longer relevant — it was our fallback, never the reader's choice —
	// so it must not outrank the new recommendation. See resolvedModel.
	prefModelResolvedForPrefix = "ai.model.resolvedfor."
	// prefBibleKeySeeded is legacy provenance: older releases stored the
	// fingerprint of the project API.Bible key they copied on-device. Current
	// releases use it only to remove that old app-owned copy safely.
	// prefBibleKeyCleared records that the reader deliberately disabled the
	// compiled runtime fallback.
	prefBibleKeySeeded  = "bible.key.seeded"
	prefBibleKeyCleared = "bible.key.cleared"
	// prefKeySavedPrefix + account is "1" while a key the reader entered is
	// saved for that account in the platform credential store, and empty once
	// they clear it. It holds no part of the key. It lives in Preferences,
	// which back up with the reader's settings, while the key itself may not
	// (keysLeftByRestore): it is how a restored device knows a key was there.
	prefKeySavedPrefix = "ai.keysaved."
)

// newKeyStore binds to the running app's Preferences. Returns an inert store
// (no app yet / unit tests) whose getters yield defaults and setters no-op.
func newKeyStore() *keyStore {
	if app := fyne.CurrentApp(); app != nil {
		k := &keyStore{prefs: app.Preferences(), secrets: newPlatformSecretStore()}
		// Releases before the runtime-fallback design copied the project's
		// bundled API.Bible key into the on-device credential store. Remove only
		// a copy whose saved fingerprint proves that the app seeded it; a
		// reader-owned key is never changed. This must precede generic key
		// migration so an old app-owned Preferences copy is not first copied into
		// the platform credential store.
		k.migrateBundledBibleKey()
		k.migrateAllKeys()
		return k
	}
	return &keyStore{}
}

// bibleKeyID is the credential-store account for the reader's own API.Bible
// key (BYOK for licensed translations). It rides the same storage as the AI
// keys: Keychain on iOS, Preferences elsewhere, env var overriding both.
const bibleKeyID = "apibible"

func (k *keyStore) bibleAPIKey() string {
	// A credential-store failure can defer legacy cleanup at startup. Retry
	// before every effective Bible-key read while provenance remains, so a
	// recovered store cannot cause apiKey to migrate an old app-owned copy.
	k.migrateBundledBibleKey()
	if key := k.apiKey(bibleKeyID); key != "" {
		return key
	}
	// A release key is a runtime fallback, not a stored credential. Requiring
	// Preferences here is intentional: before the app has bound its real store
	// we cannot know whether the reader previously cleared the bundled key.
	if k == nil || k.prefs == nil || k.bundledBibleKeyCleared() {
		return ""
	}
	return bundledBibleKey()
}

func (k *keyStore) setBibleAPIKey(v string) bool {
	v = strings.TrimSpace(v)
	if !k.setAPIKey(bibleKeyID, v) {
		return false
	}
	if v != "" && k != nil && k.prefs != nil &&
		(k.secrets == nil || strings.TrimSpace(k.prefs.String(prefKeyPrefix+bibleKeyID)) == "") {
		// A successful non-empty write is a reader action. Clear any stale
		// provenance left by an older app-seeded copy, even when the bytes happen
		// to equal the bundled key. If the credential-store write failed and
		// setAPIKey fell back to Preferences, retain the provenance: an older
		// app-seeded secure copy may still need fingerprint-gated removal.
		k.prefs.SetString(prefBibleKeySeeded, "")
	}
	return true
}

// sharedKeys serves callers without an AppState (the version registry decides
// availability from the effective Bible key). Bound to the app's Preferences on
// first use once an app exists; inert before that. A test hook: overridable.
var sharedKeys = func() *keyStore {
	if sharedKeyStore == nil && fyne.CurrentApp() != nil {
		sharedKeyStore = newKeyStore()
	}
	if sharedKeyStore == nil {
		return &keyStore{}
	}
	return sharedKeyStore
}

var sharedKeyStore *keyStore

// keyInSecureStore reports whether this provider's key is CURRENTLY held by the
// platform credential store — a definitive found-and-readable answer, not
// "this platform has a keychain". The Settings status uses it so the saved
// label describes where the key actually is: a keychain write can fail (an
// ad-hoc-signed build has no keychain entitlement, and a device store can be
// temporarily unreadable), in which case the key is still safely in
// Preferences and the UI must not claim otherwise.
// secureStoreName names the credential store this build would use, for the
// Settings sheet to say where a key went. Empty when there is none, which is
// the platforms that have no adapter and any build whose signature does not
// qualify for one.
func (k *keyStore) secureStoreName() string {
	if k == nil || k.secrets == nil {
		return ""
	}
	return k.secrets.Name()
}

func (k *keyStore) keyInSecureStore(id string) bool {
	if k == nil || k.secrets == nil {
		return false
	}
	_, found, ok := k.secrets.Read(id)
	return ok && found
}

// bibleKeyKnownAbsent reports a DEFINITIVE negative: there is no effective
// Bible key and the app was able to establish that, rather than merely
// failing to find one.
//
// The distinction is the whole point. Read returns (value, found, ok), and
// ok=false means the credential store itself failed — before the first unlock
// after a reboot, or on any error other than item-not-found. Every consumer
// that merely READS a key already honours that (apiKey serves the legacy copy
// and declines to migrate or erase); this exists for the consumers that ACT
// on the answer irreversibly, where "we could not tell" must never be
// actioned as "there is none". See D1 in docs/VERSION_STATES.md.
func (k *keyStore) bibleKeyKnownAbsent() bool {
	if k == nil || k.prefs == nil {
		return false // nothing bound yet: we cannot tell
	}
	if k.secrets != nil {
		if _, _, ok := k.secrets.Read(bibleKeyID); !ok {
			return false // the store failed — no conclusion is available
		}
	}
	return k.bibleAPIKey() == ""
}

// keyLeftOnOldDevice reports a saved key that is definitively absent: the
// settings retain its saved mark, the separate credential store answers
// item-not-found, and no copy remains in Preferences. Its historical name
// does not establish a restore or a different device. A store error cannot
// establish absence and must not show the line (D1).
func (k *keyStore) keyLeftOnOldDevice(id string) bool {
	if k == nil || k.prefs == nil || k.secrets == nil {
		return false
	}
	if left, ok := k.secrets.(keysLeftByRestore); !ok || !left.restoreCanLeaveKeys() {
		return false
	}
	if strings.TrimSpace(k.prefs.String(prefKeySavedPrefix+id)) == "" {
		return false
	}
	if _, found, ok := k.secrets.Read(id); !ok || found {
		return false
	}
	return strings.TrimSpace(k.prefs.String(prefKeyPrefix+id)) == ""
}

// keyNotRestoredDismissedSuffix names, appended to an account, the store item
// that records the reader's ✕ on that field's keyNotRestoredLine. It holds no
// part of a key.
const keyNotRestoredDismissedSuffix = ".notrestored.dismissed"

// keyNotRestoredShown reports that the account's empty field says
// keyNotRestoredLine: its saved key is definitively absent and the reader
// has not dismissed the line on this device.
func (k *keyStore) keyNotRestoredShown(id string) bool {
	return k.keyLeftOnOldDevice(id) && !k.keyNotRestoredDismissed(id)
}

// keyNotRestoredDismissed reports a device-scoped dismissal for this account.
// The record is separate from the key and holds no part of it. Android's
// no_backup store loses it after reinstall or reset. Apple's
// ThisDeviceOnly item can return to the same device from its own backup,
// preserving the dismissal, but does not migrate to another device. A store
// that cannot answer has no record to give, and the line is said.
func (k *keyStore) keyNotRestoredDismissed(id string) bool {
	if k == nil {
		return false
	}
	if k.notRestoredDismissed[id] {
		return true
	}
	if k.secrets == nil {
		return false
	}
	_, found, ok := k.secrets.Read(id + keyNotRestoredDismissedSuffix)
	return ok && found
}

// dismissKeyNotRestored records the reader's ✕ for this account without
// carrying that choice to another device (writeThisDeviceOnly). A write the
// store refuses still dismisses the line for the rest of this run; the next
// launch says it again.
func (k *keyStore) dismissKeyNotRestored(id string) {
	if k == nil {
		return
	}
	if k.notRestoredDismissed == nil {
		k.notRestoredDismissed = map[string]bool{}
	}
	k.notRestoredDismissed[id] = true
	if left, ok := k.secrets.(keysLeftByRestore); ok && left.restoreCanLeaveKeys() {
		left.writeThisDeviceOnly(id+keyNotRestoredDismissedSuffix, "1")
	}
}

// setKeySavedMark keeps the account's saved mark in step with the credential
// store: written only when it changes, so a read path that confirms it costs
// no Preferences write.
func (k *keyStore) setKeySavedMark(id string, saved bool) {
	if k == nil || k.prefs == nil {
		return
	}
	v := ""
	if saved {
		v = "1"
	}
	if k.prefs.String(prefKeySavedPrefix+id) != v {
		k.prefs.SetString(prefKeySavedPrefix+id, v)
	}
}

// markKeySaved records that a key is saved for the account, found there
// rather than just entered. An API.Bible key that an older release copied
// on-device (prefBibleKeySeeded) was never the reader's, so it is not recorded
// while that provenance stands: migrateBundledBibleKey removes it.
func (k *keyStore) markKeySaved(id string) {
	if k == nil || k.prefs == nil {
		return
	}
	if id == bibleKeyID && strings.TrimSpace(k.prefs.String(prefBibleKeySeeded)) != "" {
		return
	}
	k.setKeySavedMark(id, true)
}

// migrateAllKeys sweeps EVERY provider's pre-1.1.6 Preferences key into the
// credential store up front. Lazy per-provider migration would leave
// non-selected providers' keys in plaintext Preferences indefinitely.
// apiKey carries the same logic per read; this just runs it for each provider
// once at startup. Errors leave the legacy copy in place for the next try.
//
// The same read records each key it finds as saved (markKeySaved), so a key
// entered before the mark existed is recorded at the first launch that reads
// it, and a backup taken after that carries the mark.
func (k *keyStore) migrateAllKeys() {
	if k == nil || k.prefs == nil || k.secrets == nil {
		return
	}
	var ids []string
	for _, p := range aiProviders() {
		ids = append(ids, p.ID)
	}
	ids = append(ids, bibleKeyID) // the Bible key migrates the same way
	for _, id := range ids {
		if k.apiKey(id) != "" {
			k.markKeySaved(id)
		}
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

// setAIEnabled(false) is the reader choosing "None", which also withdraws
// every provider's permission: choosing a provider again asks again before
// its first request.
func (k *keyStore) setAIEnabled(on bool) {
	if k == nil || k.prefs == nil {
		return
	}
	v := ""
	if !on {
		v = "off"
		for _, p := range aiProviders() {
			k.clearAIAllowed(p.ID)
		}
	}
	k.prefs.SetString(prefAIEnabled, v)
}

// aiAllowedForm begins every permission record. A record holding anything
// else is no permission, so a later wording of the question can ask everyone
// again by changing it.
const aiAllowedForm = "key:"

// aiAllowedRecord is what provider id's permission record holds once the
// reader has allowed it with key: a fingerprint of the key, never the key,
// and empty for no key, since nothing can be sent without one. The permission
// ends with its key. The fingerprint is checked against the key each request
// would use, so the record is no permission beside any other key, or none: a
// key replaced outside the app, or preferences restored on Android, which
// back up, without the key, which does not (ai_secure_store_android.go).
// Eight bytes of SHA-256 tell a reader's keys apart and give back nothing of
// the key.
func aiAllowedRecord(id, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("bibletext ai permission\x00" + id + "\x00" + key))
	return aiAllowedForm + hex.EncodeToString(sum[:8])
}

// aiAllowed reports whether the reader has allowed their text to go to
// provider id with key, the key the request would be sent with. False for a
// store with nothing bound, where no permission can have been recorded, and
// for an empty key.
func (k *keyStore) aiAllowed(id, key string) bool {
	if k == nil || k.prefs == nil {
		return false
	}
	want := aiAllowedRecord(id, key)
	return want != "" && k.prefs.String(prefAIAllowedPrefix+id) == want
}

// setAIAllowed records that the reader allowed provider id with key. Only
// the AI providers have a permission; the API.Bible key, which shares the key
// store, does not. With no key there is nothing to allow, and any record is
// erased.
func (k *keyStore) setAIAllowed(id, key string) {
	if k == nil || k.prefs == nil {
		return
	}
	if _, ok := providerByID(id); !ok {
		return
	}
	rec := aiAllowedRecord(id, key)
	if rec == "" {
		k.clearAIAllowed(id)
		return
	}
	k.prefs.SetString(prefAIAllowedPrefix+id, rec)
}

// clearAIAllowed erases provider id's permission, writing only when there is
// one to erase.
func (k *keyStore) clearAIAllowed(id string) {
	if k == nil || k.prefs == nil {
		return
	}
	if _, ok := providerByID(id); !ok {
		return
	}
	if k.prefs.String(prefAIAllowedPrefix+id) != "" {
		k.prefs.SetString(prefAIAllowedPrefix+id, "")
	}
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
			k.markKeySaved(id)
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
	if _, ok := providerByID(id); ok && (key == "" || key != k.apiKey(id)) {
		// Any change of a provider's key ends the reader's permission for
		// that provider, whether or not the store manages the write: a key
		// cleared, replaced by another, or entered where there was none, as
		// after an Android restore that brought the preferences back without
		// the key. A clear ends it even where the store fails the delete,
		// since the reader asked to be done with it. Saving the same key
		// again keeps it. The app asks again before the next request
		// (ai_permission.go).
		k.clearAIAllowed(id)
	}
	if k.secrets != nil {
		if k.secrets.Write(id, key) {
			// Remove any pre-1.1.6 preference copy after a successful secure write.
			k.prefs.SetString(prefKeyPrefix+id, "")
			// The reader saved or cleared a key: the mark says which
			// (keyLeftOnOldDevice).
			k.setKeySavedMark(id, key != "")
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
		k.setKeySavedMark(id, true)
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

// resolvedModel is the model self-heal discovered for a provider when the
// default was retired (blank = none discovered yet).
//
// A cache is honoured only while it is still standing in for the SAME default
// it healed from. Upgrading the app's recommended model discards it: a reader
// on "Recommended" never chose that fallback — the app picked it when the old
// default died — so it must not silently pin them to yesterday's model. An
// explicit choice in Settings (overrideModel) is untouched by any of this.
func (k *keyStore) resolvedModel(id string) string {
	if k == nil || k.prefs == nil {
		return ""
	}
	cached := strings.TrimSpace(k.prefs.String(prefModelResolvedPrefix + id))
	if cached == "" {
		return ""
	}
	info, ok := providerByID(id)
	if !ok {
		return cached
	}
	if strings.TrimSpace(k.prefs.String(prefModelResolvedForPrefix+id)) != info.Model {
		return "" // healed from a default we no longer ship — re-resolve
	}
	return cached
}

func (k *keyStore) setResolvedModel(id, model string) {
	if k == nil || k.prefs == nil {
		return
	}
	k.prefs.SetString(prefModelResolvedPrefix+id, strings.TrimSpace(model))
	if info, ok := providerByID(id); ok {
		k.prefs.SetString(prefModelResolvedForPrefix+id, info.Model)
	}
}
