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
	"fyne.io/fyne/v2/container"
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
// answering before loading, and retains that local run even when the
// Treasury cannot cover the verse. No unavailable Treasury section is mounted.
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
	for _, want := range []string{"NKJV REFERENCES", "Matthew 28:19"} {
		if !strings.Contains(all, want) {
			t.Errorf("the panel lacks %q: %q", want, texts)
		}
	}
	for _, absent := range []string{treasuryLabel, crossRefOfflineLine, crossRefOfflineMessage, "Try again"} {
		if strings.Contains(all, absent) {
			t.Errorf("unavailable Treasury section leaked %q over local content: %q", absent, texts)
		}
	}
	// CONTROL: without the edition's run the same panel is the coverage
	// answer, given before any load.
	withPublisherCrossRefs(t, "nkjv", false)
	texts, loaded = themedCoveragePanel(t, "nkjv", publisherNotedActs(), "Acts", 8, 37, 37)
	if loaded || !strings.Contains(strings.Join(texts, "\n"), publisherUncoveredLine) {
		t.Errorf("without the edition's run: loaded %v, texts %q; want the note before loading", loaded, texts)
	}
}

func findTapCardWithText(root fyne.CanvasObject, want string) *tapCard {
	var found *tapCard
	walkTree(root, func(o fyne.CanvasObject) {
		if found != nil {
			return
		}
		if tc, ok := o.(*tapCard); ok {
			for _, s := range textsIn(tc) {
				if s == want {
					found = tc
					return
				}
			}
		}
	})
	return found
}

func panelListParts(top fyne.CanvasObject) (*container.Scroll, *fyne.Container, *widget.ProgressBarInfinite) {
	var listScroll *container.Scroll
	var listBox *fyne.Container
	var bar *widget.ProgressBarInfinite
	walkTree(top, func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *container.Scroll:
			if listScroll == nil {
				listScroll = x
			}
		case *container.ThemeOverride:
			if box, ok := x.Content.(*fyne.Container); ok && listBox == nil && listScroll != nil {
				listBox = box
			}
		case *widget.ProgressBarInfinite:
			if bar == nil {
				bar = x
			}
		}
	})
	return listScroll, listBox, bar
}

