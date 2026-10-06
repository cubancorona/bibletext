//go:build !next

package bibletext

// The cross-references panel in the shipping build says what 1.2.19's does of
// a selection the Treasury cannot cover: it asks for the Treasury first, says
// "No cross-references for this selection." alone when nothing comes back,
// and offline says to check the connection. Saying why, before anything loads,
// is the next major release's (crossref_coverage_next_test.go, docs/NEXT.md).

import (
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// Offline, a selection the Treasury cannot cover still waits for the Treasury
// and reports the connection, as 1.2.19 does.
func TestTheShippingPanelAsksForTheTreasuryFirst(t *testing.T) {
	const offline = "Couldn't load cross-references.\nCheck your connection and try again."
	for _, tc := range []struct {
		name    string
		book    string
		chapter int
		lo, hi  int
	}{
		{"a Greek addition", "Esther", 4, 20, 20},
		{"the deuterocanon", "Tobit", 1, 1, 1},
		// CONTROL: a verse the Treasury does cover, as in the next release.
		{"a Hebrew verse", "Esther", 4, 17, 17},
	} {
		t.Run(tc.name, func(t *testing.T) {
			texts, loaded := coveragePanel(t, tc.book, tc.chapter, tc.lo, tc.hi)
			if !loaded {
				t.Error("the panel answered without asking for the Treasury")
			}
			if !slices.Contains(texts, offline) {
				t.Errorf("the panel does not show the offline message; texts %q", texts)
			}
		})
	}
}

// Online, an empty panel says the first line and nothing more, and the list
// carries no coverage note.
func TestTheShippingPanelSaysTheFirstLineAlone(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	prevRun, prevLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = prevRun, prevLoad })
	crossRefsRun = func(work func()) { work() }
	crossRefsLoad = func() error { return nil }
	withCrossRefIndex(t, "")
	w := test.NewWindow(widget.NewLabel("reading"))
	defer w.Close()
	w.Resize(fyne.NewSize(800, 700))
	for _, tc := range []struct {
		book    string
		chapter int
		lo, hi  int
	}{
		{"Esther", 4, 17, 18},
		{"Esther", 4, 20, 20},
		{"Tobit", 1, 1, 1},
	} {
		st := &AppState{Bible: greekEstherBible(), CurrentBook: tc.book, CurrentChapter: tc.chapter, CurrentVersion: "webc", window: w}
		span := selSpan{lo: tc.lo, hi: tc.hi}
		lst := buildCrossRefList(st, selectionVerses(st, "", span), crossRefsForSelection(st, "", span), nil, st.pal(), func(crossRef) {})
		if lst.CoverageNote != "" || len(lst.Objects) != 0 {
			t.Errorf("%s %d:%d-%d: the list carries note %q and %d objects, want none",
				tc.book, tc.chapter, tc.lo, tc.hi, lst.CoverageNote, len(lst.Objects))
		}
		showCrossRefs(st, "", span)
		top := w.Canvas().Overlays().Top()
		if top == nil {
			t.Fatalf("%s %d:%d-%d: the panel did not open", tc.book, tc.chapter, tc.lo, tc.hi)
		}
		texts := treeTexts(top)
		if !slices.Contains(texts, crossRefNoneLine) {
			t.Errorf("%s %d:%d-%d: the panel does not say %q alone; texts %q", tc.book, tc.chapter, tc.lo, tc.hi, crossRefNoneLine, texts)
		}
		for _, s := range texts {
			if strings.Contains(s, "cross-references don't cover") {
				t.Errorf("%s %d:%d-%d: the shipping panel says %q", tc.book, tc.chapter, tc.lo, tc.hi, s)
			}
		}
		w.Canvas().Overlays().Remove(top)
	}
}
