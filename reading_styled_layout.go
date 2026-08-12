package bibletext

// The styled chapter LAYOUT engine for the desktop reading-pane rewrite
// (Windows/Linux parity project — the research verdict chose one pure-Go
// styled+selectable pane over per-platform native embedding; see the task
// notes). It produces positioned, styled runs from chapter data: the same
// greedy wrap, poetry breaks, and verse geometry as chapterText's rewrap, but
// per-run instead of per-plain-line, so a renderer can colour the words of
// Christ, raise the verse numbers, and paint a real selection — everything
// the single-style widget.Entry pane cannot do.
//
// Untagged ON PURPOSE (the android_chapter_html.go precedent): the layout is
// pure geometry over chapter data, so the whole engine unit-tests on the dev
// machine — no Windows/Linux toolchain in the loop. Only the eventual pane
// widget's platform dispatch will be tagged.
//
// The measure function is a parameter (like rewrap's) so tests can supply a
// deterministic ruler; production passes fyne.MeasureText at the pane's
// rendered sizes.

import (
	"strings"
)

// runKind distinguishes the two text styles a chapter line carries.
type runKind uint8

const (
	runWord     runKind = iota // scripture text, body size
	runVerseNum                // verse-number label, small + raised
)

// styledRun is one same-style span within a laid-out line. X/W are relative
// to the line's left edge; the renderer adds the pane's own inset.
type styledRun struct {
	Text string
	Kind runKind

	// Provenance, for styling and selection→verse attribution.
	Verse     int
	RedLetter bool
	Highlight bool

	// Geometry (set by layout).
	X, W float32

	// Offset is the flat rune offset of this run's first rune in the
	// SELECTION TEXT MODEL — the exact string shape chapterText's Entry holds
	// today (superscript numbers, spaces between words, "\n" between lines,
	// "\n\n" between paragraphs) — so plainSelection/cleanCopy and every
	// downstream consumer (share, AI, cross-refs) keep receiving the shape
	// they already handle.
	Offset int
}

// styledLine is one visual line: its runs plus vertical geometry.
type styledLine struct {
	Runs []styledRun
	Y, H float32

	// ParaFirst marks the first line of a paragraph. PoemBreakAfter marks a
	// line whose trailing break is AUTHORED (poem line / poetic verse join)
	// rather than a width wrap — the copy path keeps those breaks, exactly
	// like chapterText.hardBreakRows.
	ParaFirst      bool
	PoemBreakAfter bool

	// StartOffset/EndOffset are the line's [start,end) rune range in the flat
	// selection text model — the selection and copy layers walk lines with
	// them.
	StartOffset, EndOffset int
}

// chapterLayout is the laid-out chapter plus the geometry indexes the
// scroll-anchor persistence and highlight band rely on.
type chapterLayout struct {
	Lines  []styledLine
	Height float32

	// VerseLines mirrors chapterText.verseLines: each verse's first line
	// index, in chapter order (several short verses can share a line).
	VerseLines []verseLine

	// Text is the flat selection text model (see styledRun.Offset).
	Text string

	// HighlightStart/End are the first/last line indexes of the highlighted
	// verse range, -1 when none — the highlight band's geometry.
	HighlightStart, HighlightEnd int
}

// styledMeasure measures one run's text width at its rendered size.
type styledMeasure func(text string, kind runKind) float32

// styledLayoutParams collects the knobs so tests can pin exact geometry.
type styledLayoutParams struct {
	Width      float32 // available line width (already inset by the caller)
	LineHeight float32 // baseline-to-baseline for body lines
	ParaGap    float32 // extra gap above a paragraph's first line (not the first paragraph)
	SpaceW     float32 // width of the inter-word space at body size

	// Indent is the reporter layout's first-line paragraph indent (0 = off).
	// GEOMETRY ONLY, deliberately: the iOS HTML path has to smuggle its indent
	// in as literal em+en space characters (the importer drops text-indent), so
	// its copied text carries them; here the indent never enters the selection
	// text model, so copy stays clean. A paragraph that OPENS on a poem line
	// skips the indent — poetry is never first-line indented in print — exactly
	// the buildChapterHTML rule.
	Indent float32
}

