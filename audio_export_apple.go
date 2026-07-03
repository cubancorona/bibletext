//go:build darwin

package bibletext

// The single native→Go callback for audio playback state, shared by both Apple
// engines (audio_ios.go and audio_macos.go). It lives on its own because a file
// with an //export directive may have only C *declarations* in its cgo preamble,
// and those engine files' preambles are full of C *definitions* — so the export
// goes here (empty preamble) and the engines declare it `extern`.
// (Same split as ai_menu_darwin.go ↔ reading_ios.go.)

import "C"

// Codes posted by the native engines (BT_AUDIO_*), kept in sync with
// audioPlayState in audio_controller.go.
const (
	cAudioIdle    = 0
	cAudioPlaying = 1
	cAudioPaused  = 2
	cAudioEnded   = 3
	cAudioFailed  = 4
)

// bibleTextAudioStateChanged is posted by the AVPlayer/AVSpeechSynthesizer
// notification + delegate handlers and the Now Playing remote commands whenever
// playback state changes on its own. It runs on the native main thread; it maps
// the code and hands off to applyNativeState, which marshals the button refresh
// onto Fyne's goroutine via fyne.Do.
//
//export bibleTextAudioStateChanged
func bibleTextAudioStateChanged(code C.int) {
	var s audioPlayState
	switch int(code) {
	case cAudioPlaying:
		s = audioPlaying
	case cAudioPaused:
		s = audioPaused
	case cAudioEnded:
		s = audioEnded
	case cAudioFailed:
		s = audioFailed
	default:
		s = audioIdle
	}
	gAudio.applyNativeState(s)
}

// bibleTextAudioTimeUpdate is posted by the recorded player's periodic time observer
// (~5×/sec) with the current playback position in seconds, driving read-along verse
// highlighting. Runs on the native main thread; onTimeUpdate only touches the text
// view when the narrated verse changes.
//
//export bibleTextAudioTimeUpdate
func bibleTextAudioTimeUpdate(seconds C.double) {
	gAudio.onTimeUpdate(float64(seconds))
}

// bibleTextAudioSpeechRange is posted by the speech synthesizer's
// willSpeakRangeOfSpeechString delegate (both engines) with the UTF-16 offset of
// the utterance text about to be spoken — the TTS twin of the time observer,
// driving read-along verse highlighting for read-aloud chapters.
//
//export bibleTextAudioSpeechRange
func bibleTextAudioSpeechRange(location C.int) {
	gAudio.onSpeechRange(int(location))
}
