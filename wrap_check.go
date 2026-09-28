package bibletext

// The Settings switches — the translators' footnotes, shared notes and the
// words of Jesus in red — as a check whose label breaks between words when
// its row is narrower than the label.
//
// They were widget.Check, which draws its label as one canvas.Text, and Fyne
// paints a canvas.Text at its full width whatever width it is given. At 320pt,
// the iPhone SE's width, the cards give their rows 252pt, and all three labels
// ran past their cards until the scroll the body sits in cut them off; at
// 360pt the second and third did, and at 375pt "in red" lost 8.8pt. wrapCheck
// is widget.Check itself — the same box, focus ring, colours, taps and keys —
// with a renderer that draws the label exactly where and as the check does
// wherever the label fits its row, and breaks it between words onto further
// lines, one under the other, where it does not. The box stays centred
// against the lines, as it is against one.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type wrapCheck struct {
	widget.Check
}

func newWrapCheck(label string, changed func(bool)) *wrapCheck {
	c := &wrapCheck{}
	c.Text = label
	c.OnChanged = changed
	c.ExtendBaseWidget(c)
	return c
}

// CreateRenderer wraps the check's own renderer, which keeps drawing the box,
// the focus ring and, where it fits, the label. Should the check ever draw its
// label other than as one canvas.Text, its renderer is used as it is.
func (c *wrapCheck) CreateRenderer() fyne.WidgetRenderer {
	c.ExtendBaseWidget(c)
	base := c.Check.CreateRenderer()
	r := &wrapCheckRenderer{check: c, base: base}
	for _, o := range base.Objects() {
		if t, ok := o.(*canvas.Text); ok {
			r.label = t
		}
	}
	if r.label == nil {
		return base
	}
	r.objs = append(r.objs, base.Objects()...)
	return r
}

type wrapCheckRenderer struct {
	check *wrapCheck
	base  fyne.WidgetRenderer
	label *canvas.Text   // the check's own label, drawn where it fits
	lines []*canvas.Text // the label broken, drawn where it does not
	objs  []fyne.CanvasObject
}

// lead is where the check starts its label, past the box and its focus ring:
// where widget.Check's renderer moves its text to.
func (r *wrapCheckRenderer) lead() float32 {
	th := r.check.Theme()
	return th.Size(theme.SizeNameInlineIcon) + th.Size(theme.SizeNameInnerPadding) +
		2*th.Size(theme.SizeNameInputBorder)
}

// labelLines is the label as it is drawn in a check width wide: one line
// where the whole label fits beside the box, or while width <= 0, not yet
// laid out; otherwise broken between words to the room beside the box.
func (r *wrapCheckRenderer) labelLines(width float32) []string {
	text := r.check.Text
	room := width - r.lead()
	if text == "" || width <= 0 || r.label.MinSize().Width <= room {
		return []string{text}
	}
	return statusLines(text, room, r.label.TextSize)
}

func (r *wrapCheckRenderer) Layout(size fyne.Size) {
	r.base.Layout(size)
	r.objs = append(r.objs[:0], r.base.Objects()...)
	lines := r.labelLines(size.Width)
	if len(lines) < 2 {
		r.label.Show()
		return
	}
	r.label.Hide()
	// The lines as a block, centred down the check as its one line is.
	lineH := r.label.MinSize().Height
	top := (size.Height - float32(len(lines))*lineH) / 2
	x := r.label.Position().X
	for len(r.lines) < len(lines) {
		r.lines = append(r.lines, canvas.NewText("", nil))
	}
	for i, s := range lines {
		t := r.lines[i]
		t.Text, t.Color, t.TextSize, t.TextStyle = s, r.label.Color, r.label.TextSize, r.label.TextStyle
		t.Alignment = r.label.Alignment
		t.Move(fyne.NewPos(x, top+float32(i)*lineH))
		t.Resize(fyne.NewSize(size.Width-x, lineH))
		t.Refresh()
		r.objs = append(r.objs, t)
	}
}

// MinSize is widget.Check's wherever the label fits on one line at the width
// the check was last laid out at. Where it has been broken, it is as tall as
// the lines and as wide as the widest of them beside the box. The room the
// check keeps after its label is asked for only as far as that width has it:
// a label that fits its row does not break for the sake of empty space after
// it. The check takes taps and hover across the size it last asked for, so
// that is the box and every word of the label.
func (r *wrapCheckRenderer) MinSize() fyne.Size {
	own := r.base.MinSize()
	width := r.check.Size().Width
	if width <= 0 || width >= own.Width {
		return own
	}
	lines := r.labelLines(width)
	var widest float32
	for _, l := range lines {
		widest = max(widest, fyne.MeasureText(l, r.label.TextSize, r.label.TextStyle).Width)
	}
	lead := r.lead()
	drawn := lead + widest
	after := own.Width - lead - r.label.MinSize().Width
	lineH := r.label.MinSize().Height
	return fyne.NewSize(drawn+max(0, min(after, width-drawn)), own.Height+float32(len(lines)-1)*lineH)
}

func (r *wrapCheckRenderer) Refresh() {
	r.base.Refresh()
	r.Layout(r.check.Size())
	canvas.Refresh(r.check)
}

func (r *wrapCheckRenderer) Objects() []fyne.CanvasObject { return r.objs }
func (r *wrapCheckRenderer) Destroy()                     { r.base.Destroy() }
