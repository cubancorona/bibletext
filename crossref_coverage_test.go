package bibletext

// What the cross-references panel says of a selection the Treasury cannot
// cover: why it lists nothing, before it loads anything when it can cover
// none of it.

import (
	"strings"
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
// prayers as 4:18-47 — with Tobit 1 and its Daniel 3, 13 and 14 beside it.
func greekEstherBible() *BibleData {
	bd := xrefBible(map[string]map[int]int{"Esther": {4: 47}, "Tobit": {1: 22}, "Daniel": {3: 97, 13: 64, 14: 42}})
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
	return coveragePanelIn(t, "webc", greekEstherBible(), book, chapter, lo, hi)
}

// coveragePanelIn is coveragePanel over bd read as version.
func coveragePanelIn(t *testing.T, version string, bd *BibleData, book string, chapter, lo, hi int) (texts []string, loaded bool) {
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
	st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: chapter, CurrentVersion: version, window: w}
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

// addedVersesBible is the NKJV's Acts 8 and Luke 17 as it numbers them, with
// the verses the reference does not have (Acts 8:37, Luke 17:36), and
// Matthew 28, which a publisher's note on Acts 8:37 points into.
func addedVersesBible() *BibleData {
	return xrefBible(map[string]map[int]int{"Acts": {8: 40}, "Luke": {17: 37}, "Matthew": {28: 20}})
}

// WHAT THE CROSS-REFERENCES CANNOT COVER IS SAID, NOT LEFT BLANK. The Treasury
// is keyed by the reference's numbering and holds the 66 books, so every verse
// a translation adds that the reference does not have, and the
// deuterocanonical books, have no rows at all. One rule names every added
// verse: the Greek Esther's additions, the Song of the Three, Susanna, Bel
// and the NKJV's four.
func TestCrossRefCoverageNamesWhatTheTreasuryCannotCover(t *testing.T) {
	verses := func(bd *BibleData, book string, chapter, lo, hi int) []Verse {
		var out []Verse
		for _, v := range bd.GetChapter(book, chapter) {
			if v.Verse >= lo && v.Verse <= hi {
				out = append(out, v)
			}
		}
		return out
	}
	webc, nkjv := greekEstherBible(), addedVersesBible()
	for _, tc := range []struct {
		name, version   string
		bd              *BibleData
		book            string
		chapter, lo, hi int
		note            string
		none            bool
	}{
		{"one addition", "webc", webc, "Esther", 4, 20, 20,
			"Nothing is listed for verse 4:20, which the cross-references don't cover.", true},
		{"all the prayers", "webc", webc, "Esther", 4, 18, 47,
			"Nothing is listed for verses 4:18–47, which the cross-references don't cover.", true},
		{"partly covered", "webc", webc, "Esther", 4, 15, 20,
			"Nothing is listed for verses 4:18–20, which the cross-references don't cover.", false},
		{"a deuterocanonical book", "webc", webc, "Tobit", 1, 1, 3,
			"The cross-references don't cover the deuterocanonical books.", true},
		// The Hebrew numbers are covered, in the Greek Esther as anywhere.
		{"a Hebrew verse of the Greek Esther", "webc", webc, "Esther", 4, 14, 17, "", false},
		// CONTROL: the same numbers in the WEB are covered.
		{"the Hebrew Esther", "web", webc, "Esther", 4, 15, 17, "", false},
		// The Song of the Three, Daniel 3:24-90 in the WEB Catholic.
		{"the Song of the Three", "webc", webc, "Daniel", 3, 24, 30,
			"Nothing is listed for verses 3:24–30, which the cross-references don't cover.", true},
		{"the Song of the Three after a covered verse", "webc", webc, "Daniel", 3, 23, 24,
			"Nothing is listed for verse 3:24, which the cross-references don't cover.", false},
		// CONTROL: the WEB's 3:24-30, which the WEB Catholic prints as
		// 3:91-97, are covered there.
		{"the verses after the Song", "webc", webc, "Daniel", 3, 91, 97, "", false},
		{"Susanna", "webc", webc, "Daniel", 13, 1, 3,
			"Nothing is listed for verses 13:1–3, which the cross-references don't cover.", true},
		{"Bel", "webc", webc, "Daniel", 14, 1, 1,
			"Nothing is listed for verse 14:1, which the cross-references don't cover.", true},
		// The four verses the NKJV prints and the reference does not.
		{"the NKJV's Acts 8:37", "nkjv", nkjv, "Acts", 8, 37, 37,
			"Nothing is listed for verse 8:37, which the cross-references don't cover.", true},
		{"the NKJV's Acts 8:36-37", "nkjv", nkjv, "Acts", 8, 36, 37,
			"Nothing is listed for verse 8:37, which the cross-references don't cover.", false},
		{"the NKJV's Luke 17:36", "nkjv", nkjv, "Luke", 17, 36, 36,
			"Nothing is listed for verse 17:36, which the cross-references don't cover.", true},
		// CONTROL: the WEB has no Acts 8:37 to name; its 8:36 and 8:38 are
		// covered.
		{"the WEB's Acts 8", "web", nkjv, "Acts", 8, 36, 38, "", false},
	} {
		note, none := crossRefCoverage(tc.version, tc.book, verses(tc.bd, tc.book, tc.chapter, tc.lo, tc.hi))
		if note != tc.note || none != tc.none {
			t.Errorf("%s: got %q (none=%v), want %q (none=%v)", tc.name, note, none, tc.note, tc.none)
		}
	}
}

// AN OFFLINE READER IS NOT SENT TO RECONNECT FOR WHAT RECONNECTING CANNOT
// FIX. A selection the cross-references cannot cover is answered before the
// Treasury is asked for, so the panel never reaches its offline message.
func TestTheCoverageSentenceComesBeforeTheOfflineMessage(t *testing.T) {
	const offline = "Couldn't load cross-references.\nCheck your connection and try again."
	for _, tc := range []struct {
		name          string
		version       string
		bd            *BibleData
		book          string
		chapter       int
		lo, hi        int
		want          string
		wantLoadAsked bool
	}{
		{"a Greek addition", "webc", greekEstherBible(), "Esther", 4, 20, 20,
			"No cross-references for this selection.\nNothing is listed for verse 4:20, which the cross-references don't cover.", false},
		{"the deuterocanon", "webc", greekEstherBible(), "Tobit", 1, 1, 1,
			"No cross-references for this selection.\nThe cross-references don't cover the deuterocanonical books.", false},
		{"Susanna", "webc", greekEstherBible(), "Daniel", 13, 1, 2,
			"No cross-references for this selection.\nNothing is listed for verses 13:1–2, which the cross-references don't cover.", false},
		{"the NKJV's Acts 8:37", "nkjv", addedVersesBible(), "Acts", 8, 37, 37,
			"No cross-references for this selection.\nNothing is listed for verse 8:37, which the cross-references don't cover.", false},
		// CONTROL: a verse the Treasury does cover still needs it loaded,
		// and offline the panel says so.
		{"a Hebrew verse", "webc", greekEstherBible(), "Esther", 4, 17, 17, offline, true},
		// Partly covered: the covered verse needs the Treasury too.
		{"partly covered", "webc", greekEstherBible(), "Esther", 4, 17, 18, offline, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The NKJV without its own apparatus, as every store build has
			// it, whatever tags this build has; the build that shows it is
			// TestAPublisherBlockIsNotAnsweredForTheTreasury's.
			withPublisherCrossRefs(t, "nkjv", false)
			texts, loaded := coveragePanelIn(t, tc.version, tc.bd, tc.book, tc.chapter, tc.lo, tc.hi)
			found := false
			for _, s := range texts {
				found = found || s == tc.want
				if s == offline && tc.want != offline {
					t.Errorf("the panel tells the reader to check the connection: %q", s)
				}
			}
			if !found {
				t.Errorf("the panel does not show %q; texts %q", tc.want, texts)
			}
			if loaded != tc.wantLoadAsked {
				t.Errorf("the Treasury's load was asked for: %v, want %v", loaded, tc.wantLoadAsked)
			}
		})
	}
}

