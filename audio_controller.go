package bibletext

// The cross-platform audio controller. It owns playback state for the reader's
// play button but holds NO AVFoundation handles — those live in the per-platform
// native files (audio_ios.go for the real engine; audio_other.go no-ops for the
// rest). This struct only resolves audioForChapter(state) into native calls and
// tracks the play state the native layer reports back, so the button can render
// the right glyph (play vs pause, recorded vs voice).
//
// One controller per process (single window), mirroring the existing
// activeAIState / gReadingTV singletons. Native → Go state changes arrive on
// bibleTextAudioStateChanged (audio_export_ios.go) → applyNativeState.

import (
	"log"
	"os"
	"sync"
	"time"

	"fyne.io/fyne/v2"
)

// raDebug logs read-along diagnostics (arm/tick/verse) when BIBLETEXT_DEBUG_READALONG
// is set — off in normal use. Kept because sync bugs here are otherwise invisible: a
// failed stream silently falls back to TTS (no timings), which looks identical to
// "highlighting is broken" until you see the arm/tick trace.
var raDebugOn = os.Getenv("BIBLETEXT_DEBUG_READALONG") != ""

func raDebug(format string, args ...interface{}) {
	if raDebugOn {
		log.Printf("[readalong] "+format, args...)
	}
}

// audioPlayState is the controller's view of the native player. It drives the
// play button's glyph and whether "this chapter is the one playing".
type audioPlayState int

const (
	audioIdle      audioPlayState = iota // nothing loaded / stopped
	audioPlaying                         // actively producing sound
	audioPaused                          // loaded but paused
	audioEnded                           // reached the end of the chapter
	audioFailed                          // recorded stream failed (404 / dead network / stall)
	audioBuffering                       // recorded stream loading — intended sound, not audible yet
)

// audioController is the single Go-side owner of playback. Created at package
// init; bound to the live AppState only through the methods the UI calls.
type audioController struct {
	mu sync.Mutex

	loaded      bool           // something is loaded in the native player
	loadedFP    string         // chapterAudioFingerprint of the loaded chapter
	kind        audioKind      // recorded vs TTS of the loaded chapter
	loadedRecID string         // which recording is loaded (empty for TTS)
	state       audioPlayState // last state reported by the native layer / set on start

	// The reader's chosen source for a chapter, set from the source menu. It only
	// records the PREFERENCE — selecting never starts playback (that's the play
	// button's job). preferredFP scopes it to one chapter so navigating away falls
	// back to the per-chapter default (recording if available, else read-aloud).
	preferred      audioKind
	preferredRecID string // the chosen recording when preferred == audioRecorded
	hasPreferred   bool
	preferredFP    string

	// boundState is the live AppState of whatever is loaded, captured on start so the
	// native end-of-chapter callback can drive continuous playback (advance to the
	// next chapter + keep going). nil until something has played.
	boundState *AppState

	// startedAt is when the loaded chapter began — used to ignore a near-instant
	// ENDED (an empty/failed chapter) so continuous playback can't race ahead.
	startedAt time.Time

	// onChange re-renders the play button when the play state changes. The reading
	// header installs it (a refreshReadingOnly closure); nil in unit tests, where
	// fireChange must therefore stay a no-op (it never reaches fyne.Do).
	onChange func()

	// Read-along: the loaded chapter's verse timing table (recorded audio only) and
	// the verse currently highlighted, so a playback-time tick only touches the native
	// text view when the verse actually changes. Set on start, cleared on stop.
	// followSuspended is raised when the reader scrolls away mid-narration: the
	// highlight keeps tracking the voice, but auto-scroll stops fighting them until
	// they tap the floating "Follow narration" button (resumeReadAlongFollow).
	readAlong       []verseTiming
	readAlongVerse  int
	followSuspended bool
}

// gAudio is the process-wide controller. Single-window app.
var gAudio = &audioController{state: audioIdle}

// showFollowButton drives the floating "Follow narration" pill. An indirection
// (not a direct call) so TestReadAlongFollowSuspend can record the pill
// transitions the shipped UI actually performs; production never swaps it.
var showFollowButton = readAlongFollowButton

