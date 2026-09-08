package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// A heading is the publisher's LABEL on a passage, not part of it. It must be
// drawn and must NOT enter the flat selection text model — a reader who selects
// across it and shares the quote would otherwise be quoting the wrong thing,
// and every rune offset the pane records would shift under it.
func TestHeadingsAreDrawnButNotSelectable(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(&bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()})

	build := func(withHeading bool) *styledReadingPane {
		st := sampleState()
		ch := st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter)
		if withHeading {
			st.Bible.Headings = map[string]map[int][]Heading{
				st.CurrentBook: {st.CurrentChapter: {
					{Text: "The Word Became Flesh", BeforeVerse: ch[0].Verse},
				}},
			}
		} else {
			st.Bible.Headings = nil
		}
		p := newStyledReadingPane(st, ch)
		w := fyne.CurrentApp().NewWindow("x")
		w.SetContent(p)
		w.Resize(fyne.NewSize(900, 400))
		p.Resize(fyne.NewSize(900, 400))
		p.Refresh()
		return p
	}

	with, without := build(true), build(false)

	// Drawn: a line carries it.
	found := ""
	for _, ln := range with.lay.Lines {
		if ln.Heading != "" {
			found = ln.Heading
			break
		}
	}
	if found != "The Word Became Flesh" {
		t.Fatalf("the heading was not laid out; got %q", found)
	}
	// The control: without the heading there is no such line, so the check
	// above is really testing the heading and not something ambient.
	for _, ln := range without.lay.Lines {
		if ln.Heading != "" {
			t.Fatalf("a heading appeared with none configured: %q", ln.Heading)
		}
	}

	// NOT in the model: the flat text is identical either way.
	if with.lay.Text != without.lay.Text {
		t.Errorf("the heading entered the selection text model:\n with    %q\n without %q",
			trunc(with.lay.Text), trunc(without.lay.Text))
	}
	if strings.Contains(with.lay.Text, "The Word Became Flesh") {
		t.Error("the heading's words are in the selection text")
	}
	// And no run carries it, which is what the selection layers walk.
	for _, dr := range with.drawRuns {
		if strings.Contains(dr.Text, "Became Flesh") {
			t.Errorf("a run carries the heading: %q", dr.Text)
		}
	}
}

func trunc(s string) string {
	if len(s) > 60 {
		return s[:60] + "…"
	}
	return s
}
