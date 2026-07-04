//go:build android

package bibletext

// audioSupported is true on Android: the native engine (audio_android.go →
// BtAudio.java: MediaPlayer for recorded chapters, TextToSpeech for read-aloud,
// AudioManager audio focus) ships in the same classes2.dex as the reading
// overlay, so the reading header shows the play button and read-along works — the
// Android twin of the Apple engines. If the dex is somehow absent (a plain
// `fyne package` build that skipped scripts/build-android.sh), the runBtAudio
// JNI calls no-op safely, exactly like the reading overlay's runBta fallback.
func audioSupported() bool { return true }
