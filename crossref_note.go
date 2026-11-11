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
//     (or, where the loaded text has no such passage, its words);
//   - a note that is one parenthesised group is the group's row
//     (crossref_group.go);
//   - every other note is a row that carries the note exactly as printed and
//     folds: under it, while it is open, the note's pieces in the order the
//     edition prints them. It is open when the panel opens, and a tap on the
//     row closes it. The printed text stays on the row in both states, so
//     the edition's own punctuation — which citations a comma joins, where a
//     group stands, what was carried from the citation before — is never off
//     screen. The pieces are set in from the list's edge beside a rule, and
//     one rule at the edge closes the note, so that a reader sees which rows
//     are under the note's letter and where the next letter begins
//     (crossref_section.go).
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
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// crossRefLetterSize is a note's letter before a card's reference, and the
// letter is the group label's size before a printed line.
const crossRefLetterSize = 14

// crossRefLetter is a note's letter set before a card's reference line.
func crossRefLetter(letter string, pal palette) *canvas.Text {
	t := canvas.NewText(letter, pal.Accent)
	t.TextStyle = fyne.TextStyle{Bold: true, Italic: true}
	t.TextSize = crossRefLetterSize
	return t
}

// crossRefPrintedLine is a line of the edition's own print — a note, a
// group, words that are no citation — at the group label's size, muted and
// wrapping, with the note's letter before it when it opens the note.
func crossRefPrintedLine(letter, printed string) *widget.RichText {
	var segments []widget.RichTextSegment
	if letter != "" {
		segments = append(segments, &widget.TextSegment{Text: letter + "  ", Style: widget.RichTextStyle{
			Inline: true, ColorName: theme.ColorNamePrimary, SizeName: sizeNameCrossRefGroup,
			TextStyle: fyne.TextStyle{Bold: true, Italic: true},
		}})
	}
	segments = append(segments, &widget.TextSegment{Text: printed, Style: widget.RichTextStyle{
		Inline: true, ColorName: theme.ColorNamePlaceHolder, SizeName: sizeNameCrossRefGroup,
	}})
	rt := widget.NewRichText(segments...)
	rt.Wrapping = fyne.TextWrapWord
	return rt
}

// crossRefSelfRow is a citation of the verse or passage the reader is on:
// its reference in full, muted, in the place the edition printed it, with no
// preview and nothing to tap, since it leads nowhere the reader is not.
func crossRefSelfRow(letter, label, tag string, pal palette, rule bool) fyne.CanvasObject {
	ref := canvas.NewText(label, pal.TextMuted)
	ref.TextStyle = fyne.TextStyle{Bold: true}
	ref.TextSize = 16
	var line []fyne.CanvasObject
	if letter != "" {
		line = append(line, crossRefLetter(letter, pal))
	}
	line = append(line, ref)
	if tag != "" {
		t := canvas.NewText(tag, pal.TextMuted)
		t.TextSize = 12
		line = append(line, t)
	}
	block := []fyne.CanvasObject{container.New(layout.NewCustomPaddedLayout(6, 6, 0, 0), container.NewHBox(line...))}
	if rule {
		block = append(block, widget.NewSeparator())
	}
	return container.New(layout.NewCustomPaddedVBoxLayout(0), block...)
}

// crossRefWordsRow is a piece of a note that is no citation the reader can
// follow, exactly as printed, in its place.
func crossRefWordsRow(letter, words string, rule bool) fyne.CanvasObject {
	block := []fyne.CanvasObject{container.New(layout.NewCustomPaddedLayout(6, 6, 0, 0), crossRefPrintedLine(letter, words))}
	if rule {
		block = append(block, widget.NewSeparator())
	}
	return container.New(layout.NewCustomPaddedVBoxLayout(0), block...)
}

// targetCard is the card of one resolved citation.
func targetCard(tg publisherTarget, letter string) crossRefCard {
	return crossRefCard{Ref: tg.Ref, Label: tg.label(), Tag: tg.tag(), Title: tg.Title, Letter: letter}
}

