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
// machine — no Windows/Linux toolchain in the loop. Only the pane's platform
// dispatch is tagged (useStyledPane); the engine and the pane itself are not.
//
// The measure function is a parameter (like rewrap's) so tests can supply a
// deterministic ruler. Production passes styledReadingPane.measure — which goes
// through Driver().RenderedTextSize with the pane's FontSource, NOT
// fyne.MeasureText: the scripture serif is not the app font, and fyne.MeasureText
// cannot honour a FontSource, so measuring with it would wrap to the wrong
// width.

import (
	"strings"
)

// runKind distinguishes the two text styles a chapter line carries.
type runKind uint8

const (
	runWord     runKind = iota // scripture text, body size
	runVerseNum                // verse-number label, small + raised
)

// verseTint, its constants and overridesTextColour used to live HERE. They moved
// to tint.go when the other four renderers came onto the same model: a type five
// surfaces speak — two of them behind cgo — should not be reached for out of the
// Windows/Linux layout engine.

// styledRun is one same-style span within a laid-out line. X/W are relative
// to the line's left edge; the renderer adds the pane's own inset.
type styledRun struct {
	Text string
	Kind runKind

	// Provenance, for styling and selection→verse attribution.
	Verse     int
	RedLetter bool
	Tint      verseTint

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
// scroll-anchor persistence relies on.
//
// It carries NO highlight line range. It used to (HighlightStart/End), and the
// pane painted one full-column rectangle over those lines — which lit every
// neighbouring verse sharing the range's first or last line, because a line
// routinely carries more than one verse. The tint now lives on the runs, where
// it belongs, and the rectangles are derived per line from run geometry
// (tintSpansForLayout).
type chapterLayout struct {
	Lines  []styledLine
	Height float32

	// VerseLines mirrors chapterText.verseLines: each verse's first line
	// index, in chapter order (several short verses can share a line).
	VerseLines []verseLine

	// Text is the flat selection text model (see styledRun.Offset).
	Text string

	// BandLine is the line the note band opens above (-1 = no band), and
	// BandY/BandH are the reserved rectangle in the same content coordinates
	// as styledLine.Y.
	//
	// THE BAND IS ADVANCE, NOT LINE HEIGHT — exactly like ParaGap. No line's Y
	// or H covers it, so it is disjoint from every line box by construction,
	// and every wash this pane paints (tintSpansForLayout, verseSpansForLayout)
	// is [ln.Y, ln.Y+ln.H) for some line. That is what makes "a washed run can
	// never reach into the band" a property of the geometry rather than a
	// guard: Android inflated the anchor line's ascent instead and the verse's
	// highlight grew into the band and slid under the bubble (fixed 19 Aug by
	// reserving in the PRECEDING line's descent), and its LineHeightSpan is
	// paragraph-scoped so it inflated every FOLLOWING line until guarded.
	// Neither failure has an assignment site here.
	BandLine int
	// BandLastLine is the final line of the paragraph the band opens above —
	// the paragraph the bubble is about. Recorded here because only the
	// layout knows where the paragraph ends, and the scroll target needs it
	// (highlightY: arriving at any verse of that paragraph must show the
	// bubble, not scroll past it).
	BandLastLine int
	BandY        float32
	BandH        float32
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

	// BandVerse / BandH reserve vertical room ABOVE that verse's first line
	// for the in-text note sticker (reading_styled_note.go). BandH is measured
	// by the pane BEFORE this call — the band's height is that number, and
	// that ordering is the whole trick (the iOS twin says the same,
	// btIOSNoteHeightForWidth). 0 in either field = no band.
	//
	// GEOMETRY ONLY, like Indent: it never enters the selection text model, so
	// lay.Text is byte-identical with and without a band and copy, selection
	// offsets and every downstream consumer are untouched. A verse that starts
	// MID-LINE pushes the whole shared line down — the same coarse grain
	// Android reserves at (the preceding line's descent) and iOS reserves at
	// (the whole paragraph); forcing a break for the anchor verse would write a
	// "\n" into the model and change what a reader copies.
	BandVerse int
	BandH     float32
}

// layoutChapter lays the chapter out as styled runs. It mirrors rewrap's
// behaviour — same paragraph grouping (groupVersesIntoParagraphs), same
// poetic joins (poeticJoin), same "\n" poem sentinels (verseTokens), same
// number-glued-to-first-word wrap rule — so the text model, verse geometry,
// and copy semantics stay identical to the shipping pane while gaining
// styling.
func layoutChapter(state *AppState, verses []Verse, p styledLayoutParams, measure styledMeasure) *chapterLayout {
	lay := &chapterLayout{BandLine: -1, BandLastLine: -1}
	var text strings.Builder
	offset := 0 // rune offset into the selection text model
	appendText := func(s string) {
		text.WriteString(s)
		offset += len([]rune(s))
	}

	redLetter := redLetterEnabled()
	// ONE tint answer for the whole chapter, asked per verse below (tint.go).
	tints := chapterTint(state)
	y := float32(0)

	for pi, para := range groupVersesIntoParagraphs(verses) {
		if pi > 0 {
			// One half of the paragraph separator; the other "\n" is written
			// when the paragraph's first unit opens its line, so the model
			// carries the same "\n\n" join rewrap produced.
			appendText("\n")
			y += p.ParaGap
		}

		// THE BAND OPENS ABOVE THE WHOLE PARAGRAPH the note's verse belongs

		// should not be breaking up paragraphs… No breaking up the Word of
		// God"). It is the rule iOS has always followed, because
		// paragraphSpacingBefore is the only thing TextKit reserves with, and
		// it is now the rule everywhere.
		//
		// Reserved HERE, before the paragraph lays out a single line, so y is
		// the paragraph's own top: adding to it moves the paragraph and
		// everything after it down by exactly BandH and touches no line's H
		// and no run's X/W. The band therefore remains ADVANCE — disjoint
		// from every line box, so no wash can reach it.
		bandOpensHere := false
		if p.BandH > 0 && lay.BandLine < 0 && paraCarriesVerse(para, p.BandVerse) {
			y += p.BandH
			lay.BandLine = len(lay.Lines)
			lay.BandY, lay.BandH = y-p.BandH, p.BandH
			bandOpensHere = true
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

			// Per TOKEN, not per verse. In the BSB the words of Christ are a
			// SPAN inside the verse, so the narration and the other speaker's
			// reply beside them must stay in body colour — this used to stamp
			// one flag on every token, which put "Seven," they replied. in red
			// (red_letter_runs.go). Every other edition yields a single run and
			// every flag comes back the same, which IS the old whole-verse
			// behaviour, so nothing but the BSB moves.
			toks := verseTokens(v)
			redTok := redLetterTokenFlags(state.CurrentVersion, v, redLetter, toks)
			tint := tints.of(v)

			// Provisional first-line record; place() may wrap the first unit
			// onto a fresh line, so the index is patched after placing.
			lay.VerseLines = append(lay.VerseLines, verseLine{verse: v.Verse, line: len(lay.Lines)})
			vlIdx := len(lay.VerseLines) - 1

			first := true
			for ti, tok := range toks {
				if tok == "\n" {
					flushLine(true) // authored poem-line boundary
					continue
				}
				var unit []styledRun
				if num := superscriptNumber(v.Verse); first && strings.HasPrefix(tok, num+" ") {
					word := strings.TrimPrefix(tok, num+" ")
					unit = []styledRun{
						{Text: num, Kind: runVerseNum, Verse: v.Verse, Tint: tint,
							W: measure(num, runVerseNum)},
						{Text: word, Kind: runWord, Verse: v.Verse, RedLetter: redTok[ti], Tint: tint,
							W: measure(word, runWord)},
					}
				} else {
					unit = []styledRun{{Text: tok, Kind: runWord, Verse: v.Verse, RedLetter: redTok[ti],
						Tint: tint, W: measure(tok, runWord)}}
				}
				place(unit)
				if first {
					// The line the verse's first token ACTUALLY landed on.
					lay.VerseLines[vlIdx].line = len(lay.Lines)
					first = false
				}
			}
		}
		flushLine(false)
		if bandOpensHere {
			lay.BandLastLine = len(lay.Lines) - 1
		}
	}

	lay.Height = y
	lay.Text = text.String()
	return lay
}

// tintSpan is one painted wash rectangle: a maximal run of same-tint tokens
// within ONE line. X0/X1 are relative to the line's left edge, exactly like
// styledRun.X — the renderer adds the pane's live inset.
type tintSpan struct {
	Line int
	Tint verseTint
	// LINE-RELATIVE, exactly like styledRun.X — the renderer adds insetX().
	//
	// Named for the ruler on purpose. selectionSpan (reading_styled_select.go)
	// is the same {Line, X0, X1} shape ninety lines away and its X values are
	// ABSOLUTE widget coordinates, because xForOffset has already added the
	// inset. Two look-alike structs on opposite rulers is a copy-paste that
	// silently displaces a rect by the whole reporter inset — 183pt at 760pt
	// wide — and no test would catch it, because the tests build their expected
	// boxes with the same ruler the code under test used.
	LineX0, LineX1 float32
}

// tintSpansForLayout derives the wash rectangles from run geometry: per line,
// per contiguous same-tint stretch.
//
// PER LINE is the whole point — a full-column band over a line RANGE washes the
// neighbouring verses that share the range's first and last lines. COALESCED is
// the other half: a rect per token would be slow and would show seams at every
// inter-word space (and at every red-letter or verse-number boundary, since
// those split the drawn segments but not the tint), so a stretch is closed only
// when the tint itself changes. Spanning first.X → last.X+last.W keeps the
// joining spaces inside the wash, the same no-gaps rule the HTML dialects hold
// (reading_highlight_gap_test.go).
// paraCarriesVerse reports whether a paragraph contains the given verse — the
// question "which paragraph does the note's band open above".
func paraCarriesVerse(para []Verse, verse int) bool {
	if verse <= 0 {
		return false
	}
	for _, v := range para {
		if v.Verse == verse {
			return true
		}
	}
	return false
}

func tintSpansForLayout(lay *chapterLayout) []tintSpan {
	return runSpansForLayout(lay, func(r styledRun) verseTint { return r.Tint })
}

// verseSpansForLayout is the same walk for a wash that is not carried on the
// run — the read-along narration, which lives on the PANE (raVerse) rather
// than in the layout, and which layers OVER the highlight rather than
// replacing it (styledReadAlongTint is translucent, so both are meant to show).
//
// It exists because the read-along band had the exact defect the highlight just
// lost: one full-column rectangle over a line range, washing whatever verse
// happened to share the narrated verse's first and last lines, and drawn ON TOP
// of the corrected highlight so it won wherever they met.
func verseSpansForLayout(lay *chapterLayout, verse int) []tintSpan {
	if verse <= 0 {
		return nil
	}
	return runSpansForLayout(lay, func(r styledRun) verseTint {
		if r.Verse == verse {
			return tintReadAlong
		}
		return tintNone
	})
}

// runSpansForLayout walks each line and coalesces contiguous runs sharing a
// tint into ONE span per stretch.
//
// PER LINE is the whole point — a full-column band over a line RANGE washes the
// neighbouring verses that share the range's first and last lines. COALESCED is
// the other half: a rect per token would be slow and would show seams at every
// inter-word space (and at every red-letter or verse-number boundary, since
// those split the drawn segments but not the tint), so a stretch is closed only
// when the tint itself changes. Spanning first.X → last.X+last.W keeps the
// joining spaces inside the wash, the same no-gaps rule the HTML dialects hold
// (reading_highlight_gap_test.go).
func runSpansForLayout(lay *chapterLayout, tintOf func(styledRun) verseTint) []tintSpan {
	if lay == nil || tintOf == nil {
		return nil
	}
	var spans []tintSpan
	for li, ln := range lay.Lines {
		open := -1 // index into spans of the stretch still growing, -1 = none
		for _, run := range ln.Runs {
			t := tintOf(run)
			if t == tintNone {
				open = -1
				continue
			}
			if open >= 0 && spans[open].Tint == t {
				spans[open].LineX1 = run.X + run.W
				continue
			}
			spans = append(spans, tintSpan{Line: li, Tint: t, LineX0: run.X, LineX1: run.X + run.W})
			open = len(spans) - 1
		}
	}
	return spans
}
