package bibletext

// The SELECTION layer of the styled reading pane. Drag, double-click, and
// keyboard selection over the drawn segments; copy with the same authored-break
// fidelity as chapterText.copySelection (poem lines and paragraph blanks
// survive, width wraps flatten to spaces); and the right-click study menu with
// the exact verbs the shipping pane serves.
//
// This ships on Windows and Linux with the pane it belongs to. The study menu is
// no longer a duplicate: studyMenu calls the SHARED selectionStudyMenu that
// chapterText also uses, so a verb added there appears in both without being
// added twice.
//
// GEOMETRY. Hit-testing works against the DRAWN segments (styledDrawRun), not
// the layout tokens: within a segment, positions come from measuring rune
// prefixes of exactly the string being drawn, so pixels and hit-tests can
// never disagree. Offsets are rune indexes into the flat selection text
// model (chapterLayout.Text).

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// --- Geometry: position ⇄ model offset --------------------------------------

// lineAtY finds the line whose vertical band contains y (clamping into the
// nearest line for clicks in paragraph gaps or beyond the ends).
func (p *styledReadingPane) lineAtY(y float32) int {
	lines := p.lay.Lines
	if len(lines) == 0 {
		return -1
	}
	for i, ln := range lines {
		if y < ln.Y {
			// In the gap above this line: closer to the previous line's
			// bottom or this line's top?
			if i == 0 {
				return 0
			}
			prevBot := lines[i-1].Y + lines[i-1].H
			if y-prevBot < ln.Y-y {
				return i - 1
			}
			return i
		}
		if y < ln.Y+ln.H {
			return i
		}
	}
	return len(lines) - 1
}

// segTextSize returns the rendered size for a segment's kind.
func (p *styledReadingPane) segTextSize(kind runKind) float32 {
	if kind == runVerseNum {
		return p.textSize * styledNumRatio
	}
	return p.textSize
}

// segWidth measures segment text with the pane's serif source — the SAME
// ruler the renderer draws with, so hit-tests always agree with pixels.
func (p *styledReadingPane) segWidth(text string, kind runKind) float32 {
	w, _ := fyne.CurrentApp().Driver().RenderedTextSize(text, p.segTextSize(kind), fyne.TextStyle{}, p.font)
	return w.Width
}

// offsetAtPos maps a widget-relative position to a model rune offset.
func (p *styledReadingPane) offsetAtPos(pos fyne.Position) int {
	li := p.lineAtY(pos.Y)
	if li < 0 {
		return 0
	}
	ln := p.lay.Lines[li]
	x := pos.X - p.insetX()
	segs := p.lineSegs[li]
	if len(segs) == 0 || x <= segs[0].X {
		return ln.StartOffset
	}
	for _, seg := range segs {
		segRunes := []rune(seg.Text)
		w := p.segWidth(seg.Text, seg.Kind)
		if x < seg.X {
			return seg.FirstOffset
		}
		if x <= seg.X+w {
			// Within this segment: binary search the rune prefix whose width
			// brackets x.
			lo, hi := 0, len(segRunes)
			for lo < hi {
				mid := (lo + hi + 1) / 2
				pw := p.segWidth(string(segRunes[:mid]), seg.Kind)
				if seg.X+pw <= x {
					lo = mid
				} else {
					hi = mid - 1
				}
			}
			return seg.FirstOffset + lo
		}
	}
	return ln.EndOffset
}

// xForOffset maps a model offset within a line to its X (widget-relative).
func (p *styledReadingPane) xForOffset(li, offset int) float32 {
	segs := p.lineSegs[li]
	if len(segs) == 0 {
		return p.insetX()
	}
	for i, seg := range segs {
		segRunes := []rune(seg.Text)
		end := seg.FirstOffset + len(segRunes)
		if offset < seg.FirstOffset {
			if i == 0 {
				return p.insetX() + seg.X
			}
			// In the inter-segment space: snap to this segment's start.
			return p.insetX() + seg.X
		}
		if offset <= end {
			pw := p.segWidth(string(segRunes[:offset-seg.FirstOffset]), seg.Kind)
			return p.insetX() + seg.X + pw
		}
	}
	last := segs[len(segs)-1]
	w := p.segWidth(last.Text, last.Kind)
	return p.insetX() + last.X + w
}

// selectionSpan is one line's slice of the active selection, in widget coords.
type selectionSpan struct {
	Line   int
	X0, X1 float32
}

// selectionSpans returns the per-line rectangles of the active selection.
func (p *styledReadingPane) selectionSpans() []selectionSpan {
	if p.selStart < 0 || p.selEnd <= p.selStart || p.lay == nil {
		return nil
	}
	var spans []selectionSpan
	for li, ln := range p.lay.Lines {
		if ln.EndOffset <= p.selStart || ln.StartOffset >= p.selEnd {
			continue
		}
		s, e := ln.StartOffset, ln.EndOffset
		if p.selStart > s {
			s = p.selStart
		}
		if p.selEnd < e {
			e = p.selEnd
		}
		spans = append(spans, selectionSpan{
			Line: li,
			X0:   p.xForOffset(li, s),
			X1:   p.xForOffset(li, e),
		})
	}
	return spans
}

