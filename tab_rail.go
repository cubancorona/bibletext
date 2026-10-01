package bibletext

// THE iPAD BAR, TURNED VERTICAL.
//
// Not a new design — the same one, rotated. The grounded bar the iPad uses is:
// chrome running the full length of its edge, a hairline rule against the
// content, and the destinations CENTRED along that edge at a fixed slot each
// rather than spread or hand-spaced. This is that same visual idea, with the
// long axis turned through ninety degrees and the destinations centred
// vertically.
//
// So it borrows the bar's numbers instead of inventing its own — tabBarCellTablet
// is the slot in both, which is what makes the rail read as the same system and
// not a second one that happens to look similar. The gap between destinations is
// the slot's doing, exactly as it is in the bar; nothing here picks a spacing.
//
// The one thing that cannot rotate is the cell's cross-axis. In the bar that is
// the icon-over-label stack, narrower than the widest label the cell draws, so
// the rail's width is measured from the labels instead (tabRailWidth below)
// rather than taken from the slot.
//
// It exists because on a window wider than it is tall the scarce axis is
// vertical: a rail trades the bar's full strip of height, which is dear, for a
// column of width, which is not. So every phone and tablet draws it while its
// window is wider than tall and the bar otherwise (mobileRailWanted), and the
// desktop, where a bottom bar is a phone convention a window would inherit
// rather than choose, draws it whatever the window's shape.
//
// It keeps clear of the screen's side insets — the Dynamic Island's side of an
// iPhone held sideways, and the inset UIKit reports opposite it; an Android
// phone's side navigation bar or camera cutout, as its canvas reports them —
// without a rule of its own.
// The rail is part of the window's tree, and both mobile drivers lay that tree
// inside the canvas's safe area (the interactive area, InteractiveArea), the
// area a phone sheet is sized to (sheetArea, sheet_fit.go), so the rail stands
// right beside an inset and the content beside the rail. The native reading
// panes add the same insets back to their frames (reading_ios.go, BtBridge's
// windowContentOrigin). The two sheets that span the canvas need
// clearOfSideInsets because a sheet is a pop-up placed on the canvas, outside
// that tree; giving the rail the same padding would hold it twice as far in.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

// tabRailWidth is the rail's thickness, derived so its margins match the bar's.
//
// The bar's cross-axis gives its icon-and-label one theme padding of air on each
// side; everything wider than that in the bar is ALONG its axis, which the rail
// spends on height instead. So the rail's own cross-axis is the widest thing the
// cell ever draws plus that same padding either side — 32pt of bold "Search"
// plus 2 x 7, against the 104pt slot it started at, which read as too wide
// beside the iPad bar's margins.
//
// Derived rather than written down because the alternative drifts silently: the
// dev build adds a "Links" destination, and a translation could make any label
// the widest. Measuring the labels the cell will actually draw, at the size it
// will draw them, is the only spelling that cannot be wrong.
//
// The BOLD width is what is measured: the active tab draws bold, so the widest
// the rail must ever hold is every label at its bold width.
func tabRailWidth() float32 {
	widest := tabCellIconSize // never narrower than the glyph
	for _, d := range tabDestinations() {
		if w := fyne.MeasureText(d.label, tabCellLabelSize, fyne.TextStyle{Bold: true}).Width; w > widest {
			widest = w
		}
	}
	return widest + 2*theme.Padding()
}

// buildTabRail arranges the navigation vertically against the leading edge,
// centred on the window's height.
func buildTabRail(state *AppState) fyne.CanvasObject {
	pal := state.pal()
	cells := tabCellsFor(state, tabDestinations(), 0) // rail cells already fill their slots

	// GridWithRows is the vertical twin of the bar's GridWithColumns: one equal
	// slot per destination, so the rhythm between them is the slot and not a
	// spacing constant. Wrapped in a layout that gives the block its natural
	// height and centres it — the twin of tabBarCentreLayout.
	stack := container.NewGridWithRows(len(cells), cells...)
	centred := container.New(
		tabRailCentreLayout{want: tabBarGroupWidth(len(cells))}, stack)

	// The rule sits on the trailing edge, against the content — the same
	// relationship the bottom bar's rule has with what sits above it.
	rule := canvas.NewLine(pal.Border)
	rule.StrokeWidth = 1
	bg := canvas.NewRectangle(pal.SurfaceAlt)

	return container.NewStack(bg,
		container.NewBorder(nil, nil, nil, rule,
			container.New(fixedWidthLayout{width: tabRailWidth()}, centred)))
}

// tabRailCentreLayout gives its child a fixed height and centres it in the
// available one. The vertical twin of tabBarCentreLayout, and deliberately a
// separate type rather than a shared axis-parameterised one: two eight-line
// layouts are easier to read than one that has to be traced through an axis.
type tabRailCentreLayout struct{ want float32 }

func (l tabRailCentreLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	h := l.want
	if h > s.Height { // a window too short for the block: fill it rather than overflow
		h = s.Height
	}
	y := (s.Height - h) / 2
	for _, o := range objs {
		o.Resize(fyne.NewSize(s.Width, h))
		o.Move(fyne.NewPos(0, y))
	}
}

func (l tabRailCentreLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(tabRailWidth(), l.want)
}
