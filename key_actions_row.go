package bibletext

// The row of buttons under an API key field in Settings — Paste, Test key,
// Clear — in both key sections, the assistant's and API.Bible's.
//
// It was an HBox, which lays its children out at their own widths whatever
// width it is given. The three buttons take 305.9pt with the gaps between
// them. At 320pt, the iPhone SE's width, the key card gives its rows 252pt,
// so Clear ran past the card and the scroll the body sits in cut it off at
// its edge; at 375pt the rows get 299pt, and Clear ran 6.9pt past the end of
// the key field above it, over the card's padding to its border. This row
// lays the buttons out exactly as the HBox did wherever they fit, and where
// they do not, the button that would run past the row starts a line of its
// own below: Clear goes under Paste. Nothing is shrunk, and the labels and
// icons are the ones the HBox drew.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

// keyActionsRow is the row both key sections put under their field, so the
// two cannot come to lay their buttons out differently.
func keyActionsRow(buttons ...fyne.CanvasObject) *fyne.Container {
	return container.New(&wrapRowLayout{}, buttons...)
}

// wrapRowLayout places its visible children left to right at their MinSize
// widths, with the theme's padding between them as an HBox does, and starts a
// new line, one padding below, with a child that would run past the width it
// is given. A child wider than the whole width has a line to itself.
type wrapRowLayout struct {
	// width is the width the row was last laid out at, 0 until then. The
	// height the row asks for depends on it, as a status line's does
	// (status_line.go): the Settings sheet measures itself twice whenever it
	// sizes itself, and the second measure sees the lines the first layout
	// broke the row into.
	width float32
}

// wrapRowLines breaks the visible objects into lines no wider than width.
// width <= 0 means not yet laid out: everything is on one line.
func wrapRowLines(objs []fyne.CanvasObject, width, pad float32) [][]fyne.CanvasObject {
	var lines [][]fyne.CanvasObject
	var used float32
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		w := o.MinSize().Width
		if n := len(lines); n > 0 && (width <= 0 || used+pad+w <= width) {
			lines[n-1] = append(lines[n-1], o)
			used += pad + w
			continue
		}
		lines = append(lines, []fyne.CanvasObject{o})
		used = w
	}
	return lines
}

func wrapRowLineHeight(line []fyne.CanvasObject) float32 {
	var h float32
	for _, o := range line {
		h = max(h, o.MinSize().Height)
	}
	return h
}

func (l *wrapRowLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	l.width = size.Width
	pad := theme.Padding()
	var y float32
	for _, line := range wrapRowLines(objs, size.Width, pad) {
		h := wrapRowLineHeight(line)
		var x float32
		for _, o := range line {
			w := o.MinSize().Width
			o.Move(fyne.NewPos(x, y))
			o.Resize(fyne.NewSize(w, h))
			x += w + pad
		}
		y += h + pad
	}
}

// MinSize is as wide as the widest child, so the row never forces the card
// wider than one button, and as tall as the lines it breaks into at the width
// it was last laid out at.
func (l *wrapRowLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	pad := theme.Padding()
	var size fyne.Size
	for i, line := range wrapRowLines(objs, l.width, pad) {
		if i > 0 {
			size.Height += pad
		}
		size.Height += wrapRowLineHeight(line)
		for _, o := range line {
			size.Width = max(size.Width, o.MinSize().Width)
		}
	}
	return size
}
