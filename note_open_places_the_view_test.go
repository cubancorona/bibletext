package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// OPENING A NOTE PLACES THE VIEW. Every verb that makes a note the page's
// reason has to say so, because "place the view" is a declaration
// (AppState.forceReposition) and not something the render path can infer: the
// panes deliberately treat a presentation-only change as a sticker repaint with
// no scroll, which is right for a Hide and wrong for a Show.
//
// The next-tap declared it and the two PILL verbs did not, so pressing the pill
// at the top of a chapter expanded a bubble that was hundreds of points further
// down and left the reader looking at the same paragraph. Stated here as one
// rule over every opening verb rather than as three separate edits.
func TestEveryVerbThatOpensANotePlacesTheView(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())
	defer setNotesEnabled(true)

	for _, tc := range []struct {
		name string
		open func(st *AppState, ids []uint64)
	}{
		{"the chapter pill's restore", func(st *AppState, ids []uint64) {
			hideCurrentNote(st)
			st.forceReposition = false
			restoreCurrentNote(st)
		}},
		// A paragraph pill only EXISTS with the per-paragraph gate on, so this
		// cell turns it on for itself. Before, it silently exercised the chapter
		// pill's code path and reported the paragraph pill as covered.
		{"a paragraph pill's focus", func(st *AppState, ids []uint64) {
			prev := notesPillPerParagraph
			notesPillPerParagraph = true
			defer func() { notesPillPerParagraph = prev }()
			hideCurrentNote(st)
			st.forceReposition = false
			pressPillOnParagraphOf(t, st, st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter)[0].Verse)
		}},
		// The next-tap's own cell here is the anchor-CHANGING one; the
		// same-anchor exception has its own test
		// (TestNextTapOnTheSameVerseKeepsTheViewport), and the two fixture
		// notes below sit in different paragraphs on purpose.
		{"the counts region's next-tap", func(st *AppState, ids []uint64) {
			st.forceReposition = false
			advanceNoteFocus(st)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deleteAllNotes(appPrefs())
			// A chapter with real paragraphs: the defect is that the note is
			// somewhere the reader is not looking, which a one-verse fixture
			// cannot express.
			st := planTestState(t)
			st.Bible.Verses["John"][3] = enumerationChapter()
			verses := st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter)
			if len(verses) < 2 {
				t.Fatalf("the fixture chapter has %d verses", len(verses))
			}
			var ids []uint64
			for i, v := range []int{verses[0].Verse, verses[len(verses)-1].Verse} {
				n, _ := addNote(appPrefs(), StoredNote{
					Kind: noteKindReceived, VersionID: st.currentVersion().ID,
					Book: st.CurrentBook, Chapter: st.CurrentChapter, VerseLo: v,
					Text: "fixture note " + string(rune('a'+i)),
				})
				ids = append(ids, n.ID)
			}
			applyNoteForCurrentChapter(st)
			if st.ActiveNote == "" {
				t.Fatal("no note is open, so no opening verb can be exercised")
			}

			tc.open(st, ids)

			if st.ActiveNote == "" {
				t.Fatalf("%s left no note open, so this cell proves nothing", tc.name)
			}
			if !st.forceReposition {
				t.Errorf("%s did not declare a placement. The bubble expands where "+
					"the reader is not looking: the pane treats a presentation-only "+
					"change as a repaint with no scroll, which is right for a Hide "+
					"and wrong for a Show.", tc.name)
			}
		})
	}
}
