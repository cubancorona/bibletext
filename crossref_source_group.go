package bibletext

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// crossRefSourceGroup keeps the references belonging to one source verse,
// heading or episode together. Its numeric key is an origin verse, not a
// publisher's word marker; the margin grows with its measured width instead
// of reducing the type size used for two-letter publisher marks.
type crossRefSourceGroup struct {
	widget.BaseWidget
	id       string
	open     bool
	body     *fyne.Container
	header   *tapCard
	icon     *widget.Icon
	key      *canvas.Text
	labels   []*crossRefPrinted
	stem     *canvas.Rectangle
	box      *fyne.Container
	head     *fyne.Container
	fold     fyne.CanvasObject
	relayout *crossRefRelayout
	build    func() []fyne.CanvasObject
	built    bool
	margin   float32
	width    float32
	pal      palette
	pin      *canvas.Text
	pinned   float32
}

const (
	crossRefSourceKeyGap  float32 = 7
	crossRefSourceIconGap float32 = 4
	crossRefSourceLineGap float32 = 2
)

func newCrossRefSourceGroup(id, key, title, origin, summary string, open bool, pal palette, relayout *crossRefRelayout, build func() []fyne.CanvasObject) *crossRefSourceGroup {
	sz := crossRefPanelSizes()
	r := &crossRefSourceGroup{id: id, pal: pal, relayout: relayout, build: build}
	if key != "" {
		r.key = canvas.NewText(key, pal.Accent)
		r.key.TextStyle = fyne.TextStyle{Bold: true}
		r.key.TextSize = sz.reference
		r.key.FontSource = crossRefZeroFace(r.key.TextStyle)
		r.margin = max(sz.letterMargin, r.key.MinSize().Width+crossRefSourceKeyGap)
		r.stem = canvas.NewRectangle(crossRefStemColour(pal))
		r.stem.CornerRadius = crossRefStemWidth / 2
		r.stem.Hide()
	}
	if title != "" {
		p := newCrossRefPrinted(title, pal)
		p.size, p.style, p.color, p.wordWrap = sz.reference, fyne.TextStyle{Bold: true}, pal.Text, true
		r.labels = append(r.labels, p)
	}
	if origin != "" {
		p := newCrossRefPrinted(origin, pal)
		p.size, p.wordWrap = sz.caption, true
		r.labels = append(r.labels, p)
	}
	if summary != "" {
		r.labels = append(r.labels, newCrossRefPrinted(summary, pal))
	}
	r.icon = widget.NewIcon(crossRefGroupIcon(false))
	r.fold = crossRefFoldIcon(r.icon)
	parts := make([]fyne.CanvasObject, 0, len(r.labels)+2)
	if r.key != nil {
		parts = append(parts, r.key)
	}
	for _, p := range r.labels {
		parts = append(parts, p)
	}
	parts = append(parts, r.fold)
	r.head = container.New(crossRefSourceHeaderLayout{r}, parts...)
	r.header = newTapCard(r.head, crossRefHover(pal), r.toggle)
	r.body = container.New(layout.NewCustomPaddedVBoxLayout(0))
	r.body.Hide()
	objects := []fyne.CanvasObject{r.header, r.body}
	if r.stem != nil {
		objects = append(objects, r.stem)
	}
	r.box = container.New(crossRefSourceGroupLayout{r}, objects...)
	r.ExtendBaseWidget(r)
	if open {
		r.open = true
		r.showBody()
		r.icon.SetResource(crossRefGroupIcon(true))
	}
	return r
}

// sourceTextHeight measures wrapping before layout, so a long heading or
// full citation summary gives the disclosure row its complete height on the
// first pass at a new width.
func sourceTextHeight(p *crossRefPrinted, width float32) float32 {
	drv := fyne.CurrentApp().Driver()
	face := crossRefZeroFace(p.style)
	measure := func(s string) float32 {
		size, _ := drv.RenderedTextSize(s, p.textSize(), p.style, face)
		return size.Width
	}
	var rows []string
	if p.wordWrap {
		rows = wordWrappedRows(p.Text, max(width, 1), measure)
	} else {
		rows = printedRows(p.Text, max(width, 1), measure)
	}
	if len(rows) == 0 {
		return 0
	}
	size, _ := drv.RenderedTextSize("Ag", p.textSize(), p.style, face)
	n := float32(len(rows))
	return n*size.Height + (n-1)*crossRefPrintedLeading
}

func (r *crossRefSourceGroup) textWidth(width float32) float32 {
	return max(1, width-r.margin-crossRefFoldIconSize-crossRefSourceIconGap)
}

func (r *crossRefSourceGroup) textHeight(width float32) float32 {
	var h float32
	for i, p := range r.labels {
		if i > 0 {
			h += crossRefSourceLineGap
		}
		h += sourceTextHeight(p, r.textWidth(width))
	}
	return h
}

func (r *crossRefSourceGroup) headerHeight(width float32) float32 {
	h := max(r.textHeight(width), crossRefFoldIconSize)
	if r.key != nil {
		h = max(h, r.key.MinSize().Height)
	}
	return max(44, h+2*crossRefRowPad)
}

type crossRefSourceHeaderLayout struct{ row *crossRefSourceGroup }

