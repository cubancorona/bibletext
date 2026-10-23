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

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

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

// sideInsets is how far in from the canvas's left and right edges the safe
// area starts, read from the area a phone sheet is sized to (sheetArea): the
// sensor housing's side on an iPhone held sideways, the display cutout or a
// side navigation bar on an Android phone held sideways. Zero on both sides
// where no area is reported (a desktop window) and wherever the area spans
// the canvas's width, which is every phone held upright and every iPad.
func sideInsets(canvasW float32, pos fyne.Position, sz fyne.Size) (left, right float32) {
	if sz.Width <= 0 {
		return 0, 0
	}
	return max(pos.X, 0), max(canvasW-pos.X-sz.Width, 0)
}

// clearOfSideInsets holds the card of a phone sheet that spans the canvas —
// the note composer, the Ask sheet — inside the canvas's side safe insets.
//
// THE BUG THIS EXISTS TO PREVENT. Those two sheets are sized to the safe
// area's height but to the canvas's whole width, from its left edge, so on an
// iPhone held sideways the card ran under the Dynamic Island on one side and
// the matching inset on the other: the first characters of the composer's
// "Optional. The note travels…" and the Ask sheet's "AI answers in its own
// words…", and the left end of each text box, were drawn under the island.
//
// The sheet itself still spans the canvas, and only its card moves in. The
// two sheets are non-modal, so a tap outside the sheet's box closes it, and
// the composer's close takes the note being written with it; a box narrowed
// to the safe area would have turned both side bands into places where a
// thumb holding the phone sideways throws the note away. The bands show the
// sheet's own ground instead, as a system sheet's background reaches under
// the insets while its content keeps clear of them. Where there is no side
// inset the card is laid out exactly where it was.
//
// refit gives the card the side insets of a canvas that has changed size,
// for the composer's refit; the sheet's next layout places it.
func clearOfSideInsets(card fyne.CanvasObject, left, right float32) (wrapped *fyne.Container, refit func(left, right float32)) {
	wrapped = container.New(layout.NewCustomPaddedLayout(0, 0, left, right), card)
	refit = func(left, right float32) {
		wrapped.Layout = layout.NewCustomPaddedLayout(0, 0, left, right)
	}
	return wrapped, refit
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
// (sheet_refit.go). A sheet that cannot, on a window too short for it, covers
// the header instead (coverHeaderUnlessClear).
func headerClearance(state *AppState) float32 {
	_, bottom, ok := headerEdges(state)
	if !ok {
		return 0
	}
	return bottom + sheetHeaderGap
}

// headerEdges is where the header's top and bottom edges stand on a desktop
// window's canvas. ok is false wherever headerClearance is zero.
func headerEdges(state *AppState) (top, bottom float32, ok bool) {
	if state == nil || state.header == nil || state.window == nil || fyne.CurrentDevice().IsMobile() {
		return 0, 0, false
	}
	h := state.header
	if !h.Visible() || !objectInTree(state.window.Canvas().Content(), h) {
		return 0, 0, false
	}
	top = fyne.CurrentApp().Driver().AbsolutePositionForObject(h).Y
	return top, top + h.Size().Height, true
}

// coverHeaderUnlessClear makes a desktop sheet that Fyne centres on the window,
// and that starts above headerClearance as it has been sized, cover the header
// instead: the sheet's top edge is the header's top edge, and it takes its
// natural height, capped at the room from there to as far above the window's
// foot. A sheet either leaves the header and the gap below it uncovered or
// covers the whole header, on every window the app allows, as a phone or
// tablet sheet at its cap starts above the header's controls; no window is
// made taller to avoid it.
//
// The defect this prevents: a desktop window can be as short as 386pt, and
// below about 440pt the parts of some sheets that do not scroll are taller
// than the room below the header and its gap (clearOfHeader). Fyne never sizes
// a modal below its content's MinSize and centres it, so those sheets started
// in the gap or inside the header: at 386pt Go to and the AI answer in the
// gap, the cross-references while their list loads 7pt below the Go to chip,
// and a link notice partway down the chip.
//
// The natural height is the sheet's preferred height, what it would take with
// unlimited room, never less than its content's least height once that has
// settled. preferred is what the sheet asks for beyond its content: a card
// sized from its content passes 0, so a covering link notice, note-link offer
// or Ask is only as tall as its content, with no blank band under its last
// row; a sheet whose lists or scroll areas use the room they are given passes
// the height it takes on a window with room to spare, so Go to's book and
// chapter lists still fill the room. Covering at the room's height, a link
// notice, the note-link offer, Ask and the note composer showed 108.7 to
// 131.7pt of blank under their last rows on 386 and 401pt windows.
//
// Call it after the sheet's last Resize, each time the sheet is sized, its
// refit included: it judges where the sheet settles (sheetStartsClearOf),
// centred on the window again first (centreOnWindow). The covering height is
// a Resize like any other, so the sheet's next fit starts from its own size
// again, and a sheet that can clear the header then does. A no-op on a phone
// or tablet and with no header on screen.
func coverHeaderUnlessClear(state *AppState, popup *widget.PopUp, preferred float32) {
	top, bottom, ok := headerEdges(state)
	if !ok || popup == nil || popup.Content == nil || popup.Canvas == nil {
		return
	}
	cnv := centreOnWindow(popup)
	if sheetStartsClearOf(popup, bottom) {
		return
	}
	pad := popup.Theme().Size(theme.SizeNameInnerPadding)
	room := cnv.Size().Height - 2*top
	h := min(max(preferred, popup.MinSize().Height), room)
	// The sheet keeps the width it settled at, so nothing in it wraps again
	// and the Resize lays it out at its final height.
	popup.Resize(fyne.NewSize(popup.Content.Size().Width+pad, h))
	raiseSheet(popup, cnv, room-(popup.Content.Size().Height+pad))
}

// raisedCanvas is the window's canvas as a covering sheet shorter than the
// room from the header's top is laid out on: above points taller, the extra
// above the window's top edge.
//
// Fyne centres a modal sheet on the size its popup has, and gives the popup
// its canvas's size whenever it is refreshed, so a sheet can stand only
// where its centre is the canvas's centre. Raised by above, on a canvas
// taller by above, the popup still covers the window from top to foot, so
// its shade dims the whole page, and the sheet centred on it, frame and
// shadow included, stands above/2 higher than on the window: its top edge at
// the header's top when above is the room from the header's top less the
// sheet's height. A tap anywhere on the window goes to the top overlay
// alone, as under any modal sheet, so none reaches the page.
//
// This rests on Fyne 2.7.4's PopUp: the popup is itself the overlay, it is
// given its Canvas's size on each Refresh, BaseWidget.Move places it
// (PopUp.Move does nothing for a modal one), and nothing else moves it. Fyne
// 2.8 wraps a popup in an overlay container that the canvas's overlay stack
// sizes and places from the window's own canvas, and a hidden popup is moved
// back to the canvas's top left, so the raise has to be built again for it.
type raisedCanvas struct {
	fyne.Canvas
	above float32
}

func (c *raisedCanvas) Size() fyne.Size {
	s := c.Canvas.Size()
	s.Height += c.above
	return s
}

// raiseSheet lays a covering sheet out on the window's canvas cnv raised by
// above (raisedCanvas). A sheet as tall as the room from the header's top
// already starts at the header's top centred on the window, and stays there.
func raiseSheet(popup *widget.PopUp, cnv fyne.Canvas, above float32) {
	if above < 0.5 {
		return
	}
	popup.Canvas = &raisedCanvas{Canvas: cnv, above: above}
	popup.BaseWidget.Move(fyne.NewPos(0, -above))
	popup.Refresh()
}

// centreOnWindow lays a sheet raiseSheet raised out on the window's canvas
// again, centred on it, and returns that canvas. A sheet never raised is
// left as it is.
func centreOnWindow(popup *widget.PopUp) fyne.Canvas {
	if rc, ok := popup.Canvas.(*raisedCanvas); ok {
		popup.Canvas = rc.Canvas
		popup.BaseWidget.Move(fyne.NewPos(0, 0))
		popup.Refresh()
	}
	return popup.Canvas
}

// sheetStartsClearOf reports whether a sheet Fyne centres, as last sized,
// starts at or below headerBottom and the gap below it, to within half a
// point, once its layout has settled. It is the one test of clearing the
// header that coverHeaderUnlessClear and fitBelowOrOverHeader share; both
// centre the sheet on the window first (centreOnWindow).
//
// Settled, because a Resize lays the content out at its MinSize as measured
// before the content took the sheet's width. A wrapped label measures the
// line count of the width it had until then, so the content can measure
// shorter once it has its width, and the layout that follows, the next
// frame's on a real window, centres the shorter box lower down: a link
// notice judged at a 441 to 448pt window, where it settles 79 to 82pt down,
// was judged 4pt higher, inside the gap, and covered the header it clears.
// So the sheet is laid out again, through Refresh, until its content keeps
// its place and size (settleSheet).
func sheetStartsClearOf(popup *widget.PopUp, headerBottom float32) bool {
	settleSheet(popup)
	pad := popup.Theme().Size(theme.SizeNameInnerPadding)
	return popup.Content.Position().Y-pad/2 >= headerBottom+sheetHeaderGap-0.5
}

// settleSheet lays a sheet out again, through Refresh, until its content
// keeps its place and size: a wrapped label measures its lines at the width
// it had before the sheet's last Resize (sheetStartsClearOf).
func settleSheet(popup *widget.PopUp) {
	for i := 0; i < 4; i++ {
		pos, size := popup.Content.Position(), popup.Content.Size()
		popup.Refresh()
		if popup.Content.Position() == pos && popup.Content.Size() == size {
			break
		}
	}
}

// fitBelowOrOverHeader sizes a desktop sheet whose card gives up height to
// clear the header — excerpt rows, a box's height — and stays the height of its
// content. fitIn sizes the card to fit in room, giving up only what it must.
// It is given the room below the header first; where the card, with all it can
// give up given up, still does not clear the header (sheetStartsClearOf), it
// is given the room from the header's top edge instead, so it keeps what it
// gave up, and covers the header at the height of its content
// (coverHeaderUnlessClear).
func fitBelowOrOverHeader(state *AppState, popup *widget.PopUp, fitIn func(room float32)) {
	canvasH := centreOnWindow(popup).Size().Height
	room := clearOfHeader(canvasH, canvasH, headerClearance(state))
	fitIn(room)
	top, bottom, ok := headerEdges(state)
	if !ok || sheetStartsClearOf(popup, bottom) {
		return
	}
	fitIn(canvasH - 2*top)
	coverHeaderUnlessClear(state, popup, 0)
}

// sheetMostHeight is the tallest a sheet Fyne centres on the canvas may be and
// still lie wholly in the reader's reach. On a desktop window with the header
// on screen it is the room a sheet covering the header may take, from the
// header's top edge to as far above the window's foot, which caps its
// natural height (coverHeaderUnlessClear). Elsewhere it is the canvas less
// the deeper of its top and bottom safe insets at both edges, so a centred
// sheet stays inside the safe area (sheetArea); a desktop window reports
// none, so there it is the canvas.
func sheetMostHeight(state *AppState, cnv fyne.Canvas) float32 {
	canvasH := cnv.Size().Height
	if top, _, ok := headerEdges(state); ok {
		return canvasH - 2*top
	}
	pos, sz := sheetArea(cnv)
	if sz.Height <= 0 {
		return canvasH
	}
	return canvasH - 2*max(pos.Y, canvasH-pos.Y-sz.Height, 0)
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
