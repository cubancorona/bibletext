//go:build android

package bibletext

// The native→Go audio callbacks for Android — the twin of audio_export_apple.go
// (which is darwin-only). On Apple the C engine calls the //export functions
// directly; on Android the callbacks originate in Java (BtAudio), so they travel
// Java `native` method → JNI thunk (audio_jni_android.c) → these //export
// functions. A file with //export directives may hold only C *declarations* in its
// cgo preamble, and the thunks are *definitions*, so (as with reading, where the
// thunks live in reading_jni_android.c) they live in the separate .c file and this
// preamble stays empty.

import "C"

// State codes posted by BtAudio.java (BT_AUDIO_*), kept in sync with audioPlayState
// in audio_controller.go and the iOS enum in audio_ios.go. Redeclared here (not
// shared with the darwin-only cAudio* constants) because this file compiles only
// for Android.
const (
	aAudioIdle      = 0
	aAudioPlaying   = 1
	aAudioPaused    = 2
	aAudioEnded     = 3
	aAudioFailed    = 4
	aAudioBuffering = 5
)

// btAudioState is posted by BtAudio's MediaPlayer/TextToSpeech listeners and the
// AudioManager focus handler whenever playback state changes on its own — a
// chapter finished, a phone call took audio focus, or a completion/error landed.
// It arrives on the Android main thread (BtAudio hops its callbacks there); it maps
// the code and hands off to applyNativeState, which marshals the button refresh
// onto Fyne's goroutine via fyne.Do.
//
//export btAudioState
func btAudioState(code C.int) {
	var s audioPlayState
	switch int(code) {
	case aAudioPlaying:
		s = audioPlaying
	case aAudioPaused:
		s = audioPaused
	case aAudioEnded:
		s = audioEnded
	case aAudioFailed:
		s = audioFailed
	case aAudioBuffering:
		s = audioBuffering
	default:
		s = audioIdle
	}
	gAudio.applyNativeState(s)
}

// btAudioTime is posted by BtAudio's ~5×/sec MediaPlayer position poll with the
// current playback position in seconds, driving recorded read-along verse
// highlighting. onTimeUpdate only touches the text view when the narrated verse
// changes.
//
//export btAudioTime
func btAudioTime(seconds C.double) {
	gAudio.onTimeUpdate(float64(seconds))
}

// btAudioRange is posted by BtAudio's TextToSpeech UtteranceProgressListener
// (onRangeStart, API 26+) with the GLOBAL UTF-16 offset of the text about to be
// spoken — BtAudio adds the per-chunk base offset so the value indexes into the
// whole chapter's speech text, matching speechVerseOffsets. The TTS twin of the
// position poll, driving read-aloud read-along.
//
//export btAudioRange
func btAudioRange(location C.int) {
	gAudio.onSpeechRange(int(location))
}