func (l crossRefSourceHeaderLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	r := l.row
	var minWidth float32
	for _, p := range r.labels {
		minWidth = max(minWidth, p.MinSize().Width)
	}
	minWidth += r.margin + crossRefFoldIconSize + crossRefSourceIconGap
	width := r.width
	if width <= 0 {
		width = crossRefPrintedGuess + r.margin + crossRefFoldIconSize + crossRefSourceIconGap
	}
	return fyne.NewSize(minWidth, r.headerHeight(width))
}

func (l crossRefSourceHeaderLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	r := l.row
	r.width = size.Width
	y := (size.Height - r.textHeight(size.Width)) / 2
	for _, p := range r.labels {
		h := sourceTextHeight(p, r.textWidth(size.Width))
		p.Move(fyne.NewPos(r.margin, y))
		p.Resize(fyne.NewSize(r.textWidth(size.Width), h))
		y += h + crossRefSourceLineGap
	}
	if r.key != nil {
		r.key.Move(fyne.NewPos(0, r.keyTop()))
		r.key.Resize(r.key.MinSize())
	}
	r.fold.Move(fyne.NewPos(max(0, size.Width-crossRefFoldIconSize), 0))
	r.fold.Resize(fyne.NewSize(crossRefFoldIconSize, size.Height))
}

type crossRefSourceGroupLayout struct{ row *crossRefSourceGroup }

func (l crossRefSourceGroupLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	r := l.row
	size := r.header.MinSize()
	if r.body.Visible() {
		b := r.body.MinSize()
		size.Width = max(size.Width, b.Width+r.margin)
		size.Height += b.Height
	}
	return size
}

func (l crossRefSourceGroupLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	r := l.row
	r.width = size.Width
	top := r.headerHeight(size.Width)
	r.header.Move(fyne.NewPos(0, 0))
	r.header.Resize(fyne.NewSize(size.Width, top))
	if r.body.Visible() {
		r.body.Move(fyne.NewPos(r.margin, top))
		r.body.Resize(fyne.NewSize(max(0, size.Width-r.margin), max(0, size.Height-top)))
		r.placeStem(size.Height)
	}
}

func (r *crossRefSourceGroup) keyLine() float32 {
	if r.key == nil {
		return 0
	}
	_, line := fyne.CurrentApp().Driver().RenderedTextSize(r.key.Text, r.key.TextSize, r.key.TextStyle, r.key.FontSource)
	return line
}

func (r *crossRefSourceGroup) keyTop() float32 {
	if r.key == nil {
		return 0
	}
	if len(r.labels) == 0 {
		return (r.header.Size().Height - r.key.MinSize().Height) / 2
	}
	p := r.labels[0]
	_, line := fyne.CurrentApp().Driver().RenderedTextSize("Ag", p.textSize(), p.style, crossRefZeroFace(p.style))
	return p.Position().Y + line - r.keyLine()
}

func (r *crossRefSourceGroup) placeStem(height float32) {
	if r.stem == nil {
		return
	}
	at := r.keyTop()
	if r.pinned > 0 && r.pin != nil {
		at = r.pinned
		r.pin.Move(fyne.NewPos(0, at))
		r.pin.Resize(r.pin.MinSize())
	}
	from := at + r.keyLine() + crossRefStemGap
	x := (r.key.MinSize().Width - crossRefStemWidth) / 2
	r.stem.Move(fyne.NewPos(max(0, x), from))
	r.stem.Resize(fyne.NewSize(crossRefStemWidth, max(0, height-crossRefStemFoot-from)))
}

func (r *crossRefSourceGroup) keepLetterInView(into float32) {
	if r.key == nil {
		return
	}
	var pinned float32
	if r.open {
		at := min(into+crossRefPinTop, r.Size().Height-crossRefStemFoot-crossRefStemGap-r.keyLine())
		if at > r.keyTop() {
			pinned = at
		}
	}
	if pinned == r.pinned {
		return
	}
	r.pinned = pinned
	if pinned > 0 {
		if r.pin == nil {
			r.pin = canvas.NewText(r.key.Text, r.pal.Accent)
			r.pin.TextStyle, r.pin.TextSize, r.pin.FontSource = r.key.TextStyle, r.key.TextSize, r.key.FontSource
			r.box.Add(r.pin)
		}
		r.pin.Show()
		r.key.Hide()
	} else {
		if r.pin != nil {
			r.pin.Hide()
		}
		r.key.Show()
	}
	r.placeStem(r.Size().Height)
	r.key.Refresh()
	r.stem.Refresh()
	if r.pin != nil {
		r.pin.Refresh()
	}
}

func (r *crossRefSourceGroup) showBody() {
	if !r.built {
		r.built = true
		if r.build != nil {
			r.body.Objects = r.build()
		}
		r.build = nil
	}
	r.body.Show()
	if r.stem != nil {
		r.stem.Show()
	}
}

func (r *crossRefSourceGroup) toggle() { r.setOpen(!r.open) }

func (r *crossRefSourceGroup) setOpen(open bool) {
	if r.open == open {
		return
	}
	r.open = open
	if open {
		r.showBody()
	} else {
		r.body.Hide()
		if r.stem != nil {
			r.stem.Hide()
		}
		r.keepLetterInView(0)
	}
	r.icon.SetResource(crossRefGroupIcon(open))
	r.box.Refresh()
	r.Refresh()
	r.relayout.run()
}

func (r *crossRefSourceGroup) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.box)
}

var _ crossRefLetterKeeper = (*crossRefSourceGroup)(nil)
