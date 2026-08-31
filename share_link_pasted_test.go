package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// A link PASTED into the search box must land exactly where a tapped one does,
// highlight included. It is the only way notes reach Windows, Linux and the
// unsigned macOS build, which the OS never hands a universal link.
//
// The success path cleared the search UI with clearSearchState, whose last act
// is clearHighlightedVerse — so the mark the link had just set was wiped
// between opening the passage and drawing it. The reader arrived at the right
// chapter with nothing lit, which reads as the link half-working.
func TestAPastedLinkKeepsItsHighlight(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	mk := func(ch, n int) []Verse {
		var out []Verse
		for i := 1; i <= n; i++ {
			out = append(out, Verse{BookName: "John", Book: "John", Chapter: ch,
				Verse: i, Text: "verse text here"})
		}
		return out
	}
	john3 := mk(3, 36)

	for _, vid := range []string{"web", "nkjv"} {
		st := planTestState(t)
		st.Bible.Verses["John"] = map[int][]Verse{1: mk(1, 51), 2: mk(2, 25), 3: john3}
		st.CurrentBook, st.CurrentChapter = "Genesis", 1

		executeSearch(st, ShareLinkURL(vid, "John", 3, 27, 0))

		if st.CurrentBook != "John" || st.CurrentChapter != 3 {
			t.Fatalf("%s: the link did not open the passage: %s %d",
				vid, st.CurrentBook, st.CurrentChapter)
		}
		tints := chapterTint(st)
		lit := 0
		for _, v := range john3 {
			if tints.of(v) != tintNone {
				lit++
			}
		}
		if lit == 0 {
			t.Errorf("%s: the passage opened with NOTHING lit — the pasted link's "+
				"highlight was cleared between opening it and drawing it", vid)
		}
	}
}
