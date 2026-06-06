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
	prefKeyPrefix      = "ai.key."
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
