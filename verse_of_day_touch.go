package bibletext

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// votdShareTouchTarget gives Share the clear card padding directly above it.
// The card and its controls keep their sizes and positions; the target never
// extends sideways into the kicker or down towards the passage.
func votdShareTouchTarget(card fyne.CanvasObject, share *iconTapButton) fyne.CanvasObject {
	if !fyne.CurrentDevice().IsMobile() {
		return card
	}
	proxy := &votdShareTouchProxy{share: share}
	proxy.ExtendBaseWidget(proxy)
	l := &votdShareTouchLayout{card: card, share: share, proxy: proxy}
	// The original Share remains last, so it keeps events in its own box.
	return container.New(l, proxy, card)
}

type votdShareTouchProxy struct {
	widget.BaseWidget
	share     *iconTapButton
	ancestors []fyne.CanvasObject
}

func (p *votdShareTouchProxy) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (p *votdShareTouchProxy) Visible() bool {
	if !p.BaseWidget.Visible() || p.share.disabled || len(p.ancestors) == 0 {
		return false
	}
	for _, ancestor := range p.ancestors {
		if !ancestor.Visible() {
			return false
		}
	}
	return true
}

func (p *votdShareTouchProxy) Tapped(event *fyne.PointEvent) {
	if !p.Visible() {
		return
	}
	size := p.share.Size()
	forwarded := &fyne.PointEvent{Position: fyne.NewPos(size.Width/2, size.Height/2)}
	if event != nil {
		forwarded.AbsolutePosition = event.AbsolutePosition
	}
	p.share.Tapped(forwarded)
}

type votdShareTouchLayout struct {
	card  fyne.CanvasObject
	share *iconTapButton
	proxy *votdShareTouchProxy
}

func (l *votdShareTouchLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return l.card.MinSize()
}

func (l *votdShareTouchLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	l.card.Move(fyne.Position{})
	l.card.Resize(size)
	l.proxy.ancestors = nil
	l.proxy.Resize(fyne.Size{})
	pos, ancestors, ok := votdSharePath(l.card, l.share, fyne.Position{})
	if !ok || l.share.Size().Width <= 0 || l.share.Size().Height <= 0 {
		return
	}
	left := fyne.Max(0, pos.X)
	right := fyne.Min(size.Width, pos.X+l.share.Size().Width)
	margin := fyne.Max(0, minTapTarget-l.share.Size().Height)
	top := fyne.Max(0, pos.Y-margin)
	bottom := fyne.Min(size.Height, pos.Y+l.share.Size().Height)
	if right <= left || bottom <= top {
		return
	}
	l.proxy.ancestors = ancestors
	l.proxy.Move(fyne.NewPos(left, top))
	l.proxy.Resize(fyne.NewSize(right-left, bottom-top))
}

// Read the card's current local layout, not a cached canvas position. All
// ancestors of Share are ordinary containers; renderer internals are irrelevant.
func votdSharePath(root, share fyne.CanvasObject, offset fyne.Position) (fyne.Position, []fyne.CanvasObject, bool) {
	if root == nil || !root.Visible() {
		return fyne.Position{}, nil, false
	}
	pos := offset.Add(root.Position())
	if root == share {
		return pos, []fyne.CanvasObject{root}, true
	}
	if c, ok := root.(*fyne.Container); ok {
		for _, child := range c.Objects {
			if found, path, ok := votdSharePath(child, share, pos); ok {
				return found, append([]fyne.CanvasObject{root}, path...), true
			}
		}
	}
	return fyne.Position{}, nil, false
}
