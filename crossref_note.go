package bibletext

// One note of the publisher's apparatus as the cross-references list draws
// it — docs/SCRIPTURE_WORKLIST.md, S20.
//
// Decided 11 November 2026: nothing the edition prints in a note is lost,
// moved or merged. Each note stands under its letter, the letter its word
// carries in the passage at the head of the list (crossref_passage.go), in
// the order the notes stand in the verse:
//
//   - a note that is one citation and nothing else is that citation's card
//     (or, where the loaded text has no such passage, its words; or, where
//     it cites the verse the reader is on, its reference shown and not
//     followed);
//   - a note that is one parenthesised group is that group's row
//     (crossref_group.go), closed until tapped, beside the letter's margin
//     (or, where nothing in it can be followed, the group as printed);
//   - every other note is a row that carries the note exactly as printed and
//     folds: under it, while it is open, the note's pieces in the order the
//     edition prints them. It is open when the panel opens, and a tap on the
//     row closes it. The printed text stays on the row in both states, so
//     the edition's own punctuation — which citations a comma joins, where a
//     group stands, what was carried from the citation before — is never off
//     screen.
//
// Each note stands on a ground of its own with its letter alone in a margin
// at the left, so that a reader sees which rows are a letter's and where the
// next letter begins (crossref_section.go).
//
// A piece is drawn where it is printed (publisherPieceRows): a citation the
// reader can follow as a card; a citation of the verse or passage the reader
// is on as its reference in full, muted, with nothing to tap; a
// parenthesised group as its row; and anything else the note prints, a
// citation the feed did not link and the loaded text does not have, as its
// words. No card is dropped because another note of the verse has it, and no
// two verses are joined into one card: those would be the list's readings of
// the note, and the note is the edition's.
//
// The kinds the list bans stay out (crossref_list_test.go): no Accordion, no
// Button, no Hyperlink, no theme override to a row.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// crossRefLetterSize is the size of a note's letter in its margin: the size
// of a card's reference, so that the letters read down the margin as the
// heads of their notes. crossRefDoubleLetterSize steps down for two-letter
// marks ("aa".."zz") in a long multi-verse selection so the pair stays inside
// crossRefLetterMargin with air before the row beside it.
const (
	crossRefLetterSize       = 16
	crossRefDoubleLetterSize = 10
)

// crossRefLetter is a note's letter as it stands in the margin of its note
// (crossRefLettered).
func crossRefLetter(letter string, pal palette) *canvas.Text {
	return crossRefLetterSized(letter, crossRefPanelSizes(), pal)
}

func crossRefLetterSized(letter string, sz crossRefSizes, pal palette) *canvas.Text {
	size := sz.letter
	if len(letter) > 1 {
		size = sz.doubleLetter
	}
	return crossRefLetterWithSize(letter, size, pal)
}

func crossRefLetterWithSize(letter string, size float32, pal palette) *canvas.Text {
	t := canvas.NewText(letter, pal.Accent)
	t.TextStyle = fyne.TextStyle{Bold: true, Italic: true}
	t.TextSize = size
	return t
}

// crossRefRowPad is the air above and below the text of a row that is no
// card: the reader's own passage, words that are no citation.
const crossRefRowPad float32 = 6

// crossRefSelfRow is a citation of the verse or passage the reader is on:
// its reference in full, in the place the edition printed it, with no
// preview and nothing to tap, since it leads nowhere the reader is not. It
// is set as a line of the edition's print is, muted and at that size: in a
// card's bold it read as a heading over the cards after it.
func crossRefSelfRow(label, tag string, pal palette, rule bool) fyne.CanvasObject {
	sz := crossRefPanelSizes()
	ref := canvas.NewText(label, pal.TextMuted)
	ref.TextSize = sz.printed
	ref.FontSource = crossRefZeroFace(fyne.TextStyle{})
	var line fyne.CanvasObject = ref
	if tag != "" {
		t := canvas.NewText(tag, pal.TextMuted)
		t.TextSize = sz.title
		line = container.New(&crossRefHeadingFlow{gap: 7}, ref, t)
	}
	block := []fyne.CanvasObject{container.New(layout.NewCustomPaddedLayout(crossRefRowPad, crossRefRowPad, 0, 0), line)}
	if rule {
		block = append(block, widget.NewSeparator())
	}
	return container.New(layout.NewCustomPaddedVBoxLayout(0), block...)
}

