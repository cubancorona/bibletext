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
// control is moved to one of the two places the iPhone's sheets already
// take. A sheet at the tallest the screen allows it covers the header, as the
// tall sheets do on an iPhone. One sized to its content opens below the
// header, as the translation picker does on an iPhone, and scrolls inside
// itself if it must, unless that would leave it shorter than it can be or
// than touchSheetLeastBelow (a phone on its side); then it covers the header
// too. A card that cannot be shorter than its content and cannot clear the
// header, which only a phone on its side has too little room for, is made
// taller instead, to cover it. Covering never takes a sheet's foot under
// the bottom inset (touchSheetHeight). A sheet that already clears the
// header's controls or covers them stays exactly where it was.

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

// touchSheetLeastBelow is the shortest a sheet is made to open below the
// header: the shortest aiPanelSize lets a panel be. With less room than that
// under the header, which is where a phone on its side is, the sheet covers
// the header instead.
const touchSheetLeastBelow = 240

// headerBand is the app header on the canvas: its edges, and the part of
// each of its controls that is drawn.
type headerBand struct {
	top, bottom float32
	marks       []drawnMark
}

// drawnMark is the drawn part of one header control, on the canvas.
type drawnMark struct{ x0, x1, y0, y1 float32 }

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
		off, sz := drawnPart(m)
		p := canvasPosition(cnv, m).Add(off)
		band.marks = append(band.marks, drawnMark{p.X, p.X + sz.Width, p.Y, p.Y + sz.Height})
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
// offset into the box and a size. A line of text draws its letters below
// the leading its line box keeps above them (half of what the box holds
// beyond the text size) and its descenders down to the box's foot; the
// translation line's box is centred in a cell padded well past it
// (versionPickerAnchor); an icon button draws its icon at the theme's inline
// icon size in the middle of the button; the Go to chip's outline straddles
// its box's edge, so it reaches a unit past it. Each holds every pixel the
// control draws (TestHeaderDrawnPartsHoldTheirPixels).
func drawnPart(o fyne.CanvasObject) (fyne.Position, fyne.Size) {
	sz := o.Size()
	letters := func(text string, size float32, style fyne.TextStyle, box fyne.Size, at fyne.Position) (fyne.Position, fyne.Size) {
		line := fyne.MeasureText(text, size, style)
		lead := (line.Height - size) / 2
		return at.AddXY(0, (box.Height-line.Height)/2+lead), fyne.NewSize(line.Width, line.Height-lead)
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
			return fyne.NewPos((sz.Width-icon)/2, (sz.Height-icon)/2), fyne.NewSquareSize(icon)
		}
	case *fyne.Container:
		return fyne.NewPos(-1, -1), sz.AddWidthHeight(2, 2)
	}
	return fyne.Position{}, sz
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

// touchSheetHeight is the height to give popup, a sheet w wide that Fyne
// centres on the canvas, when h is the height it would be given: h itself,
// unless on a touch device that puts the sheet's top edge partway down a
// header control. capped says h is the tallest the screen lets the sheet be,
// its content wanting more; such a sheet covers the header. Otherwise the
// sheet opens below the header if least(), the shortest it can be, and
// touchSheetLeastBelow both allow, and covers the header if not.
//
// A centred sheet ends as far above the canvas's foot as it starts below
// its top, so one that covers the header ends the header's depth above the
// foot. Where the bottom inset is deeper than that — a phone on its side
// with a home indicator, a phone with a navigation bar taller than its
// status bar — the sheet's foot would be under the inset, and it opens below
// the header instead wherever it can be that short. A sheet that fits in
// neither place takes the least height it can be if its edge then misses
// the controls, in the header's lower margin; failing that it is left where
// it was, since a foot under the home indicator is as good as clipped
// (sheetMaxHeight), which is worse than an edge across a control. That is
// only the share image preview, whose image keeps 200 units on a phone, on
// an iPhone on its side with the header showing. least is asked at most
// once, and only when it decides.
//
// The sheet's box is the larger of the size it is given and its MinSize,
// since Fyne lays a modal out at no less than its content wants, so that is
// what is measured.
func touchSheetHeight(state *AppState, popup *widget.PopUp, w, h float32, capped bool, least func() float32) float32 {
	band, ok := touchHeaderBand(state)
	if !ok || popup == nil {
		return h
	}
	cs := popup.Canvas.Size()
	box := fyne.NewSize(w, h).Max(popup.MinSize())
	if !band.cuts((cs.Width-box.Width)/2, (cs.Width+box.Width)/2, (cs.Height-box.Height)/2) {
		return h
	}
	shortest := float32(-1)
	leastH := func() float32 {
		if shortest < 0 {
			shortest = 0
			if least != nil {
				shortest = least()
			}
		}
		return shortest
	}
	below := cs.Height - 2*(band.bottom+sheetHeaderGap)
	fitsBelow := func() bool { return below > 0 && below >= leastH() }
	pos, area := sheetArea(popup.Canvas)
	coverFits := cs.Height-band.top <= pos.Y+area.Height
	switch {
	case !capped && below >= touchSheetLeastBelow && fitsBelow():
		return below
	case coverFits:
		return cs.Height - 2*band.top
	case fitsBelow():
		return below
	}
	if l := leastH(); l > 0 && l < box.Height && !band.cuts((cs.Width-box.Width)/2, (cs.Width+box.Width)/2, (cs.Height-l)/2) {
		return l
	}
	return h
}

// touchSheetTop is the top edge to give a sheet shown where it is put (not
// centred) at x, w wide and h tall, when y is where it would go: y itself,
// unless on a touch device that is partway down a header control. Then it
// opens below the header if it fits there above the foot of the area a
// sheet is sized to (sheetArea), and at the header's top edge, covering it,
// if not.
func touchSheetTop(state *AppState, cnv fyne.Canvas, x, w, y, h float32) float32 {
	band, ok := touchHeaderBand(state)
	if !ok || !band.cuts(x, x+w, y) {
		return y
	}
	pos, sz := sheetArea(cnv)
	if below := band.bottom + sheetHeaderGap; below+h <= pos.Y+sz.Height-sheetBottomMargin {
		return below
	}
	return band.top
}

// fitTouchCard sizes popup, a card w wide that Fyne centres, to h, the
// height of its content: on a phone or tablet it covers the header's
// controls where h would start it partway down one (touchSheetHeight). A card
// is as short as it can be already, so it never opens below the header by
// being shortened.
func fitTouchCard(state *AppState, popup *widget.PopUp, w, h float32) {
	popup.Resize(fyne.NewSize(w, h))
	if t := touchSheetHeight(state, popup, w, h, false, func() float32 { return popup.MinSize().Height }); t != h {
		popup.Resize(fyne.NewSize(w, t))
	}
}
