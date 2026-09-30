package bibletext

// ON A PHONE OR TABLET NO SHEET STARTS PARTWAY DOWN A HEADER CONTROL.
//
// A sheet may cover a header control entirely or leave it alone; starting
// partway down one is the defect (headerClearance). A desktop window keeps
// every sheet below the header. A touch device's sheets were sized to the
// safe area alone, and where a sheet's top edge then fell depended on the
// device's insets: on an iPhone, whose status bar is deep, the tall sheets
// start above the header and the short ones below it, but a phone with a
// shallow status bar puts the header higher, and the same sheets started a
// few points down the header. On a 1080x2410 Android phone at its default
// 420 dpi (a canvas about 360x803) the tops of the title's letters showed
// above Settings, the cross-references, a long verse of the day and the
// audio source menu; on a shorter or wider screen the Go to chip's upper arc
// and the tops of the sparkle and gear showed above others.
//
// So on a touch device a sheet whose top edge would land partway down a
// control may move to one of the two places the iPhone's sheets already
// take. A sheet at the tallest the screen allows it covers the header, as the
// tall sheets do on an iPhone. One sized to its content opens below the
// header, as the translation picker does on an iPhone, and scrolls inside
// itself if it must, unless there is less than touchSheetLeastBelow under the
// header (a phone on its side); then it covers the header too. A card, which
// cannot be shorter than its content, grows to cover the header.
//
// A move has to be worth its cost, or the sheet stays exactly where it was.
// The defect is a few points of a control showing above a sheet's edge; a
// move must not trade it for a worse one. So a sheet covers the header only
// where its foot stays above the bottom inset, and only where it then leaves
// wholly alone every control it left alone before: a sheet is narrower than
// the header, and one grown upwards past a control it had cleared would cut
// that control with its side edge instead. And a sheet opens below the
// header only where it keeps its content: a card or a sheet that does not
// scroll no shorter than its content, and a sheet that scrolls with at least
// two thirds of its scrolling part, and never less than touchSheetLeastBody
// of it (touchSheetKeeps). Where the header's depth is a large share of the
// screen, on an iPhone on its side, opening below the header cost a sheet up
// to seven tenths of what it shows. Covering the header makes a sheet taller,
// which only adds to its scroll, unless the sheet lays its content out
// afresh for the height it has: the translation picker pins its sentences
// under the rows wherever there is room for them, and grown to cover the
// header it found that room and pinned them: on a 667x375 phone with a
// translation under evaluation, the scroll that shows its rows went from
// 123 units to 54. A sheet covers the header only where its scroll keeps
// what it would have to keep below it.
//
// Only the top edge is the defect. A sheet covering the header is narrower
// than the header, so its sides can pass through a control at the header's
// edge: on a phone held upright a tall sheet's left edge, 20 points in,
// passes through the first letter of the title, which starts at 14.8. That
// is how the iPhone's tall sheets, the Go to picker and the iPad's Settings
// have always sat, under the scrim that dims everything beside a sheet, and
// it is left as it is; only a move that newly does it is refused.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// touchSheetsKeepClearOfHeader turns the rule on. A seam: the host tests
// open every sheet with it off as well, and hold each sheet that did not
// start partway down a control exactly where it was.
var touchSheetsKeepClearOfHeader = true

// touchSheetsAlwaysCover makes a sheet that would start partway down a
// control cover the header whatever that costs. A seam: the host tests open
// each sheet that crosses a control this way as well, and measure what
// covering would leave it, rather than take the rule's word for it.
var touchSheetsAlwaysCover = false

// touchSheetLeastBelow is the shortest a sheet is made to open below the
// header: the shortest aiPanelSize lets a panel be. With less room than that
// under the header, which is where a phone on its side is, the sheet covers
// the header instead.
const touchSheetLeastBelow = 240

// touchSheetKeepShare and touchSheetLeastBody are what a sheet that scrolls
// must keep of its scrolling part to open below the header: two thirds of
// it, and never less than 120 units of it, unless it had less, when it keeps
// all of it (touchSheetKeeps). With three-button navigation on a 420 dpi
// Android phone, opening below the header costs Settings, the
// cross-references and an AI answer between a fifth and a quarter of
// theirs, and they move; on an iPhone on its side it would cost the
// translation picker seven tenths of its rows and Settings more than half of
// its own, and they stay where they were.
const (
	touchSheetKeepShare = 2.0 / 3
	touchSheetLeastBody = 120
)

