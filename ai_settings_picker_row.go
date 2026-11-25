package bibletext

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	pickerValueSize   float32 = 15 // bold value text size
	pickerMarkBox     float32 = 20 // theme.MenuDropDownIcon box
	pickerMarkGap     float32 = 7  // air between value text and dropdown icon ink
	pickerMinLabelGap float32 = 12 // minimum air between label ink and value ink on one line
	pickerStackLead   float32 = 4  // stacked: label line box bottom to value line box top

	// theme.MenuDropDownIcon is Material's arrow_drop_down (M7 10l5 5 5-5z on a
	// 24-unit square), so its visible ink spans x 7..17 and centres vertically
	// on y 12.5 of 24. outboundLink's arrow_outward.svg reaches x 23 of 24 in a
	// 12pt box ending one inner padding from the card's right edge (0.5pt inset).
	dropMarkInkLeft     float32 = 7.0 / 24.0
	dropMarkInkRight    float32 = 17.0 / 24.0
	dropMarkInkMiddle   float32 = 12.5 / 24.0
	outboundArrowInsetR float32 = 0.5
)

var settingsDropDownIcon = theme.NewColoredResource(theme.MenuDropDownIcon(), theme.ColorNamePlaceHolder)

// settingsPickerRow is one plain value row on a Settings card: label at the
// leading edge, bold 15pt value at the trailing edge followed by a muted
// dropdown triangle aligned with the card's inner right edge, and the whole
// 44pt row acting as a single button target. Embedding widget.Button preserves
// tap dispatch, focus traversal, keyboard activation, and Button.Text holding
// the full un-ellipsised choice value.
type settingsPickerRow struct {
	widget.Button

	state   *AppState
	label   *widget.Label
	wash    *canvas.Rectangle
	value   *canvas.Text
	mark    *canvas.Image
	hovered bool
	focused bool
}

func newSettingsPickerRow(state *AppState, label, value string) *settingsPickerRow {
	r := &settingsPickerRow{
		state: state,
		label: widget.NewLabel(label),
		wash:  canvas.NewRectangle(color.Transparent),
		value: canvas.NewText(value, theme.Color(theme.ColorNameForeground)),
		mark:  canvas.NewImageFromResource(settingsDropDownIcon),
	}
	r.Text = value
	r.wash.CornerRadius = 8
	r.value.TextSize = pickerValueSize
	r.value.TextStyle = fyne.TextStyle{Bold: true}
	r.mark.FillMode = canvas.ImageFillContain
	r.ExtendBaseWidget(r)
	return r
}

func (r *settingsPickerRow) pal() palette {
	if r.state != nil {
		return r.state.pal()
	}
	if isDark() {
		return darkPalette
	}
	return lightPalette
}

