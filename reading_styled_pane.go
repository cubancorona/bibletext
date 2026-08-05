package bibletext

// The styled, selectable desktop reading pane (Windows/Linux parity project,
// milestone 2: rendering). A pure-Go fyne widget that draws layoutChapter's
// styled runs as positioned canvas.Text — red letters, small raised verse
// numbers, the verse-highlight band — everything the single-style
// widget.Entry pane cannot do, with no native embedding and no new
// dependencies. Selection arrives in milestone 3; the desktop dispatch swap
// (and any change visible to other platforms) is milestone 4.
//
// Untagged so the widget builds and unit-tests on the the development environment; it is not
// yet referenced by any platform's UI.
//
// RENDERING MODEL. layoutChapter keeps TOKEN-level runs (for wrap math and,
// later, selection hit-testing); drawing merges adjacent same-style runs into
// one canvas.Text per style segment, anchored at the segment's first token's
// X. That keeps canvas object counts low (roughly one or two objects per
// line instead of one per word — Psalm 119 stays in the hundreds, not
// thousands) and makes the DRAWN text the geometry source of truth for the
// selection layer: within a segment, positions come from measuring substring
// prefixes of exactly the string being drawn, so kerning can never drift
// between hit-testing and pixels.

import (
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// styledPaneInset is the horizontal padding inside the pane. Vertical spacing
// comes from the layout's own line geometry.
const styledPaneInset = float32(12)

// styledDrawRun is one merged, drawable style segment of a line.
type styledDrawRun struct {
	Text string
	Kind runKind
	Red  bool
	HL   bool

	X float32 // relative to the line's left edge (pane adds the inset)

	// FirstOffset is the flat-model rune offset of the segment's first rune
	// (the selection layer maps drawn positions back to the model with it).
	FirstOffset int
	Line        int
}

// mergeDrawRuns collapses a line's token runs into style segments. Adjacent
// runs merge when kind, red-letter, and highlight all match; the model text
// between two adjacent runs on one line is always a single space.
func mergeDrawRuns(lineIdx int, ln styledLine) []styledDrawRun {
	var out []styledDrawRun
	for _, r := range ln.Runs {
		if n := len(out); n > 0 {
			prev := &out[n-1]
			if prev.Kind == r.Kind && prev.Red == r.RedLetter && prev.HL == r.Highlight {
				prev.Text += " " + r.Text
				continue
			}
		}
		out = append(out, styledDrawRun{
			Text: r.Text, Kind: r.Kind, Red: r.RedLetter, HL: r.Highlight,
			X: r.X, FirstOffset: r.Offset, Line: lineIdx,
		})
	}
	return out
}

// styledReadingPane renders a chapter with styled runs. It lives inside the
// existing readingColumn/VScroll exactly where chapterText does today.
type styledReadingPane struct {
	widget.BaseWidget

	state  *AppState
	verses []Verse

	textSize float32
	pal      palette
	font     fyne.Resource // the scripture serif (Georgia, or embedded Gelasio)

	lay       *chapterLayout
	drawRuns  []styledDrawRun
	lineSegs  [][]styledDrawRun // drawRuns indexed by line, for hit-testing
	lastWidth float32

	// Selection state (milestone 3): rune offsets into lay.Text, -1 = none.
	selAnchor, selStart, selEnd int

	clipboard fyne.Clipboard
}

func newStyledReadingPane(state *AppState, verses []Verse) *styledReadingPane {
	p := &styledReadingPane{
		state:     state,
		verses:    verses,
		textSize:  styledPaneTextSize(),
		pal:       state.pal(),
		font:      styledPaneFont(),
		selAnchor: -1, selStart: -1, selEnd: -1,
	}
	if state.window != nil {
		p.clipboard = state.window.Clipboard()
	}
	p.ExtendBaseWidget(p)
	p.relayout(720) // provisional; corrected when the real width arrives
	return p
}

// styledPaneTextSize matches chapterText's sizing: the theme body size scaled
// by the reader's Settings → Text size choice, with a sane default for bare
// test constructions (no running app).
func styledPaneTextSize() float32 {
	size := float32(15)
	if app := fyne.CurrentApp(); app != nil {
		size = fyneTheme.TextSize()
	}
	return size * float32(readingTextScale())
}

// styledPaneFont resolves the scripture face ONCE per process: the same
// family iOS renders through its HTML stack (font-family: Georgia, …) — real
// Georgia when the OS has it (macOS and Windows both ship it), else the
// embedded Gelasio, Georgia's metrics-compatible OFL equivalent that the
// share cards already carry. Never nil, so drawing and measuring always use
// the same face.
func styledPaneFont() fyne.Resource {
	styledFontOnce.Do(func() {
		if fonts := loadBookFonts(); fonts != nil && fonts.regular != nil {
			styledFontCached = fonts.regular
			return
		}
		styledFontCached = fyne.NewStaticResource("Gelasio-Regular.ttf", shareFontGelasio)
	})
	return styledFontCached
}

var (
	styledFontOnce   sync.Once
	styledFontCached fyne.Resource
)

// styledLineHeight is the baseline-to-baseline distance for the pane's body
// text: comfortable book leading, matching the reading feel of the shipping
// pane rather than a dense terminal.
func (p *styledReadingPane) styledLineHeight() float32 { return p.textSize * 1.55 }

func (p *styledReadingPane) relayout(width float32) {
	avail := width - 2*styledPaneInset
	if avail < 80 {
		avail = 80
	}
	lh := p.styledLineHeight()
	p.lay = layoutChapter(p.state, p.verses, styledLayoutParams{
		Width:      avail,
		LineHeight: lh,
		ParaGap:    lh * 0.65,
		SpaceW:     p.measure(" ", runWord),
	}, p.measure)
	p.drawRuns = p.drawRuns[:0]
	p.lineSegs = make([][]styledDrawRun, len(p.lay.Lines))
	for li, ln := range p.lay.Lines {
		segs := mergeDrawRuns(li, ln)
		p.lineSegs[li] = segs
		p.drawRuns = append(p.drawRuns, segs...)
	}
	p.lastWidth = width
	// Offsets shifted with the new layout — any selection is now meaningless.
	p.selAnchor, p.selStart, p.selEnd = -1, -1, -1
}

// measure is the production ruler: body text at size, verse numbers at the
// same 0.66 ratio the native overlays use — measured with the SAME serif
// source the renderer draws with (RenderedTextSize honours FontSource, which
// fyne.MeasureText cannot), so wrap geometry, hit-testing and glyphs can
// never drift apart.
func (p *styledReadingPane) measure(text string, kind runKind) float32 {
	size := p.textSize
	if kind == runVerseNum {
		size *= styledNumRatio
	}
	w, _ := fyne.CurrentApp().Driver().RenderedTextSize(text, size, fyne.TextStyle{}, p.font)
	return w.Width
}

const styledNumRatio = float32(0.66)

func (p *styledReadingPane) CreateRenderer() fyne.WidgetRenderer {
	r := &styledPaneRenderer{pane: p}
	r.rebuild()
	return r
}

// Resize re-lays-out at the new width before the renderer positions objects —
// the same responsive contract as chapterText.Resize.
func (p *styledReadingPane) Resize(size fyne.Size) {
	if size.Width > 1 && size.Width != p.lastWidth {
		p.relayout(size.Width)
		p.Refresh() // draw runs changed — the renderer must rebuild, not just reposition
	}
	p.BaseWidget.Resize(size)
}

func (p *styledReadingPane) MinSize() fyne.Size {
	h := float32(0)
	if p.lay != nil {
		h = p.lay.Height + p.styledLineHeight() // breathing room below the last line
	}
	return fyne.NewSize(200, h)
}

// highlightY is the Y of the highlighted verse band's top (scroll-to target).
func (p *styledReadingPane) highlightY() float32 {
	if p.lay == nil || p.lay.HighlightStart < 0 || p.lay.HighlightStart >= len(p.lay.Lines) {
		return 0
	}
	return p.lay.Lines[p.lay.HighlightStart].Y
}

// styledPaneRenderer draws the merged runs plus the highlight band.
type styledPaneRenderer struct {
	pane *styledReadingPane

	band     *canvas.Rectangle
	selRects []*canvas.Rectangle
	texts    []*canvas.Text
	objects  []fyne.CanvasObject
}

// rebuild recreates the canvas objects from the pane's current draw runs.
func (r *styledPaneRenderer) rebuild() {
	p := r.pane
	r.texts = r.texts[:0]
	r.objects = r.objects[:0]

	// The verse-highlight band sits BEHIND the text, like the shipping pane's;
	// selection rects sit between the band and the glyphs.
	r.band = canvas.NewRectangle(color.NRGBA{}) // transparent until positioned
	r.objects = append(r.objects, r.band)

	selColor := p.pal.Accent
	selColor.A = 70
	r.selRects = r.selRects[:0]
	for _, span := range p.selectionSpans() {
		ln := p.lay.Lines[span.Line]
		rect := canvas.NewRectangle(selColor)
		rect.Move(fyne.NewPos(span.X0, ln.Y))
		rect.Resize(fyne.NewSize(span.X1-span.X0, ln.H))
		r.selRects = append(r.selRects, rect)
		r.objects = append(r.objects, rect)
	}

	for _, dr := range p.drawRuns {
		t := canvas.NewText(dr.Text, r.runColor(dr))
		t.FontSource = p.font // the scripture serif, same face iOS shows
		t.TextSize = p.textSize
		if dr.Kind == runVerseNum {
			// The serif at the small superscript size (iOS renders numbers in
			// the same family); no Bold — the source is a single face and a
			// synthetic-bold fallback would leave the serif for a sans.
			t.TextSize = p.textSize * styledNumRatio
		}
		r.texts = append(r.texts, t)
		r.objects = append(r.objects, t)
	}
	r.position()
}

func (r *styledPaneRenderer) runColor(dr styledDrawRun) color.Color {
	p := r.pane
	switch {
	case dr.HL:
		return p.pal.HighlightText
	case dr.Red && dr.Kind == runWord:
		return p.pal.RedLetter
	case dr.Kind == runVerseNum:
		return p.pal.VerseNumber
	default:
		return p.pal.Text
	}
}

// position places every object from the layout geometry.
func (r *styledPaneRenderer) position() {
	p := r.pane
	if p.lay == nil {
		return
	}

	// Highlight band across the highlighted line span.
	if p.lay.HighlightStart >= 0 && p.lay.HighlightStart < len(p.lay.Lines) {
		top := p.lay.Lines[p.lay.HighlightStart].Y
		bot := p.lay.Lines[p.lay.HighlightEnd].Y + p.lay.Lines[p.lay.HighlightEnd].H
		// pal.Highlight IS the faint wash colour; the tagged reading_fyne
		// helper is unavailable to untagged code, so use it directly.
		r.band.FillColor = p.pal.Highlight
		r.band.Move(fyne.NewPos(0, top))
		r.band.Resize(fyne.NewSize(p.lastWidth, bot-top))
		r.band.Show()
	} else {
		r.band.Hide()
	}

	lh := p.styledLineHeight()
	drv := fyne.CurrentApp().Driver()
	bodyS, _ := drv.RenderedTextSize("Ag", p.textSize, fyne.TextStyle{}, p.font)
	numS, _ := drv.RenderedTextSize("1", p.textSize*styledNumRatio, fyne.TextStyle{}, p.font)
	bodyH, numH := bodyS.Height, numS.Height
	for i, dr := range p.drawRuns {
		ln := p.lay.Lines[dr.Line]
		y := ln.Y + (lh-bodyH)/2
		if dr.Kind == runVerseNum {
			// Raised superscript: top-align the small label against the
			// body's ascent region.
			y = ln.Y + (lh-bodyH)/2 - numH*0.18
		}
		r.texts[i].Move(fyne.NewPos(styledPaneInset+dr.X, y))
	}
}

func (r *styledPaneRenderer) Layout(size fyne.Size) {
	// Width changes arrive via the widget's Resize (which re-lays-out); the
	// renderer only needs repositioning here.
	r.position()
}

func (r *styledPaneRenderer) MinSize() fyne.Size           { return r.pane.MinSize() }
func (r *styledPaneRenderer) Refresh()                     { r.rebuild(); canvas.Refresh(r.pane) }
func (r *styledPaneRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *styledPaneRenderer) Destroy()                     {}
