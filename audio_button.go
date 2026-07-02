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

// audioControl returns a self-refreshing host: audio state changes (play/pause,
// end, source pick) and expand/collapse re-render ONLY this small control, never
// the whole reading pane. Rebuilding the pane (state.refreshReadingOnly) re-pins
// the native text overlay and visibly FLICKERS the screen — so the play button
// must not trigger it. The collapsed↔expanded card is sized to fit within the
// header, so swapping it doesn't change the header height (no overlay re-pin).
func audioControl(state *AppState, boxH float32) fyne.CanvasObject {
	host := container.NewStack()
	var rebuild func()
	rebuild = func() {
		host.Objects = []fyne.CanvasObject{audioControlContent(state, boxH, rebuild)}
		host.Refresh()
	}
	// fireChange marshals onChange onto the UI goroutine, so rebuild runs there.
	gAudio.setOnChange(rebuild)
	rebuild()
	// Reserve the EXPANDED card's footprint permanently. The collapsed↔expanded
	// swap happens in place (host.Objects mutation) and deliberately never
	// re-lays the surrounding header — but Fyne hit-testing only reaches objects
	// within their ancestors' laid-out rects, so a card bigger than the host
	// would DRAW beyond the host while its buttons are untappable (the shipped
	// bug: the visible ▶ hit skip-back / nothing). A fixed cell sized to the
	// card makes expansion geometry-neutral: taps land, header never reflows.
	probe := buildAudioCard(state, audioTTS, false, false, audioCardCallbacks{})
	return container.NewGridWrap(probe.MinSize(), host)
}

func audioControlContent(state *AppState, boxH float32, rebuild func()) fyne.CanvasObject {
	fp := chapterAudioFingerprint(state)

	if !audioPanelOpen {
		speaker := newIconTapButton(state, theme.VolumeUpIcon(), 20, boxH, func() {
			audioPanelOpen = true
			rebuild()
		})
		if fyne.CurrentDevice().IsMobile() {
			// iOS centres the control in the header gap — keep the collapsed
			// speaker in the middle of the reserved cell.
			return container.NewCenter(speaker)
		}
		// Desktop: the cell's right edge abuts the focus toggle, so pin the
		// collapsed speaker there (where it has always sat); the card grows
		// leftward into the empty header gap when expanded.
		return container.NewHBox(layout.NewSpacer(), container.NewCenter(speaker))
	}

	playing, _ := gAudio.buttonState(fp)

	// Skip + source reflect what's loaded while playing, else the reader's chosen
	// source for this chapter (effectiveKind: source-menu preference or default).
	displayKind := gAudio.effectiveKind(state)
	if show, k := gAudio.indicator(fp); show {
		displayKind = k
	}

	return buildAudioCard(state, displayKind, playing, displayKind == audioRecorded, audioCardCallbacks{
		onSrc:   func() { showAudioSourceMenu(state) },
		onBack:  func() { gAudio.skip(-15) },
		onPlay:  func() { gAudio.playPauseCurrent(state) },
		onFwd:   func() { gAudio.skip(15) },
		onClose: func() { audioPanelOpen = false; rebuild() },
	})
}

// audioCardCallbacks routes the expanded card's taps. Injected (rather than the
// buttons calling gAudio directly) so the hit-region layout test can probe the
// card's tap targets without starting real audio.
type audioCardCallbacks struct {
	onSrc, onBack, onPlay, onFwd, onClose func()
}

// buildAudioCard assembles the expanded transport card: source indicator centred
// on top, skip/play/skip below, close ✕ tucked in the upper-right corner.
func buildAudioCard(state *AppState, displayKind audioKind, playing, canSeek bool, cb audioCardCallbacks) fyne.CanvasObject {
	pal := state.pal()
	playGlyph := theme.MediaPlayIcon()
	if playing {
		playGlyph = theme.MediaPauseIcon()
	}

	// Compact rows so the two-row card stays SHORTER than the header's two text
	// lines — expanding it must not grow the header and push the reading lane down.
	const rh = 25
	src := newIconTapButton(state, audioSourceIconForKind(displayKind), 16, rh, cb.onSrc)
	back := newIconTapButton(state, iconSkipBack15, 18, rh, cb.onBack)
	back.disabled = !canSeek
	play := newIconTapButton(state, playGlyph, 18, rh, cb.onPlay)
	fwd := newIconTapButton(state, iconSkipFwd15, 18, rh, cb.onFwd)
	fwd.disabled = !canSeek

	// The box hugs the player icons: the source centred on top (so it sits above the
	// play button), the skip/play/skip transport below. A tight manual frame (not
	// surface(), which adds NewPadded theme padding) keeps it short.
	top := container.NewHBox(layout.NewSpacer(), src, layout.NewSpacer())
	bottom := container.NewHBox(back, play, fwd)
	rows := container.New(layout.NewCustomPaddedVBoxLayout(0), top, bottom)
	frame := canvas.NewRectangle(pal.SurfaceAlt)
	frame.StrokeColor = pal.Border
	frame.StrokeWidth = 1
	frame.CornerRadius = 8
	box := container.NewStack(frame, container.New(layout.NewCustomPaddedLayout(1, 1, 6, 6), rows))

	// Close ✕ with OPPOSITE shading (a muted-grey fill — the chapter-arrow colour —
	// with the glyph in the page colour), tucked in the upper-right corner.
	xBg := canvas.NewRectangle(pal.TextMuted)
	xBg.CornerRadius = 5
	xGlyph := canvas.NewImageFromResource(theme.NewColoredResource(theme.CancelIcon(), theme.ColorNameBackground))
	xGlyph.FillMode = canvas.ImageFillContain
	xGlyph.SetMinSize(fyne.NewSize(10, 10))
	xCell := newTappableArea(
		container.NewGridWrap(fyne.NewSize(22, 22), container.NewStack(xBg, container.NewCenter(xGlyph))),
		cb.onClose,
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
