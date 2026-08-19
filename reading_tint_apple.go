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

// washIsLiveMutationOnPlatform is true on the panes where changing what a verse
// is washed in is an attribute mutation rather than a re-render — the two Apple
// panes, and only them. Read through the washIsLiveMutation var seam (tint.go).
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
const washIsLiveMutationOnPlatform = true

// lastPushedBodyFP / lastPushedTintFP are the two halves of what the native
// overlay currently holds, kept apart because they are repaired differently:
// the body by a rebuild, the tint by a live mutation (chapterBodyFingerprint,
// reading.go). Written only from the UI goroutine, like lastPushedChapterFP,
// which the Android pane still uses for the combined question.
var (
	lastPushedBodyFP string
	lastPushedTintFP string
)
