//go:build !darwin && !android

package bibletext

// audioSupported is false on the desktop hosts with no native audio engine
// (Linux, Windows), so the reading header omits the play button entirely rather
// than show a dead control. (iOS + macOS get true via audio_supported_apple.go;
// Android via audio_supported_android.go.)
func audioSupported() bool { return false }
