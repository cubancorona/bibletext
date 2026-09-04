package bibletext

// THE PRESENTED READER'S ONE ROW: which control each way into the mode gets,
// and that a page can be turned in landscape without rotating back.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func fullScreenRowState() *AppState {
	bd := &BibleData{
		Books: []string{"Genesis"},
		Verses: map[string]map[int][]Verse{"Genesis": {
			1: {{BookName: "Genesis", Book: "Genesis", Chapter: 1, Verse: 1, Text: "In the beginning."}},
			2: {{BookName: "Genesis", Book: "Genesis", Chapter: 2, Verse: 1, Text: "The heavens were finished."}},
			3: {{BookName: "Genesis", Book: "Genesis", Chapter: 3, Verse: 1, Text: "Now the serpent."}},
		}},
	}
	return &AppState{Bible: bd, CurrentBook: "Genesis", CurrentChapter: 2}
}

func countIn(obj fyne.CanvasObject, match func(fyne.CanvasObject) bool) int {
	if obj == nil {
		return 0
	}
	n := 0
	if match(obj) {
		n++
	}
	if c, ok := obj.(*fyne.Container); ok {
		for _, o := range c.Objects {
			n += countIn(o, match)
		}
	}
	return n
}

func countTapButtons(obj fyne.CanvasObject) int {
	return countIn(obj, func(o fyne.CanvasObject) bool { _, ok := o.(*iconTapButton); return ok })
}

func countWidgetButtons(obj fyne.CanvasObject) int {
	return countIn(obj, func(o fyne.CanvasObject) bool { _, ok := o.(*widget.Button); return ok })
}

// Landscape: the arrows, and no restore button — the button would write
// IsFullScreen=false and rebuild into the same tree, so rotation is the way out.
func TestLandscapeRowCarriesTheChapterArrows(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	orig := phoneLandscapeReading
	phoneLandscapeReading = func() bool { return true }
	defer func() { phoneLandscapeReading = orig }()

	row := fullScreenExitRow(fullScreenRowState())
	if got := countTapButtons(row); got != 2 {
		t.Fatalf("landscape row has %d chapter arrows, want 2", got)
	}
	if got := countWidgetButtons(row); got != 0 {
		t.Errorf("landscape row offers %d restore buttons; rotation is the way out", got)
	}
}

// The reader's own full-screen keeps the restore button and stays as spare as
// it was — the arrows are landscape's compensation, not a general addition.
func TestChosenFullScreenRowKeepsOnlyItsRestoreButton(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	orig := phoneLandscapeReading
	phoneLandscapeReading = func() bool { return false }
	defer func() { phoneLandscapeReading = orig }()

	row := fullScreenExitRow(fullScreenRowState())
	if got := countWidgetButtons(row); got != 1 {
		t.Fatalf("chosen full-screen has %d restore buttons, want 1", got)
	}
	if got := countTapButtons(row); got != 0 {
		t.Errorf("chosen full-screen gained %d arrows", got)
	}
}

// The ends of the book: the arrows disable exactly as the portrait header's do.
func TestLandscapeArrowsDisableAtTheEndsOfTheBook(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	for _, tc := range []struct {
		chapter       string
		at            int
		wantPrevOff   bool
		wantNextOff   bool
		wantExplained string
	}{
		{"first", 1, true, false, "no chapter before the first"},
		{"middle", 2, false, false, "both ways open"},
		{"last", 3, false, true, "no chapter after the last"},
	} {
		st := fullScreenRowState()
		st.CurrentChapter = tc.at
		pair := chapterArrowPair(st)
		var buttons []*iconTapButton
		countIn(pair, func(o fyne.CanvasObject) bool {
			if b, ok := o.(*iconTapButton); ok {
				buttons = append(buttons, b)
			}
			return false
		})
		if len(buttons) != 2 {
			t.Fatalf("%s chapter: found %d arrows", tc.chapter, len(buttons))
		}
		if buttons[0].disabled != tc.wantPrevOff || buttons[1].disabled != tc.wantNextOff {
			t.Errorf("%s chapter (%s): prev disabled=%v next disabled=%v, want %v/%v",
				tc.chapter, tc.wantExplained, buttons[0].disabled, buttons[1].disabled,
				tc.wantPrevOff, tc.wantNextOff)
		}
	}
}

// No Bible yet: the row still builds, and offers no arrows rather than arrows
// that would move through a book that is not loaded.
func TestLandscapeRowSurvivesAnUnloadedBible(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	orig := phoneLandscapeReading
	phoneLandscapeReading = func() bool { return true }
	defer func() { phoneLandscapeReading = orig }()

	row := fullScreenExitRow(&AppState{CurrentBook: "Genesis", CurrentChapter: 1})
	if row == nil {
		t.Fatal("the row must build before a Bible is loaded")
	}
	if got := countTapButtons(row); got != 0 {
		t.Errorf("row offers %d arrows with no Bible loaded", got)
	}
	// The label is still there, so the row is not silently empty.
	if got := countIn(row, func(o fyne.CanvasObject) bool { _, ok := o.(*canvas.Text); return ok }); got != 1 {
		t.Errorf("row carries %d reference labels, want 1", got)
	}
}
