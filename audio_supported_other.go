//go:build !darwin && !android

package bibletext

import "runtime"

// audioSupported: Windows and Linux now have a real recorded-narration engine
// (audio_other.go — oto + go-mp3), so the play button appears wherever the
// chapter has a recording. js/wasm is a compile-check target only — no audio.
func audioSupported() bool { return runtime.GOOS != "js" }

// ttsSupportedOnPlatform: no read-aloud engine on these hosts (no
// cross-platform TTS worth shipping). Gates the "Read aloud" source row and,
// via chapterAudioAvailable, hides the audio button entirely for chapters that
// have no recording (licensed versions, the deuterocanon). Read through the
// ttsSupported var seam (audio.go).
const ttsSupportedOnPlatform = false