// crossRefWordsRow is a piece of a note that is no citation the reader can
// follow, exactly as printed, in its place.
func crossRefWordsRow(words string, pal palette, rule bool) fyne.CanvasObject {
	block := []fyne.CanvasObject{container.New(layout.NewCustomPaddedLayout(crossRefRowPad, crossRefRowPad, 0, 0), newCrossRefPrinted(words, pal))}
	if rule {
		block = append(block, widget.NewSeparator())
	}
	return container.New(layout.NewCustomPaddedVBoxLayout(0), block...)
}

// targetCard is the card of one resolved citation.
func targetCard(tg publisherTarget, letter string) crossRefCard {
	return crossRefCard{Ref: tg.Ref, Label: tg.label(), Tag: tg.tag(), Title: tg.Title, Letter: letter}
}

// publisherPieceRows draws pieces in the order the edition prints them, a
// rule between one and the next. The last has none under it: the note's
// ground, or the group's, ends there (crossref_section.go).
func publisherPieceRows(state *AppState, pieces []publisherPiece, pal palette, onTap func(crossRef), relayout *crossRefRelayout) []fyne.CanvasObject {
	rows := make([]fyne.CanvasObject, 0, len(pieces))
	for i, p := range pieces {
		rule := i < len(pieces)-1
		switch {
		case p.Group != nil:
			rows = append(rows, newCrossRefGroupRow(state, *p.Group, "", false, pal, onTap, relayout, rule))
		case p.Target != nil && p.Target.Current:
			rows = append(rows, crossRefSelfRow(p.Target.label(), p.Target.tag(), pal, rule))
		case p.Target != nil:
			rows = append(rows, crossRefRowRuled(state, targetCard(*p.Target, ""), pal, onTap, rule))
		default:
			rows = append(rows, crossRefWordsRow(p.Words, pal, rule))
		}
	}
	return rows
}

// crossRefNoteRow is the row of a note of more than one piece: the note as
// printed, and under it its pieces.
type crossRefNoteRow struct {
	widget.BaseWidget
	note     publisherCrossRef
	relayout *crossRefRelayout

	open   bool
	header *tapCard          // the letter, the printed note and its icon: the tap that closes and opens
	icon   *widget.Icon      // points down while open, right while closed
	head   *fyne.Container   // the letter in the margin and the printed note beside it, as they lie on the row
	letter *canvas.Text      // the note's letter, in the margin
	label  *crossRefPrinted  // the note as printed
	body   *fyne.Container   // the note's pieces, in print order, beside the margin; hidden while closed
	stem   *canvas.Rectangle // the rule from under the letter down the margin beside the pieces; hidden while closed
	box    *fyne.Container
	pieces []fyne.CanvasObject

	// The letter kept in view (keepLetterInView): pin is the letter again,
	// made the first time it is needed, and pinned how far down the row it
	// stands, 0 while the letter stands on its own row.
	pin    *canvas.Text
	pinned float32
	pal    palette
	line   float32 // the letter's line below its own top, measured once
	margin float32 // width of the note letter's margin
}

// newCrossRefNoteRow draws n as its printed line over its pieces.
func newCrossRefNoteRow(state *AppState, n publisherCrossRef, pal palette, onTap func(crossRef), relayout *crossRefRelayout) *crossRefNoteRow {
	sz := crossRefPanelSizes()
	r := &crossRefNoteRow{note: n, relayout: relayout, open: true, pal: pal, margin: sz.letterMargin}
	r.label = newCrossRefPrinted(n.Text, pal)
	r.icon = widget.NewIcon(crossRefGroupIcon(true))
	// The letter in the margin, on the first line of the printed note, and
	// the two at the middle of the row's height as the icon is, however
	// many lines the note wraps to.
	r.head = crossRefLettered(noteLetter(n.Index), pal, r.label, 0, sz.printed, fyne.TextStyle{})
	r.letter = r.head.Objects[0].(*canvas.Text)
	middle := container.NewVBox(layout.NewSpacer(), r.head, layout.NewSpacer())
	row := container.NewBorder(nil, nil, nil, crossRefFoldIcon(r.icon), middle)
	r.header = newTapCard(container.New(layout.NewCustomPaddedLayout(crossRefRowPad, crossRefRowPad, 0, 0), row), crossRefHover(pal), r.toggle)
	r.pieces = publisherPieceRows(state, n.Pieces, pal, onTap, relayout)
	r.body = container.New(layout.NewCustomPaddedVBoxLayout(0), r.pieces...)
	r.stem = canvas.NewRectangle(crossRefStemColour(pal))
	r.stem.CornerRadius = crossRefStemWidth / 2
	r.box = container.New(crossRefNoteLayout{r}, r.header, r.body, r.stem)
	r.ExtendBaseWidget(r)
	return r
}

