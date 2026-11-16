package bibletext

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// fullScreenReadingBody keeps the native reader's padded layout. Phone
// landscape arrows can also use the empty margins immediately above and below
// their row, without moving the row or taking space from the reading pane.
func fullScreenReadingBody(state *AppState, content fyne.CanvasObject) fyne.CanvasObject {
	if !fyne.CurrentDevice().IsMobile() || !phoneLandscapeReading() {
		return container.NewPadded(container.NewBorder(fullScreenTop(state), nil, nil, nil, content))
	}
	prev, next := chapterArrowButtons(state)
	if prev == nil || next == nil {
		return container.NewPadded(container.NewBorder(fullScreenTop(state), nil, nil, nil, content))
	}
	row := fullScreenExitRowWithControls(state, container.NewHBox(prev, next))
	return landscapeReadingTouchBody(row, buildNoteBanner(state), content, [2]*iconTapButton{prev, next})
}

func landscapeReadingTouchBody(row, banner, content fyne.CanvasObject, arrows [2]*iconTapButton) fyne.CanvasObject {
	top, below := row, content
	if banner != nil {
		top = container.NewVBox(row, banner)
		below = banner
	}
	body := container.NewBorder(top, nil, nil, nil, content)
	l := &landscapeReadingTouchLayout{body: body, below: below, arrows: arrows}
	for i, arrow := range arrows {
		proxy := &landscapeChapterTouchProxy{arrow: arrow}
		proxy.ExtendBaseWidget(proxy)
		l.proxies[i] = proxy
	}
	// The original widgets remain last in the traversal, retaining their own
	// events inside their original rectangles. Only the clear margins change.
	return container.New(l, l.proxies[0], l.proxies[1], body)
}

// The original arrow handles its own rectangle. This proxy handles only
// extra margin, so its forwarded point is the arrow's local center even though
// the arrow is nested several containers below it.
type landscapeChapterTouchProxy struct {
	widget.BaseWidget
	arrow     *iconTapButton
	ancestors []fyne.CanvasObject
}

func (p *landscapeChapterTouchProxy) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (p *landscapeChapterTouchProxy) Visible() bool {
	if !p.BaseWidget.Visible() || p.arrow.disabled || len(p.ancestors) == 0 {
		return false
	}
	for _, ancestor := range p.ancestors {
		if !ancestor.Visible() {
			return false
		}
	}
	return true
}

func (p *landscapeChapterTouchProxy) Tapped(event *fyne.PointEvent) {
	if !p.Visible() {
		return
	}
	size := p.arrow.Size()
	forwarded := &fyne.PointEvent{Position: fyne.NewPos(size.Width/2, size.Height/2)}
	if event != nil {
		forwarded.AbsolutePosition = event.AbsolutePosition
	}
	p.arrow.Tapped(forwarded)
}

type landscapeReadingTouchLayout struct {
	body    *fyne.Container
	below   fyne.CanvasObject
	arrows  [2]*iconTapButton
	proxies [2]*landscapeChapterTouchProxy
}

func (l *landscapeReadingTouchLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return layout.NewPaddedLayout().MinSize([]fyne.CanvasObject{l.body})
}

func (l *landscapeReadingTouchLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	layout.NewPaddedLayout().Layout([]fyne.CanvasObject{l.body}, size)
	for _, proxy := range l.proxies {
		proxy.ancestors = nil
		proxy.Resize(fyne.Size{})
	}
	below, _, ok := landscapeReadingObjectPath(l.body, l.below, fyne.Position{})
	if !ok {
		return
	}
	padding := fyne.Max(0, theme.Padding())
	for i, arrow := range l.arrows {
		pos, ancestors, found := landscapeReadingObjectPath(l.body, arrow, fyne.Position{})
		if !found || arrow.Size().Width <= 0 || arrow.Size().Height <= 0 {
			continue
		}
		left := fyne.Max(0, pos.X)
		right := fyne.Min(size.Width, pos.X+arrow.Size().Width)
		margin := fyne.Min(padding, fyne.Max(0, (minTapTarget-arrow.Size().Height)/2))
		top := fyne.Max(0, pos.Y-margin)
		bottom := fyne.Min(size.Height, fyne.Min(below.Y, pos.Y+arrow.Size().Height+margin))
		if right <= left || bottom <= top {
			continue
		}
		l.proxies[i].ancestors = ancestors
		l.proxies[i].Move(fyne.NewPos(left, top))
		l.proxies[i].Resize(fyne.NewSize(right-left, bottom-top))
	}
}

// Only this layout's container tree is searched. A driver absolute position
// can still refer to a previous placement while a layout pass is in progress.
func landscapeReadingObjectPath(root, target fyne.CanvasObject, offset fyne.Position) (fyne.Position, []fyne.CanvasObject, bool) {
	if root == nil || !root.Visible() {
		return fyne.Position{}, nil, false
	}
	pos := offset.Add(root.Position())
	if root == target {
		return pos, []fyne.CanvasObject{root}, true
	}
	if c, ok := root.(*fyne.Container); ok {
		for _, child := range c.Objects {
			if found, path, ok := landscapeReadingObjectPath(child, target, pos); ok {
				return found, append([]fyne.CanvasObject{root}, path...), true
			}
		}
	}
	return fyne.Position{}, nil, false
}
