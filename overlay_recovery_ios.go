//go:build ios

package bibletext

import "fyne.io/fyne/v2"

// THE BLANK PANE AFTER A LONG BACKGROUND.
//
// Reported from a phone left open overnight: the app came back with the chapter
// header, the navigation, the tab bar and the note debug line all drawn, and the
// reading area completely empty. No notice, no fallback — just paper. Backgrounding
// and restoring a second time brought the text back, so nothing had been lost: the
// native view had simply stopped holding the chapter, and the app could not tell.
//
// It could not tell because it never asks. pushChapterHTML gates on what the Go
// side BELIEVES it last pushed (lastPushedBodyFP, reading_ios.go), so once the
// view is emptied behind the app's back the two disagree permanently and every
// ordinary refresh is gated out. The pane cannot heal itself; only something that
// clears the gate can.
//
// Android already had this hole and this fix, for a different cause — there the
// ACTIVITY is recreated and the Java TextView comes back empty. The stub that used
// to stand here excluded the Apple platforms on the stated grounds that "the Apple
// platforms keep their native text views in the app's own (never-recreated)
// window, so the foreground hook has nothing to recover there". The report above
// is a counterexample to that sentence.
//
// This half keys on the SYMPTOM rather than on a cause. iOS has no activity
// recreation to detect, and the mechanism that empties the view is not yet known —
// it did not reproduce under simulated memory pressure — so asking the view what
// it is holding is both the more direct question and the one that stays true
// whatever the cause turns out to be.
func foregroundOverlayRecovery(state *AppState) {
	if state == nil {
		return
	}
	fyne.Do(func() {
		if state.Bible == nil {
			return // still loading; the normal path will draw the first chapter
		}
		// NOTHING PUSHED YET is not a blank pane. On a cold start the view is
		// legitimately empty until the first push, and rebuilding here would
		// fight the launch path rather than recover anything.
		if lastPushedBookChapter == "" {
			return
		}
		if nativeReadingTextLength() > 0 {
			return // the pane is holding its chapter; nothing to do
		}

		// The app believes a chapter is on screen and the view says otherwise.
		// Keep the reader's place if the view can still say where it was — on a
		// truly emptied view it cannot, and landing at the top of the right
		// chapter is a far better outcome than a blank page.
		if v, d, f, ok := captureReadingAnchor(); ok {
			state.restore = &restoreAnchor{
				Book:    state.CurrentBook,
				Chapter: state.CurrentChapter,
				Verse:   v,
				Delta:   d,
				Frac:    f,
			}
		}
		// Clear the gate, or the rebuild below pushes nothing: the body
		// fingerprint is what pushChapterHTML compares against to decide the
		// chapter is already rendered.
		lastPushedBodyFP = ""
		lastPushedBookChapter = ""
		rebuildWindow(state)
	})
}