// playPauseCurrent is the play button's tap handler — the ONLY thing that starts
// audio. If the chapter is already loaded it toggles play/pause; otherwise it
// starts the reader's chosen source (effectiveSource: the source-menu preference,
// or the per-chapter default).
func (c *audioController) playPauseCurrent(state *AppState) {
	if state == nil {
		return
	}
	fp := chapterAudioFingerprint(state)

	c.mu.Lock()
	sameChapter := c.loaded && c.loadedFP == fp
	c.mu.Unlock()

	if sameChapter {
		c.mu.Lock()
		buffering := c.state == audioBuffering
		c.mu.Unlock()
		if buffering {
			// The stream is still loading (spinner showing). A tap here used to pause
			// the not-yet-audible player — the reader heard nothing, saw ▶ again, and
			// concluded audio was broken. Ignore it; nav away still stops cleanly.
			return
		}
		// Native flips playing<->paused and posts bibleTextAudioStateChanged, which
		// updates the glyph via applyNativeState.
		nativeAudioToggle()
		return
	}
	kind, recID := c.effectiveSource(state)
	c.playSource(state, kind, recID)
}

// effectiveSource is the source the play button will start for the current chapter:
// the reader's source-menu preference when they set one for THIS chapter, else the
// default (the version's first recording if it covers the chapter, otherwise
// read-aloud). A recorded preference is honoured only where that recording actually
// exists; recID is empty whenever kind is audioTTS.
func (c *audioController) effectiveSource(state *AppState) (kind audioKind, recID string) {
	fp := chapterAudioFingerprint(state)
	c.mu.Lock()
	pref, prefRec, has, pfp := c.preferred, c.preferredRecID, c.hasPreferred, c.preferredFP
	c.mu.Unlock()
	if has && pfp == fp {
		if pref != audioRecorded {
			return audioTTS, ""
		}
		if rec, ok := recordingByID(state.CurrentVersion, prefRec); ok {
			if _, ok := rec.urlFor(state.CurrentBook, state.CurrentChapter); ok {
				return audioRecorded, rec.id
			}
		}
		return audioTTS, "" // the chosen recording doesn't cover this chapter
	}
	if recs := chapterRecordings(state); len(recs) > 0 {
		return audioRecorded, recs[0].id
	}
	return audioTTS, ""
}

// resolveAudio turns a desired source (kind + recording) into the concrete
// chapterAudio to play, falling back to read-aloud when the recording is asked
// for but doesn't exist (or doesn't cover this chapter).
func resolveAudio(state *AppState, kind audioKind, recID string) chapterAudio {
	if kind == audioRecorded && state != nil {
		if rec, ok := recordingByID(state.CurrentVersion, recID); ok {
			return audioForRecording(state, rec) // falls to TTS if the chapter is uncovered
		}
		return audioForChapter(state) // unknown id — the version default
	}
	return ttsAudioForChapter(state)
}

// selectSource records the reader's chosen source for the current chapter WITHOUT
// starting playback — the play button is the only thing that begins audio. If a
// different source is already loaded for this chapter, that now-stale audio is
// stopped so a Play tap starts the chosen one cleanly; selecting the source that's
// already loaded leaves it playing/paused. Either way the indicator refreshes.
func (c *audioController) selectSource(state *AppState, kind audioKind, recID string) {
	if state == nil {
		return
	}
	fp := chapterAudioFingerprint(state)
	c.mu.Lock()
	c.preferred = kind
	c.preferredRecID = recID
	c.hasPreferred = true
	c.preferredFP = fp
	staleLoaded := c.loaded && c.loadedFP == fp &&
		(c.kind != kind || (kind == audioRecorded && c.loadedRecID != recID))
	c.mu.Unlock()

	if staleLoaded {
		c.stop() // a different source is loaded; drop it (stop() fires the change)
		return
	}
	c.fireChange() // just refresh the indicator + skip-enabled state
}