// layoutChapter lays the chapter out as styled runs. It mirrors rewrap's
// behaviour — same paragraph grouping (groupVersesIntoParagraphs), same
// poetic joins (poeticJoin), same "\n" poem sentinels (verseTokens), same
// number-glued-to-first-word wrap rule — so the text model, verse geometry,
// and copy semantics stay identical to the shipping pane while gaining
// styling.
func layoutChapter(state *AppState, verses []Verse, p styledLayoutParams, measure styledMeasure) *chapterLayout {
	lay := &chapterLayout{HighlightStart: -1, HighlightEnd: -1}
	var text strings.Builder
	offset := 0 // rune offset into the selection text model
	appendText := func(s string) {
		text.WriteString(s)
		offset += len([]rune(s))
	}

	redLetter := redLetterEnabled()
	y := float32(0)

	for pi, para := range groupVersesIntoParagraphs(verses) {
		if pi > 0 {
			// One half of the paragraph separator; the other "\n" is written
			// when the paragraph's first unit opens its line, so the model
			// carries the same "\n\n" join rewrap produced.
			appendText("\n")
			y += p.ParaGap
		}

		var cur []styledRun
		curW := float32(0)
		paraFirst := true
		// Seeding curW is the whole indent mechanism: the first line's runs
		// start that far in (place() derives X from curW) and its wrap budget
		// shrinks by the same amount (the p.Width check); flushLine resets to 0
		// so every later line of the paragraph sits flush left.
		if p.Indent > 0 && len(para) > 0 && !verseIsPoetic(para[0].Text) {
			curW = p.Indent
		}

		flushLine := func(poemBreak bool) {
			if len(cur) == 0 {
				return
			}
			lay.Lines = append(lay.Lines, styledLine{
				Runs: cur, Y: y, H: p.LineHeight,
				ParaFirst: paraFirst, PoemBreakAfter: poemBreak,
				StartOffset: cur[0].Offset, EndOffset: offset,
			})
			paraFirst = false
			y += p.LineHeight
			cur = nil
			curW = 0
		}

		// place adds one wrap UNIT — 1..2 runs that must stay on the same
		// line (a verse number and its first word) — wrapping first if it
		// does not fit. Intra-unit spaces count toward the unit's width.
		place := func(unit []styledRun) {
			unitW := float32(0)
			for i, r := range unit {
				unitW += r.W
				if i > 0 {
					unitW += p.SpaceW
				}
			}
			add := unitW
			if len(cur) > 0 {
				add += p.SpaceW
			}
			if len(cur) > 0 && curW+add > p.Width {
				flushLine(false) // width wrap — never an authored break
			}
			if len(cur) > 0 {
				appendText(" ")
			} else if text.Len() > 0 {
				appendText("\n") // this unit opens a new line
			}
			x := curW
			if len(cur) > 0 {
				x += p.SpaceW
			}
			for i := range unit {
				if i > 0 {
					appendText(" ")
					x += p.SpaceW
				}
				unit[i].X = x
				unit[i].Offset = offset
				appendText(unit[i].Text)
				x += unit[i].W
			}
			cur = append(cur, unit...)
			curW = x
		}

		for vi, v := range para {
			if vi > 0 && poeticJoin(para[vi-1].Text, v.Text) {
				flushLine(true)
			}

			red := redLetter && isWordsOfChrist(v.BookName, v.Chapter, v.Verse)
			hl := isVerseHighlighted(state, v)

			// Provisional first-line record; place() may wrap the first unit
			// onto a fresh line, so both indexes are patched after placing.
			lay.VerseLines = append(lay.VerseLines, verseLine{verse: v.Verse, line: len(lay.Lines)})
			vlIdx := len(lay.VerseLines) - 1

			first := true
			for _, tok := range verseTokens(v) {
				if tok == "\n" {
					flushLine(true) // authored poem-line boundary
					continue
				}
				var unit []styledRun
				if num := superscriptNumber(v.Verse); first && strings.HasPrefix(tok, num+" ") {
					word := strings.TrimPrefix(tok, num+" ")
					unit = []styledRun{
						{Text: num, Kind: runVerseNum, Verse: v.Verse, Highlight: hl,
							W: measure(num, runVerseNum)},
						{Text: word, Kind: runWord, Verse: v.Verse, RedLetter: red, Highlight: hl,
							W: measure(word, runWord)},
					}
				} else {
					unit = []styledRun{{Text: tok, Kind: runWord, Verse: v.Verse, RedLetter: red,
						Highlight: hl, W: measure(tok, runWord)}}
				}
				place(unit)
				if first {
					// The line the verse's first token ACTUALLY landed on.
					line := len(lay.Lines)
					lay.VerseLines[vlIdx].line = line
					if hl && lay.HighlightStart < 0 {
						lay.HighlightStart = line
					}
					first = false
				}
			}
			if hl {
				// The verse's last content line: the one under construction,
				// or — when a trailing sentinel just flushed — the last
				// appended line.
				end := len(lay.Lines)
				if len(cur) == 0 {
					end--
				}
				if end > lay.HighlightEnd {
					lay.HighlightEnd = end
				}
			}
		}
		flushLine(false)
	}

	lay.Height = y
	lay.Text = text.String()
	if lay.HighlightStart >= 0 {
		last := len(lay.Lines) - 1
		if lay.HighlightStart > last {
			lay.HighlightStart = last
		}
		if lay.HighlightEnd > last {
			lay.HighlightEnd = last
		}
		if lay.HighlightEnd < lay.HighlightStart {
			lay.HighlightEnd = lay.HighlightStart
		}
	}
	return lay
}
