package bibletext

// THE SELECTED WORDS ON THE ADD A NOTE SHEET.
//
// The composer hides the reading text while it is open (the native reading
// view floats above the canvas and would cover the sheet), so writing about a
// phrase used to mean remembering it. The sheet now shows the words under the
// reference, small and muted: a reminder of what the note is about, not a
// second reading view, so it stops after a few lines and the last one ends in
// an ellipsis when the selection runs on.
//
// THE WORDS ARE THE SHARE'S. They are the text prepareShareQuote makes of the
// selection, from the same call that gives the sheet its reference and the
// share its citation (shareNoteQuote). So the verse numbers a selection
// carries are gone, a word the drag cut in half is repaired as the share
// repairs it, and the divine name stays in the small capitals the page draws
// it in, as a reader's share keeps it (sharedText, outbound_text.go). There is
// no second cleaning pass to disagree with the one a share runs.
//
// THEY ARE SET IN THE READING FACE, not the chrome face the rest of the sheet
// uses: the chrome face has no glyphs for the Unicode small capitals
// (small_caps_draw.go), and the reading face has them in every cut. Rows are
// canvas.Text, the only toolkit text that takes a FontSource, which is why
// this is a widget of its own rather than a RichText with a line cap; the
// verse-of-the-day card's readingParagraph is the same arrangement.
//
// IT NEVER WIDENS THE CARD. Its minimum width is zero, like the counter's
// wrapping RichText (share_note_ui.go): on a phone the non-modal popup grows
// the card to its content's minimum width, past the canvas's right edge and
// the Share button with it, so nothing in the sheet may ask for width. The
// excerpt wraps into what it is given, and a word wider than a row is broken
// between letters rather than drawn past the card's edge.
//
// ITS HEIGHT IS THE HEIGHT OF ITS ROWS AT THE WIDTH IT WAS LAST GIVEN, the
// contract readingParagraph keeps: MinSize is right once the excerpt has been
// laid out at the width it is drawn at. The sheet lays its form out before it
// reads a height (share_note_ui.go), so the note field's slot, and on iOS the
// native text view parked over it, starts below the last row, and a later
// change of width reaches the form through the toolkit's per-frame minimum
// size pass, which lays out again the parent of any object whose minimum size
// has changed.

