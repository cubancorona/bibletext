package bibletext

// Shared AI waiting column for Study sheets, the Search tab's Find, and the
// sidebar Find layout.
//
// The column is a plain *fyne.Container driven by aiWaitLayout (never a custom
// widget or ThemeOverride), so rebuildWindow's tree walk over Containers and
// Scrolls always reaches its *widget.ProgressBarInfinite.

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	aiWaitMeasure          float32 = 260
	aiWaitBarMaxW          float32 = 240
	aiWaitTop              float32 = 12
	aiWaitStatusToBar      float32 = 13
	aiWaitBarH             float32 = 4
	aiWaitBarToCaption     float32 = 10
	aiWaitCaptionPitch     float32 = 16
	aiWaitCaptionToChoices float32 = 10
	aiWaitCaptionToCancel  float32 = 16
	aiWaitCancelToChoices  float32 = 20
	aiWaitNoChoicesBottom  float32 = 6
)

const aiWaitHintCopy = "Capable models can take a minute or more."

type aiWaitColumn struct {
	status  *canvas.Text
	bar     *widget.ProgressBarInfinite
	barWrap *fyne.Container
	barCell *fyne.Container
	line    *aiModelLine
	hint    *widget.RichText
	speed   *aiSpeedControl
	cancel  fyne.CanvasObject
	box     *fyne.Container
	lay     *aiWaitLayout
}

func aiWaitIsFastModel(m aiModelInUse) bool {
	info, ok := providerByID(m.provider)
	return ok && m.model != "" && strings.EqualFold(m.model, info.FastModel)
}

func newAIWaitColumn(
	pal palette,
	status string,
	bar *widget.ProgressBarInfinite,
	line *aiModelLine,
	speed *aiSpeedControl,
	cancel fyne.CanvasObject,
	widthHint float32,
	seed aiModelInUse,
) *aiWaitColumn {
	c := &aiWaitColumn{bar: bar, line: line, speed: speed, cancel: cancel}
	c.status = canvas.NewText(status, pal.Text)
	c.status.TextSize = theme.TextSize()
	c.status.Alignment = fyne.TextAlignCenter

	c.hint = centeredCaption(aiWaitHintCopy)
	c.setFast(aiWaitIsFastModel(seed))
	if line != nil && line.String() == "" && seed.model != "" {
		line.reserve(seed)
	}

	c.lay = &aiWaitLayout{col: c, width: widthHint}
	m := c.lay.measure(widthHint)
	objs := []fyne.CanvasObject{c.status}
	if bar != nil {
		c.barWrap = container.NewGridWrap(fyne.NewSize(min(aiWaitBarMaxW, m), aiWaitBarH), bar)
		c.barCell = container.NewCenter(c.barWrap)
		objs = append(objs, c.barCell)
	}
	objs = append(objs, line.box, c.hint)
	if cancel != nil {
		objs = append(objs, cancel)
	}
	if speed != nil {
		speed.setWidth(m)
		objs = append(objs, speed.box)
	}
	c.box = container.New(c.lay, objs...)
	return c
}

func (c *aiWaitColumn) setFast(fast bool) {
	if fast {
		c.hint.Hide()
	} else {
		c.hint.Show()
	}
}

func (c *aiWaitColumn) show(m aiModelInUse) {
	if c == nil || m.model == "" {
		return
	}
	was := c.hint.Visible()
	c.setFast(aiWaitIsFastModel(m))
	c.line.show(m)
	if c.speed != nil {
		c.speed.show(m)
	}
	if c.hint.Visible() != was && c.line.relayout != nil {
		c.line.relayout()
	}
}

// statusFoot is the vertical extent from the column's top through the status
// line box, used as fitBody's least height so the waiting line is never cut.
func (c *aiWaitColumn) statusFoot() float32 {
	return aiWaitTop + c.status.MinSize().Height
}

type aiWaitLayout struct {
	col   *aiWaitColumn
	width float32
}

func (l *aiWaitLayout) measure(width float32) float32 {
	if width <= 0 {
		width = aiWaitMeasure
	}
	return min(aiWaitMeasure, width)
}

func (l *aiWaitLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	totalH, _, _ := l.place(l.width, false)
	return fyne.NewSize(l.measure(l.width), totalH)
}

func (l *aiWaitLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	l.width = size.Width
	l.place(size.Width, true)
}

// boundsAt returns the column's current height, closed height and fully
// expanded height at width, used by Find's scroll layout so opening
// "Faster options" unfolds downward without shifting Cancel.
func (l *aiWaitLayout) boundsAt(width float32) (totalH, baseH, openH float32) {
	if width > 0 {
		l.width = width
	}
	return l.place(l.width, false)
}

