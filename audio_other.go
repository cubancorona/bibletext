//go:build !darwin && !android

package bibletext

// No-op native audio for the platforms with no native engine: Linux and Windows.
// The real engines live in audio_ios.go (iOS), audio_macos.go (macOS desktop) and
// audio_android.go (Android — MediaPlayer + TextToSpeech over JNI). These stubs
// keep `go build ./...` and `go test ./...` green on the desktop hosts (and
// cgo-free), and audioSupported() (audio_supported_other.go) hides the dead button
// there so nothing tappable appears.

func nativeAudioStartURL(url, title, artist string)  {}
func nativeAudioStartTTS(text, title, artist string) {}
func nativeAudioToggle()                             {}
func nativeAudioStop()                               {}
func nativeAudioSkip(seconds float64)                {}
func nativeAudioSetArtwork(path string)              {}
