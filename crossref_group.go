package bibletext

// A parenthesised group of the publisher's apparatus, as a row of the
// cross-references list (crossref_list.go).
//
// Decided 11 November 2026: a parenthesised group is closed until it is
// tapped wherever it stands. It is a row of its own, in the place the edition
// printed it, its printed text a little larger than a caption as the row's
// label, and a fold mark beside it; and the row opens in place into the
// group's citations as the list's ordinary cards, on a ground of their own
// that shows which cards are the group's. It closes again on a second tap.
//
// The group is the edition's unit, so it is kept whole: everything between
// its parentheses is a row when it is open, in print order — a card for each
// citation the reader can follow, the reader's own verse in its place and
// not followed, and words that are no citation the loaded text has
// (publisherPieceRows). Its citations are the edition's related ones, set
// apart by the parentheses, and they are not on screen until the row is
// opened, so none of them counts for the Treasury's filter.
//
// Inside a note of several pieces the row stands among its note's rows,
// beside the letter's margin on the note's ground (crossref_section.go): as
// the first of them it has no rule above it, the note's own row standing
// there, and as the last none below, where the note's ground ends. A note
// that is one parenthesised group and nothing else is drawn as this same row,
// on its note's ground with its letter in the margin, so its label, mark and
// opened well stand in line with a group inside another note, and while open
// a rule hangs from under the letter beside the well (crossref_note.go).
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

// crossRefGroupTextSize is the size of a line of the edition's print — a
// group's label, a note's printed line — in points: above a caption's 11 and
// below a card reference's 16.
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

// crossRefGroupWellFoot is the air under an opened group's well, so it does
// not sit on the rule that closes the row or on the foot of the note's ground.
const crossRefGroupWellFoot float32 = 8

// crossRefGroupRow is the row of one parenthesised group, inside a note or
// as the whole of its note.
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
	head    *fyne.Container   // when margin is true: the letter in the margin and the printed group beside it
	letter  *canvas.Text      // when margin is true: the note's letter, in the margin
	stem    *canvas.Rectangle // when margin is true: the rule from under the letter beside the well; hidden while closed
	members *fyne.Container   // the group's cards, built the first time it opens
	inset   *fyne.Container   // the shaded ground the cards stand on; hidden while closed
	box     *fyne.Container   // header, inset, rule or stem
	label   *crossRefPrinted  // the group as printed
	shade   *canvas.Rectangle // the inset's ground
}

// margin says whether the row stands at the top of its note's ground, where
// it reserves the letter's margin on its left, sets letter in it, and hangs
// the margin's rule beside the well while open; inside a note's body that
// margin and rule belong to crossRefNoteLayout. rule says whether the row
// closes with a horizontal rule of its own: it does not as the last of a
// note's rows, or as the whole of its note, where the note's ground ends
// under it.
func newCrossRefGroupRow(state *AppState, g publisherGroup, letter string, margin bool, pal palette, onTap func(crossRef), relayout *crossRefRelayout, rule bool) *crossRefGroupRow {
	r := &crossRefGroupRow{group: g, state: state, pal: pal, onTap: onTap, relayout: relayout}
	r.label = newCrossRefPrinted(g.Text, pal)
	r.members = container.New(layout.NewCustomPaddedVBoxLayout(0))
	r.shade = canvas.NewRectangle(crossRefGroupGround(pal))
	r.shade.CornerRadius = 8
	// An edge that does not rest on the fill: in the dark theme the well's
	// ground stands 1.09:1 from the note's, and once the group's own row has
	// scrolled away the well is all that tells its cards from the note's.
	r.shade.StrokeColor = crossRefGroupEdge(pal)
	r.shade.StrokeWidth = 1
	ground := container.NewStack(r.shade, container.New(layout.NewCustomPaddedLayout(2, 2, 8, 8), r.members))
	var left float32
	line := fyne.CanvasObject(r.label)
	if margin {
		left = crossRefLetterMargin
		r.head = crossRefLettered(letter, pal, r.label, 0, crossRefGroupTextSize, fyne.TextStyle{})
		r.letter = r.head.Objects[0].(*canvas.Text)
		line = r.head
	}
	r.inset = container.New(layout.NewCustomPaddedLayout(0, crossRefGroupWellFoot, left, 0), ground)
	r.inset.Hide()

	var head fyne.CanvasObject
	if len(g.cards()) == 0 {
		// Nothing of the group can be followed: its words stand as they are
		// printed, with no icon promising cards and no tap that opens none.
		head = container.New(layout.NewCustomPaddedLayout(crossRefRowPad, crossRefRowPad, 0, 0), line)
	} else {
		r.icon = widget.NewIcon(crossRefGroupIcon(false))
		// The label stands at the middle of the row's height, as the icon
		// does, however many lines it wraps to: the tap is 44 points tall
		// and one line of the label is not.
		middle := container.NewVBox(layout.NewSpacer(), line, layout.NewSpacer())
		row := container.NewBorder(nil, nil, nil, crossRefFoldIcon(r.icon), middle)
		r.header = newTapCard(container.New(layout.NewCustomPaddedLayout(crossRefRowPad, crossRefRowPad, 0, 0), row), crossRefHover(pal), r.toggle)
		head = r.header
	}
	if margin && !rule {
		r.stem = canvas.NewRectangle(crossRefStemColour(pal))
		r.stem.CornerRadius = crossRefStemWidth / 2
		r.stem.Hide()
		r.box = container.New(crossRefGroupLayout{r, head}, head, r.inset, r.stem)
	} else {
		r.box = container.New(layout.NewCustomPaddedVBoxLayout(0), head, r.inset)
		if rule {
			r.box.Add(widget.NewSeparator())
		}
	}
	r.ExtendBaseWidget(r)
	return r
}

