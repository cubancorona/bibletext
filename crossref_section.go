package bibletext

// What stands under a letter, seen to stand under it — the cross-references
// list's publisher run (crossref_note.go; docs/SCRIPTURE_WORKLIST.md, S20).
//
// Decided 11 November 2026: a reader sees at a glance which rows are a
// note's. The note's own row, with its letter, stands at the list's edge.
// Everything under it — its cards, the reader's own passage, a parenthesised
// group's row and the cards that row opens into — is set in from the edge
// beside a rule that runs the whole height of what the note holds. One rule
// the width of the list closes the note, and the next letter stands back at
// the edge.
//
// Until then a note's pieces stood at the edge as the next note's row did: a
// group under the letter a and the note b after it read as two equal rows,
// and nothing showed where a's citations ended.
//
// A note that is one card has nothing under it and is drawn as before. A
// note that is one parenthesised group is the group's row at the edge, and
// what the group opens into is set in the same way.
//
// Width: the inset is taken from the rows under a letter, so the widest of
// them still has to fit the list on the narrowest phone
// (TestCrossRefListFitsTheNarrowestPhone, with every note and group open).

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

const (
	// crossRefUnderInset is how far the rows under a letter stand in from
	// the list's edge.
	crossRefUnderInset float32 = 16
	// crossRefUnderRuleAt is the left edge of the rule they stand beside,
	// and crossRefUnderRuleWidth its width.
	crossRefUnderRuleAt    float32 = 4
	crossRefUnderRuleWidth float32 = 2
)

// crossRefUnderRuleColour is the rule's colour: the letter's, quieter, so
// the rule reads as the letter's own.
func crossRefUnderRuleColour(pal palette) color.Color {
	return withAlpha(pal.Accent, 110)
}

// crossRefUnderLayout sets a body in from the left edge beside a rule of the
// body's whole height. Its objects are the rule, then the body.
type crossRefUnderLayout struct{}

func (crossRefUnderLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	body := objs[1].MinSize()
	return fyne.NewSize(body.Width+crossRefUnderInset, body.Height)
}

func (crossRefUnderLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	objs[0].Move(fyne.NewPos(crossRefUnderRuleAt, 0))
	objs[0].Resize(fyne.NewSize(crossRefUnderRuleWidth, size.Height))
	objs[1].Move(fyne.NewPos(crossRefUnderInset, 0))
	objs[1].Resize(fyne.NewSize(size.Width-crossRefUnderInset, size.Height))
}

// crossRefUnder is body as it stands under a letter: set in beside the rule.
func crossRefUnder(body fyne.CanvasObject, pal palette) *fyne.Container {
	rule := canvas.NewRectangle(crossRefUnderRuleColour(pal))
	rule.CornerRadius = crossRefUnderRuleWidth / 2
	return container.New(crossRefUnderLayout{}, rule, body)
}