// startChapter hands the resolved chapterAudio to the right native player and
// records what's loaded. Recorded → a seekable AVPlayer stream; TTS → on-device
// speech. Title/artist feed the lock-screen / Control Center Now Playing.
func (c *audioController) startChapter(state *AppState, a chapterAudio, fp string) {
	c.mu.Lock()
	c.loaded = true
	c.loadedFP = fp
	c.kind = a.Kind
	c.loadedRecID = a.RecordingID
	// A recorded stream starts life BUFFERING (the button shows a spinner until the
	// native layer reports actual playback); on-device speech is audible immediately.
	if a.Kind == audioRecorded {
		c.state = audioBuffering
	} else {
		c.state = audioPlaying
	}
	c.boundState = state // so the end-of-chapter callback can advance + keep playing
	c.startedAt = time.Now()
	c.mu.Unlock()

	switch a.Kind {
	case audioRecorded:
		nativeAudioStartURL(a.URL, a.Title, a.Subtitle)
	default: // audioTTS
		if !ttsSupported() {
			// Defense in depth: chapterAudioAvailable hides the button and the
			// source menu omits read-aloud where TTS doesn't exist (desktop
			// Windows/Linux), so this should be unreachable — but never hand the
			// engine a job it can't do.
			c.mu.Lock()
			c.state = audioIdle
			c.loaded = false
			c.loadedFP = ""
			c.mu.Unlock()
			return
		}
		nativeAudioStartTTS(a.Text, a.Title, a.Subtitle)
	}

	// Read-along: arm the verse timing table for this chapter (recorded only) so the
	// native time-observer ticks can highlight the verse being narrated + follow-scroll.
	c.armReadAlong(state, a)

	// Lock-screen / Control Center artwork: a "Book Chapter" card in the share-image
	// style. Rendered off the UI goroutine; the fonts are captured here (on the UI
	// goroutine) so the render never touches the live AppState. nativeAudioSetArtwork
	// is safe to call from any goroutine (it hops to the main thread).
	title, subtitle := a.Title, a.Subtitle
	regTTF := serifFontBytes(state, fyne.TextStyle{})
	boldTTF := serifFontBytes(state, fyne.TextStyle{Bold: true})
	go func() {
		if path, err := renderChapterArtwork(title, subtitle, regTTF, boldTTF); err == nil {
			nativeAudioSetArtwork(path)
		}
	}()

	c.fireChange()
}

// playSource starts the chapter from a specific source immediately. Not used by
// the source menu (which only sets the preference via selectSource); kept for
// callers that want to force-start a given source.
func (c *audioController) playSource(state *AppState, kind audioKind, recID string) {
	if state == nil {
		return
	}
	c.startChapter(state, resolveAudio(state, kind, recID), chapterAudioFingerprint(state))
}

// stop tears playback down. Idempotent; only notifies the UI if something was
// actually playing, so it's cheap to call on every navigation. Safe to call from
// the Fyne goroutine (nav/version change); the lifecycle teardown path calls the
// raw nativeAudioStop() directly instead, to avoid fyne.Do during shutdown.
func (c *audioController) stop() {
	c.mu.Lock()
	wasLoaded := c.loaded
	c.loaded = false
	c.loadedFP = ""
	c.kind = audioRecorded
	c.loadedRecID = ""
	c.state = audioIdle
	c.mu.Unlock()
	if wasLoaded {
		nativeAudioStop()
		c.fireChange()
	}
	c.clearReadAlong()
}

// armReadAlong loads the chapter's verse timing table (recorded audio that has bundled
// timings) so onTimeUpdate can highlight the narrated verse. Clears any prior highlight;
// a chapter without timings (TTS, or a version we don't bundle) simply arms nothing.
func (c *audioController) armReadAlong(state *AppState, a chapterAudio) {
	var vs []verseTiming
	switch {
	case a.Kind == audioRecorded && state != nil:
		vs = chapterTimings(a.RecordingID, state.CurrentBook, state.CurrentChapter)
	case a.Kind == audioTTS && state != nil:
		// Read-aloud needs no timing table: the synthesizer reports the utterance
		// range it's about to speak, so the "timings" are per-verse UTF-16 offsets
		// into the spoken text (onSpeechRange does the lookup).
		vs = speechVerseOffsets(state)
	}
	if state != nil {
		raDebug("arm kind=%d rec=%q book=%q ch=%d -> %d verses", a.Kind, a.RecordingID, state.CurrentBook, state.CurrentChapter, len(vs))
	}
	c.mu.Lock()
	c.readAlong = vs
	c.readAlongVerse = 0
	c.followSuspended = false
	c.mu.Unlock()
	readAlongClear()
	showFollowButton(false) // a fresh chapter always starts out following
}

