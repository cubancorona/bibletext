package bibletext

// The reader's audio control, in the blank space between the chapter navigation
// and the full-screen button (the shared header builders place it there). Plain
// Fyne chrome above the native text overlay's frame, so it's never occluded.
//
// Collapsed it's a single speaker icon. Tapping it expands, in place, into a
// bordered mini-player box: [skip-back 15s · play/pause · skip-forward 15s ·
// source indicator] with a close "✕" in an accent annex (opposite shading) on the
// right. The skips dim for read-aloud (speech can't seek); the source indicator
// (person = recording, waveform = read-aloud) opens the source menu.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
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

	pal := state.pal()
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

	const isz = 20
	back := newIconTapButton(state, iconSkipBack15, isz, boxH, func() { gAudio.skip(-15) })
	back.disabled = !canSeek
	play := newIconTapButton(state, playGlyph, isz, boxH, func() { gAudio.playPauseCurrent(state) })
	fwd := newIconTapButton(state, iconSkipFwd15, isz, boxH, func() { gAudio.skip(15) })
	fwd.disabled = !canSeek
	src := newIconTapButton(state, audioSourceIconForKind(displayKind), 18, boxH, func() { showAudioSourceMenu(state) })

	// Close ✕ in an accent annex (opposite shading): a HighImportance button is
	// accent-filled with on-accent (light) text, so it reads as the inverted tab.
	closeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		audioPanelOpen = false
		state.refreshReadingOnly()
	})
	closeBtn.Importance = widget.HighImportance

	// The icon buttons already carry internal padding, and NewHBox adds theme
	// padding between children, so even spacing comes for free — no manual gaps
	// (which stacked on top and over-widened the box). The ✕ annex sits at the right.
	controls := container.NewHBox(back, play, fwd, src, closeBtn)
	return surface(controls, pal.SurfaceAlt, pal.Border, fyne.Size{})
}

// audioSourceIconForKind maps the loaded audio kind to its source glyph: a person
// for a recorded human narration, a waveform for on-device read-aloud (TTS).
func audioSourceIconForKind(kind audioKind) fyne.Resource {
	if kind == audioRecorded {
		return theme.AccountIcon()
	}
	return iconAudioWave
}
