package bibletext

// Clearing a foreign mark must bring the note it was suppressing back WHOLE —
// bubble AND wash. The clear verbs (the native "Clear highlight" tap and the
// back-to-results bar's X and Back) release the suppression just by clearing —
// the plan derives Open again at the next render — but setMark had REPLACED
// the note's own mark with the foreign one, and only the projection re-raises
// hlNote. The bare clear therefore re-opened the bubble over an unwashed verse
// until the next navigation re-derived: bubble on John 3:16, v16 not
// highlighted. These pin clearHighlightAndRederive and the two Fyne controls
// that end on it (the native tap's export is cgo and shares the same verb).

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// suppressedNoteState seeds one note on John 3:16, projects it, then lands a
// foreign Go-to mark on the same chapter so the plan stands the note down.
func suppressedNoteState(t *testing.T) *AppState {
	t.Helper()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	if _, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "John", Chapter: 3, VerseLo: 16, Text: "the note"}); !ok {
		t.Fatal("seeding the note failed")
	}
	st := planTestState(t)
	addRecentChapter(st, "John", 3)
	if !st.mark.fromNote() {
		t.Fatal("precondition: the note should hold its own mark")
	}
	goToVerseRange(st, "John", 3, 1, 1)
	if !notesSuppressed(st) {
		t.Fatal("precondition: the foreign mark should stand the note down")
	}
	return st
}

func assertNoteRestoredWhole(t *testing.T, st *AppState) {
	t.Helper()
	if notesSuppressed(st) {
		t.Error("the suppression must release with the clear")
	}
	if !st.mark.fromNote() {
		t.Error("the note's own wash must be re-raised — the foreign mark had replaced it, " +
			"and only the projection puts it back")
	}
	if sp, ok := st.markSpan(); !ok || sp.Lo != 16 {
		t.Errorf("the wash must sit on the note's verse 16, got %+v (ok=%v)", sp, ok)
	}
	text, _, pill, _ := appleStickerPush(st, buildChapterPlan(st, appPrefs(), st.Bible))
	if text != "the note" || pill {
		t.Errorf("the pushed sticker must re-open with its note, got text=%q pill=%v", text, pill)
	}
}

// The shared verb itself — what bibleTextHighlightCleared runs on the Fyne
// goroutine after the native "Clear highlight" tap.
func TestClearingAForeignMarkRestoresTheNoteWash(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())
	st := suppressedNoteState(t)

	clearHighlightAndRederive(st)

	assertNoteRestoredWhole(t, st)
}

// The trail's X — the every-platform twin of the native clear.
func TestBackToResultsDismissRestoresTheNoteWash(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())
	st := suppressedNoteState(t)
	st.CanReturnToSearchResults = true

	bar := backToResultsBar(st)
	var clear *widget.Button
	walkTree(bar, func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && b.Text == "" && clear == nil &&
			b.Icon != nil && b.Icon.Name() == theme.CancelIcon().Name() {
			clear = b
		}
	})
	if clear == nil {
		t.Fatal("no X on the back-to-results bar")
	}
	test.Tap(clear)

	if st.CanReturnToSearchResults {
		t.Error("the X must dismiss the trail")
	}
	assertNoteRestoredWhole(t, st)
}