// headerBand is the app header on the canvas: its edges, and the part of
// each of its controls that is drawn.
type headerBand struct {
	top, bottom float32
	marks       []drawnMark
}

// drawnMark is the drawn part of one header control, on the canvas. foot is
// where its letters end but for their descenders, the baseline, for a line
// of text, and y1 for any other control.
type drawnMark struct{ x0, x1, y0, y1, foot float32 }

// touchHeaderBand returns the header a touch device's sheet has to keep
// clear of, and false when there is none: on a desktop window (whose sheets
// headerClearance places), and when no header is on screen — full-screen
// reading, the loading screen.
func touchHeaderBand(state *AppState) (headerBand, bool) {
	if !touchSheetsKeepClearOfHeader || state == nil || state.header == nil || state.window == nil ||
		!fyne.CurrentDevice().IsMobile() {
		return headerBand{}, false
	}
	cnv := state.window.Canvas()
	h := state.header
	if cnv == nil || !h.Visible() || !objectInTree(cnv.Content(), h) {
		return headerBand{}, false
	}
	at := canvasPosition(cnv, h)
	band := headerBand{top: at.Y, bottom: at.Y + h.Size().Height}
	for _, m := range state.headerMarks {
		if m == nil || !m.Visible() {
			continue
		}
		off, sz, foot := drawnPart(m)
		p := canvasPosition(cnv, m).Add(off)
		band.marks = append(band.marks, drawnMark{p.X, p.X + sz.Width, p.Y, p.Y + sz.Height, p.Y + foot})
	}
	return band, true
}

// canvasPosition is where o lies on cnv. A mobile driver reports an object's
// position from the canvas's interactive area, subtracting the safe area's
// origin; a sheet is laid out from the canvas's own origin. (A desktop
// window's interactive area starts at the origin, so this is the driver's
// own answer there.)
func canvasPosition(cnv fyne.Canvas, o fyne.CanvasObject) fyne.Position {
	inset, _ := cnv.InteractiveArea()
	return fyne.CurrentApp().Driver().AbsolutePositionForObject(o).Add(inset)
}

// drawnPart is the part of a header control's box that is drawn, as an
// offset into the box and a size, and how far down that part the control's
// letters end but for their descenders. A line of text draws its letters
// below the leading its line box keeps above them (half of what the box
// holds beyond the text size), stands them on its baseline, and hangs its
// descenders down to the box's foot; the translation line's box is centred
// in a cell padded well past it (versionPickerAnchor); an icon button draws
// its icon at the theme's inline icon size in the middle of the button; the
// Go to chip's outline straddles its box's edge, so it reaches a unit past
// it. Each holds every pixel the control draws
// (TestHeaderDrawnPartsHoldTheirPixels).
func drawnPart(o fyne.CanvasObject) (fyne.Position, fyne.Size, float32) {
	sz := o.Size()
	letters := func(text string, size float32, style fyne.TextStyle, box fyne.Size, at fyne.Position) (fyne.Position, fyne.Size, float32) {
		line, baseline := fyne.CurrentApp().Driver().RenderedTextSize(text, size, style, nil)
		lead := (line.Height - size) / 2
		return at.AddXY(0, (box.Height-line.Height)/2+lead), fyne.NewSize(line.Width, line.Height-lead), baseline - lead
	}
	switch v := o.(type) {
	case *canvas.Text:
		return letters(v.Text, v.TextSize, v.TextStyle, sz, fyne.Position{})
	case *versionPickerAnchor:
		w := fyne.MeasureText(v.text, v.size, fyne.TextStyle{}).Width
		cell := fyne.NewSize(w+tapTextHPad, v.size+26)
		return letters(v.text, v.size, fyne.TextStyle{}, cell, fyne.NewPos(tapTextHPad/2, 0))
	case *widget.Button:
		if v.Text == "" && v.Icon != nil {
			icon := v.Theme().Size(theme.SizeNameInlineIcon)
			return fyne.NewPos((sz.Width-icon)/2, (sz.Height-icon)/2), fyne.NewSquareSize(icon), icon
		}
	case *fyne.Container:
		return fyne.NewPos(-1, -1), sz.AddWidthHeight(2, 2), sz.Height + 2
	}
	return fyne.Position{}, sz, sz.Height
}