// selectedRaw is the selection exactly as the model holds it ("\n" and
// "\n\n" included) — what the share/AI pipeline normalizes downstream.
func (p *styledReadingPane) selectedRaw() string {
	if p.selStart < 0 || p.selEnd <= p.selStart {
		return ""
	}
	runes := []rune(p.lay.Text)
	if p.selEnd > len(runes) {
		return ""
	}
	return string(runes[p.selStart:p.selEnd])
}

// copySelected returns the clipboard form: authored poem breaks stay "\n",
// paragraph boundaries stay "\n\n", width wraps flatten to spaces — the same
// fidelity contract as chapterText.copySelection.
func (p *styledReadingPane) copySelected() string {
	if p.selStart < 0 || p.selEnd <= p.selStart {
		return ""
	}
	runes := []rune(p.lay.Text)
	var b strings.Builder
	for li, ln := range p.lay.Lines {
		if ln.EndOffset <= p.selStart || ln.StartOffset >= p.selEnd {
			continue
		}
		s, e := ln.StartOffset, ln.EndOffset
		if p.selStart > s {
			s = p.selStart
		}
		if p.selEnd < e {
			e = p.selEnd
		}
		b.WriteString(string(runes[s:e]))
		if p.selEnd > ln.EndOffset && li+1 < len(p.lay.Lines) {
			next := p.lay.Lines[li+1]
			switch {
			case next.ParaFirst:
				b.WriteString("\n\n")
			case ln.PoemBreakAfter:
				b.WriteString("\n")
			default:
				b.WriteString(" ")
			}
		}
	}
	return b.String()
}

func (p *styledReadingPane) setSelection(start, end int) {
	if start > end {
		start, end = end, start
	}
	if start == p.selStart && end == p.selEnd {
		return
	}
	p.selStart, p.selEnd = start, end
	p.Refresh()
}

func (p *styledReadingPane) clearSelection() {
	if p.selStart < 0 && p.selEnd < 0 {
		return
	}
	p.selStart, p.selEnd = -1, -1
	p.Refresh()
}

func (p *styledReadingPane) selectAll() {
	p.setSelection(0, len([]rune(p.lay.Text)))
}

// --- Events ------------------------------------------------------------------

var _ desktop.Mouseable = (*styledReadingPane)(nil)
var _ fyne.Draggable = (*styledReadingPane)(nil)
var _ fyne.Tappable = (*styledReadingPane)(nil)
var _ fyne.DoubleTappable = (*styledReadingPane)(nil)
var _ fyne.SecondaryTappable = (*styledReadingPane)(nil)
var _ fyne.Focusable = (*styledReadingPane)(nil)
var _ desktop.Cursorable = (*styledReadingPane)(nil)

// THE NOTE STICKER IS NOT TEXT. Four handlers ask one question first — is this
// position inside the sticker's card? — because the pane is the surface Fyne
// hands a MouseDown to (a widget.Button is Tappable, not desktop.Mouseable), so
// without the guard pressing the bubble would collapse the reader's selection
// and start a new one UNDER the card, and a drag begun there would select the
// verses the bubble is sitting over. The verbs themselves fire from the
// buttons' own callbacks; this only keeps the selection out of the way.
func (p *styledReadingPane) MouseDown(e *desktop.MouseEvent) {
	if e.Button != desktop.MouseButtonPrimary {
		return
	}
	if p.noteGeom.hits(e.Position) || p.hitsAnyPill(e.Position) {
		p.noteGrab = true
		return
	}
	// The footnote section is not text, and neither is the superscription:
	// without this, lineAtY would clamp a press in either onto the nearest
	// scripture line and start a selection there — and the latch, like
	// noteGrab, keeps the DRAG that follows such a press from doing the same
	// (reading_styled_footnotes.go, reading_styled_super.go).
	if p.fnGeom.hits(e.Position) || p.superGeom.hits(e.Position) {
		p.fnGrab = true
		return
	}
	p.focusSelf()
	p.selAnchor = p.offsetAtPos(e.Position)
	p.setSelection(p.selAnchor, p.selAnchor)
}

func (p *styledReadingPane) MouseUp(*desktop.MouseEvent) {}

func (p *styledReadingPane) Dragged(e *fyne.DragEvent) {
	if p.noteGrab || p.fnGrab {
		return
	}
	if p.selAnchor < 0 {
		p.selAnchor = p.offsetAtPos(e.Position)
	}
	p.setSelection(p.selAnchor, p.offsetAtPos(e.Position))
}

func (p *styledReadingPane) DragEnd() { p.noteGrab = false; p.fnGrab = false }