// RELEASE GIVES BACK WHAT SUPPRESSION TOOK, NEVER MORE (owner, 2026-08-19:
// "when I X the back to results a minimized note maximizes for some reason").
// The three tests above are the July rule's half — the note WAS open when the
// foreign mark arrived, so clearing re-opens it. This is the other half: the
// reader arrives on a chapter from ELSEWHERE (a search result), so its note
// was never open under their eyes — it arrived suppressed, a pill from the
// first frame — and clearing the mark must leave it a pill. The reader's own
// tap is what opens it.
func TestClearOpensNothingTheSuppressionNeverClosed(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	if _, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "John", Chapter: 3, VerseLo: 16, Text: "the note"}); !ok {
		t.Fatal("seeding the note failed")
	}
	st := planTestState(t)
	st.CurrentBook, st.CurrentChapter = "Psalms", 23
	addRecentChapter(st, "Psalms", 23)

	// A search result carries the reader onto John 3: the arrival's mark is
	// foreign, so the chapter's note arrives suppressed — the reader has only
	// ever seen the pill.
	openSearchResultRange(st, Verse{BookName: "John", Chapter: 3, Verse: 1}, 0)
	if !notesSuppressed(st) {
		t.Fatal("precondition: the arrival's mark should stand the note down")
	}

	clearHighlightAndRederive(st)

	if notesSuppressed(st) {
		t.Error("the suppression must release with the clear")
	}
	text, _, pill, _ := appleStickerPush(st, buildChapterPlan(st, appPrefs(), st.Bible))
	if text == "" {
		t.Fatal("the note must still be present (as its pill) after the clear")
	}
	if !pill {
		t.Error("the clear opened a note the suppression never closed — the owner's " +
			"\"a minimized note maximizes\", pinned")
	}
	if st.hasMark() {
		t.Errorf("a closed note carries no wash (Hide's rule), got %+v", st.mark)
	}
	if plan := buildChapterPlan(st, appPrefs(), st.Bible); func() bool {
		_, open := plan.openNote()
		return open
	}() {
		t.Error("the Fyne banner would draw the note OPEN — the plan must keep it a chip")
	}

	// The reader's own tap is the Show verb, and it opens the note whole —
	// the pill is a closed door, not a locked one.
	restoreCurrentNote(st)
	if _, _, pill, _ := appleStickerPush(st, buildChapterPlan(st, appPrefs(), st.Bible)); pill {
		t.Error("the pill press must open the note")
	}
	if !st.mark.fromNote() {
		t.Error("the opened note must bring its own wash back")
	}
}

// The same rule through the bar's X — the control the owner was pressing.
func TestBackToResultsDismissLeavesAnUnseenNoteClosed(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	if _, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "John", Chapter: 3, VerseLo: 16, Text: "the note"}); !ok {
		t.Fatal("seeding the note failed")
	}
	st := planTestState(t)
	st.CurrentBook, st.CurrentChapter = "Psalms", 23
	addRecentChapter(st, "Psalms", 23)
	openSearchResultRange(st, Verse{BookName: "John", Chapter: 3, Verse: 1}, 0)
	if !st.CanReturnToSearchResults || !notesSuppressed(st) {
		t.Fatal("precondition: a search arrival with the trail up and the note suppressed")
	}

	bar := backToResultsBar(st)
	var clear *widget.Button
	walkTree(bar, func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && b.Text == "" && clear == nil &&
			b.Icon != nil && b.Icon.Name() == theme.CancelIcon().Name() {
			clear = b
		}
	})
	if clear == nil {
		t.Fatal("no X on the back-to-results bar")
	}
	test.Tap(clear)

	if st.CanReturnToSearchResults {
		t.Error("the X must dismiss the trail")
	}
	if _, _, pill, _ := appleStickerPush(st, buildChapterPlan(st, appPrefs(), st.Bible)); !pill {
		t.Error("the X expanded a note the reader had only ever seen as a pill")
	}
}

// The Back button clears the same mark on its way to the results, so returning
// to the reading pane later without navigating (mobile's Read tab) must land
// on the same restored-whole note.
func TestBackToResultsButtonRestoresTheNoteWash(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())
	st := suppressedNoteState(t)
	st.CanReturnToSearchResults = true

	bar := backToResultsBar(st)
	var back *widget.Button
	walkTree(bar, func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && b.Text != "" && back == nil {
			back = b
		}
	})
	if back == nil {
		t.Fatal("no Back button on the bar")
	}
	test.Tap(back)

	if !st.IsSearching {
		t.Error("desktop Back must hand the pane to the results")
	}
	assertNoteRestoredWhole(t, st)
}
