package bibletext

// The reader's audio control, in the gap to the right of the chapter navigation
// (the shared header builders place it there). Plain Fyne chrome above the native
// text overlay's frame, so it's never occluded.
//
// Collapsed it's a single speaker icon. Tapping it expands, in place, into a
// bordered card that hugs the player icons, with a muted close ✕ (opposite
// shading) tucked into its upper-right corner:
//
//	┌──────────────────✕
//	│    Narrator ▾     │   top: labeled source chip, centred above play
//	│   ⟲15  ▶/⏸  15⟳  │   bottom: skip · play/pause · skip
//	└───────────────────┘
//
// The skips dim for read-aloud (speech can't seek); the source chip opens the
// source menu, and says what is actually playing: a person + "Narrator ▾" for a
// human recording, a computer + "Voice ▾" for a synthetic one, a waveform +
// "Read aloud ▾" for on-device speech.
//
// The open card is a pop-up, and may cover other controls until it is closed.
// On a phone (chapter_header_mobile.go) the gap it is centred on is narrower
// than the card, so the open card lies over the next-chapter arrow and the
// full-screen button; on a narrow desktop or Android fallback toolbar
// (reading.go's chapterHeader) it can reach over the chapter block. Both
// headers draw the open card after everything it can cover (the phone header
// lifts it only while it is open: audioControl's onRender), so the card is
// painted above those controls, and its fill is opaque in both palettes
// (SurfaceAlt, alpha 255), so nothing beneath shows through it. It also
// swallows every tap and touch across its whole rectangle (tapShield below):
// a tap on the card's own control reaches that control, and a tap anywhere
// else on it does nothing, rather than landing on whatever the card hides.
// And a small icon control whose glyph the card touches (or, for the phone's
// full-screen button, its focus highlight) is hidden altogether while the
// card is open (cardCover below), so no fragment of it shows beside the card;
// the heading and the chapter line, which name the chapter, are only covered
// where the card lies over them. The ✕ collapses the card back to the speaker
// while narration carries on, and it is the only way to, so both headers keep
// the whole open card on the screen: the desktop toolbar by placing it inside
// its right column, the phone header by moving it in from its right edge
// where centring would run it past (which took the ✕ off a 320-wide canvas in
// nearly every book).

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/mobile"
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
//
// onRender, when set, runs after every render with whether the card is open.
// Both headers use it to hide, while the card is open, the small controls it
// touches, and to bring them back when it closes (cardCover); the phone header
// also draws the control above its neighbours while the card is open and
// beneath them while it is the speaker (chapter_header_mobile.go). The desktop
// toolbar already draws it after everything it can reach.
func audioControl(state *AppState, boxH float32, onRender func(open bool)) fyne.CanvasObject {
	host := container.NewStack()
	var rebuild func()
	rebuild = func() {
		host.Objects = []fyne.CanvasObject{audioControlContent(state, boxH, rebuild)}
		host.Refresh()
		if onRender != nil {
			onRender(audioPanelOpen)
		}
	}
	// fireChange marshals onChange onto the UI goroutine, so rebuild runs there.
	gAudio.setOnChange(rebuild)
	rebuild()
	// Reserve the EXPANDED card's footprint permanently. The collapsed↔expanded
	// swap happens in place (host.Objects mutation) and deliberately never
	// re-lays the surrounding header, so a card bigger than the host was drawn
	// beyond it while its taps landed elsewhere (the shipped bug: the visible ▶
	// hit skip-back / nothing). A fixed cell sized to the card makes expansion
	// geometry-neutral: taps land, header never reflows.
	//
	// The cell is reserved open or closed, and the header lays out around it,
	// so where it is wider than the header's gap it overlaps the controls
	// either side (and the phone header moves the open card's cell in from its
	// right edge where it would pass it). While closed only the speaker in it
	// takes taps, and the controls around it keep theirs; only the open card
	// claims the whole cell.
	probe := buildAudioCard(state, audioTTS, false, false, false, false, audioCardCallbacks{})
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
	displayKind, displayRec := gAudio.effectiveSource(state)
	if show, k := gAudio.indicator(fp); show {
		displayKind = k
	}
	synthetic := displayKind == audioRecorded && recordingIsSynthetic(state.CurrentVersion, displayRec)

	return buildAudioCard(state, displayKind, synthetic, playing, buffering, displayKind == audioRecorded,
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

// buildAudioCard assembles the expanded transport card: labeled source chip on
// top (centred in the width the corner ✕ leaves it), skip/play/skip below, close
// ✕ tucked in the upper-right corner.
// While buffering, the play slot shows a SPINNER instead of a glyph — silence
// behind a pause glyph reads as "broken" — and the slot ignores taps until the
// stream resolves to playing/paused/failed.
func buildAudioCard(state *AppState, displayKind audioKind, synthetic, playing, buffering, canSeek bool, cb audioCardCallbacks) fyne.CanvasObject {
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
		// The close ✕ is overlaid on the corner through a Stack, so it adds nothing
		// to the top row's width. The row reserves its footprint explicitly instead
		// — plus a small visible gap — or the centred chip slides under it: at the
		// card's real width the longer "Read aloud ▾" ran 8px into the ✕, and even
		// "Narrator ▾" ended flush against it with no clearance at all.
		closeCellSize  = 26
		closeClearance = 4
	)
	// The source selector is a LABELED chip, not a bare glyph: a 16px person/waveform
	// icon alone (the old control) signalled neither "this is a button" nor "narrators
	// live here" — the narrator menu was effectively undiscoverable. Text + ▾ fixes
	// both, and the wide chip pulls mis-taps away from the play button beneath it.
	srcLabel := "Read aloud ▾"
	if displayKind == audioRecorded {
		// "Narrator" would claim a person read it; a machine voice is a "Voice".
		srcLabel = "Narrator ▾"
		if synthetic {
			srcLabel = "Voice ▾"
		}
	}
	src := newLabeledTapChip(state, audioSourceIcon(displayKind, synthetic), srcLabel, srcRowH, cb.onSrc)
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
	top := container.NewHBox(layout.NewSpacer(), src, layout.NewSpacer(), hgap(closeCellSize+closeClearance))
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
		container.NewGridWrap(fyne.NewSize(closeCellSize, closeCellSize), container.NewStack(xBg, container.NewCenter(xGlyph))),
		cb.onClose,
	)
	corner := container.NewVBox(container.NewHBox(layout.NewSpacer(), xCell), layout.NewSpacer())

	// The shield goes FIRST in the stack: the stack sizes it to the whole card,
	// and it is drawn beneath the card's own controls, so a tap on one of those
	// still reaches that control (Fyne's hit test gives a tap to the last
	// matching object in draw order). It exists only in the open card, never
	// in the collapsed speaker, so the speaker's reserved cell does not block
	// the controls around it.
	return container.NewStack(newTapShield(), box, corner)
}

