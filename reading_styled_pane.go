package bibletext

// The styled, selectable Windows/Linux reading pane. A pure-Go fyne widget that
// draws layoutChapter's styled runs as positioned canvas.Text — red letters,
// small raised verse numbers, the verse-highlight wash — everything the
// single-style widget.Entry pane cannot do, with no native embedding and no new
// dependencies. Selection lives in reading_styled_select.go.
//
// THE WASH IS PER LINE AND X-BOUNDED, exactly like a selection rect: one
// rectangle per contiguous same-tint stretch within a line (tintSpansForLayout).
// It used to be ONE full-column rectangle over a LINE RANGE, which lit every
// neighbouring verse sharing the range's first or last line — 26 of John 3's 63
// lines at 560pt carry more than one verse. Invisible with a single highlight;
// fatal the moment two adjacent verses carry different tints.
//
// THIS SHIPS: readingScrollArea dispatches here whenever useStyledPane() is
// true, which is Windows and Linux. It is untagged so it also builds and
// unit-tests on the the development environment, which is why the file carries no //go:build
// line — not because it is unreferenced.
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
	Tint verseTint

	X float32 // relative to the line's left edge (pane adds the inset)

	// FirstOffset is the flat-model rune offset of the segment's first rune
	// (the selection layer maps drawn positions back to the model with it).
	FirstOffset int
	Line        int
}

// mergeDrawRuns collapses a line's token runs into style segments. Adjacent
// runs merge when kind, red-letter, and tint all match; the model text
// between two adjacent runs on one line is always a single space.
func mergeDrawRuns(lineIdx int, ln styledLine) []styledDrawRun {
	var out []styledDrawRun
	for _, r := range ln.Runs {
		if n := len(out); n > 0 {
			prev := &out[n-1]
			if prev.Kind == r.Kind && prev.Red == r.RedLetter && prev.Tint == r.Tint {
				prev.Text += " " + r.Text
				continue
			}
		}
		out = append(out, styledDrawRun{
			Text: r.Text, Kind: r.Kind, Red: r.RedLetter, Tint: r.Tint,
			X: r.X, FirstOffset: r.Offset, Line: lineIdx,
		})
	}
	return out
}

