package bibletext

import "fyne.io/fyne/v2"

// nativeHostFrame is the rectangle the phone and tablet reading panes (the
// UITextView on iOS, the TextView in its Dialog on Android) take from their
// Fyne host: the host's position on its canvas and its size, in canvas units,
// with the canvas's scale. ok is false for a frame the pane must not take.
//
// A host not yet laid out has no frame. Nor does a host that runs past its
// canvas: the mobile driver's paint first grows the window's content to its
// minimum size wherever that minimum changed (common.Canvas.EnsureMinSize),
// and then sizes the content to the screen again (sizeContent), so a content
// whose minimum is wider than the screen is laid out twice in one paint, the
// first time wider than the screen. The phone's chapter header makes such a
// content: with John 3 on the page it asks for about 432 units, Song of
// Solomon 8 for about 500, on a Pixel 7's 360-unit canvas and an iPhone's 375
// to 402. Each paint after the minimum changed (closing Settings, a rebuild
// of the window) handed the pane a frame 425 units wide and then its real
// one, 339. On Android the first width armed a reflow that the second undid
// before the text ever laid out at it, so the reflow stayed armed: every
// scroll of the reader's was dropped, the saved position stayed where the
// reader had arrived, and turning the phone or relaunching put the reader
// back there. A placement still pending (a link's verse, a restore) was made
// in the wide layout and lost when the text narrowed again: a relaunch asked
// for verse 13 landed at verse 11. A host always ends a layout inside its
// canvas, so a frame outside it is only ever the first of those two passes,
// and the second follows in the same paint.
func nativeHostFrame(h fyne.CanvasObject) (pos fyne.Position, size fyne.Size, scale float32, ok bool) {
	d := fyne.CurrentApp().Driver()
	pos, size, scale = d.AbsolutePositionForObject(h), h.Size(), 1
	if size.Width <= 0 || size.Height <= 0 {
		return pos, size, scale, false
	}
	if c := d.CanvasForObject(h); c != nil {
		scale = c.Scale()
		if !hostFitsCanvas(pos, size, c.Size()) {
			return pos, size, scale, false
		}
	}
	return pos, size, scale, true
}

// hostFitsCanvas reports whether a rectangle at pos of size ends inside a
// canvas of size canvas, to within a unit for rounding. A canvas not yet
// sized says nothing, and the rectangle is taken. Only the far edges are
// asked about: the minimum-size pass grows the content and never moves it,
// and a host above or left of the canvas is the toolkit's first placement of
// a new window, which a placement inside it follows.
func hostFitsCanvas(pos fyne.Position, size, canvas fyne.Size) bool {
	if canvas.Width <= 0 || canvas.Height <= 0 {
		return true
	}
	const slack = 1
	return pos.X+size.Width <= canvas.Width+slack && pos.Y+size.Height <= canvas.Height+slack
}