func (r *settingsPickerRow) Cursor() desktop.Cursor {
	if r.Disabled() {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

func (r *settingsPickerRow) MouseIn(e *desktop.MouseEvent) {
	r.hovered = true
	r.Button.MouseIn(e)
}

func (r *settingsPickerRow) MouseOut() {
	r.hovered = false
	r.Button.MouseOut()
}

func (r *settingsPickerRow) FocusGained() {
	r.focused = true
	r.Button.FocusGained()
}

func (r *settingsPickerRow) FocusLost() {
	r.focused = false
	r.Button.FocusLost()
}

func (r *settingsPickerRow) TypedKey(ev *fyne.KeyEvent) {
	if ev != nil && (ev.Name == fyne.KeyReturn || ev.Name == fyne.KeyEnter) {
		r.Tapped(nil)
		return
	}
	r.Button.TypedKey(ev)
}

func (r *settingsPickerRow) CreateRenderer() fyne.WidgetRenderer {
	r.ExtendBaseWidget(r)
	return &settingsPickerRowRenderer{
		row:     r,
		objects: []fyne.CanvasObject{r.wash, r.value, r.mark},
	}
}

type pickerRowGeometry struct {
	stacked      bool
	height       float32
	labelPos     fyne.Position
	labelSize    fyne.Size
	valueTop     float32
	valueHeight  float32
	valueMinLeft float32
	valueRight   float32
	valueMaxW    float32
	markPos      fyne.Position
	textBottom   float32
}

func (r *settingsPickerRow) geometry(w float32) pickerRowGeometry {
	in := r.Theme().Size(theme.SizeNameInnerPadding)
	drv := fyne.CurrentApp().Driver()
	ltSize, ltBase := drv.RenderedTextSize(r.label.Text, r.label.Theme().Size(theme.SizeNameText), fyne.TextStyle{}, nil)
	vtSize, vtBase := drv.RenderedTextSize(r.Text, pickerValueSize, fyne.TextStyle{Bold: true}, nil)

	labelH := ltSize.Height + 2*in
	var g pickerRowGeometry
	g.labelSize = fyne.NewSize(ltSize.Width+2*in, labelH)
	g.valueHeight = vtSize.Height

	markInkR := w - in - outboundArrowInsetR
	markBoxX := markInkR - dropMarkInkRight*pickerMarkBox
	markInkLeft := markBoxX + dropMarkInkLeft*pickerMarkBox
	g.valueRight = markInkLeft - pickerMarkGap

	inlineMinLeft := in + ltSize.Width + pickerMinLabelGap
	fullValueW := fyne.MeasureText(r.Text, pickerValueSize, fyne.TextStyle{Bold: true}).Width
	g.stacked = w > 0 && inlineMinLeft+fullValueW > g.valueRight

	if g.stacked {
		g.labelPos = fyne.NewPos(0, 0)
		g.valueTop = in + ltSize.Height + pickerStackLead
		g.height = max(minTapTarget, g.valueTop+vtSize.Height+in)
		g.valueMinLeft = in
	} else {
		g.height = max(minTapTarget, labelH)
		g.labelPos = fyne.NewPos(0, (g.height-labelH)/2)
		labelBaseline := g.labelPos.Y + in + ltBase
		g.valueTop = labelBaseline - vtBase
		g.valueMinLeft = inlineMinLeft
	}

	g.valueMaxW = max(0, g.valueRight-g.valueMinLeft)
	g.markPos = fyne.NewPos(markBoxX, g.valueTop+vtSize.Height/2-dropMarkInkMiddle*pickerMarkBox)
	g.textBottom = max(g.labelPos.Y+in+ltSize.Height, g.valueTop+vtSize.Height)
	return g
}

func truncatePickerText(full string, textSize float32, style fyne.TextStyle, maxW float32) string {
	if full == "" || maxW <= 0 {
		return ""
	}
	if fyne.MeasureText(full, textSize, style).Width <= maxW {
		return full
	}
	const ellipsis = "…"
	if fyne.MeasureText(ellipsis, textSize, style).Width > maxW {
		return ""
	}
	runes := []rune(full)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if fyne.MeasureText(string(runes[:mid])+ellipsis, textSize, style).Width <= maxW {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return string(runes[:lo]) + ellipsis
}

func truncatePickerValue(full string, maxW float32) string {
	return truncatePickerText(full, pickerValueSize, fyne.TextStyle{Bold: true}, maxW)
}

type pickerItemTextLayout struct {
	text  *canvas.Text
	check fyne.CanvasObject
	full  string
}

func (l *pickerItemTextLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, fyne.MeasureText("M", l.text.TextSize, l.text.TextStyle).Height)
}

func (l *pickerItemTextLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	maxW := size.Width
	if l.check != nil && l.check.Visible() {
		maxW = max(0, maxW-6)
	}
	if maxW > 0 {
		l.text.Text = truncatePickerText(l.full, l.text.TextSize, l.text.TextStyle, maxW)
	} else {
		l.text.Text = l.full
	}
	ms := l.text.MinSize()
	l.text.Move(fyne.NewPos(0, (size.Height-ms.Height)/2))
	l.text.Resize(fyne.NewSize(min(ms.Width, max(0, maxW)), ms.Height))
}

type settingsPickerRowRenderer struct {
	row     *settingsPickerRow
	objects []fyne.CanvasObject
}

func (rr *settingsPickerRowRenderer) Layout(size fyne.Size) {
	r := rr.row
	g := r.geometry(size.Width)
	r.wash.Move(fyne.NewPos(0, 0))
	r.wash.Resize(size)

	pal := r.pal()
	r.value.Color = pal.Text
	drawn := truncatePickerValue(r.Text, g.valueMaxW)
	r.value.Text = drawn
	drawnW := fyne.MeasureText(drawn, pickerValueSize, fyne.TextStyle{Bold: true}).Width
	valueX := max(g.valueMinLeft, g.valueRight-drawnW)
	r.value.Move(fyne.NewPos(valueX, g.valueTop))
	r.value.Resize(fyne.NewSize(drawnW, g.valueHeight))

	r.mark.Move(g.markPos)
	r.mark.Resize(fyne.NewSquareSize(pickerMarkBox))
}

func (rr *settingsPickerRowRenderer) MinSize() fyne.Size {
	r := rr.row
	g := r.geometry(r.Size().Width)
	return fyne.NewSize(r.label.MinSize().Width, g.height)
}

func (rr *settingsPickerRowRenderer) Refresh() {
	r := rr.row
	pal := r.pal()
	r.value.Color = pal.Text
	switch {
	case r.focused:
		r.wash.FillColor = theme.Color(theme.ColorNameFocus)
	case r.hovered:
		r.wash.FillColor = paletteHover(pal)
	default:
		r.wash.FillColor = color.Transparent
	}
	r.wash.Refresh()
	rr.Layout(r.Size())
	r.value.Refresh()
	r.mark.Refresh()
	canvas.Refresh(r)
}

func (rr *settingsPickerRowRenderer) Objects() []fyne.CanvasObject { return rr.objects }
func (rr *settingsPickerRowRenderer) Destroy()                     {}
