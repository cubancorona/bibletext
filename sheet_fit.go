package bibletext

// Sizing a card sheet so it can never run off the bottom of the screen.
//
// THE BUG THIS EXISTS TO PREVENT. A card sheet (the Settings sheet, the audio
// source menu, the note offer) is a non-modal widget.PopUp sized from its
// content's MinSize and pinned near the top of the canvas. Nothing in Fyne
// clamps that. popUpRenderer.Layout takes
//
//	innerSize := p.innerSize.Max(p.MinSize())
//
// so the popup is NEVER smaller than its content wants, whatever you pass to
// Resize; and when the result is taller than the canvas it pins innerPos.Y to 0
// and lets the rest hang off the bottom of the screen, unreachable. Fyne's own
// source marks the spot: "TODO here we may need a scroller as it's longer than
// our canvas".
//
// So a height clamp ALONE does nothing. The growable part of the sheet has to
// sit in a container.Scroll — that is what drops the content's MinSize to
// something small, which is what finally lets an explicit Resize take effect.
// Clamp without scroll = no change; scroll without clamp = no change. Both.
//
// Settings shipped clipped on an iPhone 16 Pro Max — the largest phone there
// is — because a section was added to a sheet that was already near the limit
// and nothing measured it. These two functions are pure arithmetic precisely so
// the measurement is testable without a canvas.

import "fyne.io/fyne/v2"

// sheetBottomMargin is the breathing room left under a sheet, matching the 16pt
// gap the sheets leave above themselves.
const sheetBottomMargin = 16

// sheetChromeWidth is how much of a sheet's width goes to its own frame — the
// surface's rounded border plus the padding inside it, both edges. Measured on a
// laid-out sheet, not guessed: a 354pt card gives its body 340pt.
const sheetChromeWidth = 14

// squeezeWidthLayout hands its child the full width it is given — even when that
// is narrower than the child's MinSize — and reports no width of its own.
//
// It exists because of the OTHER half of the scroll trap. Fyne's scroll renderer
// sizes its content with
//
//	c.Resize(c.MinSize().Max(size))
//
// in BOTH dimensions. So a vertical scroll silently WIDENS its content to
// whatever the content asks for and clips the excess sideways — with no
// horizontal scrollbar to reach it, the direction being vertical-only. Drop a
// body straight into a VScroll and any row wanting more width than the sheet has
// loses its right-hand end: doing exactly that cost the Settings sheet the end of
// "Get a key ↗", leaving "Get a ke".
//
// The plain container this replaced (a Border centre) squeezed instead: it
// resized the child to the width available and let the child cope. This layout
// restores that, so putting a body inside a scroll cannot change how it behaves
// horizontally. Reporting width 0 is the mechanism — that is what stops the
// scroll widening the content in the first place.
type squeezeWidthLayout struct{}

func (squeezeWidthLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	h := float32(0)
	for _, o := range objs {
		if m := o.MinSize(); m.Height > h {
			h = m.Height
		}
	}
	return fyne.NewSize(0, h)
}

func (squeezeWidthLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
}

// minSheetHeight is a floor for absurd canvases (a mid-rotation frame, a desktop
// window dragged to nothing). Below this the sheet is unusable either way, and a
// zero or negative height would collapse it entirely.
const minSheetHeight = 160

// sheetMaxHeight is the tallest a sheet pinned at y=top may be and still sit
// wholly on screen.
//
// It measures against the INTERACTIVE area, not the raw canvas: on a phone the
// home indicator and status bar are canvas the reader cannot usefully touch, and
// a sheet whose last row lands under the home indicator is as good as clipped.
// safeH <= 0 means the platform reports no insets (desktop), so the raw canvas
// height is the honest answer.
func sheetMaxHeight(canvasH, safeTop, safeH, top float32) float32 {
	bottom := canvasH
	if safeH > 0 {
		bottom = safeTop + safeH
	}
	h := bottom - top - sheetBottomMargin
	if h < minSheetHeight {
		h = minSheetHeight
	}
	return h
}

// scrollingSheetHeight is the height to hand widget.PopUp.Resize for a sheet
// whose growable middle lives inside a container.Scroll.
//
// The scroll hides the body's real height from MinSize (that is the whole point
// of it), so the sheet's natural height has to be reconstructed: take what the
// popup reports with the scroll collapsed to its minimum, then add back the
// height the body actually wants over that minimum. Working from the scroll's
// own MinSize rather than Fyne's internal 32pt scroll floor keeps this correct
// if that constant ever moves.
//
// Under the cap the sheet is its natural height and the scroll never engages —
// so on a roomy screen it looks exactly as it did before, no scrollbar. Over the
// cap it is capped, and the overflow becomes scrollable rather than invisible.
func scrollingSheetHeight(popupMinH, scrollMinH, bodyMinH, maxH float32) float32 {
	natural := popupMinH + bodyMinH - scrollMinH
	if natural > maxH {
		return maxH
	}
	return natural
}
