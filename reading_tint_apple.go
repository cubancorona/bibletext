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

// lastPushedBodyFP / lastPushedTintFP are the two halves of what the native
// overlay currently holds, kept apart because they are repaired differently:
// the body by a rebuild, the tint by a live mutation (chapterBodyFingerprint,
// reading.go). Written only from the UI goroutine, like lastPushedChapterFP,
// which the Android pane still uses for the combined question.
var (
	lastPushedBodyFP string
	lastPushedTintFP string
)