// audioSourceIcon maps the loaded audio source to its glyph: a person for a
// recorded HUMAN narration, a computer for a recorded synthetic voice, a waveform
// for on-device read-aloud (TTS). The person is the app's documented mark for a
// human reader (icons_embed.go, README), so a machine voice must not wear it.
func audioSourceIcon(kind audioKind, synthetic bool) fyne.Resource {
	if kind == audioRecorded {
		if synthetic {
			return theme.ComputerIcon()
		}
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

// tapShield is the open audio card's floor: an invisible widget spanning the
// card, beneath its controls, that claims every tap and touch landing on the
// card and does nothing with it. Without it the card's background — a plain
// canvas.Rectangle, which no driver matches — let a tap fall through to
// whatever control the card was covering: the next-chapter arrow or the
// full-screen button on a phone. (Those are hidden now wherever the card
// touches their glyphs, cardCover below; the shield still keeps the heading,
// the chapter line and the edge of a tap box the card overlaps from taps
// landing on the card.)
//
// Fyne's drivers walk the tree in draw order and hand an event to the LAST
// object under the point that matches the event's interfaces, and they do not
// clip a child to its parent's rectangle. So the shield has to match every
// event a covered control could otherwise take, and be drawn after all of
// them (the headers draw the open card last for this):
//
//   - fyne.Tappable matches a mobile tap-up and a desktop click. Having won
//     the match the shield is the object the driver asks about a long press, a
//     double tap or a right click, and as it is neither SecondaryTappable nor
//     DoubleTappable those do nothing — they are not passed on to anything
//     beneath.
//   - mobile.Touchable matches a mobile tap-down and move. A tap-down is how a
//     Touchable control starts a long press of its own (the Android fallback
//     pane's selectableParagraph arms its menu timer on TouchDown), so the
//     shield must win that match too; winning the move match also means a
//     drag begun on the card starts nothing beneath it.
//
// It is deliberately not fyne.Focusable, which would put an invisible stop in
// the keyboard's tab order, nor fyne.Draggable. Nor does it take the desktop
// pointer's hover or cursor (desktop.Hoverable, desktop.Cursorable). The
// mobile driver dispatches neither, so on a phone, a tablet or the Android
// fallback pane there is nothing to take; and on the desktop toolbar the card
// sits beside the one control that reacts to hover, the focus toggle, and can
// reach over only the chapter block, none of whose controls reacts to the
// pointer. A covered widget.Button would only light its hover fill beneath
// the opaque card, where it cannot be seen. chapter_header_cover_test.go
// checks that premise, so a control that does react to the pointer arriving
// under the card fails there rather than going unnoticed.
//
// Keyboard focus is not the shield's to stop, and does not need to be. The
// one focusable control the card can reach is the phone header's full-screen
// button, and it is hidden whenever the card touches what it draws
// (cardCover), which Fyne's Tab passes over; it gives the focus up if it held
// it, and takes no key while it is hidden (coverButton.TypedKey). Where the
// card overlaps only the edge of its tap box, the button's glyph is whole
// beside the card and Tab reaches it as before. But a focused button fills
// its whole box with its focus highlight, which the card would cut in two, so
// the moment the focus lands on it there it is hidden as well, and it stays
// hidden until the card closes.
//
// One thing the shield does not stop: the second tap of a double tap on the
// ✕. The ✕ is a plain Tappable, which the mobile driver fires on the first
// tap-up without waiting to see whether a second follows, so the first tap
// closes the card and the second lands on whatever the ✕ lay over, back in
// view — on most phones the full-screen button, which the card was hiding —
// as any tap goes to what is on top when it lands. A ✕ that waited for a
// second tap (fyne.DoubleTappable) would take both, at the cost of the
// double-tap interval on every close; that choice is left open
// (docs/BACKLOG.md).
type tapShield struct {
	widget.BaseWidget
}

func newTapShield() *tapShield {
	s := &tapShield{}
	s.ExtendBaseWidget(s)
	return s
}

func (s *tapShield) Tapped(*fyne.PointEvent)        {}
func (s *tapShield) TouchDown(*mobile.TouchEvent)   {}
func (s *tapShield) TouchUp(*mobile.TouchEvent)     {}
func (s *tapShield) TouchCancel(*mobile.TouchEvent) {}

// CreateRenderer draws nothing: the card's look is its frame's, and the shield
// only has to be there to be hit.
func (s *tapShield) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewWithoutLayout())
}