// styledReadingPane renders a chapter with styled runs. It lives inside the
// existing readingColumn/VScroll exactly where chapterText does today.
type styledReadingPane struct {
	// extraInset centres the reporter measure: the dynamic left/right margin
	// ADDED to styledPaneInset while the column is narrower than the pane.
	// Draw, selection and hit-testing all read it through insetX(), so glyphs
	// and hit-tests cannot disagree about where the column sits.
	extraInset float32
	// lh is the laid-out baseline distance (reporter 1.3 vs cozy 1.55) —
	// stored so the renderer centres glyphs in the SAME line height the
	// layout used.
	lh float32

	widget.BaseWidget

	state  *AppState
	verses []Verse

	textSize float32
	pal      palette
	font     fyne.Resource // the scripture serif (Georgia, or embedded Gelasio)

	lay       *chapterLayout
	drawRuns  []styledDrawRun
	lineSegs  [][]styledDrawRun // drawRuns indexed by line, for hit-testing
	tintSpans []tintSpan        // the wash rects, per line, X-bounded to the tinted runs
	raSpans   []tintSpan        // the narration wash, same shape, its own layer above
	lastWidth float32

	// Selection state (milestone 3): rune offsets into lay.Text, -1 = none.
	selAnchor, selStart, selEnd int

	// raVerse is the verse the narration is on (read-along tint), 0 = none.
	// Driven by reading_styled_readalong.go on the UI goroutine.
	raVerse int

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
// family iOS renders through its HTML stack (font-family: Georgia, …) — the
// first serif loadBookFonts finds (real Georgia on Windows/macOS; on Linux
// typically DejaVu Serif, the usual distro serif), else the embedded Gelasio,
// Georgia's metrics-compatible OFL equivalent that the share cards already
// carry. Never nil, so drawing and measuring always use the same face.
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
	paraGap := lh * 0.65
	indent := float32(0)
	p.extraInset = 0

	// desktop reads like the iPad). Same U.S. Reports set the iPad uses —
	// centred 27.5em measure, 1.3 leading, first-line indents with no
	// paragraph gap — gated purely on width, so a narrow window keeps
	// today's cozy narrow-pane layout and a resize glides between the two.
	// The em is the pane's own body size, exactly as the iPad's measure is
	// 27.5 × ITS body px.
	if m := reporterMeasureEm * p.textSize; avail > m {
		p.extraInset = float32(int((avail - m) / 2)) // whole px: keep glyphs crisp
		avail = m
		lh = p.textSize * 1.3
		paraGap = 0
		indent = 1.5 * p.textSize
	}
	p.lh = lh
	p.lay = layoutChapter(p.state, p.verses, styledLayoutParams{
		Width:      avail,
		LineHeight: lh,
		ParaGap:    paraGap,
		SpaceW:     p.measure(" ", runWord),
		Indent:     indent,
	}, p.measure)
	p.drawRuns = p.drawRuns[:0]
	p.lineSegs = make([][]styledDrawRun, len(p.lay.Lines))
	for li, ln := range p.lay.Lines {
		segs := mergeDrawRuns(li, ln)
		p.lineSegs[li] = segs
		p.drawRuns = append(p.drawRuns, segs...)
	}
	p.tintSpans = tintSpansForLayout(p.lay)
	p.raSpans = verseSpansForLayout(p.lay, p.raVerse)
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

// highlightFirstLine is the first line index carrying the search/cross-ref
// highlight, -1 when nothing is highlighted. tintSpans is in line order, so the
// first matching span is the first line.
func (p *styledReadingPane) highlightFirstLine() int {
	for _, sp := range p.tintSpans {
		if sp.Tint == tintHighlight {
			return sp.Line
		}
	}
	return -1
}

// highlightY is the Y of the highlighted verse's first line (scroll-to target).
func (p *styledReadingPane) highlightY() float32 {
	if p.lay == nil {
		return 0
	}
	li := p.highlightFirstLine()
	if li < 0 || li >= len(p.lay.Lines) {
		return 0
	}
	return p.lay.Lines[li].Y
}

// styledPaneRenderer draws the merged runs plus the wash rects.
type styledPaneRenderer struct {
	pane *styledReadingPane

	// tintRects is one rectangle per tintSpan — per line, bounded to the
	// tinted runs' own X range, never the full column.
	tintRects []*canvas.Rectangle
	raRects   []*canvas.Rectangle
	selRects  []*canvas.Rectangle
	texts     []*canvas.Text
	objects   []fyne.CanvasObject
}

// rebuild recreates the canvas objects from the pane's current draw runs.
func (r *styledPaneRenderer) rebuild() {
	p := r.pane
	r.texts = r.texts[:0]
	r.objects = r.objects[:0]

	// The verse wash sits BEHIND the text, like the shipping pane's; the
	// read-along tint sits over it (the narration wash wins where the two
	// overlap, as on the native overlays); selection rects sit between the
	// washes and the glyphs.
	r.tintRects = r.tintRects[:0]
	for _, sp := range p.tintSpans {
		// Colour HERE, not in position(). p.tintSpans can only change inside
		// relayout, and the only production relayout caller Refreshes straight
		// after, so a rect's wash cannot change between rebuilds. Setting
		// FillColor per layout pass boxed a 4-byte NRGBA into color.Color on
		// every frame — measured at 514 allocations for a whole-chapter span,
		// on a pane that also ships an llvmpipe (software) path.
		rect := canvas.NewRectangle(r.tintColor(sp.Tint)) // geometry in position()
		r.tintRects = append(r.tintRects, rect)
		r.objects = append(r.objects, rect)
	}
	// The narration wash, on its own layer ABOVE the verse wash: the two are
	// meant to be seen together (styledReadAlongTint is translucent), so this
	// cannot be folded into the run's single Tint value. Same per-line,
	// X-bounded shape though — a full-column band here washed whatever verse
	// shared the narrated verse's first and last lines, and won over the
	// corrected highlight wherever they met.
	r.raRects = r.raRects[:0]
	for range p.raSpans {
		rect := canvas.NewRectangle(styledReadAlongTint)
		r.raRects = append(r.raRects, rect)
		r.objects = append(r.objects, rect)
	}

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
	case dr.Tint.overridesTextColour() && dr.Red && dr.Kind == runWord:
		return p.pal.RedLetter // the wash highlights it; the red still means something
	case dr.Tint.overridesTextColour():
		return p.pal.Text
	case dr.Red && dr.Kind == runWord:
		return p.pal.RedLetter
	case dr.Kind == runVerseNum:
		return p.pal.VerseNumber
	default:
		return p.pal.Text
	}
}

// tintColor is the wash a tint paints in, from the shared table (verseTint.wash
// in tint.go) rather than from a switch of its own.
//
// It had its own switch over pal.Highlight until the Android dialect and the
// Apple stylesheet came onto the same tint model and turned out to be reaching
// for the same field independently — three surfaces, three chances for a new
// tint to land in two of them. tintNone paints color.NRGBA{}: fully transparent,
// which is what an untinted stretch was always drawn as here.
func (r *styledPaneRenderer) tintColor(t verseTint) color.Color {
	c, _ := t.wash(r.pane.pal)
	return c
}

