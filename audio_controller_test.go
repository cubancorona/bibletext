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
		chapterAudio{Kind: audioRecorded, URL: "https://github.com/cubancorona/bibletext-audio/releases/download/web-williams-nt-v1/WEB_43_020.mp3"},
		"web|John|20",
	)
	// A recorded start starts life BUFFERING (spinner) until the native layer
	// reports audible playback — but it is loaded and bound to its chapter.
	if !gAudio.buffering("web|John|20") || gAudio.playingFingerprint() != "web|John|20" {
		t.Fatalf("startChapter did not load: buffering=%v fp=%q",
			gAudio.buffering("web|John|20"), gAudio.playingFingerprint())
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

// TestSelectSourceNarratorStaleness locks the narrator-aware staleness rule:
// choosing a DIFFERENT recording of the same kind for the loaded chapter must
// stop the now-stale audio (so Play starts the chosen narrator cleanly), while
// re-choosing the recording that's already loaded leaves it playing.
func TestSelectSourceNarratorStaleness(t *testing.T) {
	state := &AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 20}
	fp := chapterAudioFingerprint(state)
	load := func() {
		gAudio.mu.Lock()
		gAudio.loaded = true
		gAudio.loadedFP = fp
		gAudio.kind = audioRecorded
		gAudio.loadedRecID = "web-williams"
		gAudio.state = audioPlaying
		gAudio.mu.Unlock()
	}
	reset := func() {
		gAudio.mu.Lock()
		gAudio.loaded, gAudio.loadedFP, gAudio.loadedRecID = false, "", ""
		gAudio.kind, gAudio.state = audioRecorded, audioIdle
		gAudio.hasPreferred, gAudio.preferredFP, gAudio.preferredRecID = false, "", ""
		gAudio.mu.Unlock()
	}
	defer reset()

	// Same recording re-chosen → stays loaded.
	load()
	gAudio.selectSource(state, audioRecorded, "web-williams")
	if !gAudio.isPlaying() {
		t.Fatal("re-choosing the loaded recording must not stop it")
	}

	// A different narrator chosen → the loaded audio is stale and stops.
	gAudio.selectSource(state, audioRecorded, "web-somebody-else")
	if gAudio.isPlaying() {
		t.Fatal("choosing a different recording must stop the stale loaded audio")
	}

	// And the preference survives for the play button.
	if kind, recID := gAudio.effectiveSource(state); kind != audioTTS && recID == "web-williams" {
		t.Fatalf("effectiveSource after choosing another narrator = (%v, %q)", kind, recID)
	}
	reset()

	// effectiveSource default: the version's recording where it covers the chapter…
	if kind, recID := gAudio.effectiveSource(state); kind != audioRecorded || recID != "web-williams" {
		t.Fatalf("default effectiveSource = (%v, %q), want (recorded, web-williams)", kind, recID)
	}
	// …and TTS where it doesn't (deuterocanon).
	tobit := &AppState{CurrentVersion: "webc", CurrentBook: "Tobit", CurrentChapter: 1}
	if kind, recID := gAudio.effectiveSource(tobit); kind != audioTTS || recID != "" {
		t.Fatalf("deuterocanon effectiveSource = (%v, %q), want (TTS, \"\")", kind, recID)
	}
}

// TestReadAlongFollowSuspend locks the scroll-tug-of-war contract: a user scroll
// during read-along suspends the follow (highlight keeps tracking), the chip
// state reports it, resume re-attaches, and arming a new chapter resets it.
func TestReadAlongFollowSuspend(t *testing.T) {
	state := &AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 20}
	fp := chapterAudioFingerprint(state)
	reset := func() {
		gAudio.mu.Lock()
		gAudio.loaded, gAudio.loadedFP, gAudio.loadedRecID = false, "", ""
		gAudio.kind, gAudio.state = audioRecorded, audioIdle
		gAudio.readAlong, gAudio.readAlongVerse, gAudio.followSuspended = nil, 0, false
		gAudio.mu.Unlock()
	}
	defer reset()

	gAudio.mu.Lock()
	gAudio.loaded, gAudio.loadedFP, gAudio.state = true, fp, audioPlaying
	gAudio.readAlong = []verseTiming{{1, 0, 1}, {2, 10, 20}}
	gAudio.mu.Unlock()

	// A user scroll suspends the follow; a second report is a no-op.
	gAudio.onReadAlongUserScroll()
	if !gAudio.followSuspendedFor(fp) {
		t.Fatal("user scroll did not suspend the follow")
	}
	gAudio.onReadAlongUserScroll()
	if !gAudio.followSuspendedFor(fp) {
		t.Fatal("second scroll report flipped the suspension")
	}

	// Ticks keep updating the tracked verse while suspended.
	gAudio.onTimeUpdate(12)
	gAudio.mu.Lock()
	v := gAudio.readAlongVerse
	gAudio.mu.Unlock()
	if v != 2 {
		t.Fatalf("verse tracking stopped while suspended: verse=%d, want 2", v)
	}

	// Resume re-attaches.
	gAudio.resumeReadAlongFollow()
	if gAudio.followSuspendedFor(fp) {
		t.Fatal("resume did not clear the suspension")
	}

	// Arming a new chapter always starts attached.
	gAudio.onReadAlongUserScroll()
	gAudio.armReadAlong(state, chapterAudio{Kind: audioTTS})
	if gAudio.followSuspendedFor(fp) {
		t.Fatal("armReadAlong did not reset the suspension")
	}

	// No read-along armed → scroll reports are ignored entirely.
	reset()
	gAudio.onReadAlongUserScroll()
	if gAudio.followSuspendedFor(fp) {
		t.Fatal("scroll with no read-along armed suspended something")
	}
}