var (
	_ fyne.Tappable    = (*tapShield)(nil)
	_ mobile.Touchable = (*tapShield)(nil)
)

// cardCover hides, for as long as the narration card is open, each of a
// chapter header's small icon controls that the card touches: the copy icon
// and the two chapter arrows in both headers, and the phone header's
// full-screen button. (The desktop toolbar's focus toggle sits beside the
// card's cell in the same box, so the card never reaches it.) Drawing the card
// above those controls and shielding its rectangle covered only the part of a
// control beneath it, and the card's edge seldom falls between two controls:
// on a 402-wide iPhone in Matthew 27 the → arrow's tail showed to the left of
// the card and a bracket of the full-screen glyph to its right, and on a
// 360-wide Android phone in Psalm 23 half the copy icon showed to its left. A
// fragment of an icon beside the card is not covered. So a control the card
// touches goes altogether while the card is open: it is not drawn, no tap
// anywhere on its box reaches it, inside the card or out, and Tab passes it by
// (Fyne's focus traversal walks only what is visible, and a control that held
// the focus gives it up). When the card closes, it is back.
//
// "Touches" is decided by what the control draws, read off its renderer's
// objects, not by its tap box, which is about twice as wide: a control whose
// box the card overlaps only at its edge still reads whole beside the card,
// and is left exactly as it is. What an icon control draws is its glyph, the
// canvas.Image among those objects (glyphBounds). The full-screen button, a
// widget.Button, also fills its whole box with its focus highlight while it
// has the keyboard focus (fillBounds, filler), and the card would cut that
// fill in two just as it cut a glyph, so a button whose fill the card touches
// is hidden as well, and gives the focus up. Its fill goes with the focus, and
// the glyph alone would then bring it back beside the card the moment it had
// gone, so the cover remembers where the fill was (lit) and keeps the button
// hidden wherever the card lies over that, until the card closes. The heading
// and the chapter line are never hidden: they name the chapter, and where the
// card lies over them it covers that part.
//
// Hiding moves nothing. Fyne's own layouts leave a hidden child out of their
// MinSize, and its boxes leave it out of their layout too, so a bare Hide on
// the next arrow would shorten the chapter row, and at the row's next layout
// narrow the chapter block, beside which the card is laid out, and so move the
// card; and a hidden full-screen button would take with it the width of the
// phone header's right column, which fixes the header's right edge and so
// where the card stops. So each hideable control sits in a slot (coverSlot)
// that keeps its size whether it is shown or not, and only the control's own
// visibility changes. Opening or closing the card stays a repaint of the row
// (and, on the phone, a move of the card), never a relayout of the header.
//
// What the card touches is worked out again whenever it can change: when the
// card opens or closes (the audio control's onRender); after every layout of
// the header row (coverRowLayout), which is where a width change or a rotation
// lands, and where the phone header moves the card in from its right edge; and
// when the full-screen button gains or loses the focus, which lights or puts
// out its fill. It is worked out once the row's layout is finished, never
// inside it: the row's BorderLayout resizes each part, which lays out
// everything within it, before it moves the part, so nothing in the row is in
// its final place until that layout returns.
type cardCover struct {
	row      *fyne.Container   // the header row the card and the controls are laid out in
	card     fyne.CanvasObject // the audio control's reserved cell, which the open card fills
	controls []coverable
	open     bool
	lit      map[coverable]coverRect // where the fill was of each control hidden for it, until the card closes
}

