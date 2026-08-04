//go:build !ios

package bibletext

func newPlatformSecretStore() secretStore { return nil }
