package bibletext

// The status line under an API key field — "✓ Included with BibleText — or
// paste your own.", "✓ Saved in the Keychain.", a provider's key hint — in both
// key sections of Settings.
//
// It was a bare canvas.Text, which draws its whole string on one line whatever
// width it is given. Nothing broke it when the text was longer than the row:
// on a 320pt phone the error lines ("Couldn't save this key securely. Please
// try again.") are wider than the card, and the scroll the body sits in cut
// them off at its edge. This one draws the same text, in the same size and
// colour and in the same place, and breaks it between words when the row is
// narrower than the text. One line reads exactly as the canvas.Text did.

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

type statusLine struct {
	widget.BaseWidget
	text  string
	color color.Color
	size  float32
	style fyne.TextStyle
}

func newStatusLine(size float32) *statusLine {
	s := &statusLine{size: size}
	s.ExtendBaseWidget(s)
	return s
}

// newStatusText is a statusLine that says text in c from the start, in a
// style — the translation picker's rows, whose names are bold and whose
// captions italic (versions_ui.go). Measured in that style, so a bold line
// breaks where its bold width says.
func newStatusText(text string, size float32, style fyne.TextStyle, c color.Color) *statusLine {
	s := &statusLine{text: text, color: c, size: size, style: style}
	s.ExtendBaseWidget(s)
	return s
}

// set shows text in c. It reports whether the line's height changed at the
// width it is laid out at — one line became two, or two one — which the sheet
// holding it has to be re-measured for.
func (s *statusLine) set(text string, c color.Color) (heightChanged bool) {
	before := s.MinSize().Height
	s.text, s.color = text, c
	s.Refresh()
	return s.MinSize().Height != before
}

func (s *statusLine) CreateRenderer() fyne.WidgetRenderer {
	s.ExtendBaseWidget(s)
	return &statusLineRenderer{line: s}
}

// statusLines breaks text into lines no wider than width at the given size,
// between words. A dash that stands between two words stays at the end of the
// line before it, so no line opens with one. width <= 0 means not yet laid
// out: the text is one line. A single word wider than width has a line to
// itself rather than being cut.
func statusLines(text string, width, size float32) []string {
	return statusLinesStyled(text, width, size, fyne.TextStyle{})
}

// statusLinesStyled is statusLines measured in a style.
func statusLinesStyled(text string, width, size float32, style fyne.TextStyle) []string {
	if text == "" {
		return nil
	}
	var words []string
	for _, w := range strings.Split(text, " ") {
		if (w == "—" || w == "–") && len(words) > 0 {
			words[len(words)-1] += " " + w
			continue
		}
		words = append(words, w)
	}
	lines := []string{words[0]}
	for _, w := range words[1:] {
		last := &lines[len(lines)-1]
		joined := *last + " " + w
		if width > 0 && fyne.MeasureText(joined, size, style).Width > width {
			lines = append(lines, w)
			continue
		}
		*last = joined
	}
	return lines
}

type statusLineRenderer struct {
	line  *statusLine
	texts []*canvas.Text
	objs  []fyne.CanvasObject
}

// rowHeight is the height of one line: the height a canvas.Text of this size
// reports, which is what the single canvas.Text before it measured.
func (r *statusLineRenderer) rowHeight() float32 {
	return fyne.MeasureText("M", r.line.size, r.line.style).Height
}

func (r *statusLineRenderer) Layout(size fyne.Size) {
	lines := statusLinesStyled(r.line.text, size.Width, r.line.size, r.line.style)
	for len(r.texts) < len(lines) {
		t := canvas.NewText("", r.line.color)
		r.texts = append(r.texts, t)
	}
	r.texts = r.texts[:len(lines)]
	r.objs = r.objs[:0]
	row := r.rowHeight()
	for i, s := range lines {
		t := r.texts[i]
		t.Text, t.Color, t.TextSize, t.TextStyle = s, r.line.color, r.line.size, r.line.style
		t.Move(fyne.NewPos(0, float32(i)*row))
		t.Resize(fyne.NewSize(size.Width, row))
		t.Refresh()
		r.objs = append(r.objs, t)
	}
}

// MinSize is as wide as the widest word, so the line never forces its row
// wider than a word, and as tall as the lines the text breaks into at the
// width it was last laid out at.
func (r *statusLineRenderer) MinSize() fyne.Size {
	lines := statusLinesStyled(r.line.text, r.line.Size().Width, r.line.size, r.line.style)
	n := len(lines)
	if n == 0 {
		n = 1 // an empty line keeps its row, as the canvas.Text did
	}
	var widest float32
	for _, w := range strings.Fields(r.line.text) {
		if ww := fyne.MeasureText(w, r.line.size, r.line.style).Width; ww > widest {
			widest = ww
		}
	}
	return fyne.NewSize(widest, float32(n)*r.rowHeight())
}

func (r *statusLineRenderer) Refresh() {
	r.Layout(r.line.Size())
	canvas.Refresh(r.line)
}

func (r *statusLineRenderer) Objects() []fyne.CanvasObject { return r.objs }
func (r *statusLineRenderer) Destroy()                     {}
