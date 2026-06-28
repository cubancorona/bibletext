//go:build !ios

package bibletext

// No-op native audio for every platform except iOS. The audio engine
// (AVPlayer / AVSpeechSynthesizer + AVAudioSession + Now Playing + remote
// commands) is iOS-only in this build — that's where per-chapter audio is used
// and tested. macOS desktop, Linux, Windows and Android ship without it: these
// stubs keep `go build ./...` and `go test ./...` green for every build tag (and
// cgo-free on the host), and audioSupported() (audio_supported_other.go) hides
// the dead button off iOS so nothing tappable appears.

func nativeAudioStartURL(url, title, artist string)  {}
func nativeAudioStartTTS(text, title, artist string) {}
func nativeAudioToggle()                             {}
func nativeAudioStop()                               {}
func nativeAudioSkip(seconds float64)                {}
