package bibletext

// On-device storage for the user's AI keys and provider choice. Backed by Fyne
// Preferences (per-app, persisted across launches). Wrapped behind prefStore so
// tests can substitute an in-memory map.
//
// NOTE: Fyne Preferences are NOT encrypted (iOS UserDefaults / a desktop config
// file). That's acceptable for the user's own key on their own device, but
// hardening to the platform Keychain is a planned follow-up — keep all key I/O
// going through this type so that swap stays localized.

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

type keyStore struct {
	prefs prefStore
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
		return &keyStore{prefs: app.Preferences()}
	}
	return &keyStore{}
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
	return strings.TrimSpace(k.prefs.String(prefKeyPrefix + id))
}

func (k *keyStore) setAPIKey(id, key string) {
	if k == nil || k.prefs == nil {
		return
	}
	k.prefs.SetString(prefKeyPrefix+id, strings.TrimSpace(key))
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
