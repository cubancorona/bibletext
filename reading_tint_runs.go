package bibletext

// The tint-run MODEL — pure Go, portable, and deliberately untagged: the
// Apple panes are its only consumer today (reading_tint_apple.go pushes the
// runs through cgo), but the flattening itself computes from chapterTint and
// the verses being drawn, nothing platform in it — and the wash-shape tests
// exercise it on every platform CI runs, which is how its darwin-only home
// broke the Linux and Windows builds (undefined: nativeTintRuns, first red
// CI of the notes era).

import "image/color"

// nativeTintRunCap bounds the run table the native side keeps as a fixed C
// array (BT_MAX_TINT_RUNS in both preambles — the three numbers must agree).
//
// 32 is far above what the model can produce: today the answer is one span, and
// the flattened plural-notes answer is at most 2k-1 runs for k notes on one
// chapter. Runs past the cap are DROPPED rather than truncating the last one to
// fit, so an overflow renders as a missing wash — the documented failure mode
// for this whole seam (markupFor, tint.go) — rather than as a band that runs on
// past where any note reaches.
const nativeTintRunCap = 32

// nativeTintRun is one contiguous stretch of verses under one wash, in the
// numbering of the verses being drawn.
//
// VERSES, not character ranges. The character range is the native side's own
// business: it holds a verse index built from the imported attributed string
// (btIOSBuildVerseIndex / btMacLocForVerse), it is the only side that knows
// where the importer actually put things, and it is the side that has to
// recompute those locations after every re-import anyway. Handing it verse
// numbers means Go never has to model the importer's output, and a run stays
// valid across a relayout.
type nativeTintRun struct {
	Lo, Hi int // inclusive verse range
	Wash   color.NRGBA
}

// nativeTintRuns flattens the chapter's tint answer into the runs the native
// panes paint.
//
// It walks the verses BEING DRAWN and asks chapterTint per verse, rather than
// reading the mark's span and handing that over. Two reasons, both load-bearing.
// The span is expressed in a translation's numbering and may not correspond to
// verses this chapter actually has (a mark can name verse 24 of a chapter that
// stops at 23); chapterTints.of is the function that already knows to answer
// tintNone there, and it is the same function every other surface asks. And a
// tint answer with holes in it — the plural-notes case, where one note's verses
// are split by another's — coalesces here into disjoint ascending runs without
// any call site learning a new shape.
//
// Verses arrive in document order, so a run extends only while the tint is the
// same AND the verse numbers are consecutive: a gap in the numbering ends the
// run, because the native side fills a run by walking lo..hi and would
// otherwise wash a verse the tint answer says nothing about.
func nativeTintRuns(state *AppState, verses []Verse) []nativeTintRun {
	tints := chapterTint(state)
	if tints.tint == tintNone {
		return nil // the overwhelmingly common case: nothing marked, no allocation
	}
	pal := state.pal()
	var runs []nativeTintRun
	last := tintNone
	for _, v := range verses {
		t := tints.of(v)
		wash, painted := t.wash(pal)
		if !painted {
			last = tintNone
			continue
		}
		if t == last && len(runs) > 0 && runs[len(runs)-1].Hi == v.Verse-1 {
			runs[len(runs)-1].Hi = v.Verse
			continue
		}
		if len(runs) >= nativeTintRunCap {
			break
		}
		runs = append(runs, nativeTintRun{Lo: v.Verse, Hi: v.Verse, Wash: wash})
		last = t
	}
	return runs
}