// coverRect is a rectangle: in the header row for the card, relative to the
// control for what a control draws.
type coverRect struct {
	at   fyne.Position
	size fyne.Size
}

// setOpen records whether the card is open and works out again what it
// hides. It is the desktop toolbar's onRender; the phone header's onRender
// calls it once it has moved the card.
func (c *cardCover) setOpen(open bool) {
	c.open = open
	c.update()
}

// update hides each control the open card touches and shows every other one;
// with the card closed, or before the header has a row or a card, it shows
// them all, and forgets which were lit. Show and Hide do nothing to a control
// that is already so, so an update that changes nothing repaints nothing.
//
// A control it hides gives up the focus, if it holds it and the canvas says
// so too. It asks the control, because a focus manager tells a control it has
// lost the focus before it forgets it, so a control that is losing the focus
// is still the canvas's focused object while the update its FocusLost runs is
// under way. And it asks the canvas, because while an overlay is up (a sheet,
// a dialog, the source menu) the canvas's focus is the overlay's: unfocusing
// then would take the focus from whatever the reader is using there, and leave
// the hidden control holding the focus of the page beneath. So such a control
// keeps that focus, and refuses every key while it is hidden
// (coverButton.TypedKey).
func (c *cardCover) update() {
	if c.row == nil {
		return
	}
	if !c.open {
		c.lit = nil
	}
	var card coverRect
	if c.open && c.card != nil {
		if at, ok := offsetIn(c.row, c.card); ok {
			card = coverRect{at, c.card.Size()}
		}
	}
	for _, ctl := range c.controls {
		if !c.touches(ctl, card) {
			ctl.Show()
			continue
		}
		if f, ok := ctl.(focusHolder); ok && f.hasFocus() {
			if cv := fyne.CurrentApp().Driver().CanvasForObject(ctl); cv != nil && cv.Focused() == f {
				cv.Unfocus()
			}
		}
		ctl.Hide()
	}
}

// touches reports whether the card's rectangle in the row overlaps what ctl
// draws: its glyph, the fill it draws now, or, for a control hidden for its
// fill, where that fill was. Rectangles that only meet at an edge do not
// overlap.
func (c *cardCover) touches(ctl coverable, card coverRect) bool {
	if card.size.Width <= 0 || card.size.Height <= 0 {
		return false
	}
	ctlAt, ok := offsetIn(c.row, ctl)
	if !ok {
		return false
	}
	over := func(at fyne.Position, size fyne.Size) bool {
		at = ctlAt.Add(at)
		return size.Width > 0 && size.Height > 0 &&
			at.X < card.at.X+card.size.Width && card.at.X < at.X+size.Width &&
			at.Y < card.at.Y+card.size.Height && card.at.Y < at.Y+size.Height
	}
	if at, size, ok := ctl.glyphBounds(); ok && over(at, size) {
		return true
	}
	f, ok := ctl.(filler)
	if !ok {
		return false
	}
	if at, size, ok := f.fillBounds(); ok && over(at, size) {
		if c.lit == nil {
			c.lit = map[coverable]coverRect{}
		}
		c.lit[ctl] = coverRect{at, size}
		return true
	}
	was, ok := c.lit[ctl]
	return ok && over(was.at, was.size)
}

