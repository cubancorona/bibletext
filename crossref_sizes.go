package bibletext

// Reader-adjustable text sizing for the cross-references panel — S20.
//
// The reading page scales across Normal (1.0), Large (1.15) and Extra large
// (1.3) via readingTextScale (textsize.go). Without matching scaling in the
// cross-references modal, a reader using Extra large tapped into a fixed 17pt
// passage and 14pt printed notes.
//
// Prose that wraps freely (the selected passage at the head of the list and
// each card's verse preview) follows readingTextScale in full. Headings, note
// letters, printed shorthand rows, tags and the note-letter margin step up on
// a gentler ramp capped at Extra large's geometry so the hierarchy
// (caption < tag/title < printed < reference < preview) holds strictly at every
// setting and every reference fits the narrowest 375pt phone's list and group
// wells without clipping. At Normal (1.0), every size and margin is unchanged.

import (
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type crossRefSizes struct {
	passage      float32 // nominal passage size before readingOpticalScale (17 at Normal)
	preview      float32 // card preview body inside listScope (17 at Normal)
	reference    float32 // card reference heading (16 at Normal)
	letter       float32 // single-letter margin mark, equal to reference (16 at Normal)
	doubleLetter float32 // two-letter margin mark "aa".."zz" (10 at Normal)
	letterMargin float32 // width of the note letter's margin (18 at Normal)
	printed      float32 // printed shorthand, self-row and inline retry (14 at Normal)
	title        float32 // card basis/title tag and parallel pericope title (12 at Normal)
	caption      float32 // inline list caption inside listScope (11 at Normal)
}

func crossRefSizesFor(scale float64) crossRefSizes {
	if scale <= 0 {
		scale = 1.0
	}
	if scale < 0.8 {
		scale = 0.8
	}
	prose := float32(crossRefPassageTextSize * scale)
	c := scale
	if c > 1.3 {
		c = 1.3
	}
	if c <= 1.0 {
		f := float32(c)
		ref := float32(crossRefLetterSize) * f
		return crossRefSizes{
			passage:      prose,
			preview:      prose,
			reference:    ref,
			letter:       ref,
			doubleLetter: float32(crossRefDoubleLetterSize) * f,
			letterMargin: float32(math.Round(float64(crossRefLetterMargin) * c)),
			printed:      float32(crossRefGroupTextSize) * f,
			title:        12 * f,
			caption:      11 * f,
		}
	}
	d := float32(c - 1.0)
	ref := float32(crossRefLetterSize) + 10*d
	ratio := ref / float32(crossRefLetterSize)
	return crossRefSizes{
		passage:      prose,
		preview:      prose,
		reference:    ref,
		letter:       ref,
		doubleLetter: float32(crossRefDoubleLetterSize) * ratio,
		letterMargin: float32(math.Ceil(float64(float32(crossRefLetterMargin) * ratio))),
		printed:      float32(crossRefGroupTextSize) + 10*d,
		title:        12 + 7*d,
		caption:      11 + 7*d,
	}
}

func crossRefPanelSizes() crossRefSizes {
	return crossRefSizesFor(readingTextScale())
}

// crossRefScopeTheme scopes the list's base text size (card previews) and
// inline caption size to the active crossRefSizes while delegating zero inner
// padding and all other metrics to the wrapped theme.
type crossRefScopeTheme struct {
	fyne.Theme
	text    float32
	caption float32
}

func (t crossRefScopeTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return t.text
	case theme.SizeNameCaptionText:
		return t.caption
	}
	return t.Theme.Size(name)
}

// crossRefHeadingFlow places a reference and its trailing tag or badge side by
// side when the available width fits both, and drops the trailing object onto
// a second line when narrower so neither overflows a narrow phone's group well
// at Large or Extra large text sizes.
type crossRefHeadingFlow struct {
	gap   float32
	width float32
}

func (f *crossRefHeadingFlow) MinSize(objs []fyne.CanvasObject) fyne.Size {
	if len(objs) == 0 {
		return fyne.Size{}
	}
	if len(objs) == 1 {
		return objs[0].MinSize()
	}
	lead, trail := objs[0].MinSize(), objs[1].MinSize()
	minW := max(lead.Width, trail.Width)
	avail := f.width
	if avail <= 0 {
		avail = 283
	}
	if lead.Width+f.gap+trail.Width <= avail {
		return fyne.NewSize(minW, max(lead.Height, trail.Height))
	}
	return fyne.NewSize(minW, lead.Height+2+trail.Height)
}

func (f *crossRefHeadingFlow) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	f.width = size.Width
	if len(objs) == 0 {
		return
	}
	if len(objs) == 1 {
		objs[0].Move(fyne.NewPos(0, 0))
		objs[0].Resize(size)
		return
	}
	lead, trail := objs[0].MinSize(), objs[1].MinSize()
	if lead.Width+f.gap+trail.Width <= size.Width {
		rowH := max(lead.Height, trail.Height)
		objs[0].Move(fyne.NewPos(0, (rowH-lead.Height)/2))
		objs[0].Resize(lead)
		objs[1].Move(fyne.NewPos(lead.Width+f.gap, (rowH-trail.Height)/2))
		objs[1].Resize(trail)
		return
	}
	objs[0].Move(fyne.NewPos(0, 0))
	objs[0].Resize(lead)
	objs[1].Move(fyne.NewPos(0, lead.Height+2))
	objs[1].Resize(trail)
}