// clearReadAlong drops the table and removes the on-screen highlight.
func (c *audioController) clearReadAlong() {
	c.mu.Lock()
	had := c.readAlong != nil
	c.readAlong = nil
	c.readAlongVerse = 0
	c.followSuspended = false
	c.mu.Unlock()
	raDebug("clearReadAlong had=%v", had)
	if had {
		readAlongClear()
	}
	showFollowButton(false) // nothing to follow → no way-back button
}

// onTimeUpdate is posted from the native player's periodic time observer (recorded
// audio) with the current playback position. It runs on the native main thread, so it
// may call the native highlight directly. Only touches the text view when the narrated
// verse actually changes.
func (c *audioController) onTimeUpdate(t float64) {
	c.mu.Lock()
	vs := c.readAlong
	last := c.readAlongVerse
	follow := !c.followSuspended
	c.mu.Unlock()
	if raDebugOn {
		raDebug("tick t=%.2f armed=%d last=%d", t, len(vs), last)
	}
	if len(vs) == 0 {
		return
	}
	v := verseAtTime(vs, t)
	if v == last {
		return
	}
	c.mu.Lock()
	c.readAlongVerse = v
	c.mu.Unlock()
	readAlongHighlight(v, follow)
}

// onReadAlongUserScroll is posted from the native reading views when the READER
// scrolls (a gesture — never our own programmatic follow) while read-along is
// live. The highlight keeps tracking the narration; the view stops chasing it
// until resumeReadAlongFollow. Runs on the native main thread.
func (c *audioController) onReadAlongUserScroll() {
	c.mu.Lock()
	// Gate on loaded too (the old chip's followSuspendedFor did): a scroll that
	// lands just after playback ended must not raise a pill with no narration
	// behind it.
	fresh := c.loaded && c.readAlong != nil && !c.followSuspended
	if fresh {
		c.followSuspended = true
	}
	c.mu.Unlock()
	if fresh {
		showFollowButton(true) // surface the floating "Follow narration" button
	}
}

// followSuspendedFor reports whether the loaded chapter's read-along is armed but
// no longer steering the scroll — i.e. the floating "Follow narration" button is up.
func (c *audioController) followSuspendedFor(fp string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loaded && c.loadedFP == fp && c.readAlong != nil && c.followSuspended
}

// resumeReadAlongFollow re-attaches the view to the narration: scrolls the current
// verse back into the comfortable band and lets subsequent ticks steer again.
// Reached from the floating button's native tap (main thread) via the
// bibleTextReadAlongFollowTapped export.
func (c *audioController) resumeReadAlongFollow() {
	c.mu.Lock()
	if !c.followSuspended {
		c.mu.Unlock()
		return
	}
	c.followSuspended = false
	v := c.readAlongVerse
	c.mu.Unlock()
	if v > 0 {
		readAlongHighlight(v, true)
	}
	showFollowButton(false) // way back taken — drop the button
}

// reassertReadAlong re-issues the current highlight + follow-pill state to the
// native reading overlay after it was rebuilt from scratch — Android recreates the
// activity (rotation, background→foreground) and BtBridge resets its read-along
// state (verse index, current span, pill) while the controller's copy stays intact.
// Called from afterRebuild (reading_android.go). No-op when nothing is armed. On the
// Apple platforms the overlay isn't torn down this way, so it's simply never called.
func (c *audioController) reassertReadAlong() {
	c.mu.Lock()
	armed := c.loaded && c.readAlong != nil
	v := c.readAlongVerse
	suspended := c.followSuspended
	c.mu.Unlock()
	if !armed {
		return
	}
	if v > 0 {
		readAlongHighlight(v, !suspended)
	}
	showFollowButton(suspended) // restore the "Follow narration" pill if it was up
}

// onSpeechRange is posted from the speech synthesizer's willSpeakRangeOfSpeechString
// delegate (TTS read-along) with the UTF-16 offset of the text about to be spoken.
// The armed table holds per-verse offsets in the same unit, so the recorded path's
// last-start-at-or-before lookup applies unchanged.
func (c *audioController) onSpeechRange(loc int) {
	c.onTimeUpdate(float64(loc))
}

// skip seeks the recorded player by ±seconds (the ±15s controls). A no-op for
// TTS, which can't seek — gated here so the UI never offers a control that lies.
func (c *audioController) skip(seconds float64) {
	c.mu.Lock()
	canSeek := c.loaded && c.kind == audioRecorded
	c.mu.Unlock()
	if canSeek {
		nativeAudioSkip(seconds)
	}
}

