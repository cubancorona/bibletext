package bibletext

import "testing"

func TestChapterAudioFingerprint(t *testing.T) {
	got := chapterAudioFingerprint(&AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 20})
	if got != "web|John|20" {
		t.Fatalf("fingerprint = %q, want web|John|20", got)
	}
	if got := chapterAudioFingerprint(nil); got != "" {
		t.Fatalf("nil fingerprint = %q, want empty", got)
	}
}

func TestAudioControllerStop(t *testing.T) {
	c := &audioController{loaded: true, loadedFP: "web|John|20", kind: audioRecorded, state: audioPlaying}
	c.stop()
	if c.isPlaying() {
		t.Fatal("isPlaying() true after stop")
	}
	if fp := c.playingFingerprint(); fp != "" {
		t.Fatalf("playingFingerprint() = %q after stop, want empty", fp)
	}
}

func TestStopAudioForNav(t *testing.T) {
	gAudio.stop() // clean slate (other tests share the global)
	defer gAudio.stop()

	gAudio.startChapter(
		&AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 20},
		chapterAudio{Kind: audioRecorded, URL: "https://ebible.org/webaudio/John20.mp3"},
		"web|John|20",
	)
	if !gAudio.isPlaying() || gAudio.playingFingerprint() != "web|John|20" {
		t.Fatalf("startChapter did not load: playing=%v fp=%q", gAudio.isPlaying(), gAudio.playingFingerprint())
	}

	// Re-landing on the SAME chapter must leave playback alone.
	stopAudioForNav(&AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 20})
	if gAudio.playingFingerprint() != "web|John|20" {
		t.Fatal("same-chapter navigation stopped audio; it should continue")
	}

	// Navigating to a DIFFERENT chapter must stop it.
	stopAudioForNav(&AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 21})
	if gAudio.isPlaying() || gAudio.playingFingerprint() != "" {
		t.Fatalf("different-chapter navigation did not stop audio: playing=%v fp=%q",
			gAudio.isPlaying(), gAudio.playingFingerprint())
	}
}

func TestAdvanceToNextChapter(t *testing.T) {
	gAudio.stop() // the controller is a shared global; start clean
	defer gAudio.stop()

	bd := &BibleData{
		Books: []string{"BookA", "BookB"},
		Verses: map[string]map[int][]Verse{
			"BookA": {1: {}, 2: {}},
			"BookB": {1: {}, 2: {}},
		},
	}
	state := &AppState{Bible: bd, CurrentVersion: "web", CurrentBook: "BookA", CurrentChapter: 1}

	// Within a book: ch1 → ch2.
	if !advanceToNextChapter(state) || state.CurrentBook != "BookA" || state.CurrentChapter != 2 {
		t.Fatalf("within-book advance = %s %d, want BookA 2", state.CurrentBook, state.CurrentChapter)
	}
	// Across a book boundary: BookA's last chapter → BookB ch1.
	if !advanceToNextChapter(state) || state.CurrentBook != "BookB" || state.CurrentChapter != 1 {
		t.Fatalf("cross-book advance = %s %d, want BookB 1", state.CurrentBook, state.CurrentChapter)
	}
	// Within BookB: ch1 → ch2.
	if !advanceToNextChapter(state) || state.CurrentBook != "BookB" || state.CurrentChapter != 2 {
		t.Fatalf("BookB advance = %s %d, want BookB 2", state.CurrentBook, state.CurrentChapter)
	}
	// End of the Bible (last book, last chapter): no next, state unchanged.
	if advanceToNextChapter(state) {
		t.Fatal("advancing past the last chapter of the last book should return false")
	}
	if state.CurrentBook != "BookB" || state.CurrentChapter != 2 {
		t.Fatalf("end-of-Bible advance mutated state to %s %d", state.CurrentBook, state.CurrentChapter)
	}
}

func TestAudioSourceIconForKind(t *testing.T) {
	// Read-aloud (TTS) → the waveform glyph; recorded → something else (the person).
	if got := audioSourceIconForKind(audioTTS); got != iconAudioWave {
		t.Fatalf("TTS source icon = %v, want iconAudioWave", got)
	}
	if got := audioSourceIconForKind(audioRecorded); got == iconAudioWave {
		t.Fatal("recorded source icon should not be the waveform glyph")
	}
}
