//go:build darwin && !ios

package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// THE MACOS PANE IS TOLD. TestTheSentNoteIsOnThePaneOnScreen holds the
// Windows and Linux pane a window shows; the macOS pane is a native text view
// the host test cannot look into, so this holds what the Go side told it,
// which is what it shows: after the send, the chapter it was last given is
// the chapter as it now stands, the note in it (the body fingerprint folds
// the mirror) and the note's wash over it (the tint), and the placement the
// send asked for has been made. Both of refreshNoteOnly's paths leave all
// three: the rebuild the send takes today, the mirror having moved the body,
// and the in-place push it would take if the body stood still. A send that
// repainted nothing leaves the body and the wash of the page before it, and
// the placement pending.
//
// Mutation: showSentNote without its refreshNoteOnly.
func TestASentNoteIsPushedToTheMacPane(t *testing.T) {
	h := newAppearanceHarness(t, false)
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	t.Cleanup(func() { deleteAllNotes(appPrefs()) })
	catchShares(t)
	// What the pane was last given is process-wide, shared with every other
	// test (paneHolds restores it the same way).
	bc, body, tint := lastPushedBookChapter, lastPushedBodyFP, lastPushedTintFP
	t.Cleanup(func() { lastPushedBookChapter, lastPushedBodyFP, lastPushedTintFP = bc, body, tint })
	st := h.state
	if useStyledPane() || findStyledPane(st.window.Canvas().Content()) != nil {
		t.Fatal("control: the window must show the macOS pane, not the styled one")
	}
	// The pane given the chapter exactly as it stands before the send, so a
	// push the send did not make cannot pass for one it did.
	st.refreshReadingOnly()
	tintBefore := chapterTint(st).fingerprint()
	if lastPushedBookChapter != "John|1" || lastPushedBodyFP != chapterBodyFingerprint(st) || lastPushedTintFP != tintBefore {
		t.Fatalf("control: the pane must hold John 1 as it stands; holds %q", lastPushedBookChapter)
	}

	promptShareNote(st, drawnSelection(t, st, 1, 1), selSpanFromNative(1, 1))
	composerField(h.top()).SetText("fixture mac alpha")
	test.Tap(findTreeButton(h.top().Content, "Share"))
	if st.ActiveNote != "fixture mac alpha" || !st.mark.fromNote() {
		t.Fatal("control: the send must show the note it kept")
	}
	if chapterTint(st).fingerprint() == tintBefore {
		t.Fatal("control: the note's wash must change the chapter's tint")
	}

	if lastPushedBookChapter != "John|1" {
		t.Errorf("the pane was given %q, want John 1", lastPushedBookChapter)
	}
	if lastPushedBodyFP != chapterBodyFingerprint(st) {
		t.Error("the pane holds the page as it was before the send, without the note")
	}
	if lastPushedTintFP != chapterTint(st).fingerprint() {
		t.Error("the pane was never given the note's wash")
	}
	if st.forceReposition {
		t.Error("the view was never placed on the note: the placement is still pending")
	}
}