// isPlaying reports the controller's tracked state (cheap, no cgo).
func (c *audioController) isPlaying() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state == audioPlaying
}

// buffering reports whether the loaded chapter's recorded stream is still coming
// up — the card shows a spinner and play taps are ignored until it resolves.
func (c *audioController) buffering(fp string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loaded && c.loadedFP == fp && c.state == audioBuffering
}

// playingFingerprint is the fingerprint of the loaded chapter, or "" when idle —
// so a caller can tell whether a given chapter is the one playing.
func (c *audioController) playingFingerprint() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loaded {
		return ""
	}
	return c.loadedFP
}

// buttonState reports, under a SINGLE lock (no torn read), whether the chapter
// identified by fp is actively playing and whether it's loaded here at all
// (loaded-but-paused counts). Lets the play button pick play / pause / resume.
func (c *audioController) buttonState(fp string) (playing, loadedHere bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	loadedHere = c.loaded && c.loadedFP == fp
	playing = loadedHere && c.state == audioPlaying
	return
}

// indicator reports whether the source indicator should show for the chapter
// identified by fp (true while a source is loaded here — playing or paused) and,
// if so, the loaded kind so the glyph can reflect recording vs read-aloud.
func (c *audioController) indicator(fp string) (show bool, kind audioKind) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loaded && c.loadedFP == fp {
		return true, c.kind
	}
	return false, audioRecorded
}

// setOnChange installs the button's refresh closure (re-set on each header build).
func (c *audioController) setOnChange(fn func()) {
	c.mu.Lock()
	c.onChange = fn
	c.mu.Unlock()
}

// fireChange invokes onChange on the Fyne goroutine. Callers may be on the Fyne
// goroutine (UI taps) or the native main thread (the export callback), so it
// always marshals through fyne.Do. No-op when onChange is nil (unit tests).
func (c *audioController) fireChange() {
	c.mu.Lock()
	fn := c.onChange
	c.mu.Unlock()
	if fn == nil {
		return
	}
	fyne.Do(fn)
}

// applyNativeState is called (via the //export callback) when the native player
// changes state on its own — finished a chapter, was paused by a phone-call
// interruption, or toggled from the lock screen / Control Center.
func (c *audioController) applyNativeState(s audioPlayState) {
	c.mu.Lock()
	// Capture identity BEFORE the switch clears it: the chapter that's ending, and
	// whether it ended suspiciously fast (an empty / failed chapter that produced no
	// real audio — guards continuous playback against racing through chapters).
	endedFP := c.loadedFP
	tooFast := time.Since(c.startedAt) < 2*time.Second
	if (s == audioPlaying || s == audioPaused || s == audioBuffering) && c.loadedFP == "" {
		// An activity report with NO chapter identity: this controller already
		// disowned the session (a stop/nav teardown cleared loadedFP while the
		// report was in flight through its fyne.Do hop — engines check staleness
		// at POST time, so a report can still land after a stop). Applying it
		// would manufacture "playing but anonymous" — nav-stop couldn't match it
		// and the button couldn't either — so drop it whole. Every legitimate
		// report has identity: startChapter stamps loadedFP before any engine
		// starts. (Caught by TestStopAudioForNav under -race on Windows, where
		// the oto engine's PLAYING landed just after the nav stop.)
		c.mu.Unlock()
		return
	}
	c.state = s
	switch s {
	case audioPlaying, audioPaused, audioBuffering:
		// The engine reports it's actively producing (or holding) sound, so a
		// source IS loaded — re-assert it (belt-and-suspenders against a stale
		// teardown callback having cleared the flag a moment before this lands;
		// the native mode guards, audio_ios.go, are the primary defense).
		c.loaded = true
	case audioIdle, audioEnded, audioFailed:
		// Chapter ended, stream failed, or the session was torn down: nothing is
		// actively loaded for play/pause purposes, so a tap re-starts cleanly.
		c.loaded = false
		c.loadedFP = ""
		c.followSuspended = false
	}
	endedKind, endedRecID, endedState := c.kind, c.loadedRecID, c.boundState
	c.mu.Unlock()
	c.fireChange()

	// The floating "Follow narration" pill must not outlive playback: the old
	// card chip derived its visibility from c.loaded and vanished on this
	// rebuild; the imperative pill needs the explicit drop. Harmless on the
	// self-healing paths — a continuation or TTS fallback re-arms via
	// armReadAlong, which starts the fresh chapter following anyway.
	if s == audioIdle || s == audioEnded || s == audioFailed {
		showFollowButton(false)
	}

	// Continuous playback: a chapter that finishes on its own (ENDED — NOT a user
	// pause or a manual stop, which post PAUSED / go through gAudio.stop()) rolls
	// onto the next chapter and keeps playing in the same mode, carrying the reading
	// pane with it, until the reader pauses or the Bible ends.
	if s == audioEnded && endedState != nil && !tooFast {
		c.advanceAndContinue(endedState, endedKind, endedRecID, endedFP)
	} else if s == audioEnded {
		// Ended with no continuation possible (no bound state, or a suspiciously
		// instant end) — the listening session is over. Release the native side
		// explicitly: it can't tell a between-chapters ENDED from a final one, so
		// without this the lock-screen card (and, on Android, the foreground
		// service + notification) would sit parked forever.
		nativeAudioStop()
	}

	// A recorded stream that failed (404, dead network, hung buffer) restarts the
	// SAME chapter as on-device read-aloud — TTS needs no network, so the reader
	// hears the chapter instead of watching the button silently revert. One-shot by
	// construction: FAILED is only posted from the recorded (AVPlayer) paths, so a
	// TTS retry can never re-enter here.
	if s == audioFailed && endedState != nil && endedKind == audioRecorded {
		c.fallbackToTTS(endedState, endedFP)
	} else if s == audioFailed {
		// FAILED with no fallback possible — end the session on the native side
		// (Android holds its foreground service through FAILED expecting the
		// fallback; without one it must be told the session is over).
		nativeAudioStop()
	}
}

