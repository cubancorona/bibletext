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
	"fyne.io/fyne/v2/canvas"
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
	play := newIconTapButton(state, glyph, 18, boxH, func() {
		gAudio.playPauseCurrent(state)
	})
	gAudio.setOnChange(func() { state.refreshReadingOnly() })

	// Source indicator: only while a source is loaded for this chapter; the glyph
	// reflects what is actually loaded (person = recording, waveform = read-aloud).
	if show, kind := gAudio.indicator(fp); show {
		return container.NewHBox(play, hgap(4), audioSourceTag(audioSourceIconForKind(kind), 16, boxH))
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

// audioSourceTag renders the source glyph flat and muted. (The tap-to-explain
// menu is added in a follow-up; for now it's a passive indicator.)
func audioSourceTag(res fyne.Resource, size, boxH float32) fyne.CanvasObject {
	img := canvas.NewImageFromResource(theme.NewColoredResource(res, colorNameMuted))
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(size, size))
	return container.NewGridWrap(fyne.NewSize(size+6, boxH), container.NewCenter(img))
}
