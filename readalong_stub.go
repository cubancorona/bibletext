//go:build ios || !darwin

package bibletext

// Read-along verse highlighting is implemented in the native macOS reading overlay
// (reading_macos.go, via the AVPlayer time observer in audio_macos.go). iOS — which
// has its own native UITextView overlay — and the plain-Fyne platforms get no-op
// stubs for now: recorded audio still plays; wiring the highlight into the iOS
// overlay + the TTS word callback is the next phase.
func readAlongHighlight(verse int) {}
func readAlongClear()              {}
