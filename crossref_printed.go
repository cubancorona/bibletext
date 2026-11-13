package bibletext

// A line of the edition's own print in the cross-references list — a note,
// a parenthesised group, words that are no citation — wrapped so that a
// citation is not cut in two (crossref_note.go; docs/SCRIPTURE_WORKLIST.md,
// S20).
//
// The toolkit's word wrap turns a line at any space, the no-break space
// included, so a printed note wrapped by it could end one row on "(1" and
// begin the next with "John 4:9", which reads as another book, or part
// "Eph." from "6:18". The print has its own places to turn: after the "; "
// between two citations, and, where one citation's list of verses is
// itself longer than a row, after a ", " inside it. Only a piece that has
// neither and is still longer than a row is turned at a space.
//
// Nothing of the text is changed or dropped. Every turn is at a space, and
// the rows, joined again by a space, are the printed line
// (TestAPrintedLineTurnsBetweenCitations).

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// crossRefPrintedLeading is the air between two rows of a printed line.
const crossRefPrintedLeading float32 = 3

// crossRefPrintedGuess is the width a printed line is wrapped for before
// any layout has handed it one: the narrowest phone's list less a note's
// ground, margin and icon, so that the first height reported is not short.
const crossRefPrintedGuess float32 = 220

// crossRefSingleLineMax is the narrowest group well at Extra large (229pt):
// any reference without a cross-book en dash that fits inside it needs only a
// single canvas.Text; cross-book ranges and wider headings use crossRefPrinted
// so they stay on one line while there is room and turn cleanly after the en
// dash (or between words) on a narrow phone.
const crossRefSingleLineMax float32 = 229

// crossRefPrinted is one printed line, wrapped italic title (wordWrap), or
// wrapped reference heading (refWrap).
type crossRefPrinted struct {
	widget.BaseWidget
	Text     string
	color    color.Color
	size     float32
	style    fyne.TextStyle
	guess    float32
	wordWrap bool
	refWrap  bool
}

func newCrossRefPrinted(text string, pal palette) *crossRefPrinted {
	p := &crossRefPrinted{
		Text:  text,
		color: pal.TextMuted,
		size:  crossRefPanelSizes().printed,
		guess: crossRefPrintedGuess,
	}
	p.ExtendBaseWidget(p)
	return p
}

func newCrossRefTitle(text string, size float32, pal palette) *crossRefPrinted {
	p := &crossRefPrinted{
		Text:     text,
		color:    pal.TextMuted,
		size:     size,
		style:    fyne.TextStyle{Italic: true},
		guess:    283,
		wordWrap: true,
	}
	p.ExtendBaseWidget(p)
	return p
}

func newCrossRefRef(text string, size float32, style fyne.TextStyle, col color.Color) *crossRefPrinted {
	p := &crossRefPrinted{
		Text:    text,
		color:   col,
		size:    size,
		style:   style,
		guess:   283,
		refWrap: true,
	}
	p.ExtendBaseWidget(p)
	return p
}

func hasCrossBookEnDash(text string) bool {
	_, after, ok := strings.Cut(text, "–")
	return ok && strings.Contains(after, " ")
}

func newCrossRefHeadingText(text string, size float32, style fyne.TextStyle, col color.Color) fyne.CanvasObject {
	face := crossRefZeroFace(style)
	sz, _ := fyne.CurrentApp().Driver().RenderedTextSize(text, size, style, face)
	if !hasCrossBookEnDash(text) && sz.Width <= crossRefSingleLineMax {
		t := canvas.NewText(text, col)
		t.TextStyle = style
		t.TextSize = size
		t.FontSource = face
		return t
	}
	return newCrossRefRef(text, size, style, col)
}

func (p *crossRefPrinted) textSize() float32 {
	if p.size > 0 {
		return p.size
	}
	return crossRefGroupTextSize
}

func (p *crossRefPrinted) guessWidth() float32 {
	if p.guess > 0 {
		return p.guess
	}
	return crossRefPrintedGuess
}

func (p *crossRefPrinted) CreateRenderer() fyne.WidgetRenderer {
	return &crossRefPrintedRenderer{p: p}
}

// crossRefPrintedMeasure is the width of s as a printed line draws it.
func crossRefPrintedMeasure(s string) float32 {
	sz, _ := fyne.CurrentApp().Driver().RenderedTextSize(s, crossRefPanelSizes().printed, fyne.TextStyle{}, crossRefZeroFace(fyne.TextStyle{}))
	return sz.Width
}

// printedRows is text in rows no wider than width, turned at the print's
// own places: between citations where it can, inside a list of verses where
// one citation is longer than a row, and at a space only where a piece with
// neither is. Every turn takes one space, and no row is empty.
func printedRows(text string, width float32, measure func(string) float32) []string {
	var rows []string
	row := ""
	// put places piece at the end of the row, or opens a new row with it.
	put := func(piece string) {
		if row != "" && measure(strings.TrimRight(row+piece, " ")) > width {
			rows = append(rows, strings.TrimRight(row, " "))
			row = ""
		}
		row += piece
	}
	// A piece that fits a row of its own is kept whole; one that does not is
	// taken apart at the next finer place the print has.
	var place func(piece string, seps []string)
	place = func(piece string, seps []string) {
		if measure(strings.TrimRight(piece, " ")) <= width || len(seps) == 0 {
			put(piece)
			return
		}
		for _, part := range splitAfter(piece, seps[0]) {
			place(part, seps[1:])
		}
	}
	for _, piece := range splitAfter(text, "; ") {
		place(piece, []string{", ", " "})
	}
	if row != "" {
		rows = append(rows, strings.TrimRight(row, " "))
	}
	return rows
}