// THE EDITION'S NOTES AND EMBEDDED PARALLELS SHOW BEFORE THE TREASURY LANDS.
// While the Treasury downloads for the first time, the panel shows the
// publisher's notes and embedded Gospel parallels immediately, with the
// spinner scoped under TREASURY OF SCRIPTURE KNOWLEDGE; reader folds and
// scroll offset survive both an offline failure, an inline retry, and the
// eventual arrival of the Treasury rows.
func TestPublisherNotesShowImmediatelyWhileTreasuryLoads(t *testing.T) {
	app := themedTestApp()
	t.Cleanup(app.Quit)
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	withCrossRefIndex(t, "")

	var pending func()
	loads := 0
	prevRun, prevLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = prevRun, prevLoad })
	crossRefsRun = func(work func()) { pending = work }
	crossRefsLoad = func() error {
		loads++
		if loads == 1 {
			return errOfflineForTest
		}
		idx, _, err := parseCrossRefRows(strings.NewReader("From Verse\tTo Verse\tVotes\nMatt.3.1\tRom.5.8\t10\n"))
		if err != nil {
			t.Fatal(err)
		}
		crossRefMu.Lock()
		crossRefIndex = idx
		crossRefMu.Unlock()
		return nil
	}

	bd := publisherFixture()
	bd.Verses["Matthew"][3][0].Footnotes = []Footnote{
		{Anchor: 14, Text: "John 7:50", Kind: footnoteKindCrossref, Caller: "-", Refs: []NoteRef{{"JHN.7.50", 0, 9}}},
		{Anchor: 30, Text: "(Gal. 6:15; Eph. 2:4)", Kind: footnoteKindCrossref, Caller: "-", Refs: []NoteRef{{"GAL.6.15", 1, 10}}},
	}
	bd.Verses["Mark"][1] = []Verse{
		{BookName: "Mark", Book: "Mark", Chapter: 1, Verse: 1, Text: "The beginning of the fixture gospel."},
		{BookName: "Mark", Book: "Mark", Chapter: 1, Verse: 6, Text: "Clothed in camel's hair in the fixture text."},
	}
	bd.PrepareSearchIndex()

	limit := aiPanelSize(fyne.NewSize(375, 667)).Width - crossRefListChrome
	w := test.NewWindow(widget.NewLabel("reading"))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(402, 874))
	st := &AppState{Bible: bd, CurrentBook: "Matthew", CurrentChapter: 3, CurrentVersion: "nkjv", window: w}

	showCrossRefs(st, "", selSpan{lo: 1, hi: 1})
	if pending == nil {
		t.Fatal("the panel did not start the background Treasury load")
	}
	top := w.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("the panel did not open")
	}

	// 1. Before pending() runs: NKJV notes, Gospel parallels, and the inline
	// Treasury waiting state are all mounted in the scrollable list.
	listScroll, listBox, bar := panelListParts(top)
	if listScroll == nil || listBox == nil || len(listBox.Objects) == 0 {
		t.Fatalf("the scrollable list is not mounted while waiting for the Treasury; texts %q", treeTexts(top))
	}
	if bar == nil || !bar.Running() {
		t.Fatalf("the inline Treasury progress bar is not running; bar=%v", bar)
	}
	all := strings.Join(treeTexts(top), "\n")
	for _, want := range []string{"NKJV REFERENCES", "John 7:50", "GOSPEL PARALLELS", "Mark 1:1–6", "TREASURY OF SCRIPTURE KNOWLEDGE", "Finding related passages…", parallelsCredit} {
		if !strings.Contains(all, want) {
			t.Errorf("while Treasury loads, panel lacks %q:\n%s", want, all)
		}
	}
	if strings.Contains(all, "OpenBible.info") {
		t.Errorf("footer credits OpenBible.info before the Treasury loaded:\n%s", all)
	}
	if misfits := crossRefListMisfits(listBox.Objects, limit); len(misfits) != 0 {
		t.Errorf("loading list misfits narrowest phone: %v", misfits)
	}

	// Open the parenthesised group and scroll while the Treasury is still in flight.
	groups := groupRowsIn(listBox.Objects)
	if len(groups) != 1 || groups[0].open {
		t.Fatalf("expected 1 closed group row, got %d", len(groups))
	}
	groups[0].toggle()
	listScroll.Offset = fyne.NewPos(0, 64)
	listScroll.Refresh()
	if listScroll.OnScrolled != nil {
		listScroll.OnScrolled(listScroll.Offset)
	}

	// 2. First load fails offline: folds and scroll stay, inline Try again appears.
	work := pending
	pending = nil
	work()
	if bar.Running() {
		t.Error("the first progress bar kept running after the load failed")
	}
	listScroll, listBox, _ = panelListParts(top)
	if misfits := crossRefListMisfits(listBox.Objects, limit); len(misfits) != 0 {
		t.Errorf("offline list misfits narrowest phone: %v", misfits)
	}
	groups = groupRowsIn(listBox.Objects)
	if len(groups) != 1 || !groups[0].open {
		t.Errorf("the group fold did not survive the Treasury landing: open=%v", len(groups) == 1 && groups[0].open)
	}
	if listScroll.Offset.Y != 64 {
		t.Errorf("scroll offset reset on Treasury failure: got %v, want 64", listScroll.Offset.Y)
	}
	all = strings.Join(treeTexts(top), "\n")
	if !strings.Contains(all, crossRefOfflineLine) || strings.Contains(all, "Finding related passages…") || strings.Contains(all, "OpenBible.info") {
		t.Errorf("unexpected offline Treasury state:\n%s", all)
	}
	retry := findTapCardWithText(listBox, "Try again")
	if retry == nil {
		t.Fatalf("no inline Try again tapCard under the Treasury heading:\n%s", all)
	}

	// 3. Tap inline Try again: spins in place without losing the open group or scroll, then lands Treasury rows.
	test.Tap(retry)
	if pending == nil {
		t.Fatal("tapping inline Try again did not start a retry load")
	}
	listScroll, listBox, bar = panelListParts(top)
	if bar == nil || !bar.Running() {
		t.Fatal("inline retry did not start a progress bar under the Treasury heading")
	}
	groups = groupRowsIn(listBox.Objects)
	if len(groups) != 1 || !groups[0].open || listScroll.Offset.Y != 64 {
		t.Errorf("inline retry lost reader state while loading: open=%v, scrollY=%v", len(groups) == 1 && groups[0].open, listScroll.Offset.Y)
	}
	work = pending
	pending = nil
	work()
	if bar.Running() {
		t.Error("the retry progress bar kept running after the Treasury landed")
	}
	listScroll, listBox, _ = panelListParts(top)
	groups = groupRowsIn(listBox.Objects)
	if len(groups) != 1 || !groups[0].open || listScroll.Offset.Y != 64 {
		t.Errorf("Treasury landing lost reader state: open=%v, scrollY=%v", len(groups) == 1 && groups[0].open, listScroll.Offset.Y)
	}
	all = strings.Join(treeTexts(top), "\n")
	for _, want := range []string{"Romans 5:8", treasuryCredit, parallelsCredit} {
		if !strings.Contains(all, want) {
			t.Errorf("after inline retry succeeded, panel lacks %q:\n%s", want, all)
		}
	}
	if strings.Contains(all, crossRefOfflineLine) || findTapCardWithText(listBox, "Try again") != nil {
		t.Errorf("offline state outlived a successful inline retry:\n%s", all)
	}

	// CONTROL 1: a selection with no local content (Psalms 3:2) still uses the
	// centered spinner rather than showing an empty publisher line over a spinner.
	closeBtn := findTreeButton(top, "Close")
	test.Tap(closeBtn)
	st.CurrentBook, st.CurrentChapter = "Psalms", 3
	pending = nil
	showCrossRefs(st, "", selSpan{lo: 2, hi: 2})
	top2 := w.Canvas().Overlays().Top()
	scroll2, box2, bar2 := panelListParts(top2)
	if scroll2 != nil && box2 != nil && len(box2.Objects) > 0 {
		t.Errorf("selection with no local content mounted a list instead of centered spinner: %q", treeTexts(top2))
	}
	if bar2 == nil || !bar2.Running() || strings.Contains(strings.Join(treeTexts(top2), "\n"), "No NKJV cross-references") {
		t.Errorf("selection with no local content did not show centered spinner cleanly: %q", treeTexts(top2))
	}

	// CONTROL 2: closing the panel mid-load stops the spinner and ignores a late completion.
	test.Tap(findTreeButton(top2, "Close"))
	if bar2.Running() {
		t.Error("closing the panel mid-load left the progress bar running")
	}
	if pending != nil {
		pending()
	}
	if top2.Visible() || strings.Contains(strings.Join(treeTexts(top2), "\n"), crossRefNoneLine) {
		t.Error("a late load completion mutated or re-showed a closed panel")
	}
}

// TRY AGAIN ASKS FOR THE TREASURY AGAIN. The panel's connection error carries
// the verb the AI panel's does, and the verb is honest: the loader forgets a
// failure (ensureCrossRefs), so the tap is a second fetch, and the list lands
// in the error's place when it succeeds, with the footer naming the set that
// is now on screen.
func TestTryAgainAsksForTheTreasuryAgain(t *testing.T) {
	th := appThemeForTest(t)
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
	st := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3, CurrentVersion: "web", window: w, theme: th}
	showCrossRefs(st, "", selSpan{lo: 16, hi: 16})
	top := w.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("the panel did not open")
	}
	if !treeHasText(top, crossRefOfflineMessage) {
		t.Fatalf("offline, the panel does not show the connection error; texts %q", treeTexts(top))
	}
	if treeHasText(top, treasuryCredit) {
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
	if !treeHasText(top, treasuryCredit) {
		t.Errorf("the footer does not name the Treasury once its rows are on screen; texts %q", treeTexts(top))
	}
	// CONTROL: the whole-panel error is the one state with the verb; a Treasury
	// that fails under a list with content shows no button (crossref_list_test.go).
	if findTreeButton(top, "Try again") != nil {
		t.Error("Try again outlived the error state")
	}
}