func (p *styledReadingPane) Tapped(e *fyne.PointEvent) {
	if p.noteGrab || (e != nil && (p.noteGeom.hits(e.Position) || p.hitsAnyPill(e.Position))) {
		p.noteGrab = false
		return // a click on the card must not clear what the reader selected
	}
	if p.fnGrab || (e != nil && (p.fnGeom.hits(e.Position) || p.superGeom.hits(e.Position))) {
		p.fnGrab = false
		return // the apparatus and the title are inert — no clear, no selection change
	}
	p.focusSelf()
	p.clearSelection()
}

// DoubleTapped selects the word under the pointer.
func (p *styledReadingPane) DoubleTapped(ev *fyne.PointEvent) {
	if ev != nil && (p.noteGeom.hits(ev.Position) || p.hitsAnyPill(ev.Position)) {
		return // no word-select under the bubble
	}
	if ev != nil && (p.fnGeom.hits(ev.Position) || p.superGeom.hits(ev.Position)) {
		return // no word-select in the apparatus or the title
	}
	off := p.offsetAtPos(ev.Position)
	runes := []rune(p.lay.Text)
	if len(runes) == 0 {
		return
	}
	if off >= len(runes) {
		off = len(runes) - 1
	}
	isWordRuneAt := func(i int) bool {
		r := runes[i]
		return r != ' ' && r != '\n'
	}
	if !isWordRuneAt(off) && off > 0 {
		off--
	}
	if !isWordRuneAt(off) {
		return
	}
	start, end := off, off+1
	for start > 0 && isWordRuneAt(start-1) {
		start--
	}
	for end < len(runes) && isWordRuneAt(end) {
		end++
	}
	p.focusSelf()
	p.selAnchor = start
	p.setSelection(start, end)
}

func (p *styledReadingPane) TappedSecondary(e *fyne.PointEvent) {
	if e != nil && (p.noteGeom.hits(e.Position) || p.hitsAnyPill(e.Position)) {
		return // no study menu over somebody else's words
	}
	if e != nil && (p.fnGeom.hits(e.Position) || p.superGeom.hits(e.Position)) {
		return // no study menu over the translators' words or the title
	}
	menu := p.studyMenu()
	c := fyne.CurrentApp().Driver().CanvasForObject(p)
	if c == nil {
		return
	}
	showSelectionMenuFitting(menu, c, e.AbsolutePosition)
}

func (p *styledReadingPane) Cursor() desktop.Cursor { return desktop.TextCursor }

func (p *styledReadingPane) focusSelf() {
	if c := fyne.CurrentApp().Driver().CanvasForObject(p); c != nil {
		c.Focus(p)
	}
}

func (p *styledReadingPane) FocusGained()   {}
func (p *styledReadingPane) FocusLost()     {}
func (p *styledReadingPane) TypedRune(rune) {}
func (p *styledReadingPane) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyEscape {
		p.clearSelection()
	}
}

func (p *styledReadingPane) TypedShortcut(sc fyne.Shortcut) {
	switch sc.(type) {
	case *fyne.ShortcutCopy:
		p.copyToClipboard()
	case *fyne.ShortcutSelectAll:
		p.selectAll()
	}
}

func (p *styledReadingPane) copyToClipboard() {
	if p.clipboard == nil || p.selStart < 0 || p.selEnd <= p.selStart {
		return
	}
	p.clipboard.SetContent(p.copySelected())
}

// --- The study menu ----------------------------------------------------------

// selectedVerseSpan resolves the selection's verse range POSITIONALLY: every
// laid-out run carries its verse (styledRun.Verse — verse numbers included, so
// a selection touching a number resolves to that verse), and a run overlaps the
// selection when its rune range [Offset, Offset+len) crosses [selStart,
// selEnd). Runs are in chapter order, so the first overlap is lo and the last
// is hi. A selection covering only separator whitespace overlaps no run and
// yields the zero span (the matching fallback applies).
func (p *styledReadingPane) selectedVerseSpan() selSpan {
	if p.selStart < 0 || p.selEnd <= p.selStart || p.lay == nil {
		return selSpan{}
	}
	lo, hi := 0, 0
	for _, ln := range p.lay.Lines {
		if ln.StartOffset >= p.selEnd {
			break
		}
		if ln.EndOffset <= p.selStart {
			continue
		}
		for _, r := range ln.Runs {
			if r.Offset >= p.selEnd {
				break
			}
			if r.Offset+len([]rune(r.Text)) <= p.selStart {
				continue
			}
			if lo == 0 {
				lo = r.Verse
			}
			hi = r.Verse
		}
	}
	return selSpanFromNative(lo, hi)
}

// studyMenu serves the pane's right-click menu through the SAME shared
// builder chapterText uses (selectionStudyMenu, reading.go) — the milestone-4
// unification that retired this file's temporary duplicate.
func (p *styledReadingPane) studyMenu() *fyne.Menu {
	return selectionStudyMenu(p.state, plainSelection(p.selectedRaw()), p.selectedVerseSpan(),
		p.copyToClipboard, p.selectAll)
}