// splitAfter cuts s after every sep, each part keeping the sep that ends it.
func splitAfter(s, sep string) []string {
	var out []string
	for {
		i := strings.Index(s, sep)
		if i < 0 {
			break
		}
		out = append(out, s[:i+len(sep)])
		s = s[i+len(sep):]
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}

type crossRefPrintedRenderer struct {
	p     *crossRefPrinted
	width float32
	size  float32
	rowH  float32
	rows  []*canvas.Text
	objs  []fyne.CanvasObject
}

func (r *crossRefPrintedRenderer) measure(s string) float32 {
	sz, _ := fyne.CurrentApp().Driver().RenderedTextSize(s, r.p.textSize(), r.p.style, crossRefZeroFace(r.p.style))
	return sz.Width
}

func wordWrappedRows(text string, width float32, measure func(string) float32) []string {
	var rows []string
	row := ""
	for _, word := range strings.Fields(text) {
		if row != "" && measure(row+" "+word) > width {
			rows = append(rows, row)
			row = word
			continue
		}
		if row != "" {
			row += " "
		}
		row += word
	}
	if row != "" {
		rows = append(rows, row)
	}
	return rows
}

// referenceRows keeps a reference heading on one row whenever it fits width,
// turns a range after its en dash ("1 Thessalonians 5:28–" / "2 Thessalonians
// 1:2") before splitting words, and falls back to word wrapping only inside a
// piece that is still wider than width.
func referenceRows(text string, width float32, measure func(string) float32) []string {
	if text == "" {
		return nil
	}
	if measure(text) <= width {
		return []string{text}
	}
	if before, after, ok := strings.Cut(text, "–"); ok && after != "" {
		first := before + "–"
		if measure(first) <= width && measure(after) <= width {
			return []string{first, after}
		}
		return append(wordWrappedRows(first, width, measure), wordWrappedRows(after, width, measure)...)
	}
	return wordWrappedRows(text, width, measure)
}

func (r *crossRefPrintedRenderer) wrap(width float32) {
	if width <= 0 {
		width = r.p.guessWidth()
	}
	size := r.p.textSize()
	if width == r.width && size == r.size && r.rows != nil {
		return
	}
	r.width = width
	r.size = size
	face := crossRefZeroFace(r.p.style)
	sz, _ := fyne.CurrentApp().Driver().RenderedTextSize("Ag", size, r.p.style, face)
	r.rowH = sz.Height
	r.rows, r.objs = r.rows[:0], r.objs[:0]
	var lines []string
	switch {
	case r.p.refWrap:
		lines = referenceRows(r.p.Text, width, r.measure)
	case r.p.wordWrap:
		lines = wordWrappedRows(r.p.Text, width, r.measure)
	default:
		lines = printedRows(r.p.Text, width, r.measure)
	}
	for _, s := range lines {
		t := canvas.NewText(s, r.p.color)
		t.TextSize = size
		t.TextStyle = r.p.style
		t.FontSource = face
		r.rows = append(r.rows, t)
		r.objs = append(r.objs, t)
	}
}

func (r *crossRefPrintedRenderer) Layout(size fyne.Size) {
	r.wrap(size.Width)
	y := float32(0)
	for _, t := range r.rows {
		t.Move(fyne.NewPos(0, y))
		t.Resize(fyne.NewSize(size.Width, r.rowH))
		y += r.rowH + crossRefPrintedLeading
	}
}

// MinSize is the height of the rows at the last width wrapped for, as the
// passage's paragraph reports its own (verse_of_day.go), and a width no
// greater than the longest indivisible word, which a row cannot be narrower than.
func (r *crossRefPrintedRenderer) MinSize() fyne.Size {
	r.wrap(r.width)
	var w float32
	tokens := r.p.Text
	if r.p.refWrap {
		tokens = strings.ReplaceAll(tokens, "–", "– ")
	}
	for _, word := range strings.Fields(tokens) {
		if m := r.measure(word); m > w {
			w = m
		}
	}
	n := float32(len(r.rows))
	if n == 0 {
		return fyne.NewSize(w, 0)
	}
	return fyne.NewSize(w, n*r.rowH+(n-1)*crossRefPrintedLeading)
}

func (r *crossRefPrintedRenderer) Refresh() {
	r.rows = nil
	r.wrap(r.width)
	r.Layout(r.p.Size())
	canvas.Refresh(r.p)
}

func (r *crossRefPrintedRenderer) Objects() []fyne.CanvasObject { return r.objs }
func (r *crossRefPrintedRenderer) Destroy()                     {}

// rowTexts is the rows as drawn, for a test to read.
func (r *crossRefPrintedRenderer) rowTexts() []string {
	out := make([]string, 0, len(r.rows))
	for _, t := range r.rows {
		out = append(out, t.Text)
	}
	return out
}
