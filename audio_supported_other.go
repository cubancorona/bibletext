//go:build !darwin

package bibletext

// audioSupported is false off the Apple platforms — no native audio engine, so
// the reading header omits the play button entirely rather than show a dead
// control. (iOS + macOS get true via audio_supported_apple.go.)
func audioSupported() bool { return false }
