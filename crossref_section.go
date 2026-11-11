package bibletext

// One note of the publisher's apparatus set apart from the next — the
// cross-references list's publisher run (crossref_note.go;
// docs/SCRIPTURE_WORKLIST.md, S20).
//
// Decided 11 November 2026: a reader sees at a glance which rows are a
// note's. Each note stands on a ground of its own, rounded, with a gap
// between one note's ground and the next. The note's letter stands alone in
// a margin at the ground's left, and everything else of the note stands
// beside that margin: the note as printed, its cards, the reader's own
// passage, a parenthesised group's row and the cards that row opens into.
// From under the letter a rule runs down the margin beside the note's rows,
// so that a note taller than the list still shows, with its ends scrolled
// out of sight, that its rows are one note's; it is the one device here that
// holds on a dim screen, where a ground's edge is faint. A group opened
// inside a note stands on a third ground, neither the note's nor the
// panel's, a well in the note's (crossRefGrounds).
//
// It was first built the same day as rows set in from the list's edge
// beside a quiet rule, with a rule the width of the list closing the note.
// On a phone that did not show where a letter's rows ended: a 2-point rule
// and a 16-point inset were too slight beside cards at the full type size.
//
// A parenthesised group is closed until it is tapped wherever it stands: a
// note's own direct citations stand open when the panel opens, and a group —
// whether inside a note of several pieces or the whole of its note — is drawn
// as the group's row beside the letter's margin and opens onto the group's
// well (crossref_group.go).
//
// Width: the ground's padding and the letter's margin are taken from the
// note's rows, so the widest of them still has to fit the list on the
// narrowest phone: TestCrossRefListFitsTheNarrowestPhone measures every
// object of the list, TestTheListFitsTheNarrowestPhoneWithAGroupOpenInsideANote
// measures it with a group open, and
// TestTheCanonFitsTheNarrowestPhoneWithEveryGroupOpen measures every verse
// of a fetched canon that has a note.

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
)

const (
	// crossRefSectionPadX and crossRefSectionPadY are a note's ground's
	// inner padding, at its sides and above and below.
	crossRefSectionPadX float32 = 8
	crossRefSectionPadY float32 = 2
	// crossRefSectionGap is the space between one note's ground and the
	// next thing in the list: the widest interval in the publisher's run,
	// where the rows of one note are a hairline apart.
	crossRefSectionGap float32 = 10
	// crossRefSectionRadius rounds the ground's corners.
	crossRefSectionRadius float32 = 10
	// crossRefLetterMargin is the width of the margin the letter stands in.
	crossRefLetterMargin float32 = 18
	// crossRefStemAt is the left edge of the rule under the letter, which
	// puts it under the middle of the letter, and crossRefStemWidth its
	// width. crossRefStemGap is how far under the line the letter stands on
	// the rule starts, and crossRefStemFoot how far above the foot of the
	// note's rows it stops, so that it hangs from the letter and does not
	// run into the ground's edge.
	crossRefStemAt    float32 = 3
	crossRefStemWidth float32 = 3
	crossRefStemGap   float32 = 5
	crossRefStemFoot  float32 = 6
)

// crossRefStemColour is the rule's colour: the letter's, a little quieter
// than the letter, and better than 4:1 against the note's ground in both
// themes, so that it still reads where the ground's own edge does not.
func crossRefStemColour(pal palette) color.NRGBA { return withAlpha(pal.Accent, 200) }

// crossRefGrounds are the two grounds of the publisher's run: the one a
// note stands on, and the one a group opened inside a note stands on. The
// panel stands on the palette's alternate surface. Of the palette's two
// other grounds a note takes the one further in lightness from the panel's
// — the page's near-white in the light theme, the window's near-black in
// the dark — and an opened group the other. So a note stands out from the
// panel as far as the palette allows in both themes, and a group opened
// inside it is a well in the note's ground and not the panel showing
// through it: the gap between two notes is the panel's colour, and a well
// of that colour would cut a long note in two.
func crossRefGrounds(pal palette) (note, group color.NRGBA) {
	away := func(c color.NRGBA) float64 {
		d := crossRefLightness(c) - crossRefLightness(pal.SurfaceAlt)
		if d < 0 {
			return -d
		}
		return d
	}
	if away(pal.Surface) >= away(pal.Background) {
		return pal.Surface, pal.Background
	}
	return pal.Background, pal.Surface
}