// offsetIn is where obj lies in root, found by looking down through the
// containers root holds, shown or hidden, and adding up their positions.
func offsetIn(root *fyne.Container, obj fyne.CanvasObject) (fyne.Position, bool) {
	for _, o := range root.Objects {
		if o == obj {
			return o.Position(), true
		}
		if c, ok := o.(*fyne.Container); ok {
			if at, ok := offsetIn(c, obj); ok {
				return c.Position().Add(at), true
			}
		}
	}
	return fyne.Position{}, false
}

// coverRowLayout is a header row's layout: the row's own, a BorderLayout, and
// then, once every part of the row is in its place, the card cover worked out
// again for where that layout put everything.
type coverRowLayout struct {
	inner fyne.Layout
	cover *cardCover
}

func (l coverRowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	l.inner.Layout(objects, size)
	l.cover.update()
}

func (l coverRowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return l.inner.MinSize(objects)
}

// coverable is a header control the open card can hide: it can say where it
// draws its glyph, relative to itself, and false until it has been rendered.
type coverable interface {
	fyne.CanvasObject
	glyphBounds() (fyne.Position, fyne.Size, bool)
}

// focusHolder is a coverable that can take the keyboard focus and knows
// whether it holds it: the full-screen button.
type focusHolder interface {
	coverable
	fyne.Focusable
	hasFocus() bool
}

// filler is a coverable that can also draw a fill around its glyph, and can
// say where it draws one now, relative to itself, and false while it draws
// none: the full-screen button's focus highlight.
type filler interface {
	coverable
	fillBounds() (fyne.Position, fyne.Size, bool)
}

// coverSlot holds a control the open card can hide, at the size its row gives
// it, shown or hidden, so that hiding it moves nothing in the row.
func coverSlot(control coverable) *fyne.Container {
	return container.New(keepSizeLayout{}, control)
}

// keepSizeLayout lays its child out over the whole slot, at the place and size
// the row would have given the child itself, and asks for the child's MinSize
// whether the child is visible or not. Fyne's own layouts leave a hidden child
// out, which is right for a control that has gone and wrong for one the card
// hides only for as long as it is open.
type keepSizeLayout struct{}

func (keepSizeLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Move(fyne.Position{})
		o.Resize(size)
	}
}

func (keepSizeLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var min fyne.Size
	for _, o := range objects {
		min = min.Max(o.MinSize())
	}
	return min
}

// imageIn finds the first canvas.Image among a widget renderer's objects,
// looking down through the containers that place it, and returns its rectangle
// relative to the widget: the glyph an icon control draws. It takes the image
// whether it is visible or not, because widget.Button hides its icon when it is
// refreshed while it is hidden itself, and what matters is where the icon is
// laid out.
func imageIn(objects []fyne.CanvasObject, at fyne.Position) (fyne.Position, fyne.Size, bool) {
	for _, o := range objects {
		switch v := o.(type) {
		case *canvas.Image:
			return at.Add(v.Position()), v.Size(), true
		case *fyne.Container:
			if p, s, ok := imageIn(v.Objects, at.Add(v.Position())); ok {
				return p, s, true
			}
		}
	}
	return fyne.Position{}, fyne.Size{}, false
}

// fillIn finds the canvas.Rectangles among a widget renderer's objects that
// paint something, a fill or an outline in a colour that shows, looking down
// through the containers that place them, and returns the rectangle that
// takes them all in, relative to the widget; false where none paints. A
// widget.Button's background is such a rectangle while it is focused or
// hovered, and transparent otherwise at LowImportance.
func fillIn(objects []fyne.CanvasObject, at fyne.Position) (fyne.Position, fyne.Size, bool) {
	var from, to fyne.Position
	found := false
	var walk func(objects []fyne.CanvasObject, at fyne.Position)
	walk = func(objects []fyne.CanvasObject, at fyne.Position) {
		for _, o := range objects {
			switch v := o.(type) {
			case *canvas.Rectangle:
				if !v.Visible() || !(colourShows(v.FillColor) || (v.StrokeWidth > 0 && colourShows(v.StrokeColor))) {
					continue
				}
				p, s := at.Add(v.Position()), v.Size()
				if !found {
					from, to, found = p, p.Add(s), true
					continue
				}
				from = fyne.NewPos(min(from.X, p.X), min(from.Y, p.Y))
				to = fyne.NewPos(max(to.X, p.X+s.Width), max(to.Y, p.Y+s.Height))
			case *fyne.Container:
				walk(v.Objects, at.Add(v.Position()))
			}
		}
	}
	walk(objects, at)
	return from, fyne.NewSize(to.X-from.X, to.Y-from.Y), found
}

