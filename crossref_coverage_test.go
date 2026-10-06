package bibletext

// What the cross-references panel says of a selection the Treasury cannot
// cover. The sentences and the helpers hold in both states of the next
// switch; the panel says them only in the next major release
// (crossref_coverage_next_test.go), and the shipping build's panel says what
// 1.2.19's does (crossref_coverage_current_test.go).

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// The sentences the panel shows for a selection the cross-references cannot
// cover, word for word. Reader-facing: a change here is a change a reader
// sees.
func TestTheCoverageSentencesArePinned(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{crossRefNoneLine, "No cross-references for this selection."},
		{crossRefDeuterocanonLine, "The cross-references don't cover the deuterocanonical books."},
		{crossRefUncoveredLine, "Nothing is listed for %s, which the cross-references don't cover."},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}

// greekEstherBible is WEB Catholic's Esther 4 as it numbers it — the Hebrew
// numbers less the 4:6 it has nothing at, then Mordecai's and Esther's
// prayers as 4:18-47 — with Tobit 1 and Daniel 3 beside it.
func greekEstherBible() *BibleData {
	bd := xrefBible(map[string]map[int]int{"Esther": {4: 47}, "Tobit": {1: 22}, "Daniel": {3: 97}})
	bd.Verses["Esther"][4] = append(bd.Verses["Esther"][4][:5:5], bd.Verses["Esther"][4][6:]...)
	return bd
}

func TestVerseRunsPhrase(t *testing.T) {
	for _, tc := range []struct {
		verses []int
		want   string
	}{
		{[]int{20}, "verse 4:20"},
		{[]int{18, 19}, "verses 4:18–19"},
		{[]int{18, 19, 20, 25}, "verses 4:18–20 and 4:25"},
		{[]int{18, 20, 22, 23}, "verses 4:18, 4:20 and 4:22–23"},
	} {
		if got := verseRunsPhrase(4, tc.verses); got != tc.want {
			t.Errorf("%v: got %q, want %q", tc.verses, got, tc.want)
		}
	}
}

// coveragePanel opens the panel over one selection of greekEstherBible in the
// WEB Catholic, offline: the Treasury's load fails, and runs where the test
// stands. It returns the panel's texts and whether the load was asked for.
func coveragePanel(t *testing.T, book string, chapter, lo, hi int) (texts []string, loaded bool) {
	t.Helper()
	app := test.NewApp()
	t.Cleanup(app.Quit)
	prevRun, prevLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = prevRun, prevLoad })
	crossRefsRun = func(work func()) { work() }
	crossRefsLoad = func() error { loaded = true; return errOfflineForTest }
	withCrossRefIndex(t, "")

	w := test.NewWindow(widget.NewLabel("reading"))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(800, 700))
	st := &AppState{Bible: greekEstherBible(), CurrentBook: book, CurrentChapter: chapter, CurrentVersion: "webc", window: w}
	showCrossRefs(st, "", selSpan{lo: lo, hi: hi})
	top := w.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("the panel did not open")
	}
	return treeTexts(top), loaded
}

// The deuterocanon sentence is true only while the Treasury holds the 66
// books and none of the seven: every book of the 66-book canon is one it can
// name, and no deuterocanonical book is.
func TestTheTreasuryHoldsTheSixtySixBooksAndNoOthers(t *testing.T) {
	for book := range protestantCanonBooks {
		if _, ok := tskBookNumbers[book]; !ok {
			t.Errorf("the Treasury cannot name %s", book)
		}
	}
	for _, book := range []string{"Tobit", "Judith", "Wisdom", "Sirach", "Baruch", "1 Maccabees", "2 Maccabees"} {
		if protestantCanonBooks[book] {
			t.Errorf("%s is counted in the 66-book canon", book)
		}
		if _, ok := tskBookNumbers[book]; ok {
			t.Errorf("the Treasury names %s; the deuterocanon sentence is no longer true", book)
		}
	}
	if len(protestantCanonBooks) != 66 || len(tskBookNumbers) != 66 {
		t.Errorf("%d books in the canon and %d in the Treasury, want 66 each", len(protestantCanonBooks), len(tskBookNumbers))
	}
}