func (l *aiWaitLayout) place(width float32, apply bool) (totalH, baseH, openH float32) {
	c := l.col
	if width <= 0 {
		width = aiWaitMeasure
	}
	m := l.measure(width)
	left := (width - m) / 2
	pad := theme.InnerPadding()
	put := func(o fyne.CanvasObject, x, y, w, h float32) {
		if apply {
			o.Move(fyne.NewPos(x, y))
			o.Resize(fyne.NewSize(w, h))
		}
	}

	// Status line box at aiWaitTop.
	sh := c.status.MinSize().Height
	put(c.status, 0, aiWaitTop, width, sh)
	statusFoot := aiWaitTop + sh

	// Slim progress bar cell, when present (Study and Search-tab Find; absent on sidebar Find).
	var captionTop float32
	if c.barCell != nil && c.bar != nil && c.bar.Visible() {
		bw := min(aiWaitBarMaxW, m)
		c.barWrap.Layout = layout.NewGridWrapLayout(fyne.NewSize(bw, aiWaitBarH))
		barTop := statusFoot + aiWaitStatusToBar
		put(c.barCell, 0, barTop, width, aiWaitBarH)
		if apply {
			c.barCell.Layout.Layout(c.barCell.Objects, fyne.NewSize(width, aiWaitBarH))
		}
		captionTop = barTop + aiWaitBarH + aiWaitBarToCaption
	} else {
		captionTop = statusFoot + aiWaitStatusToBar
	}

	// Caption block: modelLine (one or more wrapped rows) followed at
	// aiWaitCaptionPitch by the one-line hint when not on the fast model.
	if lay, ok := c.line.box.Layout.(modelLineLayout); !ok || lay.width != m || lay.measure != c.line.measure {
		c.line.box.Layout = modelLineLayout{width: m, measure: c.line.measure}
	}
	lineH := c.line.box.MinSize().Height
	put(c.line.box, left, captionTop-pad, m, lineH)
	if apply {
		c.line.box.Layout.Layout(c.line.box.Objects, fyne.NewSize(m, lineH))
	}
	rows := c.line.rowsAt(m)
	lastRowTop := captionTop + float32(rows-1)*captionRow()
	captionFoot := captionTop + float32(rows)*captionRow()

	if c.hint.Visible() {
		if c.hint.Size().Width != m {
			c.hint.Resize(fyne.NewSize(m, c.hint.MinSize().Height))
		}
		hh := c.hint.MinSize().Height
		hintTop := lastRowTop + aiWaitCaptionPitch
		put(c.hint, left, hintTop-pad, m, hh)
		hintRows := captionLines(hh)
		captionFoot = hintTop + float32(hintRows)*captionRow()
	}

	hasChoices := c.speed != nil && c.speed.box.Visible() && len(c.speed.box.Objects) > 0
	if c.cancel != nil && c.cancel.Visible() {
		cs := c.cancel.MinSize()
		cancelTop := captionFoot + aiWaitCaptionToCancel
		put(c.cancel, (width-cs.Width)/2, cancelTop, cs.Width, cs.Height)
		cancelFoot := cancelTop + cs.Height
		if !hasChoices {
			return cancelFoot, cancelFoot, cancelFoot
		}
		c.speed.setWidth(m)
		ch := c.speed.box.MinSize().Height
		choicesTop := cancelFoot + aiWaitCancelToChoices
		put(c.speed.box, left, choicesTop, m, ch)
		if apply {
			c.speed.box.Layout.Layout(c.speed.box.Objects, fyne.NewSize(m, ch))
		}
		return choicesTop + ch, choicesTop + c.speed.closedHeight(), choicesTop + c.speed.openHeight()
	}

	if hasChoices {
		c.speed.setWidth(m)
		ch := c.speed.box.MinSize().Height
		choicesTop := captionFoot + aiWaitCaptionToChoices
		put(c.speed.box, left, choicesTop, m, ch)
		if apply {
			c.speed.box.Layout.Layout(c.speed.box.Objects, fyne.NewSize(m, ch))
		}
		return choicesTop + ch, choicesTop + c.speed.closedHeight(), choicesTop + c.speed.openHeight()
	}

	total := captionFoot + aiWaitNoChoicesBottom
	return total, total, total
}

// scrollExpandedChoicesIntoView scrolls sc only when expanding "Faster options"
// pushes the revealed action rows past sc's visible viewport. Refresh runs
// before ScrollToOffset so Fyne's scroll renderer updates its content bounds
// instead of clamping or resetting the offset.
func scrollExpandedChoicesIntoView(sc *container.Scroll, speed *aiSpeedControl) {
	if sc == nil || speed == nil || !speed.box.Visible() || len(speed.rows) == 0 {
		return
	}
	sc.Refresh()
	if sc.Content != nil {
		minSz := sc.Content.MinSize()
		sc.Content.Resize(fyne.NewSize(max(minSz.Width, sc.Size().Width), max(minSz.Height, sc.Size().Height)))
	}
	vh := sc.Size().Height
	if vh <= 0 {
		return
	}
	firstBox := speed.rows[0].box
	lastBox := speed.rows[len(speed.rows)-1].box
	firstPos, ok1 := speedObjectPosIn(sc.Content, firstBox)
	lastPos, ok2 := speedObjectPosIn(sc.Content, lastBox)
	if !ok1 || !ok2 {
		return
	}
	lastBottom := lastPos.Y + lastBox.Size().Height
	if lastBox.Size().Height <= 0 {
		lastBottom = lastPos.Y + lastBox.MinSize().Height
	}
	if lastBottom <= sc.Offset.Y+vh {
		return
	}
	wantY := min(firstPos.Y, max(0, lastBottom-vh))
	sc.ScrollToOffset(fyne.NewPos(sc.Offset.X, wantY))
}

func speedObjectPosIn(root, target fyne.CanvasObject) (fyne.Position, bool) {
	if root == target {
		return fyne.NewPos(0, 0), true
	}
	if root == nil {
		return fyne.Position{}, false
	}
	switch v := root.(type) {
	case *fyne.Container:
		for _, k := range v.Objects {
			if p, ok := speedObjectPosIn(k, target); ok {
				return k.Position().Add(p), true
			}
		}
	case *container.Scroll:
		if p, ok := speedObjectPosIn(v.Content, target); ok {
			return v.Content.Position().Add(p), true
		}
	case *container.ThemeOverride:
		if p, ok := speedObjectPosIn(v.Content, target); ok {
			return v.Content.Position().Add(p), true
		}
	}
	return fyne.Position{}, false
}
