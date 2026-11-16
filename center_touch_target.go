package bibletext

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// centerTouchTarget draws content exactly as NewCenter does. On a touch device,
// spare space already allocated to the cell can accept taps for the centered
// control. It reserves no extra space and never extends beyond the cell.
func centerTouchTarget(content fyne.CanvasObject) fyne.CanvasObject {
	if !fyne.CurrentDevice().IsMobile() {
		return container.NewCenter(content)
	}
	if _, ok := content.(fyne.Tappable); !ok {
		return container.NewCenter(content)
	}
	proxy := &centerTouchProxy{content: content}
	proxy.ExtendBaseWidget(proxy)
	return container.New(&centerTouchLayout{content: content, proxy: proxy}, proxy, content)
}

type centerTouchLayout struct {
	content fyne.CanvasObject
	proxy   *centerTouchProxy
}

func (l *centerTouchLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return layout.NewCenterLayout().MinSize([]fyne.CanvasObject{l.content})
}

func (l *centerTouchLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	layout.NewCenterLayout().Layout([]fyne.CanvasObject{l.content}, size)
	// Only the proxy takes the extra space. Keeping its actual bounds small
	// lets taps elsewhere fall through to an enclosing card or other control.
	width := fyne.Max(0, fyne.Min(size.Width, minTapTarget))
	height := fyne.Max(0, fyne.Min(size.Height, minTapTarget))
	l.proxy.Resize(fyne.NewSize(width, height))
	l.proxy.Move(fyne.NewPos((size.Width-width)/2, (size.Height-height)/2))
}

// centerTouchProxy is drawn before the control, so the original control still
// handles taps within its own rectangle, focus and pointer interactions.
type centerTouchProxy struct {
	widget.BaseWidget
	content fyne.CanvasObject
}

func (p *centerTouchProxy) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (p *centerTouchProxy) Visible() bool {
	if !p.BaseWidget.Visible() || !p.content.Visible() {
		return false
	}
	disabled, ok := p.content.(fyne.Disableable)
	return !ok || !disabled.Disabled()
}

func (p *centerTouchProxy) Tapped(event *fyne.PointEvent) {
	if !p.Visible() {
		return
	}
	if disabled, ok := p.content.(fyne.Disableable); ok && disabled.Disabled() {
		return
	}
	forwarded := fyne.PointEvent{}
	size := p.content.Size()
	forwarded.Position = fyne.NewPos(size.Width/2, size.Height/2)
	if event != nil {
		forwarded.AbsolutePosition = event.AbsolutePosition
		position := event.Position.Add(p.Position()).Subtract(p.content.Position())
		if position.X >= 0 && position.Y >= 0 && position.X < size.Width && position.Y < size.Height {
			forwarded.Position = position
		}
	}
	p.content.(fyne.Tappable).Tapped(&forwarded)
}

var _ fyne.Tappable = (*centerTouchProxy)(nil)