import (
	"image/color"
	"strings"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

const (
	// noteExcerptMaxLines is three. A row at the excerpt's size holds about
	// forty-five characters on a phone in portrait, so three rows hold a
	// phrase whole and most of an average verse, which is enough to recognise
	// what the note is about. Each row also pushes the note field down by its
	// height, 21.6pt, toward the soft keyboard: the phone sheet is sized to
	// the canvas before the keyboard rises and is not resized for it. In
	// portrait the field stays clear of the keyboard with three rows, down to
	// the 568pt-tall canvas of the smallest iPhone.
	noteExcerptMaxLines = 3

	// noteExcerptShortMaxLines is the budget on a phone in landscape, where
	// the keyboard, about 200pt with its suggestion bar, covers half the
	// canvas. The title and the reference take the first 70pt of the sheet.
	// On a 393pt-tall canvas, with one excerpt row the field starts about
	// 107pt down and shows three lines of the note above the keyboard; with
	// three rows it starts about 150pt down and shows one.
	noteExcerptShortMaxLines = 1

	// noteExcerptShortCanvas is the canvas height below which a phone sheet
	// takes the short budget. Phones in landscape are 320 to 440pt tall, in
	// portrait 568pt and up, and tablets are taller either way. It is read
	// from the canvas, which the keyboard does not shrink (the keyboard is an
	// inset), and not from the interactive area, which it does: a composer
	// reopened after a light/dark change while the keyboard is still going
	// down gets the budget it opened with.
	noteExcerptShortCanvas = 480

	// noteExcerptTextSize is the excerpt's reference size, a step above the
	// 13pt reference line over it and well under the 18pt the note is typed
	// at. It is set through readingGlyphSize, like every other size the
	// reading face is drawn at, so it looks the size the number says.
	noteExcerptTextSize float32 = 14

	// noteExcerptGuessWidth is the width wrapped for before anything has
	// handed one over: roughly a phone's form, so an unsized excerpt reports
	// a plausible height rather than one word per row.
	noteExcerptGuessWidth float32 = 300
)

// noteExcerptEllipsis ends the last row of a selection that runs on.
const noteExcerptEllipsis = "…"

// noteExcerptMaxLinesFor is the line budget for a sheet on this canvas: the
// short budget on a phone in landscape, three everywhere else. Desktops keep
// three at any window height, since no soft keyboard covers their sheet.
func noteExcerptMaxLinesFor(mobile bool, canvasHeight float32) int {
	if mobile && canvasHeight > 0 && canvasHeight < noteExcerptShortCanvas {
		return noteExcerptShortMaxLines
	}
	return noteExcerptMaxLines
}

// noteExcerpt draws the selected words, wrapped, at most maxLines rows.
type noteExcerpt struct {
	widget.BaseWidget
	text     string
	maxLines int
	size     float32
	color    color.Color
	face     fyne.Resource
}

// newNoteExcerpt is the excerpt for text in col, at most maxLines rows, in the
// reading face at the excerpt's size.
func newNoteExcerpt(text string, maxLines int, col color.Color) *noteExcerpt {
	if maxLines < 1 {
		maxLines = 1
	}
	e := &noteExcerpt{
		text:     strings.Join(strings.Fields(text), " "),
		maxLines: maxLines,
		size:     readingGlyphSize(noteExcerptTextSize),
		color:    col,
		face:     styledPaneFont(),
	}
	e.ExtendBaseWidget(e)
	return e
}

// setMaxLines gives the excerpt a new row budget, re-wrapping it when the
// budget changes: the phone sheet's refit, when the canvas it is on changes
// size (share_note_ui.go).
func (e *noteExcerpt) setMaxLines(n int) {
	if n < 1 {
		n = 1
	}
	if n == e.maxLines {
		return
	}
	e.maxLines = n
	e.Refresh()
}

func (e *noteExcerpt) CreateRenderer() fyne.WidgetRenderer {
	return &noteExcerptRenderer{e: e}
}

// noteExcerptRenderer re-wraps only when the width changes. MinSize reports
// the rows at the LAST width wrapped for (see the file comment).
type noteExcerptRenderer struct {
	e     *noteExcerpt
	width float32
	rowH  float32
	lines []string
	rows  []*canvas.Text
	objs  []fyne.CanvasObject
}

func (r *noteExcerptRenderer) measure(s string) fyne.Size {
	sz, _ := fyne.CurrentApp().Driver().RenderedTextSize(s, r.e.size, fyne.TextStyle{}, r.e.face)
	return sz
}

func (r *noteExcerptRenderer) wrap(width float32) {
	if width <= 0 {
		width = noteExcerptGuessWidth
	}
	if width == r.width && r.rows != nil {
		return
	}
	r.width = width
	if r.rowH == 0 {
		r.rowH = r.measure("Ag").Height
	}
	r.lines = noteExcerptLines(r.e.text, width, r.e.maxLines, func(s string) float32 {
		return r.measure(s).Width
	})
	r.rows = r.rows[:0]
	r.objs = r.objs[:0]
	for _, line := range r.lines {
		t := canvas.NewText(line, r.e.color)
		t.TextSize = r.e.size
		t.FontSource = r.e.face
		t.Resize(fyne.NewSize(r.measure(line).Width, r.rowH))
		r.rows = append(r.rows, t)
		r.objs = append(r.objs, t)
	}
}

func (r *noteExcerptRenderer) Layout(size fyne.Size) {
	r.wrap(size.Width)
	y := float32(0)
	for _, t := range r.rows {
		t.Move(fyne.NewPos(0, y))
		y += r.rowH
	}
}

func (r *noteExcerptRenderer) MinSize() fyne.Size {
	if r.rows == nil {
		r.wrap(r.width)
	}
	return fyne.NewSize(0, float32(len(r.rows))*r.rowH)
}

func (r *noteExcerptRenderer) Refresh() {
	w := r.width
	r.rows = nil
	r.wrap(w)
	r.Layout(r.e.Size())
	canvas.Refresh(r.e)
}

func (r *noteExcerptRenderer) Objects() []fyne.CanvasObject { return r.objs }
func (r *noteExcerptRenderer) Destroy()                     {}

// drawn is what the rows say, one string per row, for a test to read.
func (r *noteExcerptRenderer) drawn() []string { return append([]string(nil), r.lines...) }

// rowWidths reports each drawn row's width, for a test to hold against the
// width the excerpt was given.
func (r *noteExcerptRenderer) rowWidths() []float32 {
	out := make([]float32, 0, len(r.rows))
	for _, t := range r.rows {
		out = append(out, r.measure(t.Text).Width)
	}
	return out
}

// noteExcerptLines breaks text into at most max rows no wider than width, as
// measure reports widths, and ends the last row with an ellipsis when text
// runs on past them. Words are kept whole where they fit a row; one that is
// wider than a whole row is broken between letters. It stops measuring once
// the rows are full, so a selection of a whole chapter costs what three rows
// cost.
func noteExcerptLines(text string, width float32, max int, measure func(string) float32) []string {
	if max < 1 {
		max = 1
	}
	words := strings.Fields(text)
	var lines []string
	cur := ""
	cut := false
fill:
	for i := 0; i < len(words); i++ {
		w := words[i]
		if cur != "" {
			if next := cur + " " + w; measure(next) <= width {
				cur = next
				continue
			}
			lines = append(lines, cur)
			cur = ""
			if len(lines) == max {
				cut = true
				break
			}
		}
		// A row of its own for w, broken between letters while it is wider
		// than a row.
		for measure(w) > width {
			head, tail := noteExcerptFit(w, width, measure)
			lines = append(lines, head)
			w = tail
			if len(lines) == max {
				cut = w != "" || i+1 < len(words)
				break fill
			}
		}
		cur = w
	}
	if !cut && cur != "" {
		lines = append(lines, cur)
	}
	if cut && len(lines) > 0 {
		last := len(lines) - 1
		lines[last] = noteExcerptEllipsize(lines[last], width, measure)
	}
	return lines
}

// noteExcerptFit splits a word wider than width into the longest head of whole
// letters that fits and the rest; at least one letter goes in the head, so a
// row narrower than a letter still makes progress.
func noteExcerptFit(w string, width float32, measure func(string) float32) (head, tail string) {
	rs := []rune(w)
	lo, hi := 1, len(rs)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if measure(string(rs[:mid])) <= width {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return string(rs[:lo]), string(rs[lo:])
}

// noteExcerptEllipsize ends line with the ellipsis, taking words off its end,
// and then letters when one word is left, until the two fit width together.
// Punctuation and space the cut leaves at the end go too, so the row reads
// "…gave his only Son…", not "…gave his only Son,…".
func noteExcerptEllipsize(line string, width float32, measure func(string) float32) string {
	trim := func(s string) string {
		return strings.TrimRightFunc(s, func(r rune) bool {
			return unicode.IsSpace(r) || unicode.IsPunct(r) && r != ')' && r != ']' && r != '’' && r != '”'
		})
	}
	s := trim(line)
	for s != "" && measure(s+noteExcerptEllipsis) > width {
		if i := strings.LastIndexByte(s, ' '); i > 0 {
			s = trim(s[:i])
			continue
		}
		rs := []rune(s)
		s = trim(string(rs[:len(rs)-1]))
	}
	return s + noteExcerptEllipsis
}
