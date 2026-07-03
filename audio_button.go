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
//	│    Narrator ▾     │   top: labeled source chip, centred above play
//	│   ⟲15  ▶/⏸  15⟳  │   bottom: skip · play/pause · skip
//	└───────────────────┘
//
// The skips dim for read-aloud (speech can't seek); the source chip (person =
// recording, waveform = read-aloud, labeled "Narrator ▾"/"Read aloud ▾") opens
// the source menu.

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
	probe := buildAudioCard(state, audioTTS, false, false, false, audioCardCallbacks{})
	return container.NewGridWrap(probe.MinSize(), host)
}

func audioControlContent(state *AppState, boxH float32, rebuild func()) fyne.CanvasObject {
	fp := chapterAudioFingerprint(state)

	if !audioPanelOpen {
		// Card closed + nothing actively narrating = the reader is done with audio,
		// so a lingering read-along tint is stale markup — drop it. This runs on
		// every close AND every audio state change (fireChange → rebuild), so it
		// catches both orders: pausing then closing the card, and closing the card
		// then pausing from the lock screen / Now Playing. While audio still PLAYS
		// (or is buffering toward playing) with the card collapsed, the highlight
		// keeps following the narration.
		if playing, _ := gAudio.buttonState(fp); !playing && !gAudio.buffering(fp) {
			gAudio.clearReadAlong()
		}
		speaker := newIconTapButton(state, theme.VolumeUpIcon(), 24, boxH, func() {
			audioPanelOpen = true
			rebuild()
		})
		if fyne.CurrentDevice().IsMobile() {
			// iOS centres the control in the header gap — keep the collapsed
			// control in the middle of the reserved cell.
			return container.NewCenter(speaker)
		}
		// Desktop: the cell's right edge abuts the focus toggle, so pin the
		// collapsed control there (where it has always sat); the card grows
		// leftward into the empty header gap when expanded.
		return container.NewHBox(layout.NewSpacer(), container.NewCenter(speaker))
	}

	playing, _ := gAudio.buttonState(fp)
	buffering := gAudio.buffering(fp)

	// Skip + source reflect what's loaded while playing, else the reader's chosen
	// source for this chapter (effectiveSource: source-menu preference or default).
	displayKind, _ := gAudio.effectiveSource(state)
	if show, k := gAudio.indicator(fp); show {
		displayKind = k
	}

	return buildAudioCard(state, displayKind, playing, buffering, displayKind == audioRecorded,
		audioCardCallbacks{
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

// buildAudioCard assembles the expanded transport card: labeled source chip
// centred on top, skip/play/skip below, close ✕ tucked in the upper-right corner.
// While buffering, the play slot shows a SPINNER instead of a glyph — silence
// behind a pause glyph reads as "broken" — and the slot ignores taps until the
// stream resolves to playing/paused/failed.
func buildAudioCard(state *AppState, displayKind audioKind, playing, buffering, canSeek bool, cb audioCardCallbacks) fyne.CanvasObject {
	pal := state.pal()
	playGlyph := theme.MediaPlayIcon()
	if playing {
		playGlyph = theme.MediaPauseIcon()
	}

	// Two row heights: a labeled source chip on top, and a TALLER transport row
	// (skip · play · skip) below. The transport controls are the ones the reader
	// actually taps mid-listen, so they get the full Apple-HIG minTapTarget height
	// with correspondingly larger glyphs. srcRowH + transportRowH + the box's 2px
	// vertical padding must stay within the header's two-text-line height (mobile
	// boxH 36 ×2 +2 = 74; desktop 38+7+34 = 79), so the reserved card footprint never
	// grows the header / pushes the reading lane down.
	const (
		srcRowH       = 26
		transportRowH = minTapTarget // 44
	)
	// The source selector is a LABELED chip, not a bare glyph: a 16px person/waveform
	// icon alone (the old control) signalled neither "this is a button" nor "narrators
	// live here" — the narrator menu was effectively undiscoverable. Text + ▾ fixes
	// both, and the wide chip pulls mis-taps away from the play button beneath it.
	srcLabel := "Read aloud ▾"
	if displayKind == audioRecorded {
		srcLabel = "Narrator ▾"
	}
	src := newLabeledTapChip(state, audioSourceIconForKind(displayKind), srcLabel, srcRowH, cb.onSrc)
	back := newIconTapButton(state, iconSkipBack15, 26, transportRowH, cb.onBack)
	back.disabled = !canSeek
	var play fyne.CanvasObject
	if buffering {
		spin := widget.NewActivity()
		spin.Start()
		play = container.NewGridWrap(fyne.NewSize(transportRowH, transportRowH), container.NewCenter(spin))
	} else {
		play = newIconTapButton(state, playGlyph, 28, transportRowH, cb.onPlay)
	}
	fwd := newIconTapButton(state, iconSkipFwd15, 26, transportRowH, cb.onFwd)
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
	xGlyph.SetMinSize(fyne.NewSize(12, 12))
	xCell := newTappableArea(
		container.NewGridWrap(fyne.NewSize(26, 26), container.NewStack(xBg, container.NewCenter(xGlyph))),
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

// labeledTapChip is the audio card's source selector: a small icon + muted text
// label in one tappable pill. Its own type (rather than a reused tappableArea) so
// the hit-region test can find it unambiguously.
type labeledTapChip struct {
	widget.BaseWidget
	state *AppState
	icon  fyne.Resource
	label string
	boxH  float32
	onTap func()
}

func newLabeledTapChip(state *AppState, icon fyne.Resource, label string, boxH float32, onTap func()) *labeledTapChip {
	c := &labeledTapChip{state: state, icon: icon, label: label, boxH: boxH, onTap: onTap}
	c.ExtendBaseWidget(c)
	return c
}

func (c *labeledTapChip) Tapped(*fyne.PointEvent) {
	if c.onTap != nil {
		c.onTap()
	}
}

func (c *labeledTapChip) CreateRenderer() fyne.WidgetRenderer {
	img := canvas.NewImageFromResource(theme.NewColoredResource(c.icon, colorNameMuted))
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(14, 14))
	lbl := canvas.NewText(c.label, c.state.pal().TextMuted)
	lbl.TextSize = 13
	w := fyne.MeasureText(c.label, 13, lbl.TextStyle).Width + 14 + 6 + 16
	row := container.NewHBox(container.NewCenter(img), hgap(6), container.NewCenter(lbl))
	box := container.NewGridWrap(fyne.NewSize(w, c.boxH), container.NewCenter(row))
	return widget.NewSimpleRenderer(box)
}

var _ fyne.Tappable = (*labeledTapChip)(nil)

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