// crossRefLightness is a colour's lightness, enough to order two grounds.
func crossRefLightness(c color.NRGBA) float64 {
	return 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
}

func crossRefSectionGround(pal palette) color.NRGBA {
	note, _ := crossRefGrounds(pal)
	return note
}

func crossRefGroupGround(pal palette) color.NRGBA {
	_, group := crossRefGrounds(pal)
	return group
}

// crossRefGroupEdge is the edge of an opened group's ground: the palette's
// card edge, which reads against a note's ground in both themes where the
// group's fill alone is the faintest step of the three grounds.
func crossRefGroupEdge(pal palette) color.NRGBA { return pal.Border }

// crossRefHover is the wash under the pointer on a row or a card of the
// list, on a desktop: the theme's own, a tint and not a ground, so that it
// shows on the panel's ground, a note's and an opened group's alike. The
// panel's own colour stood here while every card stood on the panel, where
// it showed nothing, and would have shown on the publisher's rows alone.
func crossRefHover(pal palette) color.NRGBA { return paletteHover(pal) }

// crossRefSectionLayout lays a note's block on its ground, with the gap
// under it. Its objects are the ground, then the block.
type crossRefSectionLayout struct{}

func (crossRefSectionLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	b := objs[1].MinSize()
	return fyne.NewSize(b.Width+2*crossRefSectionPadX, b.Height+2*crossRefSectionPadY+crossRefSectionGap)
}

func (crossRefSectionLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	objs[0].Move(fyne.NewPos(0, 0))
	objs[0].Resize(fyne.NewSize(size.Width, size.Height-crossRefSectionGap))
	objs[1].Move(fyne.NewPos(crossRefSectionPadX, crossRefSectionPadY))
	objs[1].Resize(fyne.NewSize(size.Width-2*crossRefSectionPadX, size.Height-crossRefSectionGap-2*crossRefSectionPadY))
}

// crossRefSection is one note's block on its ground.
func crossRefSection(block fyne.CanvasObject, pal palette) *fyne.Container {
	ground := canvas.NewRectangle(crossRefSectionGround(pal))
	ground.CornerRadius = crossRefSectionRadius
	return container.New(crossRefSectionLayout{}, ground, block)
}

// crossRefLetteredLayout stands a note's letter in the margin and what it
// heads beside it. Its objects are the letter, then the content. drop is how
// far below the content's top the letter stands, so that it sits on the
// line of the content's first row of text.
type crossRefLetteredLayout struct{ drop float32 }

func (l crossRefLetteredLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	c, letter := objs[1].MinSize(), objs[0].MinSize()
	h := c.Height
	if lh := letter.Height + l.drop; lh > h {
		h = lh
	}
	return fyne.NewSize(c.Width+crossRefLetterMargin, h)
}

func (l crossRefLetteredLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	objs[0].Move(fyne.NewPos(0, l.drop))
	objs[0].Resize(objs[0].MinSize())
	objs[1].Move(fyne.NewPos(crossRefLetterMargin, 0))
	objs[1].Resize(fyne.NewSize(size.Width-crossRefLetterMargin, size.Height))
}

// crossRefLettered is content with the note's letter in the margin beside
// its first row of text, which is set at size in style and stands top below
// the content's own top. Every shape sets that first row on a canvas.Text or
// crossRefPrinted, neither of which takes theme.SizeNameInnerPadding, so the
// letter sits on the line outside the panel's theme scope as well as inside
// it (a card's widget.Label preview shares the reference's left edge only
// inside passageCitationTheme, where inner padding is 0). A note keyed to no
// word has no letter, and its content stands beside an empty margin, in line
// with the other notes'.
func crossRefLettered(letter string, pal palette, content fyne.CanvasObject, top, size float32, style fyne.TextStyle) *fyne.Container {
	mark := crossRefLetter(letter, pal)
	drop := top
	if letter != "" {
		drv := fyne.CurrentApp().Driver()
		_, first := drv.RenderedTextSize("Ag", size, style, nil)
		_, own := drv.RenderedTextSize(letter, mark.TextSize, mark.TextStyle, nil)
		drop += first - own
	}
	return container.New(crossRefLetteredLayout{drop: drop}, mark, content)
}

// crossRefBeside is a row that stands beside the margin with no letter in
// it and no rule: the one card of a note keyed to no word.
func crossRefBeside(row fyne.CanvasObject) *fyne.Container {
	return container.New(layout.NewCustomPaddedLayout(0, 0, crossRefLetterMargin, 0), row)
}
