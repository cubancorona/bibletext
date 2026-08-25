//go:build darwin

package bibletext

import "fmt"

// refreshNoteInPlace is the Apple panes' answer to refreshNoteOnly: mutate the
// note and the wash on the pane already on screen, and do not rebuild the Fyne
// tree the native view's frame tracks. See notes_refresh.go for the defect this
// removes and why a rebuild is what caused it.
//
// EVERY REFUSAL BELOW IS A CASE WHERE THE REBUILD IS THE CORRECT ANSWER, not a
// case where this is merely unsure. They are listed in the order they are
// cheapest to ask.
func refreshNoteInPlace(state *AppState) bool {
	if state == nil || state.Bible == nil {
		return false
	}
	// The mimic dev mode builds the STYLED pane on darwin (reading_styled_area.go).
	// There the note is a Fyne band inside the tree, with reserved height — a
	// selection change is a layout change, and only a rebuild carries it.
	if useStyledPane() {
		return false
	}
	// THE ONE THING THE FYNE TREE STILL SAYS ABOUT NOTES ON THESE PANES. The
	// banner stands down for the native sticker in every case but one: a payload
	// that could not be decoded is told in the note's place (buildNoteBanner's
	// plan.Notice branch, which runs BEFORE the native-sticker suppression on
	// purpose). While a notice is up, the tree is carrying note state and only a
	// rebuild can change it.
	plan := buildChapterPlan(state, appPrefs(), state.Bible)
	if plan.Notice != "" {
		return false
	}
	// THE PANE MUST ALREADY HOLD THIS CHAPTER'S TEXT. Otherwise the sticker and
	// the wash would be painted onto the wrong scripture — a note anchored to a
	// verse that is not on screen, over a highlight range that means something
	// else. This is the rebuild's own fast-path question (newMacReadingHost /
	// pushChapterHTML), asked here before taking the same shortcut without the
	// rebuild wrapped around it; when the answer is no, the rebuild is exactly
	// what is needed, so falling back is not a compromise.
	//
	// A pending restore counts as "not holding it yet": the restore forces the
	// slow re-import path precisely so the scroll lands where the reader left
	// off, and swallowing it here would strand the position.
	if state.restore != nil {
		return false
	}
	if lastPushedBookChapter != fmt.Sprintf("%s|%d", state.CurrentBook, state.CurrentChapter) {
		return false
	}
	if lastPushedBodyFP != chapterBodyFingerprint(state) {
		return false
	}

	// From here it is the fast path's own body, verbatim in effect: the wash as
	// a live range mutation when it moved, then the sticker's compare-and-refresh,
	// then any placement the verb asked for.
	verses := state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter)
	if tintFP := chapterTint(state).fingerprint(); tintFP != lastPushedTintFP {
		lastPushedTintFP = tintFP
		applyNativeTint(state, verses)
	}
	pushNoteToPane(state)
	// forceReposition means "place the view", which is a SCROLL and never a
	// re-import (reading_tint_apple.go). Note cycling sets it only when the next
	// note has another verse anchor; honoring it here keeps the fast path a true
	// substitute for the rebuild rather than a subset of it.
	if state.forceReposition {
		state.forceReposition = false
		nativeScrollToHighlight()
	}
	return true
}
