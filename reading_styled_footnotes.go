package bibletext

// The chapter-bottom translators'-footnote section on the styled pane
// (Windows/Linux): GEOMETRY-ONLY, in the note-sticker's mould. The section
// never enters lay.Text, so select-all, copy, share and verse attribution
// exclude it BY CONSTRUCTION — the same purity the Apple panes buy with
// native content-end detectors, obtained here for free by keeping the
// apparatus out of the selection model entirely (reading_styled_select.go's
// "the sticker is not text" discipline). The block is measured inside
// relayout, beside the layout it hangs under, so width changes re-wrap it
// with the chapter; the renderer owns its canvas objects in their own slices
// (never r.texts, which is index-parallel to p.drawRuns).

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
)

// styledFnRatio sizes the section against the body — the same 0.85 the Apple
// panes use, for visual parity (their native-scan pact does not apply here;
// this is typography only).
const styledFnRatio = float32(0.85)

// styledFnText is one positioned text run of the section: a verse-number key
// or a wrapped note line. Coordinates are section-relative until place().
type styledFnText struct {
	Text string
	Key  bool // verse-number key — drawn in the verse-number colour
	X, Y float32
}

// styledFnGeom is every rect the section draws and hit-guards with — written
// ONLY in relayout, beside the layout it was measured for (the noteGeom
// "assigned together or not at all" discipline).
type styledFnGeom struct {
	present bool
	rect    styledNoteRect // the whole section, for the selection guards
	rule    styledNoteRect // the short flush-left hairline
	texts   []styledFnText
	height  float32 // total, including the air below the rule and a bottom pad
}

// measureStyledFootnotes wraps the entries at fnSize to the layout's own
// column width. First lines wrap short of the verse-number key; continuation
// lines run the full measure, flush left — the slip-opinion page's own
// grammar.
func measureStyledFootnotes(entries []footnoteEntry, avail, fnSize float32, meas func(string) float32) styledFnGeom {
	if len(entries) == 0 || avail <= 0 || fnSize <= 0 {
		return styledFnGeom{}
	}
	lh := fnSize * 1.4
	g := styledFnGeom{present: true}

	ruleW := fnSize * 6
	if ruleW > avail {
		ruleW = avail
	}
	y := float32(0)
	g.rule = styledNoteRect{X: 0, Y: y, W: ruleW, H: 1}
	y += 1 + lh*0.6

	for _, e := range entries {
		key := strconv.Itoa(e.Verse)
		keyW := meas(key)
		bodyX := keyW + fnSize*0.35
		lines := styledFnWrap(e.Text, avail-bodyX, avail, meas)
		g.texts = append(g.texts, styledFnText{Text: key, Key: true, X: 0, Y: y})
		for i, ln := range lines {
			x := bodyX
			if i > 0 {
				x = 0
			}
			g.texts = append(g.texts, styledFnText{Text: ln, X: x, Y: y})
			y += lh
		}
		if len(lines) == 0 {
			y += lh
		}
		y += lh * 0.25 // entry gap
	}
	y += lh * 0.35 // bottom pad
	g.height = y
	g.rect = styledNoteRect{X: 0, Y: 0, W: avail, H: y}
	return g
}

// styledFnWrap is a greedy word wrap with a distinct first-line width (the
// verse key claims the start of line one). Modeled on styledNoteWrap, but
// measuring through the caller's closure so the SCRIPTURE face is measured
// (fyne.MeasureText cannot honour FontSource — the p.measure lesson).
func styledFnWrap(text string, firstW, restW float32, meas func(string) float32) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var out []string
	line, w := words[0], firstW
	for _, word := range words[1:] {
		if cand := line + " " + word; meas(cand) <= w {
			line = cand
			continue
		}
		out = append(out, line)
		line, w = word, restW
	}
	return append(out, line)
}

// place moves the section-relative geometry to absolute content coordinates,
// in one call — rule, texts and the guard rect cannot disagree.
func (g *styledFnGeom) place(x, y float32) {
	if !g.present {
		return
	}
	g.rect.X += x
	g.rect.Y += y
	g.rule.X += x
	g.rule.Y += y
	for i := range g.texts {
		g.texts[i].X += x
		g.texts[i].Y += y
	}
}

// hits reports whether a press landed inside the section — the guard the
// four press handlers ask, so the apparatus can never start a selection or
// serve a study menu (lineAtY would otherwise clamp such clicks onto the
// last scripture line).
func (g styledFnGeom) hits(p fyne.Position) bool {
	return g.present && g.rect.contains(p)
}
