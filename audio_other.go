//go:build !darwin

package bibletext

// No-op native audio for the non-Apple platforms. The real engine (AVPlayer /
// AVSpeechSynthesizer + Now Playing + remote commands) lives in audio_ios.go
// (iOS) and audio_macos.go (macOS desktop). Linux, Windows and Android ship
// without it: these stubs keep `go build ./...` and `go test ./...` green for
// those build tags (and cgo-free), and audioSupported() (audio_supported_other.go)
// hides the dead button there so nothing tappable appears.

func nativeAudioStartURL(url, title, artist string)  {}
func nativeAudioStartTTS(text, title, artist string) {}
func nativeAudioToggle()                             {}
func nativeAudioStop()                               {}
func nativeAudioSkip(seconds float64)                {}
func nativeAudioSetArtwork(path string)              {}