// publisherPieceRows draws pieces in the order the edition prints them.
// ruleLast says whether the last piece closes with a rule. It does not where
// the pieces stand under a note's row or on a group's shaded ground: there
// the rule that closes the note or the group, the width of the list, stands
// under them (crossref_section.go).
func publisherPieceRows(state *AppState, pieces []publisherPiece, pal palette, onTap func(crossRef), relayout *crossRefRelayout, ruleLast bool) []fyne.CanvasObject {
	rows := make([]fyne.CanvasObject, 0, len(pieces))
	for i, p := range pieces {
		rule := ruleLast || i < len(pieces)-1
		switch {
		case p.Group != nil:
			rows = append(rows, newCrossRefGroupRow(state, *p.Group, "", pal, onTap, relayout, rule))
		case p.Target != nil && p.Target.Current:
			rows = append(rows, crossRefSelfRow("", p.Target.label(), p.Target.tag(), pal, rule))
		case p.Target != nil:
			rows = append(rows, crossRefRowRuled(state, targetCard(*p.Target, ""), pal, onTap, rule))
		default:
			rows = append(rows, crossRefWordsRow("", p.Words, rule))
		}
	}
	return rows
}

// crossRefNoteRow is the row of a note that is more than one piece: the note
// as printed, and under it its pieces.
type crossRefNoteRow struct {
	widget.BaseWidget
	note     publisherCrossRef
	relayout *crossRefRelayout

	open   bool
	header *tapCard          // the printed note and its icon: the tap that closes and opens
	icon   *widget.Icon      // points down while open, right while closed
	label  *widget.RichText  // the note's letter and the note as printed
	body   *fyne.Container   // the note's pieces, in print order, set in under the letter; hidden while closed
	foot   *widget.Separator // the rule that closes the note, the width of the list, open or closed
	box    *fyne.Container
	pieces []fyne.CanvasObject
}

func newCrossRefNoteRow(state *AppState, n publisherCrossRef, pal palette, onTap func(crossRef), relayout *crossRefRelayout) *crossRefNoteRow {
	r := &crossRefNoteRow{note: n, relayout: relayout, open: true}
	r.label = crossRefPrintedLine(noteLetter(n.Index), n.Text)
	r.icon = widget.NewIcon(crossRefGroupIcon(true))
	middle := container.NewVBox(layout.NewSpacer(), r.label, layout.NewSpacer())
	row := container.NewBorder(nil, nil, nil, container.NewCenter(r.icon), middle)
	r.header = newTapCard(container.New(layout.NewCustomPaddedLayout(6, 6, 0, 0), row), pal.SurfaceAlt, r.toggle)
	// The pieces stand under the letter: set in from the edge beside the
	// rule, with the note's own rule under them at the edge
	// (crossref_section.go).
	r.pieces = publisherPieceRows(state, n.Pieces, pal, onTap, relayout, false)
	r.body = crossRefUnder(container.New(layout.NewCustomPaddedVBoxLayout(0), r.pieces...), pal)
	r.foot = widget.NewSeparator()
	r.box = container.New(layout.NewCustomPaddedVBoxLayout(0), r.header, r.body, r.foot)
	r.ExtendBaseWidget(r)
	return r
}

func (r *crossRefNoteRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.box)
}

// toggle closes the note to its printed line, or opens it again.
func (r *crossRefNoteRow) toggle() {
	r.open = !r.open
	if r.open {
		r.body.Show()
	} else {
		r.body.Hide()
	}
	r.icon.SetResource(crossRefGroupIcon(r.open))
	r.box.Refresh()
	r.Refresh()
	r.relayout.run()
}

// publisherNoteBlock is one note as the list draws it, under its letter, and
// what it put on screen: the passages its cards follow (for the Treasury's
// filter), and its counts.
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
	if tg, ok := n.alone(); ok {
		if tg.Current {
			return crossRefSelfRow(letter, tg.label(), tg.tag(), pal, true), cards, groups, lines
		}
		return crossRefRowRuled(state, targetCard(tg, letter), pal, onTap, true), cards, groups, lines
	}
	if g, ok := n.groupAlone(); ok {
		return newCrossRefGroupRow(state, *g, letter, pal, onTap, relayout, true), cards, groups, lines
	}
	if words, ok := n.wordsAlone(); ok {
		return crossRefWordsRow(letter, words, true), cards, groups, lines
	}
	return newCrossRefNoteRow(state, n, pal, onTap, relayout), cards, groups, lines
}
