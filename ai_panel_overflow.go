package bibletext

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// aiErrorOverflow keeps the ordinary panel composition available while an
// exceptionally short error card puts its variable-height content in one scroll.
// Only the title and Close remain fixed; no control or text changes size.
type aiErrorOverflow struct {
	message    *container.Scroll
	scroll     *container.Scroll
	top        fyne.CanvasObject
	bottom     fyne.CanvasObject
	normalBody fyne.CanvasObject
	readable   float32
	active     bool
}

func newAIErrorOverflow(title fyne.CanvasObject, context []fyne.CanvasObject, message, recovery, disclaimer fyne.CanvasObject, report, copy, close *widget.Button, messageScroll *container.Scroll, normalBody fyne.CanvasObject) *aiErrorOverflow {
	items := append([]fyne.CanvasObject(nil), context...)
	items = append(items, message, recovery, widget.NewSeparator(), disclaimer,
		container.NewHBox(report, layout.NewSpacer(), copy))
	column := container.NewVBox(items...)
	return &aiErrorOverflow{
		message: messageScroll,
		scroll:  container.NewVScroll(container.New(squeezeWidthLayout{}, column)),
		top:     container.NewVBox(title),
		bottom: container.NewVBox(widget.NewSeparator(),
			container.NewHBox(layout.NewSpacer(), close)),
		normalBody: normalBody,
		readable:   widget.NewLabel("x").MinSize().Height,
	}
}

func (o *aiErrorOverflow) show(content *fyne.Container) {
	o.active = true
	content.Layout = layout.NewBorderLayout(o.top, o.bottom, nil, nil)
	content.Objects = []fyne.CanvasObject{o.scroll, o.top, o.bottom}
	content.Refresh()
}

func (o *aiErrorOverflow) restore(content, header, footer, body *fyne.Container) {
	if !o.active {
		return
	}
	o.active = false
	body.Objects = []fyne.CanvasObject{o.normalBody}
	content.Layout = layout.NewBorderLayout(header, footer, nil, nil)
	content.Objects = []fyne.CanvasObject{body, header, footer}
	header.Refresh()
	footer.Refresh()
	body.Refresh()
	content.Refresh()
}
