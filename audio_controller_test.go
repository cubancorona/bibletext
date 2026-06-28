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

func TestAudioSourceIcon(t *testing.T) {
	bd := &BibleData{
		Books:  []string{"John"},
		Verses: map[string]map[int][]Verse{"John": {20: {{Text: "Now on the first day"}}}},
	}
	// BSB (not WEB) → TTS chapter → the voice glyph.
	bsb := &AppState{CurrentVersion: "bsb", CurrentBook: "John", CurrentChapter: 20, Bible: bd}
	if got := audioSourceIcon(bsb); got != iconSpeak {
		t.Fatalf("TTS chapter source icon = %v, want iconSpeak", got)
	}
	// WEB John 20 → recorded → the headphones glyph.
	web := &AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 20, Bible: bd}
	if got := audioSourceIcon(web); got != iconHeadphones {
		t.Fatalf("recorded chapter source icon = %v, want iconHeadphones", got)
	}
}
