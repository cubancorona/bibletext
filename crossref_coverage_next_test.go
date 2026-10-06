//go:build next

package bibletext

// The cross-references panel in the next major release (docs/NEXT.md) says
// why it lists nothing for a selection the Treasury cannot cover, before it
// loads anything when it can cover none of it.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// WHAT THE CROSS-REFERENCES CANNOT COVER IS SAID, NOT LEFT BLANK. The Treasury
// is keyed by the reference's numbering and holds the 66 books, so the Greek
// Esther's additions, which the reference does not have, and the
// deuterocanonical books have no rows at all.
func TestCrossRefCoverageNamesWhatTheTreasuryCannotCover(t *testing.T) {
	bd := greekEstherBible()
	verses := func(book string, chapter, lo, hi int) []Verse {
		var out []Verse
		for _, v := range bd.GetChapter(book, chapter) {
			if v.Verse >= lo && v.Verse <= hi {
				out = append(out, v)
			}
		}
		return out
	}
	for _, tc := range []struct {
		name, version, book string
		chapter, lo, hi     int
		note                string
		none                bool
	}{
		{"one addition", "webc", "Esther", 4, 20, 20,
			"Nothing is listed for verse 4:20, which the cross-references don't cover.", true},
		{"all the prayers", "webc", "Esther", 4, 18, 47,
			"Nothing is listed for verses 4:18–47, which the cross-references don't cover.", true},
		{"partly covered", "webc", "Esther", 4, 15, 20,
			"Nothing is listed for verses 4:18–20, which the cross-references don't cover.", false},
		{"a deuterocanonical book", "webc", "Tobit", 1, 1, 3,
			"The cross-references don't cover the deuterocanonical books.", true},
		// The Hebrew numbers are covered, in the Greek Esther as anywhere.
		{"a Hebrew verse of the Greek Esther", "webc", "Esther", 4, 14, 17, "", false},
		// CONTROL: the same numbers in the WEB are covered.
		{"the Hebrew Esther", "web", "Esther", 4, 15, 17, "", false},
		// Only Esther's additions are named (crossRefNotedAdditions): the
		// Song of the Three has no rows either and shows the first line alone.
		{"the Song of the Three", "webc", "Daniel", 3, 24, 30, "", false},
	} {
		note, none := crossRefCoverage(tc.version, tc.book, verses(tc.book, tc.chapter, tc.lo, tc.hi))
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
		book          string
		chapter       int
		lo, hi        int
		want          string
		wantLoadAsked bool
	}{
		{"a Greek addition", "Esther", 4, 20, 20,
			"No cross-references for this selection.\nNothing is listed for verse 4:20, which the cross-references don't cover.", false},
		{"the deuterocanon", "Tobit", 1, 1, 1,
			"No cross-references for this selection.\nThe cross-references don't cover the deuterocanonical books.", false},
		// CONTROL: a verse the Treasury does cover still needs it loaded,
		// and offline the panel says so.
		{"a Hebrew verse", "Esther", 4, 17, 17, offline, true},
		// Partly covered: the covered verse needs the Treasury too.
		{"partly covered", "Esther", 4, 17, 18, offline, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			texts, loaded := coveragePanel(t, tc.book, tc.chapter, tc.lo, tc.hi)
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