// cuts reports whether a sheet spanning x0..x1 whose top edge is at top
// starts partway down one of the controls it spans.
func (b headerBand) cuts(x0, x1, top float32) bool {
	for _, m := range b.marks {
		if m.x1 <= x0 || m.x0 >= x1 {
			continue
		}
		if top > m.y0 && top < m.y1 {
			return true
		}
	}
	return false
}

// partly is the set of controls, one bit each in the order of marks, that a
// sheet spanning x0..x1 and top..bottom covers in part: it overlaps the
// control's drawn part without holding all of it. An edge that runs along a
// control's edge is not in it. With letters set, a line of text is taken
// down to its baseline only, since an edge through the room left for
// descenders may cross no letter at all (few letters have one): the set is
// then the controls the sheet surely covers in part, and without it every
// control it may.
func (b headerBand) partly(x0, x1, top, bottom float32, letters bool) uint {
	var set uint
	for i, m := range b.marks {
		y1 := m.y1
		if letters {
			y1 = m.foot
		}
		overlaps := m.x1 > x0 && m.x0 < x1 && y1 > top && m.y0 < bottom
		holds := m.x0 >= x0 && m.x1 <= x1 && m.y0 >= top && y1 <= bottom
		if overlaps && !holds {
			set |= 1 << i
		}
	}
	return set
}

// passesThrough reports whether a sheet spanning x0..x1 moved from was..
// wasBottom to top..bottom covers in part a control it did not cover in
// part where it was: every control it may cover in part must be one it
// certainly did (partly).
func (b headerBand) passesThrough(x0, x1, was, wasBottom, top, bottom float32) bool {
	return b.partly(x0, x1, top, bottom, false)&^b.partly(x0, x1, was, wasBottom, true) != 0
}

// touchSheetKeeps reports whether a sheet whose scrolling part is body tall
// keeps enough of it when the sheet is made by shorter: two thirds of it,
// and at least touchSheetLeastBody of it, or all of it if it had less than
// that.
func touchSheetKeeps(body, by float32) bool {
	left := body - by
	return left >= body*touchSheetKeepShare && left >= min(body, touchSheetLeastBody)
}

