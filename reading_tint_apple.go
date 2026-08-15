//go:build darwin

package bibletext

// WHAT A VERSE'S BACKGROUND IS ON THE APPLE PANES — the model the native side
// is told, instead of the bool it used to remember.
//
// Shared by iOS and macOS, so it carries an explicit `darwin` constraint rather
// than a filename suffix: `_ios.go` is iOS only and `_darwin.go` excludes iOS,
// and this has to be both. Everything here is ordinary Go — the cgo call that
// hands it across lives in reading_ios.go / reading_macos.go behind one pair of
// names (setNativeTint / applyNativeTint), which is what keeps the two panes
// moving together.
//
// THE PROBLEM IT EXISTS FOR. TextKit gives a character exactly ONE
// NSBackgroundColorAttributeName. Two things want it: the chapter's own wash
// (a search hit, a note, a link's span, a Go-to — chapterTint in tint.go) and
// the narration wash that walks the chapter while audio plays. The narration
// used to take the attribute and, on moving off, REMOVE it — which is only
// right if nothing was underneath. Over a verse carrying a note it was a quiet
// deletion: the reader's message lost its mark as the audio passed it, with
// nothing to bring it back until the next full re-render.
//
// The fix is not a second flag remembering "there was a highlight here". It is
// that the native side is told what a verse's wash SHOULD BE NOW, from the one
// function that answers that question for every surface, and sets it. Then
// "the narration moved off this verse" and "this verse has a note" cannot
// disagree, because only one of them is a fact and the other is a redraw.
//
// AND IT IS THE FAST PATH. A wash change used to go through
// chapterRenderFingerprint → buildChapterHTML → a whole NSAttributedString
// re-import. The wash is one attribute over a range of an attributed string
// that is already on screen; it does not need the rebuild, and on Psalm 119 the
// rebuild is two orders of magnitude dearer than the mutation.

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

// washIsLiveMutation is true on the panes where changing what a verse is washed
// in is an attribute mutation rather than a re-render — the two Apple panes, and
// only them.
//
// It is here because that is exactly where the CONSEQUENCE lives: a mutation
// carries no scroll, so on these panes an explicit arrival has to declare
// state.forceReposition or "Go to John 3:16" while already on John 3 lights the
// verse without moving the view. Everywhere else the wash is still part of the
// one render fingerprint, the rebuild still carries the scroll, and setting the
// flag would only cost something: on Android it turns a Go-to onto the chapter
// already open into a full re-render the gate used to skip
// (reading_android.go reads and clears it), and on Windows/Linux nothing reads or
// clears it at all, so it would latch true for the life of the process.
const washIsLiveMutation = true

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

// lastPushedBodyFP / lastPushedTintFP are the two halves of what the native
// overlay currently holds, kept apart because they are repaired differently:
// the body by a rebuild, the tint by a live mutation (chapterBodyFingerprint,
// reading.go). Written only from the UI goroutine, like lastPushedChapterFP,
// which the Android pane still uses for the combined question.
var (
	lastPushedBodyFP string
	lastPushedTintFP string
)
