//go:build darwin && !ios

package bibletext

import "testing"

// TestMacAudio verifies the macOS build wires up the native audio engine. The
// reading-header narration / read-aloud control is shown only where
// audioSupported() is true, and on macOS that capability comes from the
// AVFoundation engine in audio_macos.go — so this pins the platform gate the
// feature depends on. (The tag matches audio_macos.go so it never compiles into
// the non-Apple builds.)
func TestMacAudio(t *testing.T) {
	if !audioSupported() {
		t.Fatal("audioSupported() must be true on macOS — the reading-header audio control depends on it")
	}
}
