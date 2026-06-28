package bibletext

// The reader's per-chapter play button. It lives in the shared chapterHeader
// (reading.go), so one code path serves both iOS and desktop and the glyph is
// recomputed on every navigation. It's plain Fyne chrome above the native text
// overlay's frame, so it's never occluded and needs no hide/restore dance.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// audioButton builds the play/pause control. It installs gAudio.onChange so a
// native state change (chapter finished, a phone-call interruption, or a
// lock-screen / Control Center toggle) re-renders the button — refreshReadingOnly
// rebuilds the cheap Fyne header without re-pushing chapter HTML to the overlay
// (the push path skips on an unchanged chapterRenderFingerprint, which an audio
// toggle never changes).
func audioButton(state *AppState, boxH float32) fyne.CanvasObject {
	btn := newIconTapButton(state, audioButtonIcon(state), 20, boxH, func() {
		gAudio.playPauseCurrent(state)
	})
	gAudio.setOnChange(func() { state.refreshReadingOnly() })
	return btn
}

// audioButtonIcon picks the glyph for the current chapter:
//
//	playing this chapter        → MediaPauseIcon
//	paused on this chapter      → MediaPlayIcon  (a resume triangle)
//	idle, recorded chapter      → MediaPlayIcon  ("play the recording")
//	idle, TTS chapter           → iconSpeak      (a distinct "read aloud" glyph)
//
// It uses chapterHasRecording (a cheap version/book/chapter map lookup) to choose
// recorded-vs-TTS — NOT audioForChapter, which would eagerly build the whole
// chapter's speech text on every header rebuild just to discard it.
func audioButtonIcon(state *AppState) fyne.Resource {
	playing, loadedHere := gAudio.buttonState(chapterAudioFingerprint(state))
	switch {
	case playing:
		return theme.MediaPauseIcon()
	case loadedHere: // paused on this chapter → resume
		return theme.MediaPlayIcon()
	case !chapterHasRecording(state): // a TTS chapter, not yet started
		return iconSpeak
	default: // a recorded chapter, not yet started
		return theme.MediaPlayIcon()
	}
}
