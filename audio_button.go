package bibletext

// The reader's per-chapter listen control, in the shared header builders
// (reading.go chapterHeader + reading_ios.go chapterHeaderMobile), recomputed on
// every navigation. Plain Fyne chrome above the native text overlay's frame, so
// it's never occluded and needs no hide/restore dance.
//
//   - a plain play/pause button (same muted style as the copy/arrow glyphs);
//   - a source indicator that appears ONLY while a source is loaded for this
//     chapter (playing or paused): a person for a recorded narration, a waveform
//     for read-aloud. It is hidden when nothing is playing.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

// audioButton builds the play/pause control plus, while audio is loaded, the
// source indicator. It installs gAudio.onChange so a native state change (chapter
// finished, a phone-call interruption, or a lock-screen / Control Center toggle)
// re-renders the cluster — refreshReadingOnly rebuilds the cheap Fyne header
// without re-pushing chapter HTML to the overlay.
func audioButton(state *AppState, boxH float32) fyne.CanvasObject {
	fp := chapterAudioFingerprint(state)
	playing, _ := gAudio.buttonState(fp)
	glyph := theme.MediaPlayIcon()
	if playing {
		glyph = theme.MediaPauseIcon()
	}
	play := newIconTapButton(state, glyph, 20, boxH, func() {
		gAudio.playPauseCurrent(state)
	})
	gAudio.setOnChange(func() { state.refreshReadingOnly() })

	// Source indicator: only while a source is loaded for this chapter; the glyph
	// reflects what is actually loaded (person = recording, waveform = read-aloud).
	// Tapping it opens the source menu (explain + switch).
	if show, kind := gAudio.indicator(fp); show {
		ind := newIconTapButton(state, audioSourceIconForKind(kind), 18, boxH, func() {
			showAudioSourceMenu(state)
		})
		return container.NewHBox(play, hgap(4), ind)
	}
	return play
}

// audioSourceIconForKind maps the loaded audio kind to its source glyph: a person
// for a recorded human narration, a waveform for on-device read-aloud (TTS).
func audioSourceIconForKind(kind audioKind) fyne.Resource {
	if kind == audioRecorded {
		return theme.AccountIcon()
	}
	return iconAudioWave
}
