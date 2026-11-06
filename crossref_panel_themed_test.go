//go:build !race

package bibletext

// The panel opened with the publisher's run on, under the app's own theme.
// The run's section labels are bold monospace, a face the toolkit's test
// theme does not carry (its font map stops at bold, italic and monospace
// alone), so laying the run out under that theme is not possible; the app's
// theme answers for every style. Setting a theme is kept out of the race run
// (widgettree_test.go), which is why these stand apart from
// crossref_coverage_test.go.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// themedCoveragePanel is coveragePanelIn under the app's theme: the panel over
// one selection of bd read as version, offline — the Treasury's load fails,
// and runs where the test stands. It returns the panel's texts and whether the
// load was asked for.
func themedCoveragePanel(t *testing.T, version string, bd *BibleData, book string, chapter, lo, hi int) (texts []string, loaded bool) {
	t.Helper()
	app := themedTestApp()
	t.Cleanup(app.Quit)
	prevRun, prevLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = prevRun, prevLoad })
	crossRefsRun = func(work func()) { work() }
	crossRefsLoad = func() error { loaded = true; return errOfflineForTest }
	withCrossRefIndex(t, "")

	w := test.NewWindow(widget.NewLabel("reading"))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(800, 700))
	st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: chapter, CurrentVersion: version, window: w}
	showCrossRefs(st, "", selSpan{lo: lo, hi: hi})
	top := w.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("the panel did not open")
	}
	return treeTexts(top), loaded
}

// The panel over an edition whose own apparatus it shows loads rather than
// answering before loading, and shows the run — offline, with the Treasury's
// label and its one line beneath — for a verse the Treasury cannot cover.
func TestAPublisherRunIsNotAnsweredForTheTreasuryOnThePanel(t *testing.T) {
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	texts, loaded := themedCoveragePanel(t, "nkjv", publisherNotedActs(), "Acts", 8, 37, 37)
	if !loaded {
		t.Error("the panel answered before loading, over the edition's own run")
	}
	all := strings.Join(texts, "\n")
	if strings.Contains(all, publisherUncoveredLine) {
		t.Errorf("the panel says the selection is not covered over the edition's own run: %q", texts)
	}
	for _, want := range []string{"NKJV REFERENCES", "Matthew 28:19", "TREASURY OF SCRIPTURE KNOWLEDGE", crossRefOfflineLine} {
		if !strings.Contains(all, want) {
			t.Errorf("the panel lacks %q: %q", want, texts)
		}
	}
	if strings.Contains(all, crossRefOfflineMessage) {
		t.Errorf("the whole-panel error stands over a run with content: %q", texts)
	}
	// CONTROL: without the edition's run the same panel is the coverage
	// answer, given before any load.
	withPublisherCrossRefs(t, "nkjv", false)
	texts, loaded = themedCoveragePanel(t, "nkjv", publisherNotedActs(), "Acts", 8, 37, 37)
	if loaded || !strings.Contains(strings.Join(texts, "\n"), publisherUncoveredLine) {
		t.Errorf("without the edition's run: loaded %v, texts %q; want the note before loading", loaded, texts)
	}
}
