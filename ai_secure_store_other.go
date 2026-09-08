//go:build !darwin && !android

package bibletext

// Windows and Linux have no credential-store adapter yet, so keys stay in
// fyne.Preferences there. Android has one now (ai_secure_store_android.go), and
// Apple platforms have the Keychain, so this is the desktop-without-a-store
// case. nil is the documented way to say so: the keystore
// keeps using Preferences and keyInSecureStore reports false, which is what
// makes the Settings sheet say "Saved on this device" instead of naming a store
// that is not holding anything.
func newPlatformSecretStore() secretStore { return nil }
