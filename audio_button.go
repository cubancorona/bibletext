package bibletext

// The reader's audio control, in the gap to the right of the chapter navigation
// (the shared header builders place it there). Plain Fyne chrome above the native
// text overlay's frame, so it's never occluded.
//
// Collapsed it's a single speaker icon. Tapping it expands, in place, into a
// bordered card that hugs the player icons, with a muted close ✕ (opposite
// shading) tucked in the upper-right corner outside the box:
//
//	            ✕
//	┌───────────────────┐
//	│        (source)   │   top: source indicator, centred above play
//	│   ⟲15  ▶/⏸  15⟳  │   bottom: skip · play/pause · skip
//	└───────────────────┘
//
// The skips dim for read-aloud (speech can't seek); the source indicator
// (person = recording, waveform = read-aloud) opens the source menu.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
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

	src := newIconTapButton(state, audioSourceIconForKind(displayKind), 18, boxH, func() { showAudioSourceMenu(state) })
	back := newIconTapButton(state, iconSkipBack15, 20, boxH, func() { gAudio.skip(-15) })
	back.disabled = !canSeek
	play := newIconTapButton(state, playGlyph, 20, boxH, func() { gAudio.playPauseCurrent(state) })
	fwd := newIconTapButton(state, iconSkipFwd15, 20, boxH, func() { gAudio.skip(15) })
	fwd.disabled = !canSeek

	// The box hugs the player icons: the source centred on top (so it sits above the
	// play button), the skip/play/skip transport below.
	top := container.NewHBox(layout.NewSpacer(), src, layout.NewSpacer())
	bottom := container.NewHBox(back, play, fwd)
	box := surface(container.NewVBox(top, bottom), pal.SurfaceAlt, pal.Border, fyne.Size{})

	// Close ✕ with OPPOSITE shading (a muted-grey fill — the chapter-arrow colour —
	// with the glyph in the page colour), tucked in the upper-right corner outside
	// the box.
	xBg := canvas.NewRectangle(pal.TextMuted)
	xBg.CornerRadius = 6
	xGlyph := canvas.NewImageFromResource(theme.NewColoredResource(theme.CancelIcon(), theme.ColorNameBackground))
	xGlyph.FillMode = canvas.ImageFillContain
	xGlyph.SetMinSize(fyne.NewSize(12, 12))
	xCell := newTappableArea(
		container.NewGridWrap(fyne.NewSize(26, 26), container.NewStack(xBg, container.NewCenter(xGlyph))),
		func() { audioPanelOpen = false; state.refreshReadingOnly() },
	)
	corner := container.NewVBox(container.NewHBox(layout.NewSpacer(), xCell), layout.NewSpacer())

	return container.NewStack(box, corner)
}

// audioSourceIconForKind maps the loaded audio kind to its source glyph: a person
// for a recorded human narration, a waveform for on-device read-aloud (TTS).
func audioSourceIconForKind(kind audioKind) fyne.Resource {
	if kind == audioRecorded {
		return theme.AccountIcon()
	}
	return iconAudioWave
}

// tappableArea makes an arbitrary composed object tappable — used for the close ✕
// cell, which is a styled rectangle + glyph rather than a plain icon button.
type tappableArea struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onTap   func()
}

func newTappableArea(content fyne.CanvasObject, onTap func()) *tappableArea {
	t := &tappableArea{content: content, onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappableArea) Tapped(*fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

func (t *tappableArea) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.content)
}

var _ fyne.Tappable = (*tappableArea)(nil)
