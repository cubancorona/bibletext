//go:build !darwin

package bibletext

// Read-along verse highlighting is implemented in the native Apple reading
// overlays (reading_macos.go / reading_ios.go, driven by the AVPlayer time
// observers in audio_macos.go / audio_ios.go). The plain-Fyne platforms have no
// recorded-audio engine (audio_other.go), so these are never reached — they
// exist only so the untagged audio_controller.go compiles everywhere.
func readAlongHighlight(verse int) {}
func readAlongClear()              {}