// fallbackToTTS restarts the chapter whose recorded stream just failed as
// read-aloud. Runs on the Fyne goroutine (the FAILED report arrives on the
// native thread; ttsAudioForChapter reads live state). Bails if the reader has
// navigated away since the failure — don't speak a chapter they left.
func (c *audioController) fallbackToTTS(state *AppState, failedFP string) {
	fyne.Do(func() {
		if chapterAudioFingerprint(state) != failedFP {
			// Reader left the failed chapter — no fallback. Release the native
			// session surfaces (Android keeps its service up through FAILED
			// precisely so this fallback could reuse it; tell it we won't).
			nativeAudioStop()
			return
		}
		c.startChapter(state, ttsAudioForChapter(state), failedFP)
	})
}

// advanceAndContinue moves the reader to the next chapter (across book boundaries)
// and starts its audio in the same mode the previous chapter was using. Runs the
// navigation + start on the Fyne goroutine (applyNativeState arrives on the native
// thread). Stops silently at the end of the Bible. endedFP is the chapter that just
// finished: if the reader has since navigated elsewhere (a manual jump that raced
// the chapter's end), we must NOT hijack their new position — so bail unless they're
// still on the chapter that ended.
func (c *audioController) advanceAndContinue(state *AppState, kind audioKind, recID, endedFP string) {
	fyne.Do(func() {
		if chapterAudioFingerprint(state) != endedFP {
			// Reader moved on while the chapter was finishing — don't yank them.
			// Their navigation already stopped any mismatched playback, but the
			// native session surfaces may still be up; release them (idempotent).
			nativeAudioStop()
			return
		}
		if !advanceToNextChapter(state) {
			// End of the Bible: nothing follows Revelation 22 — end the session so
			// the lock-screen card / Android foreground service don't sit parked.
			nativeAudioStop()
			return
		}
		state.refresh() // carry the reading pane onto the new chapter
		c.startChapter(state, resolveAudio(state, kind, recID), chapterAudioFingerprint(state))
	})
}

// stopAudioForNav stops playback when the reader navigates to a DIFFERENT chapter
// than the one playing (the audio is bound to the displayed text). Re-landing on
// the same chapter that's playing leaves it alone — a nice property for free.
func stopAudioForNav(state *AppState) {
	if state == nil {
		return
	}
	playing := gAudio.playingFingerprint()
	if playing != "" && playing != chapterAudioFingerprint(state) {
		gAudio.stop()
	}
}
