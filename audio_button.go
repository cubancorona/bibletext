package bibletext

// The reader's audio control, in the gap to the right of the chapter navigation
// (the shared header builders place it there). Plain Fyne chrome above the native
// text overlay's frame, so it's never occluded.
//
// Collapsed it's a single speaker icon. Tapping it expands, in place, into a
// bordered two-row mini-player card:
//
//	┌───────────────────────────┐
//	│            (source)        │   top row: the source indicator alone
//	│   ⟲15    ▶/⏸    15⟳    ✕  │   bottom row: skip · play · skip · close
//	└───────────────────────────┘
//
// The skips dim for read-aloud (speech can't seek); the source indicator
// (person = recording, waveform = read-aloud) opens the source menu; the ✕ — muted
// like the chapter arrows — collapses the card.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
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

	src := newIconTapButton(state, audioSourceIconForKind(displayKind), 18, boxH, func() { showAudioSourceMenu(state) })
	back := newIconTapButton(state, iconSkipBack15, 20, boxH, func() { gAudio.skip(-15) })
	back.disabled = !canSeek
	play := newIconTapButton(state, playGlyph, 20, boxH, func() { gAudio.playPauseCurrent(state) })
	fwd := newIconTapButton(state, iconSkipFwd15, 20, boxH, func() { gAudio.skip(15) })
	fwd.disabled = !canSeek
	closeBtn := newIconTapButton(state, theme.CancelIcon(), 16, boxH, func() {
		audioPanelOpen = false
		state.refreshReadingOnly()
	})

	top := container.NewHBox(layout.NewSpacer(), src, layout.NewSpacer())
	bottom := container.NewHBox(back, play, fwd, hgap(10), closeBtn)
	return surface(container.NewVBox(top, bottom), pal.SurfaceAlt, pal.Border, fyne.Size{})
}

// audioSourceIconForKind maps the loaded audio kind to its source glyph: a person
// for a recorded human narration, a waveform for on-device read-aloud (TTS).
func audioSourceIconForKind(kind audioKind) fyne.Resource {
	if kind == audioRecorded {
		return theme.AccountIcon()
	}
	return iconAudioWave
}
