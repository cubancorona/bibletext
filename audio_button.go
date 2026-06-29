package bibletext

// The reader's audio control, in the blank space between the chapter navigation
// and the full-screen button (the shared header builders place it there). It's
// plain Fyne chrome above the native text overlay's frame, so it's never occluded.
//
// Collapsed it's a single speaker icon. Tapping it expands, in place, into a
// compact mini-player: [speaker(collapse) · skip-back 15s · play/pause ·
// skip-forward 15s · source indicator]. The skips dim for read-aloud (speech can't
// seek); the source indicator (person = recording, waveform = read-aloud) opens
// the source menu.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

// audioPanelOpen tracks the control's collapsed/expanded state. Touched only on
// the UI goroutine; persists across header rebuilds.
var audioPanelOpen bool

func audioControl(state *AppState, boxH float32) fyne.CanvasObject {
	gAudio.setOnChange(func() { state.refreshReadingOnly() })
	fp := chapterAudioFingerprint(state)

	if !audioPanelOpen {
		return newIconTapButton(state, theme.VolumeUpIcon(), 20, boxH, func() {
			audioPanelOpen = true
			state.refreshReadingOnly()
		})
	}

	playing, _ := gAudio.buttonState(fp)
	playGlyph := theme.MediaPlayIcon()
	if playing {
		playGlyph = theme.MediaPauseIcon()
	}

	// Skip + source reflect what's loaded while playing, else the chapter's default.
	displayKind := audioTTS
	if chapterHasRecording(state) {
		displayKind = audioRecorded
	}
	if show, k := gAudio.indicator(fp); show {
		displayKind = k
	}
	canSeek := displayKind == audioRecorded

	collapse := newIconTapButton(state, theme.VolumeUpIcon(), 18, boxH, func() {
		audioPanelOpen = false
		state.refreshReadingOnly()
	})
	back := newIconTapButton(state, iconSkipBack15, 18, boxH, func() { gAudio.skip(-15) })
	back.disabled = !canSeek
	play := newIconTapButton(state, playGlyph, 18, boxH, func() { gAudio.playPauseCurrent(state) })
	fwd := newIconTapButton(state, iconSkipFwd15, 18, boxH, func() { gAudio.skip(15) })
	fwd.disabled = !canSeek
	src := newIconTapButton(state, audioSourceIconForKind(displayKind), 18, boxH, func() { showAudioSourceMenu(state) })

	return container.NewHBox(collapse, back, play, fwd, src)
}

// audioSourceIconForKind maps the loaded audio kind to its source glyph: a person
// for a recorded human narration, a waveform for on-device read-aloud (TTS).
func audioSourceIconForKind(kind audioKind) fyne.Resource {
	if kind == audioRecorded {
		return theme.AccountIcon()
	}
	return iconAudioWave
}
