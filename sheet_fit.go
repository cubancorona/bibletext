package bibletext

// Sizing a card sheet so it can never run off the bottom of the screen.
//
// THE BUG THIS EXISTS TO PREVENT. A card sheet (the Settings sheet, the audio
// source menu) is a NON-MODAL widget.PopUp sized from its content's MinSize and
// pinned near the top of the canvas. Nothing in Fyne clamps that. (Modal popups
// are a different animal and are not what this file is for: their renderer DOES
// clamp the frame to the canvas — though children can still overflow it
// invisibly, which is its own trap. The note offer is one of those.)
// popUpRenderer.Layout takes
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

// canvasArea is the canvas's interactive area as the driver reports it: the
// canvas less the device's safe insets, and on iOS less a raised soft keyboard
// as well (sheetArea). A seam, so a host test can give the test driver's
// canvas — which has no insets and no keyboard — a phone's insets and a
// keyboard.
var canvasArea = func(c fyne.Canvas) (fyne.Position, fyne.Size) { return c.InteractiveArea() }

// keyboardFootFloor separates a safe inset from a keyboard: no device's
// bottom safe inset — the home indicator's 34pt, a tablet's 20pt, an Android
// navigation bar's — is near it, and no soft keyboard is under it.
const keyboardFootFloor = 100

// keyboardFreeFoot is the canvas's bottom inset as last read with no
// keyboard in it: what a phone sheet leaves uncovered at the canvas's foot.
// UI goroutine only.
var keyboardFreeFoot struct {
	known bool
	foot  float32
}

// sheetArea is the part of the canvas a phone sheet is sized to: the
// interactive area, with the soft keyboard given back.
//
// THE BUG THIS EXISTS TO PREVENT. Fyne's iOS driver reports a raised keyboard
// as the canvas's whole bottom inset (getDevicePadding: inset.bottom =
// keyboardHeight), and moves it back only from the later WillHide
// notification. A sheet sized from the interactive area while the keyboard
// was up therefore ended at the keyboard's top and stayed that short once the
// keyboard went down, the page showing beneath it: no phone sheet is sized
// again for the keyboard, on purpose. An ordinary open never reads the area
// that way, since the keyboard comes up after the sheet is sized, but the
// light/dark reopen does: the appearance gate unfocuses the canvas, rebuilds
// and reopens the sheet on top in one call, before the keyboard's WillHide
// has moved the inset, and the note composer's native field keeps its
// keyboard up through the reopen altogether. Fyne's Android driver counts
// the keyboard the same way (the activity is adjust-resize, and the
// system-window insets the driver reads fold the IME in — measured on
// Android 15; older releases were not checked), so the rule serves both.
//
// The keyboard's height is not subtracted back, because the driver replaces
// the safe inset with it rather than adding to it. Instead the keyboard-free
// foot is remembered from every read made with no keyboard in the inset —
// the keyboard reported down (softKeyboardShown, from the keyboard
// observers) and the inset no deeper than a safe inset can be — and a read
// that finds the inset deeper than that foot takes the foot instead. The
// driver's inset and the observers' report travel by different paths, so
// either can be a frame ahead of the other; the two conditions together keep
// a keyboard-deep inset from ever being remembered as the foot, and a read
// made before any keyboard-free foot is known gives the area as reported,
// which is what every read gave before this rule.
func sheetArea(c fyne.Canvas) (fyne.Position, fyne.Size) {
	pos, sz := canvasArea(c)
	if sz.Height <= 0 {
		return pos, sz
	}
	foot := c.Size().Height - pos.Y - sz.Height
	if !softKeyboardShown && foot <= keyboardFootFloor {
		keyboardFreeFoot.known, keyboardFreeFoot.foot = true, foot
		return pos, sz
	}
	if keyboardFreeFoot.known && foot > keyboardFreeFoot.foot {
		sz.Height = c.Size().Height - pos.Y - keyboardFreeFoot.foot
	}
	return pos, sz
}

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

// sheetHeaderGap is the air a desktop sheet leaves between the header's
// bottom edge and its own top.
const sheetHeaderGap = 8

// headerClearance is how far down the canvas a sheet's top edge has to sit on
// a desktop window to leave the app header uncovered: the header's bottom edge
// plus sheetHeaderGap. Zero when there is nothing to clear — no header on
// screen (full-screen reading, the loading screen) — and on a phone or a
// tablet, whose sheets are sized to the safe area instead (sheetMaxHeight).
//
// THE BUG THIS EXISTS TO PREVENT. The desktop sheets are modal, and Fyne
// centres a modal popup on the canvas whatever position it is shown at, so a
// sheet's top edge is (canvas height - sheet height) / 2 and nothing else.
// The Settings sheet was capped only by the canvas, which at 1280x800 put its
// top at 18.5pt: inside the header, about a point below the top of the
// centred Go to chip, whose outline showed above the sheet as a small grey
// arc. A sheet may cover a header control entirely or leave it alone;
// starting partway down one is the defect. The Go to picker and the
// translation picker already opened below the header at that size, so every
// desktop sheet now does, and is sized again when the window changes size
// (sheet_refit.go).
func headerClearance(state *AppState) float32 {
	if state == nil || state.header == nil || state.window == nil || fyne.CurrentDevice().IsMobile() {
		return 0
	}
	h := state.header
	if !h.Visible() || !objectInTree(state.window.Canvas().Content(), h) {
		return 0
	}
	top := fyne.CurrentApp().Driver().AbsolutePositionForObject(h).Y
	return top + h.Size().Height + sheetHeaderGap
}

// objectInTree reports whether target is root or sits under it through
// containers — how the header hangs off the window's content.
func objectInTree(root, target fyne.CanvasObject) bool {
	if root == nil {
		return false
	}
	if root == target {
		return true
	}
	if c, ok := root.(*fyne.Container); ok {
		for _, o := range c.Objects {
			if objectInTree(o, target) {
				return true
			}
		}
	}
	return false
}

// clearOfHeader caps h, the height of a sheet Fyne will centre on a canvas
// canvasH tall, so its top edge lands at or below clearance. clearance 0
// leaves h alone.
//
// The cap holds only if the sheet can be that short. Fyne lays a modal out at
// the larger of the size it is given and its content's MinSize, and centres
// that, so a sheet whose parts that do not scroll are taller than the cap
// ignores it and starts (canvasH - MinSize) / 2 down the canvas, inside the
// header. Every sheet this is applied to keeps those parts within it: what
// grows scrolls or is sized from the height it is given, and a part whose
// height is data, such as the translation picker's sentences naming
// translations, goes into the scroll when there is no room to pin it
// (showVersionPickerWith).
func clearOfHeader(h, canvasH, clearance float32) float32 {
	if clearance <= 0 {
		return h
	}
	limit := canvasH - 2*clearance
	if limit < minSheetHeight {
		limit = minSheetHeight // a window too short to clear the header at all
	}
	if h > limit {
		h = limit
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
