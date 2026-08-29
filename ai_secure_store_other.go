//go:build !darwin

package bibletext

// Windows, Linux and Android have no credential-store adapter yet, so keys stay
// in fyne.Preferences there. nil is the documented way to say so: the keystore
// keeps using Preferences and keyInSecureStore reports false, which is what
// makes the Settings sheet say "Saved on this device" instead of naming a store
// that is not holding anything.
func newPlatformSecretStore() secretStore { return nil }
