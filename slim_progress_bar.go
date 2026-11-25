package bibletext

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const (
	slimProgressHeight   float32 = 4
	slimProgressMaxWidth float32 = 240
)

// slimProgressLayout constrains a stock *widget.ProgressBarInfinite to a 4pt
// height and at most maxWidth, either centred or leading-aligned inside a
// plain *fyne.Container so rebuildWindow's stopInfiniteBars sweep reaches it.
type slimProgressLayout struct {
	maxWidth float32
	center   bool
}

func (l slimProgressLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, slimProgressHeight)
}

func (l slimProgressLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) == 0 {
		return
	}
	w := size.Width
	if l.maxWidth > 0 && w > l.maxWidth {
		w = l.maxWidth
	}
	if w < 0 {
		w = 0
	}
	x := float32(0)
	if l.center {
		x = (size.Width - w) / 2
	}
	objs[0].Move(fyne.NewPos(x, 0))
	objs[0].Resize(fyne.NewSize(w, slimProgressHeight))
}

func newSlimProgressBar(bar *widget.ProgressBarInfinite) *fyne.Container {
	return container.New(slimProgressLayout{maxWidth: slimProgressMaxWidth, center: true}, bar)
}

func newLeadingSlimProgressBar(bar *widget.ProgressBarInfinite) *fyne.Container {
	return container.New(slimProgressLayout{maxWidth: slimProgressMaxWidth, center: false}, bar)
}

func stopInfiniteBars(o fyne.CanvasObject) {
	switch t := o.(type) {
	case *widget.ProgressBarInfinite:
		t.Stop()
	case *widget.Activity:
		t.Stop()
	case *fyne.Container:
		for _, c := range t.Objects {
			stopInfiniteBars(c)
		}
	case *container.Scroll:
		stopInfiniteBars(t.Content)
	case *widget.PopUp:
		stopInfiniteBars(t.Content)
	case contentWrapper:
		stopInfiniteBars(t.wrappedContent())
	}
}