func (r *crossRefNoteRow) letterMargin() float32 {
	if r.margin > 0 {
		return r.margin
	}
	return crossRefLetterMargin
}

// crossRefNoteLayout lays a note's row out: its own row across the top, its
// pieces under it beside the margin, and the rule in the margin from under
// the letter to a little above the foot of the pieces (crossref_section.go).
type crossRefNoteLayout struct{ row *crossRefNoteRow }

func (l crossRefNoteLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	r := l.row
	size := r.header.MinSize()
	if r.body.Visible() {
		b := r.body.MinSize()
		if w := b.Width + r.letterMargin(); w > size.Width {
			size.Width = w
		}
		size.Height += b.Height
	}
	return size
}

func (l crossRefNoteLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	r := l.row
	top := r.header.MinSize().Height
	r.header.Move(fyne.NewPos(0, 0))
	r.header.Resize(fyne.NewSize(size.Width, top))
	if !r.body.Visible() {
		return
	}
	m := r.letterMargin()
	r.body.Move(fyne.NewPos(m, top))
	r.body.Resize(fyne.NewSize(size.Width-m, size.Height-top))
	r.placeRule(top, size.Height)
}

// letterTop is where the letter stands on the note's own row, from the
// row's top, and letterLine the line it stands on, from its own top.
func (r *crossRefNoteRow) letterTop() float32 {
	return crossRefRowPad + r.head.Position().Y + r.letter.Position().Y
}

func (r *crossRefNoteRow) letterLine() float32 {
	if r.line == 0 {
		_, r.line = fyne.CurrentApp().Driver().RenderedTextSize(r.letter.Text, r.letter.TextSize, r.letter.TextStyle, nil)
	}
	return r.line
}

// placeRule hangs the rule from the letter: it starts a little under the
// line the letter stands on — on the note's own row, or where the letter is
// kept in view — and runs down the margin beside the pieces to a little
// above their foot. A note keyed to no word has no letter, and its rule
// starts with its pieces, at top.
func (r *crossRefNoteRow) placeRule(top, height float32) {
	from := top
	if r.letter.Text != "" {
		at := r.letterTop()
		if r.pinned > 0 && r.pin != nil {
			at = r.pinned
			r.pin.Move(fyne.NewPos(0, at))
			r.pin.Resize(r.pin.MinSize())
		}
		from = at + r.letterLine() + crossRefStemGap
	}
	r.stem.Move(fyne.NewPos(crossRefStemAt, from))
	r.stem.Resize(fyne.NewSize(crossRefStemWidth, height-crossRefStemFoot-from))
}

// crossRefPinTop is how far under the top of the list's view a letter kept
// in view stands.
const crossRefPinTop float32 = 4

// keepLetterInView keeps the note's letter on screen while the list is
// scrolled into the note: a note can be taller than the list, and with its
// own row scrolled out above nothing on screen said which letter its rows
// were under. into is how far the top of the list's view stands below the
// row's top. Past the letter's own line, the letter stands at the top of
// the view, in the margin, the rule hanging from it there; it goes no lower
// than its line's place above the rule's foot, so it leaves the view with
// the foot of the note's rows. The letter on the note's own row is hidden
// meanwhile: the two would pass through each other as the row leaves.
func (r *crossRefNoteRow) keepLetterInView(into float32) {
	pinned := float32(0)
	if r.open && r.letter.Text != "" {
		at := into + crossRefPinTop
		if lowest := r.Size().Height - crossRefStemFoot - crossRefStemGap - r.letterLine(); at > lowest {
			at = lowest
		}
		if at > r.letterTop() {
			pinned = at
		}
	}
	if pinned == r.pinned {
		return
	}
	r.pinned = pinned
	if pinned > 0 {
		if r.pin == nil {
			r.pin = crossRefLetterWithSize(r.letter.Text, r.letter.TextSize, r.pal)
			r.box.Add(r.pin)
		}
		r.pin.Show()
		r.letter.Hide()
	} else {
		if r.pin != nil {
			r.pin.Hide()
		}
		r.letter.Show()
	}
	r.placeRule(0, r.Size().Height)
	r.letter.Refresh()
	r.stem.Refresh()
	if r.pin != nil {
		r.pin.Refresh()
	}
}

