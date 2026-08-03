//go:build !darwin

package bibletext

func newPlatformSecretStore() secretStore { return nil }
