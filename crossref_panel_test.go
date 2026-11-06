package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// TRY AGAIN ASKS FOR THE TREASURY AGAIN. The panel's connection error carries
// the verb the AI panel's does, and the verb is honest: the loader forgets a
// failure (ensureCrossRefs), so the tap is a second fetch, and the list lands
// in the error's place when it succeeds, with the footer naming the set that
// is now on screen.
func TestTryAgainAsksForTheTreasuryAgain(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	withPublisherCrossRefs(t, "nkjv", false)
	prevRun, prevLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = prevRun, prevLoad })
	crossRefsRun = func(work func()) { work() }
	withCrossRefIndex(t, "") // nothing until the second load lands
	loads := 0
	crossRefsLoad = func() error {
		loads++
		if loads == 1 {
			return errOfflineForTest
		}
		idx, _, err := parseCrossRefRows(strings.NewReader("From Verse\tTo Verse\tVotes\nJohn.3.16\tRom.5.8\t10\n"))
		if err != nil {
			t.Fatal(err)
		}
		crossRefMu.Lock()
		crossRefIndex = idx
		crossRefMu.Unlock()
		return nil
	}

	bd := xrefBible(map[string]map[int]int{"John": {3: 36}, "Romans": {5: 21}})
	w := test.NewWindow(widget.NewLabel("reading"))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(800, 700))
	st := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3, CurrentVersion: "web", window: w}
	showCrossRefs(st, "", selSpan{lo: 16, hi: 16})
	top := w.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("the panel did not open")
	}
	if !treeHasText(top, crossRefOfflineMessage) {
		t.Fatalf("offline, the panel does not show the connection error; texts %q", treeTexts(top))
	}
	if treeHasText(top, tskCredit) {
		t.Errorf("the footer credits a Treasury that did not load; texts %q", treeTexts(top))
	}
	retry := findTreeButton(top, "Try again")
	if retry == nil {
		t.Fatalf("the error state has no Try again; texts %q", treeTexts(top))
	}
	test.Tap(retry)
	if loads != 2 {
		t.Errorf("Try again asked for the Treasury %d times in all, want 2", loads)
	}
	if !treeHasText(top, "Romans 5:8") || treeHasText(top, crossRefOfflineMessage) {
		t.Errorf("after Try again the list did not take the error's place; texts %q", treeTexts(top))
	}
	// The footer names the set now on screen.
	if !treeHasText(top, tskCredit) {
		t.Errorf("the footer does not name the Treasury once its rows are on screen; texts %q", treeTexts(top))
	}
	// CONTROL: the whole-panel error is the one state with the verb; a Treasury
	// that fails under a list with content shows no button (crossref_list_test.go).
	if findTreeButton(top, "Try again") != nil {
		t.Error("Try again outlived the error state")
	}
}
