package bibletext

// The cross-references panel says what it cannot cover by the selection's
// book (crossRefCoverage, panelCoverage), so the book is the selection's
// (selSpan.book), not the reader's, which narration may have turned into the
// next book by the time the panel opens. An invented Nehemiah 13 with Tobit
// after it, as the Catholic order has them.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

const nehemiahWords = "The gatekeepers counted the cedar beams by the light of the lamps."

func nehemiahThenTobitState() *AppState {
	v := func(book string, ch, n int, text string) Verse {
		return Verse{BookName: book, Book: book, Chapter: ch, Verse: n, Text: text}
	}
	bd := &BibleData{
		Books: []string{"Nehemiah", "Tobit"},
		Verses: map[string]map[int][]Verse{
			"Nehemiah": {13: {
				v("Nehemiah", 13, 1, "On that day the scroll was read aloud in the square."),
				v("Nehemiah", 13, 2, "The singers stood by the eastern stair."),
				v("Nehemiah", 13, 3, nehemiahWords),
			}},
			"Tobit": {1: {
				v("Tobit", 1, 1, "This is the account of a traveller from the hill country."),
				v("Tobit", 1, 2, "He walked the long road to the river."),
				v("Tobit", 1, 3, "He kept the feast with the people of the town."),
			}},
		},
	}
	return &AppState{Bible: bd, CurrentVersion: "webc", CurrentBook: "Nehemiah", CurrentChapter: 13}
}

// A selection in Nehemiah 13, opened in the cross-references panel after
// narration has turned into Tobit, is answered for Nehemiah, which the
// Treasury covers: the panel neither says the deuterocanonical books are not
// covered nor stops before loading. Control: the same verse numbers read in
// the reader's chapter are Tobit's, and are not covered.
func TestTheCrossReferenceCoverageIsTheSelectionsBookAfterATurn(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	w := test.NewWindow(widget.NewLabel("reading"))
	defer w.Close()
	w.Resize(fyne.NewSize(800, 700))
	prev := crossRefsRun
	loads := 0
	crossRefsRun = func(func()) { loads++ } // the load never lands
	t.Cleanup(func() { crossRefsRun = prev })

	st := nehemiahThenTobitState()
	st.window = w
	span := paneSelSpan(3, 3, "3 ", true, "Nehemiah", 13)
	if !advanceToNextChapter(st) || st.CurrentBook != "Tobit" {
		t.Fatalf("control: the turn did not reach Tobit (%s %d)", st.CurrentBook, st.CurrentChapter)
	}

	if lst := buildCrossRefList(st, selectionVerses(st, nehemiahWords, span), nil, nil, st.pal(), func(crossRef) {}); lst.CoverageNote != "" {
		t.Errorf("Nehemiah 13:3's list carries %q", lst.CoverageNote)
	}
	showCrossRefs(st, nehemiahWords, span)
	top := w.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("the panel did not open")
	}
	defer top.Hide()
	if loads != 1 || !treeHasText(top, "Nehemiah 13:3") {
		t.Errorf("the panel did not load for Nehemiah 13:3 (%d loads); texts %v", loads, treeTexts(top))
	}
	for _, s := range treeTexts(top) {
		if strings.Contains(s, crossRefDeuterocanonLine) {
			t.Errorf("Nehemiah 13:3 is answered as a deuterocanonical book: %q", s)
		}
	}

	// Control.
	bare := selSpanWithLead(3, 3, "3 ")
	if note, none := crossRefCoverage(st.currentVersion().ID, st.CurrentBook, selectionVerses(st, "", bare)); note != crossRefDeuterocanonLine || !none {
		t.Errorf("control: Tobit 1:3 got %q (none=%v), want the deuterocanonical line", note, none)
	}
}