// position places every object from the layout geometry.
func (r *styledPaneRenderer) position() {
	p := r.pane
	if p.lay == nil {
		return
	}

	// One wash rect per tinted stretch: the tinted runs' own X range on ONE
	// line, so a verse sharing a line with an untinted (or differently tinted)
	// neighbour never washes it.
	for i, sp := range p.tintSpans {
		if i >= len(r.tintRects) || sp.Line >= len(p.lay.Lines) {
			break
		}
		ln := p.lay.Lines[sp.Line]
		rect := r.tintRects[i]
		rect.Move(fyne.NewPos(p.insetX()+sp.LineX0, ln.Y))
		rect.Resize(fyne.NewSize(sp.LineX1-sp.LineX0, ln.H))
		rect.Show()
	}
	// Anything left over is HIDDEN, never merely skipped. The single band this
	// replaced was immune by construction — one rect, with an explicit Hide on
	// the else — and dropping out of the loop quietly removed that safety net:
	// a relayout that shrinks the span list without a rebuild left the surplus
	// rectangles painted at their old geometry. Measured as 11 ghost washes
	// after clearing a mark. Unreachable through today's one production caller,
	// which Refreshes immediately; restored anyway, because "unreachable" here
	// is a property of one call site rather than of the code.
	for i := len(p.tintSpans); i < len(r.tintRects); i++ {
		r.tintRects[i].Hide()
	}

	// The narration wash, bounded the same way — and through insetX() like
	// everything else. It used to position at extraInset, twelve points
	// (styledPaneInset) left of the column every other object starts at.
	for i, sp := range p.raSpans {
		if i >= len(r.raRects) || sp.Line >= len(p.lay.Lines) {
			break
		}
		ln := p.lay.Lines[sp.Line]
		rect := r.raRects[i]
		rect.Move(fyne.NewPos(p.insetX()+sp.LineX0, ln.Y))
		rect.Resize(fyne.NewSize(sp.LineX1-sp.LineX0, ln.H))
		rect.Show()
	}
	for i := len(p.raSpans); i < len(r.raRects); i++ {
		r.raRects[i].Hide()
	}

	lh := p.lh
	if lh == 0 {
		lh = p.styledLineHeight()
	}
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
		r.texts[i].Move(fyne.NewPos(p.insetX()+dr.X, y))
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

// --- Scroll-anchor geometry (the pane's exact-line answers to chapterText's
// proportional model) ---------------------------------------------------------

// verseAtY maps a content Y to (top-visible verse, px past its first line) —
// the capture half of the within-chapter scroll anchor.
func (p *styledReadingPane) verseAtY(y float32) (verse int, delta float64) {
	if p.lay == nil || len(p.lay.VerseLines) == 0 {
		return 0, 0
	}
	best := p.lay.VerseLines[0]
	for _, vl := range p.lay.VerseLines {
		if p.lay.Lines[vl.line].Y <= y {
			best = vl
		} else {
			break
		}
	}
	return best.verse, float64(y - p.lay.Lines[best.line].Y)
}

// yForVerse maps a verse number to its first line's exact Y — the restore half.
func (p *styledReadingPane) yForVerse(verse int) (float32, bool) {
	if p.lay == nil {
		return 0, false
	}
	for _, vl := range p.lay.VerseLines {
		if vl.verse == verse {
			return p.lay.Lines[vl.line].Y, true
		}
	}
	return 0, false
}

// highlightOwnsScroll reports whether a search/cross-ref highlight should own
// the scroll position (mirrors chapterText.highlightLine >= 0).
func (p *styledReadingPane) highlightOwnsScroll() bool {
	return p.lay != nil && p.highlightFirstLine() >= 0
}

// setReadAlongVerse moves the narration wash, keeping raVerse and raSpans in
// lockstep.
//
// ONE WRITER, on purpose. The wash used to be a single always-present rectangle
// whose geometry position() derived from raVerse on the spot, so it could not go
// stale. Per-line rects are sized in rebuild() from raSpans, and Refresh() calls
// rebuild() WITHOUT a relayout — so a raVerse written on its own would have left
// the narration lighting the previous verse until the next resize. Two fields
// standing for one fact is the shape that produced this subsystem's worst
// defects; here they are assigned together or not at all.
func (p *styledReadingPane) setReadAlongVerse(verse int) {
	if p == nil || p.raVerse == verse {
		return
	}
	p.raVerse = verse
	p.raSpans = verseSpansForLayout(p.lay, verse)
	p.Refresh()
}

// verseLineSpan returns the [first,last] line indexes the verse's runs touch —
// exact even when a verse shares its first or last wrapped line with a
// neighbour (continuous prose). Drives the read-along tint band.
func (p *styledReadingPane) verseLineSpan(verse int) (first, last int, ok bool) {
	if verse <= 0 || p.lay == nil {
		return 0, 0, false
	}
	first = -1
	for i, ln := range p.lay.Lines {
		for _, run := range ln.Runs {
			if run.Verse == verse {
				if first < 0 {
					first = i
				}
				last = i
				break
			}
		}
		if first >= 0 && last < i {
			break // past the verse's contiguous lines — done
		}
	}
	if first < 0 {
		return 0, 0, false
	}
	return first, last, true
}

// insetX is the pane's live left inset: the fixed padding plus whatever margin
// centres the reporter measure. Every consumer of run X coordinates — drawing,
// selection, hit-testing — must use THIS, never styledPaneInset directly, or
// clicks land beside the glyphs whenever the column is centred.
func (p *styledReadingPane) insetX() float32 { return styledPaneInset + p.extraInset }
