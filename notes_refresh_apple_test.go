//go:build darwin

package bibletext

// THE NOTE-SELECTION REPAINT, on the panes where it was visibly wrong.
//
// THE DEFECT: clicking a note group with several notes, to cycle to the next
// note, produced a delay and then a flash of a missized, misplaced reading
// pane before the pane settled.
//
// The cause was not the note and not the wash — both were already cheap. It was
// that every note verb ended in a WHOLE-TREE rebuild of the reading column, and
// the Apple panes' scripture is a native view whose frame tracks a widget in
// that tree. A rebuild hands the tracker a host with no geometry, and the
// correction rides pushFrame's shared 60 ms re-assert timer. Hence a delay, and
// hence a pane at the wrong rect while it lasts.
//
// So what these tests hold is not "the note is right" — notes_next_test.go and
// notes_verb_screen_test.go already hold that, and this change does not touch
// the projection. What they hold is the GATE: that the in-place path is taken
// only where it is a true substitute for the rebuild, and that every refusal
// named in notes_refresh_apple.go really refuses. A gate that quietly stopped
// refusing would paint a sticker and a wash onto the wrong chapter's text —
// the exact failure the rebuild cannot have, and the reason this is worth a
// test rather than a comment.

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2/test"
)

// paneHolds points the Apple push bookkeeping at the chapter the state is on,
// i.e. "the native view currently displays this text" — the precondition the
// in-place path exists for. Restores whatever was there, because these are
// process-wide vars the other tests share.
func paneHolds(t *testing.T, st *AppState) {
	t.Helper()
	bc, body, tint := lastPushedBookChapter, lastPushedBodyFP, lastPushedTintFP
	t.Cleanup(func() {
		lastPushedBookChapter, lastPushedBodyFP, lastPushedTintFP = bc, body, tint
	})
	lastPushedBookChapter = fmt.Sprintf("%s|%d", st.CurrentBook, st.CurrentChapter)
	lastPushedBodyFP = chapterBodyFingerprint(st)
	lastPushedTintFP = chapterTint(st).fingerprint()
	st.restore = nil
}

func TestNoteRefreshStaysInPlaceWhenThePaneAlreadyHoldsTheChapter(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())

	st, _ := nextTestState(t)
	paneHolds(t, st)

	if !refreshNoteInPlace(st) {
		t.Fatal("the in-place note refresh refused a pane that holds this very chapter.\n" +
			"That returns every note verb to the whole-tree rebuild, and with it the\n" +
			"~60ms misplaced-pane flash this path exists to avoid.")
	}
}

