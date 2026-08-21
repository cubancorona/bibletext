package bibletext

// THE iPAD BAR, TURNED VERTICAL.
//

// the iPad is: chrome running the full length of its edge, a hairline rule
// against the content, and the destinations CENTRED along that edge at a fixed
// slot each rather than spread or hand-spaced. This is that, with the long axis

// idea of the iPad bar made vertical", "they should be centered vertically").
//
// So it borrows the bar's numbers instead of inventing its own — tabBarCellTablet
// is the slot in both, which is what makes the rail read as the same system and
// not a second one that happens to look similar. The gap between destinations is
// the slot's doing, exactly as it is in the bar; nothing here picks a spacing.
//
// The one thing that cannot rotate is the cell's cross-axis. In the bar that is
// ~35pt of icon-over-label, and a 35pt-wide rail could not print "Search". The
// rail therefore takes the slot number for its width too — the same 104 — which
// keeps it inside the bar's system while giving the label room.
//
// It exists because a bottom bar is a phone convention a desktop window
// inherits rather than chooses, and because on a landscape window the scarce
// axis is vertical: a rail trades ~72pt of height, which is dear, for 96–104pt
// of width, which is not.

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

// little less wide, perhaps try matching the ipad margins").
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
	cells := tabCellsFor(state, tabDestinations())

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
