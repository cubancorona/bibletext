//go:build darwin

package bibletext

// audioSupported reports whether per-chapter audio (play button + engine) is
// available. It's wired on the Apple platforms — iOS (audio_ios.go) and macOS
// desktop (audio_macos.go). Everywhere else it's false (audio_supported_other.go),
// so the reading header shows no audio control.
func audioSupported() bool { return true }