// touchSheetHeight is the height to give popup, a sheet w wide that Fyne
// centres on the canvas, when h is the height it would be given: h itself,
// unless on a touch device that puts the sheet's top edge partway down a
// header control and a better place is open to it. capped says h is the
// tallest the screen lets the sheet be, its content wanting more; such a
// sheet covers the header. Otherwise the sheet opens below the header if it
// can be that short and touchSheetLeastBelow allows, and covers the header
// if not.
//
// give is the part of the sheet that takes whatever height the sheet has
// beyond its MinSize, its scroll, or nil when nothing in it scrolls. A sheet
// that moves keeps what its scroll shows, as touchSheetKeeps asks. Below the
// header a sheet is shorter, and it is the scroll that gives up the height:
// the sheet opens there only if its scroll keeps enough. Over the header a
// sheet is taller, and its scroll only gains, unless the sheet lays its
// content out afresh for its height. relayout, when not nil, is how such a
// sheet is laid out: it lays the sheet out as its site would for a height t
// and returns the least the sheet can then be and the height of its scroll.
// The translation picker's sentences follow the rows into the scroll where
// the pinned part would not fit, and are pinned again where it would. The
// sheet opens below the header only if it can be that short, and covers the
// header only if its scroll keeps enough at that height. (Below the header
// what the scroll keeps is still taken as all the height the sheet gives up:
// the sentences only join the rows in the scroll there, so the rows keep at
// least that much.) relayout moves the sheet's content, so a site that passes
// it lays the sheet out again for the height it is given.
//
// A centred sheet ends as far above the canvas's foot as it starts below its
// top, so one that covers the header ends the header's depth above the foot.
// Where the bottom inset is deeper than that — a phone on its side with a
// home indicator, a phone with a navigation bar taller than its status bar —
// the sheet's foot would be under the inset, as good as clipped
// (sheetMaxHeight), and it does not cover the header. Nor does it where the
// taller sheet's sides would pass through a control the sheet as given left
// wholly alone, which on a phone on its side is the title: the move would
// trade the cut it removes for another. A sheet with no better place is
// left as it was.
//
// The sheet's box is the larger of the size it is given and its MinSize,
// since Fyne lays a modal out at no less than its content wants, so that is
// what is measured.
func touchSheetHeight(state *AppState, popup *widget.PopUp, w, h float32, capped bool, give fyne.CanvasObject, relayout func(t float32) (least, scroll float32)) float32 {
	band, ok := touchHeaderBand(state)
	if !ok || popup == nil {
		return h
	}
	cs := popup.Canvas.Size()
	minSize := popup.MinSize()
	box := fyne.NewSize(w, h).Max(minSize)
	x0, x1 := (cs.Width-box.Width)/2, (cs.Width+box.Width)/2
	top := (cs.Height - box.Height) / 2
	if !band.cuts(x0, x1, top) {
		return h
	}
	below := cs.Height - 2*(band.bottom+sheetHeaderGap)
	cover := cs.Height - 2*band.top
	if touchSheetsAlwaysCover {
		return cover
	}
	// What the sheet's scroll shows as given: the scroll takes the sheet's
	// height beyond its MinSize.
	var shows float32
	if give != nil {
		shows = box.Height - minSize.Height + give.MinSize().Height
	}
	// Whether the sheet may open below the header, decided once.
	var belowOK *bool
	fitsBelow := func() bool {
		if belowOK == nil {
			ok := below > 0
			switch {
			case !ok:
			case give != nil:
				ok = touchSheetKeeps(shows, box.Height-below)
			default:
				ok = below >= minSize.Height // nothing scrolls: its content must fit
			}
			if ok && relayout != nil {
				least, _ := relayout(below)
				ok = below >= least
			}
			belowOK = &ok
		}
		return *belowOK
	}
	// Whether the sheet may cover the header, decided once.
	var coverOK *bool
	coverFits := func() bool {
		if coverOK == nil {
			pos, area := sheetArea(popup.Canvas)
			ok := cs.Height-band.top <= pos.Y+area.Height &&
				!band.passesThrough(x0, x1, top, cs.Height-top, band.top, cs.Height-band.top)
			if ok && give != nil && relayout != nil {
				_, scroll := relayout(cover)
				ok = touchSheetKeeps(shows, shows-scroll)
			}
			coverOK = &ok
		}
		return *coverOK
	}
	switch {
	case !capped && below >= touchSheetLeastBelow && fitsBelow():
		return below
	case coverFits():
		return cover
	case fitsBelow():
		return below
	}
	return h
}

// touchSheetTop is the top edge to give a sheet shown where it is put (not
// centred) at x, w wide and h tall, when y is where it would go: y itself,
// unless on a touch device that is partway down a header control. Then it
// opens below the header if it fits there above the foot of the area a
// sheet is sized to (sheetArea), and at the header's top edge, covering it,
// if not, unless its sides would then pass through a control it left wholly
// alone at y (touchSheetHeight); then it stays at y.
func touchSheetTop(state *AppState, cnv fyne.Canvas, x, w, y, h float32) float32 {
	band, ok := touchHeaderBand(state)
	if !ok || !band.cuts(x, x+w, y) {
		return y
	}
	if touchSheetsAlwaysCover {
		return band.top
	}
	pos, sz := sheetArea(cnv)
	if below := band.bottom + sheetHeaderGap; below+h <= pos.Y+sz.Height-sheetBottomMargin {
		return below
	}
	if !band.passesThrough(x, x+w, y, y+h, band.top, band.top+h) {
		return band.top
	}
	return y
}

// fitTouchCard sizes popup, a card w wide that Fyne centres, to h, the
// height of its content: on a phone or tablet it covers the header's
// controls where h would start it partway down one and covering is open to
// it (touchSheetHeight). A card is as short as it can be already, so it
// never opens below the header by being shortened.
func fitTouchCard(state *AppState, popup *widget.PopUp, w, h float32) {
	popup.Resize(fyne.NewSize(w, h))
	if t := touchSheetHeight(state, popup, w, h, false, nil, nil); t != h {
		popup.Resize(fyne.NewSize(w, t))
	}
}
