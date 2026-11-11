package bibletext

// A parenthesised group of the publisher's apparatus, as a row of the
// cross-references list (crossref_list.go).
//
// Decided 11 November 2026: a group is a row of its own, in the place the
// edition printed it, with a rule above and below; its printed text, a little
// larger than a caption, is the row's label; and the row opens in place into
// the group's citations as the list's ordinary cards, on a shaded inset that
// shows which cards are the group's. It starts closed, and closes again on a
// second tap.
//
// The group is the edition's unit, so it is kept whole: every citation of it
// the loaded text has is a card when it is open, in print order, with no
// de-duplication against the cards outside it, and none of them counts as
// carded for the Treasury's filter while the group may be closed.
//
// Width: the label wraps beside an icon of fixed size, so the row wants no
// more than the narrowest phone gives the list. Opening and closing changes
// the list's height inside its scroll and nothing else: the panel keeps its
// size and its place (showCrossRefs, fit).

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// sizeNameCrossRefGroup is the size of a group's label: above a caption's 11
// points and below a card reference's 16 (bibleTheme.Size).
const sizeNameCrossRefGroup fyne.ThemeSizeName = "bibletext.crossRefGroup"

// crossRefGroupTextSize is that size in points.
const crossRefGroupTextSize = 14

// crossRefRelayout is how a row that changes its own height asks for the
// list to be laid out again. The list is composed before it is mounted, so
// the panel sets fn once the list stands in its scroll; until then, and in a
// list no panel holds, a toggle refreshes the row alone.
type crossRefRelayout struct{ fn func() }

func (r *crossRefRelayout) run() {
	if r != nil && r.fn != nil {
		r.fn()
	}
}

// crossRefGroupRow is the row of one parenthesised group.
type crossRefGroupRow struct {
	widget.BaseWidget
	group    publisherGroup
	state    *AppState
	pal      palette
	onTap    func(crossRef)
	relayout *crossRefRelayout

	open    bool
	header  *tapCard          // the label and its icon: the tap that opens and closes; nil for a group with nothing to open
	icon    *widget.Icon      // points right while closed, down while open
	members *fyne.Container   // the group's cards, built the first time it opens
	inset   *fyne.Container   // the shaded ground the cards stand on; hidden while closed
	box     *fyne.Container   // header, inset, rule
	label   *widget.RichText  // the group as printed
	shade   *canvas.Rectangle // the inset's ground
}

func newCrossRefGroupRow(state *AppState, g publisherGroup, pal palette, onTap func(crossRef), relayout *crossRefRelayout) *crossRefGroupRow {
	r := &crossRefGroupRow{group: g, state: state, pal: pal, onTap: onTap, relayout: relayout}
	r.label = widget.NewRichText(&widget.TextSegment{Text: g.Text, Style: widget.RichTextStyle{
		ColorName: theme.ColorNamePlaceHolder, SizeName: sizeNameCrossRefGroup,
	}})
	r.label.Wrapping = fyne.TextWrapWord
	r.members = container.New(layout.NewCustomPaddedVBoxLayout(0))
	r.shade = canvas.NewRectangle(pal.Surface)
	r.shade.CornerRadius = 8
	// The ground, with a little air under it so it does not sit on the rule
	// that closes the row.
	ground := container.NewStack(r.shade, container.New(layout.NewCustomPaddedLayout(2, 2, 8, 8), r.members))
	r.inset = container.New(layout.NewCustomPaddedLayout(0, 8, 0, 0), ground)
	r.inset.Hide()

	var head fyne.CanvasObject
	if len(g.cards()) == 0 {
		// Nothing of the group can be followed: its words stand as they are
		// printed, with no icon promising cards and no tap that opens none.
		head = container.New(layout.NewCustomPaddedLayout(6, 6, 0, 0), r.label)
	} else {
		r.icon = widget.NewIcon(crossRefGroupIcon(false))
		// The label stands at the middle of the row's height, as the icon
		// does, however many lines it wraps to: the tap is 44 points tall
		// and one line of the label is not.
		middle := container.NewVBox(layout.NewSpacer(), r.label, layout.NewSpacer())
		row := container.NewBorder(nil, nil, nil, container.NewCenter(r.icon), middle)
		r.header = newTapCard(container.New(layout.NewCustomPaddedLayout(6, 6, 0, 0), row), pal.SurfaceAlt, r.toggle)
		head = r.header
	}
	r.box = container.New(layout.NewCustomPaddedVBoxLayout(0), head, r.inset, widget.NewSeparator())
	r.ExtendBaseWidget(r)
	return r
}

// crossRefGroupIcon is the row's disclosure mark, in the label's muted colour.
func crossRefGroupIcon(open bool) fyne.Resource {
	res := theme.MenuExpandIcon()
	if open {
		res = theme.MenuDropDownIcon()
	}
	return theme.NewColoredResource(res, theme.ColorNamePlaceHolder)
}

func (r *crossRefGroupRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.box)
}

// toggle opens the group, or closes it. The cards are built the first time
// it opens: a list can hold many groups, and most are never opened.
func (r *crossRefGroupRow) toggle() {
	r.open = !r.open
	if r.open {
		if len(r.members.Objects) == 0 {
			cards := r.group.cards()
			for i, tg := range cards {
				card := crossRefCard{Ref: tg.Ref, Label: tg.label(), Tag: tg.tag(), Title: tg.Title}
				// The last card's rule would stand at the foot of the shaded
				// ground with the row's own rule a few points under it.
				r.members.Add(crossRefRowRuled(r.state, card, r.pal, r.onTap, i < len(cards)-1))
			}
		}
		r.inset.Show()
	} else {
		r.inset.Hide()
	}
	if r.icon != nil {
		r.icon.SetResource(crossRefGroupIcon(r.open))
	}
	r.box.Refresh()
	r.Refresh()
	r.relayout.run()
}