// crossRefGroupLayout lays a lettered group row out: its own row across the
// top, its inset well under it, and the rule in the margin from under the
// letter to crossRefStemFoot above the well's foot while open.
type crossRefGroupLayout struct {
	row  *crossRefGroupRow
	head fyne.CanvasObject
}

func (l crossRefGroupLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	r := l.row
	size := l.head.MinSize()
	if r.inset.Visible() {
		in := r.inset.MinSize()
		if in.Width > size.Width {
			size.Width = in.Width
		}
		size.Height += in.Height
	}
	return size
}

func (l crossRefGroupLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	r := l.row
	top := l.head.MinSize().Height
	l.head.Move(fyne.NewPos(0, 0))
	l.head.Resize(fyne.NewSize(size.Width, top))
	if !r.inset.Visible() {
		return
	}
	r.inset.Move(fyne.NewPos(0, top))
	r.inset.Resize(fyne.NewSize(size.Width, size.Height-top))
	from := top
	if r.letter != nil && r.letter.Text != "" {
		_, line := fyne.CurrentApp().Driver().RenderedTextSize(r.letter.Text, r.letter.TextSize, r.letter.TextStyle, nil)
		from = crossRefRowPad + r.head.Position().Y + r.letter.Position().Y + line + crossRefStemGap
	}
	wellFoot := size.Height - crossRefGroupWellFoot
	r.stem.Move(fyne.NewPos(crossRefStemAt, from))
	r.stem.Resize(fyne.NewSize(crossRefStemWidth, wellFoot-crossRefStemFoot-from))
}

// crossRefFoldIconSize is the box a row's disclosure mark is drawn in: a
// little over the toolkit's inline size, at which the mark was a triangle
// eight points by four.
const crossRefFoldIconSize float32 = 26

// crossRefFoldIcon is icon at that size, at the middle of its row's height.
func crossRefFoldIcon(icon *widget.Icon) fyne.CanvasObject {
	return container.NewCenter(container.NewGridWrap(fyne.NewSize(crossRefFoldIconSize, crossRefFoldIconSize), icon))
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
			// Everything between the parentheses, in print order: the cards,
			// a citation of the reader's own verse in its place, and words
			// that cannot be followed. The last row's rule would stand at
			// the foot of the shaded ground with the row's own rule a few
			// points under it.
			for _, row := range publisherPieceRows(r.state, r.group.Pieces, r.pal, r.onTap, r.relayout) {
				r.members.Add(row)
			}
		}
		r.inset.Show()
		if r.stem != nil {
			r.stem.Show()
		}
	} else {
		r.inset.Hide()
		if r.stem != nil {
			r.stem.Hide()
		}
	}
	if r.icon != nil {
		r.icon.SetResource(crossRefGroupIcon(r.open))
	}
	r.box.Refresh()
	r.Refresh()
	r.relayout.run()
}