// A SELECTION THE CROSS-REFERENCES COVER IN PART lists what they have, under
// a note naming the verses they do not; and with nothing to list, the note is
// the empty panel's second line.
func TestAPartlyCoveredSelectionIsListedUnderTheNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	withCrossRefIndex(t, "Esth.4.16\tGen.1.1\t10\n")
	bd := greekEstherBible()
	bd.Books = append(bd.Books, "Genesis")
	bd.Verses["Genesis"] = map[int][]Verse{1: {{BookName: "Genesis", Book: "Genesis", Chapter: 1, Verse: 1, Text: "In the beginning."}}}
	st := &AppState{Bible: bd, CurrentBook: "Esther", CurrentChapter: 4, CurrentVersion: "webc"}
	const note = "Nothing is listed for verses 4:18–19, which the cross-references don't cover."

	span := selSpan{lo: 16, hi: 19}
	refs := crossRefsForSelection(st, "", span)
	lst := buildCrossRefList(st, selectionVerses(st, "", span), refs, nil, st.pal(), func(crossRef) {})
	if lst.TSKRows != 1 || len(lst.Objects) != 2 {
		t.Fatalf("want the note and one row, got %d rows in %d objects", lst.TSKRows, len(lst.Objects))
	}
	if got := treeTexts(lst.Objects[0]); len(got) != 1 || got[0] != note {
		t.Errorf("the list does not open with the note: %q", got)
	}
	if len(refs) != 1 || refs[0].label() != "Genesis 1:1" {
		t.Errorf("the covered verse's row is not the one listed: %v", refs)
	}

	// Nothing to list: the note is not a list of its own.
	span = selSpan{lo: 17, hi: 19}
	refs = crossRefsForSelection(st, "", span)
	lst = buildCrossRefList(st, selectionVerses(st, "", span), refs, nil, st.pal(), func(crossRef) {})
	if len(lst.Objects) != 0 || lst.CoverageNote != "Nothing is listed for verses 4:18–19, which the cross-references don't cover." {
		t.Errorf("an empty list carries %d objects and note %q", len(lst.Objects), lst.CoverageNote)
	}

	// CONTROL: in the WEB the same numbers are the Hebrew Esther's, all
	// covered, and the list is the row alone.
	st.CurrentVersion = "web"
	span = selSpan{lo: 16, hi: 17}
	lst = buildCrossRefList(st, selectionVerses(st, "", span), crossRefsForSelection(st, "", span), nil, st.pal(), func(crossRef) {})
	if lst.CoverageNote != "" || len(lst.Objects) != 1 {
		t.Errorf("the WEB's Esther 4:16-17 is covered; got note %q and %d objects", lst.CoverageNote, len(lst.Objects))
	}
}