// colourShows reports whether c paints anything: it is set and not wholly
// transparent.
func colourShows(c color.Color) bool {
	if c == nil {
		return false
	}
	_, _, _, a := c.RGBA()
	return a > 0
}

// coverButton is a widget.Button the open card can hide: the phone header's
// full-screen button. It is a widget.Button in every respect, extended only to
// keep the renderer Fyne makes for it, so that it can say where that renderer
// draws its icon and its focus highlight; to have the cover work out again
// what the card hides when it gains or loses the focus, which lights or puts
// out that highlight; and to take no key while it is hidden.
type coverButton struct {
	widget.Button
	renderer      fyne.WidgetRenderer
	focused       bool   // between FocusGained and FocusLost
	onFocusChange func() // the cover's update, run once the button has gained or lost the focus
}

func newCoverButton(icon fyne.Resource, tapped func()) *coverButton {
	b := &coverButton{}
	b.Icon = icon
	b.OnTapped = tapped
	b.ExtendBaseWidget(b)
	return b
}

// CreateRenderer is widget.Button's own. Fyne calls it again for a widget
// whose renderer it has let go, so the renderer kept is the one drawn.
func (b *coverButton) CreateRenderer() fyne.WidgetRenderer {
	b.renderer = b.Button.CreateRenderer()
	return b.renderer
}

// FocusGained is widget.Button's, which lights the highlight at once, and
// then the cover's update, which hides the button if the open card lies over
// that highlight.
func (b *coverButton) FocusGained() {
	b.focused = true
	b.Button.FocusGained()
	if b.onFocusChange != nil {
		b.onFocusChange()
	}
}

// FocusLost is widget.Button's, and then the cover's update.
func (b *coverButton) FocusLost() {
	b.focused = false
	b.Button.FocusLost()
	if b.onFocusChange != nil {
		b.onFocusChange()
	}
}

func (b *coverButton) hasFocus() bool { return b.focused }

// TypedKey is widget.Button's, save that a button the open card hides takes
// no key: Space would otherwise press it (widget.Button.TypedKey taps without
// asking whether it is shown). The cover takes the focus from the button when
// it hides it, but where an overlay was up at the time the button kept the
// focus of the page beneath (cardCover.update), and has it again once the
// overlay has gone; so a key that reaches it hidden makes it give the focus
// up instead. (widget.Button takes no runes, so TypedRune needs no such care.)
func (b *coverButton) TypedKey(ev *fyne.KeyEvent) {
	if !b.Visible() {
		if c := fyne.CurrentApp().Driver().CanvasForObject(b); c != nil && c.Focused() == b {
			c.Unfocus()
		}
		return
	}
	b.Button.TypedKey(ev)
}

// glyphBounds is where the button draws its icon: the canvas.Image among its
// renderer's objects, which widget.Button centres in its padded box.
func (b *coverButton) glyphBounds() (fyne.Position, fyne.Size, bool) {
	if b.renderer == nil {
		return fyne.Position{}, fyne.Size{}, false
	}
	return imageIn(b.renderer.Objects(), fyne.Position{})
}

// fillBounds is where the button paints around its icon: its background,
// which widget.Button spreads across its whole box and fills only while it is
// focused (the mobile driver, the only one that shows the phone header, never
// hovers it).
func (b *coverButton) fillBounds() (fyne.Position, fyne.Size, bool) {
	if b.renderer == nil {
		return fyne.Position{}, fyne.Size{}, false
	}
	return fillIn(b.renderer.Objects(), fyne.Position{})
}

var (
	_ filler      = (*coverButton)(nil)
	_ focusHolder = (*coverButton)(nil)
	_ coverable   = (*iconTapButton)(nil)
)
