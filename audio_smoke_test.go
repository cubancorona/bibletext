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
//
// IT ONLY MEANS THAT ON A MACHINE WITH AN AUDIO ENDPOINT. oto's Windows driver
// falls back to a silent nullContext when neither WASAPI nor WinMM finds a
// device, without returning an error, so on a headless runner every state
// above can go green with nothing reaching a speaker — and the drain at the
// end, which is timed by a real device, then fails and looks like a defect in
// the app. The backend is checked below for exactly that reason: this test
// says "the audio works here" only when it can show which backend carried it.

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

	// WHAT IS ACTUALLY CARRYING THE SAMPLES. Asked here, once playback has
	// started, because the driver is chosen asynchronously while the first
	// buffer is prepared — ask any earlier and the answer is "nothing yet".
	if mods, probed := audioBackendModules(); probed {
		if len(mods) == 0 {
			t.Fatalf("no audio backend is loaded in this process: oto found no endpoint and " +
				"installed its silent nullContext, so every state this test checks can pass " +
				"with nothing reaching a speaker. Run it where an audio endpoint exists " +
				"and the backend names itself: AUDIOSES.DLL and MMDevAPI.dll for WASAPI, " +
				"winmm.dll for WinMM.")
		}
		t.Logf("✓ audio backend in use: %v", mods)
	}

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

	// Race to the end: the engine clamps the seek to the last frame, the buffer
	// drains, the watcher posts a natural ENDED, and the controller's continuous
	// playback must stop cleanly at the end of our one-book Bible (no runaway
	// restart).
	//
	// The nudge is deliberately RARE. This loop used to seek on every poll —
	// four times a second — which is a race against the thing it is waiting
	// for: each seek refills the buffer the drain is trying to empty, so on a
	// driver whose buffer is larger or whose drain is slower than the poll
	// interval the end never arrives and the test blames the app. That is how
	// it read on its first ever run, on a Windows runner. Seek once, then
	// mostly watch; re-nudge only if the engine has genuinely stalled.
	t.Log("seeking to the end for the natural-ENDED path…")
	engineSkip(600) // clamped to the final frame; the drain follows
	lastNudge := time.Now()
	waitFor(t, "natural end → controller idle (end of Bible)", 90*time.Second, func() bool {
		if gAudio.playingFingerprint() == "" && !gAudio.isPlaying() {
			return true
		}
		if time.Since(lastNudge) > 10*time.Second {
			engineSkip(600)
			lastNudge = time.Now()
		}
		return false
	})

	if gAudio.buffering(fp) || gAudio.isPlaying() || gAudio.playingFingerprint() != "" {
		t.Fatalf("controller must finish clean-idle; playing=%v fp=%q",
			gAudio.isPlaying(), gAudio.playingFingerprint())
	}
	t.Log("✓ full engine round-trip: buffering → playing → skip → pause → resume → ended → idle")
}