// The panel's empty state carries the note as its second line when a partly
// covered selection has no rows, online.
func TestAnEmptyPartlyCoveredPanelSaysWhy(t *testing.T) {
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
	st := &AppState{Bible: greekEstherBible(), CurrentBook: "Esther", CurrentChapter: 4, CurrentVersion: "webc", window: w}
	showCrossRefs(st, "", selSpan{lo: 17, hi: 18})
	want := "No cross-references for this selection.\nNothing is listed for verse 4:18, which the cross-references don't cover."
	if top := w.Canvas().Overlays().Top(); top == nil || !treeHasText(top, want) {
		var texts []string
		if top != nil {
			texts = treeTexts(top)
		}
		t.Errorf("the panel does not show %q; texts %q", want, texts)
	}
}

// An edition whose own apparatus the panel shows is not answered for the
// Treasury. Its block (publisher_xrefs.go) needs no network and answers for
// the selection whatever the Treasury covers, so a verse it adds that the
// reference does not have, the NKJV's Acts 8:37, gets neither the note nor
// the answer before loading: the panel loads and shows the edition's block.
func TestAPublisherBlockIsNotAnsweredForTheTreasury(t *testing.T) {
	licenseNKJVForTest(t)
	note := func(bd *BibleData) *BibleData {
		v := &bd.Verses["Acts"][8][36]
		v.Footnotes = []Footnote{{Anchor: 0, Text: "Matt. 28:19", Kind: footnoteKindCrossref, Caller: "-",
			Refs: []NoteRef{{"MAT.28.19", 0, 11}}}}
		return bd
	}
	const uncovered = "Nothing is listed for verse 8:37, which the cross-references don't cover."

	withPublisherCrossRefs(t, "nkjv", true)
	texts, loaded := coveragePanelIn(t, "nkjv", note(addedVersesBible()), "Acts", 8, 37, 37)
	if !loaded {
		t.Error("the panel answered before loading, over the edition's own block")
	}
	all := strings.Join(texts, "\n")
	if strings.Contains(all, uncovered) {
		t.Errorf("the panel says the selection is not covered over the edition's own block: %q", texts)
	}
	if !strings.Contains(all, "NKJV cross references") {
		t.Errorf("the edition's block is not shown: %q", texts)
	}
	st := &AppState{Bible: note(addedVersesBible()), CurrentBook: "Acts", CurrentChapter: 8, CurrentVersion: "nkjv"}
	span := selSpan{lo: 36, hi: 37}
	if lst := buildCrossRefList(st, selectionVerses(st, "", span), nil, nil, st.pal(), func(crossRef) {}); lst.CoverageNote != "" {
		t.Errorf("the list over the edition's block carries the note %q", lst.CoverageNote)
	}

	// CONTROL: without the edition's block the same selection is answered
	// before loading, and the list carries the note.
	withPublisherCrossRefs(t, "nkjv", false)
	texts, loaded = coveragePanelIn(t, "nkjv", note(addedVersesBible()), "Acts", 8, 37, 37)
	if loaded || !strings.Contains(strings.Join(texts, "\n"), uncovered) {
		t.Errorf("without the edition's block: loaded %v, texts %q; want the note before loading", loaded, texts)
	}
	if lst := buildCrossRefList(st, selectionVerses(st, "", span), nil, nil, st.pal(), func(crossRef) {}); lst.CoverageNote != uncovered {
		t.Errorf("without the edition's block the list carries %q, want %q", lst.CoverageNote, uncovered)
	}
}
