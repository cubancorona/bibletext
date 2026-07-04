//go:build !darwin && !android

package bibletext

// Read-along verse highlighting is implemented in the native reading overlays:
// the Apple ones (reading_macos.go / reading_ios.go, driven by the AVPlayer time
// observers) and the Android one (readalong_android.go, driving BtBridge's
// TextView). The plain-Fyne desktop hosts (Linux, Windows) have no recorded-audio
// engine (audio_other.go), so these are never reached — they exist only so the
// untagged audio_controller.go compiles everywhere.
func readAlongHighlight(verse int, follow bool) {}
func readAlongClear()                           {}
func readAlongFollowButton(show bool)           {}