// Each refusal, one at a time, from the same known-good starting point — so a
// failure names WHICH gate stopped answering rather than just "it refused".
func TestNoteRefreshFallsBackToTheRebuildWhenItMust(t *testing.T) {
	for _, tc := range []struct {
		name string
		// breakIt runs AFTER the pane is pinned: the case is a change.
		breakIt func(t *testing.T, st *AppState)
		// beforePin runs BEFORE it: the case is a standing condition the
		// fingerprints have already recorded, so nothing else can catch it.
		beforePin func(st *AppState)
		why       string
	}{
		{
			name:    "a different chapter is on screen",
			breakIt: func(_ *testing.T, _ *AppState) { lastPushedBookChapter = "John|4" },
			why: "the sticker and the wash would be painted onto the wrong scripture — " +
				"a note anchored to a verse that is not displayed, over a highlight range " +
				"that means something else there",
		},
		{
			name:    "the body on screen is stale",
			breakIt: func(_ *testing.T, _ *AppState) { lastPushedBodyFP = "stale" },
			why: "the chapter's TEXT has changed (theme, red-letter, a background data " +
				"swap) and only the re-import repairs that; mutating over it would leave " +
				"the old decode on screen",
		},
		{
			name:    "a scroll restore is pending",
			breakIt: func(_ *testing.T, st *AppState) { st.restore = &restoreAnchor{Book: "John", Chapter: 3, Verse: 16} },
			why: "the restore forces the slow path precisely so the scroll lands where the " +
				"reader left off; swallowing it here would strand the position",
		},
		{
			// SET BEFORE paneHolds, and that is the whole point of the case.
			// The notice is folded into chapterBodyFingerprint, so RAISING one
			// is caught by the body guard on its way past. What this has to
			// catch is a notice ALREADY STANDING when a later verb arrives:
			// the body has not moved, every other gate is open, and the only
			// thing left saying "the tree is carrying note state" is this one.
			name:      "a notice banner is already standing",
			beforePin: func(st *AppState) { st.NoteNotice = noteDamagedMessage },
			why: "the notice is the ONE thing the Fyne tree still says about notes on these " +
				"panes (buildNoteBanner's plan.Notice branch runs BEFORE the native-sticker " +
				"suppression), so the tree is carrying note state and only a rebuild can change it",
		},
		{
			name:    "the styled pane is the surface (darwin mimic dev mode)",
			breakIt: func(t *testing.T, _ *AppState) { withStyledPane(t) },
			why: "there the note is a Fyne band with reserved height inside the tree — a " +
				"selection change is a LAYOUT change",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := test.NewApp()
			defer app.Quit()
			defer deleteAllNotes(appPrefs())

			st, _ := nextTestState(t)
			paneHolds(t, st)
			if !refreshNoteInPlace(st) {
				t.Fatal("setup is wrong: the in-place path was already refused before the case applied")
			}

			st2, _ := nextTestState(t)
			if tc.beforePin != nil {
				tc.beforePin(st2)
			}
			paneHolds(t, st2)
			if tc.breakIt != nil {
				tc.breakIt(t, st2)
			}
			if refreshNoteInPlace(st2) {
				t.Errorf("the in-place note refresh was taken although %s.\n%s\n"+
					"Falling back to state.refreshReadingOnly() here is not a compromise — "+
					"it is the correct answer.", tc.name, tc.why)
			}
		})
	}
}

// withStyledPane points the surface selector at the styled pane for one test.
func withStyledPane(t *testing.T) {
	t.Helper()
	orig := useStyledPane
	t.Cleanup(func() { useStyledPane = orig })
	useStyledPane = func() bool { return true }
}

// THE PLACEMENT MUST NOT BE DROPPED. forceReposition means "place the view",
// which the rebuild's own fast path honours with a scroll rather than a
// re-import. If the in-place path took the flag's cost without doing its work,
// "Go to John 3:16" onto the chapter already open would light the verse and
// never move — the exact defect notes_reposition_test.go was written for, back
// through a new door.
func TestInPlaceNoteRefreshStillPlacesTheView(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())

	st, _ := nextTestState(t)
	paneHolds(t, st)
	st.forceReposition = true

	if !refreshNoteInPlace(st) {
		t.Fatal("the in-place path refused a pane that holds this chapter")
	}
	if st.forceReposition {
		t.Error("forceReposition survived the in-place note refresh — the flag is a one-shot, " +
			"and leaving it set means the NEXT unrelated push scrolls the reader somewhere " +
			"they did not ask to go")
	}
}

// THE WASH RECORD MUST TRACK WHAT WAS PAINTED. lastPushedTintFP is how the next
// rebuild decides whether the wash on screen is current. If the in-place path
// painted a new wash and left the record behind, the following rebuild would
// look at an unchanged fingerprint and skip a wash the reader can see is wrong;
// if it moved the record without painting, the same in reverse.
func TestInPlaceNoteRefreshRecordsTheWashItPainted(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())

	st, _ := nextTestState(t)
	paneHolds(t, st)
	lastPushedTintFP = "deliberately stale"

	if !refreshNoteInPlace(st) {
		t.Fatal("the in-place path refused a pane that holds this chapter")
	}
	if want := chapterTint(st).fingerprint(); lastPushedTintFP != want {
		t.Errorf("after the in-place refresh the wash record is %q, want %q.\n"+
			"The record is what the next rebuild's gate reads; out of step with the pane "+
			"it either skips a wash the reader needs or repaints one they already have.",
			lastPushedTintFP, want)
	}
}
