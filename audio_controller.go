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
	"sync"

	"fyne.io/fyne/v2"
)

// audioPlayState is the controller's view of the native player. It drives the
// play button's glyph and whether "this chapter is the one playing".
type audioPlayState int

const (
	audioIdle    audioPlayState = iota // nothing loaded / stopped
	audioPlaying                       // actively producing sound
	audioPaused                        // loaded but paused
	audioEnded                         // reached the end of the chapter
)

// audioController is the single Go-side owner of playback. Created at package
// init; bound to the live AppState only through the methods the UI calls.
type audioController struct {
	mu sync.Mutex

	loaded   bool           // something is loaded in the native player
	loadedFP string         // chapterAudioFingerprint of the loaded chapter
	kind     audioKind      // recorded vs TTS of the loaded chapter
	state    audioPlayState // last state reported by the native layer / set on start

	// The reader's chosen source for a chapter, set from the source menu. It only
	// records the PREFERENCE — selecting never starts playback (that's the play
	// button's job). preferredFP scopes it to one chapter so navigating away falls
	// back to the per-chapter default (recording if available, else read-aloud).
	preferred    audioKind
	hasPreferred bool
	preferredFP  string

	// onChange re-renders the play button when the play state changes. The reading
	// header installs it (a refreshReadingOnly closure); nil in unit tests, where
	// fireChange must therefore stay a no-op (it never reaches fyne.Do).
	onChange func()
}

// gAudio is the process-wide controller. Single-window app.
var gAudio = &audioController{state: audioIdle}

// playPauseCurrent is the play button's tap handler — the ONLY thing that starts
// audio. If the chapter is already loaded it toggles play/pause; otherwise it
// starts the reader's chosen source (effectiveKind: the source-menu preference,
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
		// Native flips playing<->paused and posts bibleTextAudioStateChanged, which
		// updates the glyph via applyNativeState.
		nativeAudioToggle()
		return
	}
	c.playSource(state, c.effectiveKind(state))
}

// effectiveKind is the source the play button will start for the current chapter:
// the reader's source-menu preference when they set one for THIS chapter, else the
// default (a recording if the chapter has one, otherwise read-aloud). A preference
// for the recorded source is honoured only where a recording actually exists.
func (c *audioController) effectiveKind(state *AppState) audioKind {
	fp := chapterAudioFingerprint(state)
	c.mu.Lock()
	pref, has, pfp := c.preferred, c.hasPreferred, c.preferredFP
	c.mu.Unlock()
	if has && pfp == fp {
		if pref == audioRecorded && !chapterHasRecording(state) {
			return audioTTS
		}
		return pref
	}
	if chapterHasRecording(state) {
		return audioRecorded
	}
	return audioTTS
}

// resolveAudio turns a desired source kind into the concrete chapterAudio to play,
// falling back to read-aloud when a recording is asked for but none exists.
func resolveAudio(state *AppState, kind audioKind) chapterAudio {
	if kind == audioRecorded && chapterHasRecording(state) {
		return audioForChapter(state)
	}
	return ttsAudioForChapter(state)
}

// selectSource records the reader's chosen source for the current chapter WITHOUT
// starting playback — the play button is the only thing that begins audio. If a
// different source is already loaded for this chapter, that now-stale audio is
// stopped so a Play tap starts the chosen one cleanly; selecting the source that's
// already loaded leaves it playing/paused. Either way the indicator refreshes.
func (c *audioController) selectSource(state *AppState, kind audioKind) {
	if state == nil {
		return
	}
	fp := chapterAudioFingerprint(state)
	c.mu.Lock()
	c.preferred = kind
	c.hasPreferred = true
	c.preferredFP = fp
	staleLoaded := c.loaded && c.loadedFP == fp && c.kind != kind
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
	c.state = audioPlaying
	c.mu.Unlock()

	switch a.Kind {
	case audioRecorded:
		nativeAudioStartURL(a.URL, a.Title, a.Subtitle)
	default: // audioTTS
		nativeAudioStartTTS(a.Text, a.Title, a.Subtitle)
	}

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
func (c *audioController) playSource(state *AppState, kind audioKind) {
	if state == nil {
		return
	}
	c.startChapter(state, resolveAudio(state, kind), chapterAudioFingerprint(state))
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
	c.state = audioIdle
	c.mu.Unlock()
	if wasLoaded {
		nativeAudioStop()
		c.fireChange()
	}
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
	c.state = s
	switch s {
	case audioPlaying, audioPaused:
		// The engine reports it's actively producing (or holding) sound, so a source
		// IS loaded — re-assert it. Belt-and-suspenders against a stale teardown
		// callback having just cleared the flag a moment before this one lands; the
		// native mode guards (audio_ios.go) are the primary defense.
		c.loaded = true
	case audioIdle, audioEnded:
		// Chapter ended (or the session was torn down): nothing is actively loaded
		// for play/pause purposes, so a tap re-starts cleanly.
		c.loaded = false
		c.loadedFP = ""
	}
	c.mu.Unlock()
	c.fireChange()
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
