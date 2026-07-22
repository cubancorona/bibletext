//go:build audiosmoke && !darwin && !android

package bibletext

// End-to-end smoke for the DESKTOP audio engine (oto: WASAPI on Windows, ALSA
// on Linux) — excluded from the normal suite by the audiosmoke build tag
// because it uses real network and a real audio device:
//
//	go test -tags audiosmoke -run TestDesktopAudioEndToEnd -v -timeout 15m .
//
// It plays a REAL narration chapter through the REAL engine and walks the
// whole controller state machine: download → buffering → playing → ±15s skip
// → pause → resume → seek to the end → natural ENDED → continuous-playback
// advance (one-book Bible, so it stops cleanly at the "end of the Bible").
// This is the closest an automated check gets to "the audio works on this
// platform" short of recording the output.

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
)

const smokeChapterURL = "https://github.com/cubancorona/bibletext-audio/releases/download/web-williams-nt-v1/WEB_43_020.mp3"

// waitFor polls cond every 250ms until it holds or the deadline passes.
func waitFor(t *testing.T, what string, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			t.Logf("✓ %s", what)
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("timed out after %v waiting for: %s", d, what)
}

func TestDesktopAudioEndToEnd(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	gAudio.stop()
	defer gAudio.stop()

	// One-book, one-chapter Bible: the natural-end advance has nowhere to go,
	// so the controller must finish IDLE instead of fetching another chapter.
	bd := &BibleData{
		Books:  []string{"John"},
		Verses: map[string]map[int][]Verse{"John": {20: {}}},
	}
	state := &AppState{Bible: bd, CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 20}
	fp := "web|John|20"

	t.Log("starting real recorded chapter (WEB John 20)…")
	gAudio.startChapter(state, chapterAudio{Kind: audioRecorded, URL: smokeChapterURL}, fp)

	if !gAudio.buffering(fp) {
		t.Fatalf("start must enter BUFFERING for %s", fp)
	}
	// Download + decode + device init: generous, runners vary.
	waitFor(t, "PLAYING (download + decode + device up)", 120*time.Second, func() bool {
		return gAudio.isPlaying() && gAudio.playingFingerprint() == fp
	})

	t.Log("skip +15s while playing…")
	engineSkip(15)
	time.Sleep(2 * time.Second)
	if !gAudio.isPlaying() {
		t.Fatal("skip must not stop playback")
	}

	t.Log("pause…")
	engineToggle()
	waitFor(t, "PAUSED (still loaded)", 10*time.Second, func() bool {
		return !gAudio.isPlaying() && gAudio.playingFingerprint() == fp && !gAudio.buffering(fp)
	})

	t.Log("resume…")
	engineToggle()
	waitFor(t, "PLAYING again", 10*time.Second, func() bool { return gAudio.isPlaying() })

	// Race straight to the end: the engine clamps each seek to the last frame,
	// so the buffer drains and the watcher must post a natural ENDED, and the
	// controller's continuous playback must stop cleanly at the end of our
	// one-book Bible (no runaway restart).
	t.Log("seeking to the end for the natural-ENDED path…")
	waitFor(t, "natural end → controller idle (end of Bible)", 90*time.Second, func() bool {
		if gAudio.playingFingerprint() == "" && !gAudio.isPlaying() {
			return true
		}
		engineSkip(600) // clamped to the final frame; drain follows
		return false
	})

	if gAudio.buffering(fp) || gAudio.isPlaying() || gAudio.playingFingerprint() != "" {
		t.Fatalf("controller must finish clean-idle; playing=%v fp=%q",
			gAudio.isPlaying(), gAudio.playingFingerprint())
	}
	t.Log("✓ full engine round-trip: buffering → playing → skip → pause → resume → ended → idle")
}