type crossRefLetterKeeper interface {
	keepLetterInView(into float32)
}

// keepCrossRefLettersInView keeps in view the letter of whichever open note
// or opened lone group the top of the list's view stands inside. list is the
// list's own box and top the scroll's offset; a note's ground is one of the
// box's objects.
func keepCrossRefLettersInView(list *fyne.Container, top float32) {
	for _, o := range list.Objects {
		section, ok := o.(*fyne.Container)
		if !ok {
			continue
		}
		if _, ok := section.Layout.(crossRefSectionLayout); !ok {
			continue
		}
		if row, ok := section.Objects[1].(crossRefLetterKeeper); ok {
			row.keepLetterInView(top - section.Position().Y - crossRefSectionPadY)
		}
	}
}

func (r *crossRefNoteRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.box)
}

// toggle closes the note to its printed line, or opens it again.
func (r *crossRefNoteRow) toggle() {
	r.open = !r.open
	if r.open {
		r.body.Show()
		r.stem.Show()
	} else {
		r.body.Hide()
		r.stem.Hide()
		r.keepLetterInView(0) // closed, the letter stands on its own row
	}
	r.icon.SetResource(crossRefGroupIcon(r.open))
	r.box.Refresh()
	r.Refresh()
	r.relayout.run()
}

// publisherNoteBlock is one note as the list draws it, under its letter, and
// what it put on screen: the passages followed by the cards that stand open
// when the panel opens (for the Treasury's filter), and its counts.
func publisherNoteBlock(state *AppState, n publisherCrossRef, pal palette, onTap func(crossRef), relayout *crossRefRelayout) (row fyne.CanvasObject, cards []crossRef, groups, lines int) {
	letter := noteLetter(n.Index)
	for _, p := range n.Pieces {
		switch {
		case p.Group != nil:
			groups++
		case p.Target != nil && !p.Target.Current:
			cards = append(cards, p.Target.Ref)
		default:
			lines++ // the reader's own passage, or words: shown, not followed
		}
	}
	// Each shape on the note's own ground, the letter in its margin
	// (crossref_section.go).
	sz := crossRefPanelSizes()
	var block fyne.CanvasObject
	switch tg, alone := n.alone(); {
	case alone && tg.Current:
		block = crossRefLettered(letter, pal, crossRefSelfRow(tg.label(), tg.tag(), pal, false), crossRefRowPad, sz.printed, fyne.TextStyle{})
	case alone && letter == "":
		block = crossRefBeside(crossRefRowRuled(state, targetCard(tg, ""), pal, onTap, false))
	case alone:
		block = crossRefRowRuled(state, targetCard(tg, letter), pal, onTap, false)
	default:
		g, group := n.groupAlone()
		words, bare := n.wordsAlone()
		switch {
		case group && len(g.cards()) == 0:
			// Nothing of the group can be followed: the group as printed,
			// with no row promising citations under it.
			block = crossRefLettered(letter, pal, crossRefWordsRow(n.Text, pal, false), crossRefRowPad, sz.printed, fyne.TextStyle{})
		case group:
			block = newCrossRefGroupRow(state, *g, letter, true, pal, onTap, relayout, false)
		case bare:
			block = crossRefLettered(letter, pal, crossRefWordsRow(words, pal, false), crossRefRowPad, sz.printed, fyne.TextStyle{})
		default:
			block = newCrossRefNoteRow(state, n, pal, onTap, relayout)
		}
	}
	return crossRefSection(block, pal), cards, groups, lines
}
