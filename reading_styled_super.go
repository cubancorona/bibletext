package bibletext

// The Psalm superscription on the styled pane (Windows/Linux): the italic,
// unnumbered title line print sets above verse 1, drawn GEOMETRY-ONLY in the
// TopPad advance layoutChapter reserves — the title never enters lay.Text,
// so selection, copy, share and verse attribution exclude it by construction
// (the footnote section's discipline, at the other end of the chapter). Its
// notes join the bottom section keyed "Title" (footnote_section.go).

import (
	"strings"
	"sync"

	"fyne.io/fyne/v2"
)

// styledSuperGeom is every rect the title draws and hit-guards with —
// written ONLY in relayout, beside the layout whose TopPad it measured.
type styledSuperGeom struct {
	present bool
	rect    styledNoteRect   // the whole title block, for the press guards
	lines   []styledFnText   // positioned lines (Key unused)
	washes  []styledNoteRect // one per line: the narration wash, on the lines' own ruler
	height  float32          // total advance to reserve, including the gap below
}

// measureStyledSuperscription wraps the title at body size to the layout's
// own column. The gap below the last line is part of the reserved height, so
// verse 1 clears the title by the same breath print gives it.
func measureStyledSuperscription(text string, avail, size, lineH float32, meas func(string) float32) styledSuperGeom {
	text = strings.TrimSpace(text)
	if text == "" || avail <= 0 || size <= 0 {
		return styledSuperGeom{}
	}
	g := styledSuperGeom{present: true}
	y := float32(0)
	for _, ln := range styledFnWrap(text, avail, avail, meas) {
		g.lines = append(g.lines, styledFnText{Text: ln, X: 0, Y: y})
		// The wash is the line's own box, as wide as the glyphs the SAME meas
		// wrapped it with — the pixel twin of a verse wash (per line, bounded
		// to the run). The gap below the title stays unwashed.
		g.washes = append(g.washes, styledNoteRect{X: 0, Y: y, W: meas(ln), H: lineH})
		y += lineH
	}
	y += lineH * 0.45 // the gap between the title and verse 1
	g.height = y
	g.rect = styledNoteRect{X: 0, Y: 0, W: avail, H: y}
	return g
}

// place moves the title to absolute content coordinates in one call.
func (g *styledSuperGeom) place(x, y float32) {
	if !g.present {
		return
	}
	g.rect.X += x
	g.rect.Y += y
	for i := range g.lines {
		g.lines[i].X += x
		g.lines[i].Y += y
	}
	for i := range g.washes {
		g.washes[i].X += x
		g.washes[i].Y += y
	}
}

// hits reports whether a press landed on the title — inert, like the
// footnote section: lineAtY would otherwise clamp such presses onto the
// FIRST scripture line and start a selection there.
func (g styledSuperGeom) hits(p fyne.Position) bool {
	return g.present && g.rect.contains(p)
}

// styledSuperFont is the face the title draws and measures with: a true
// italic, which this surface now always has. It used to depend on the
// operating system supplying one — Linux distributions often did not, and the
// embedded fallback had no italic at all, so the Psalm title lost its register
// and was carried by colour alone. The shipped family carries all four cuts,
// because canvas.Text cannot synthesize an italic from a single-face
// FontSource any more than it can synthesize a bold. Resolved ONCE per
// process, exactly like styledPaneFont and for the same reason: relayout runs
// continuously during a window drag-resize.
func styledSuperFont() (fyne.Resource, bool) {
	styledSuperFontOnce.Do(func() {
		if fonts := loadReadingFonts(); fonts != nil && fonts.italic != nil {
			styledSuperFontCached, styledSuperFontItalic = fonts.italic, true
			return
		}
		styledSuperFontCached, styledSuperFontItalic = styledPaneFont(), false
	})
	return styledSuperFontCached, styledSuperFontItalic
}

var (
	styledSuperFontOnce   sync.Once
	styledSuperFontCached fyne.Resource
	styledSuperFontItalic bool
)
