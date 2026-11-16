package bibletext

import "testing"

func TestPublisherHeadingOriginsKeepDistinctLabelsAtOneVerse(t *testing.T) {
	bd := xrefBible(map[string]map[int]int{"Genesis": {1: 5}, "Exodus": {1: 3}})
	bd.Headings = map[string]map[int][]Heading{"Genesis": {1: {
		{Text: "A narrow path", Style: "heading", BeforeVerse: 3, Footnotes: []Footnote{
			{Text: "Exodus 1:1", Kind: footnoteKindCrossref, Caller: "-"},
			{Text: "Exodus 1:2", Kind: footnoteKindCrossref, Caller: "-"},
		}},
		{Text: "Another route", Style: "heading", BeforeVerse: 3, Footnotes: []Footnote{
			{Text: "Exodus 1:3", Kind: footnoteKindCrossref, Caller: "-"},
		}},
	}}}
	bd.Verses["Genesis"][1][2].Footnotes = []Footnote{
		{Text: "Exodus 1:1", Anchor: 5, Kind: footnoteKindCrossref, Caller: "+"},
	}

	notes := publisherCrossRefsFor(bd, "Genesis", 1, versesBetween(bd, "Genesis", 1, 3, 3))
	if len(notes) != 4 {
		t.Fatalf("got %d notes, want three heading notes followed by the verse note", len(notes))
	}
	wantHeadings := []string{"A narrow path", "A narrow path", "Another route", ""}
	wantTexts := []string{"Exodus 1:1", "Exodus 1:2", "Exodus 1:3", "Exodus 1:1"}
	for i, n := range notes {
		if n.HeadingText != wantHeadings[i] || n.Text != wantTexts[i] {
			t.Errorf("note %d has heading %q and text %q, want %q and %q", i, n.HeadingText, n.Text, wantHeadings[i], wantTexts[i])
		}
		if n.Verse != 3 || n.Index != i || n.Heading != (i < 3) || n.Unkeyed {
			t.Errorf("note %d origin or ordering changed: verse=%d index=%d heading=%v unkeyed=%v", i, n.Verse, n.Index, n.Heading, n.Unkeyed)
		}
		wantMark := 0
		if i == 3 {
			wantMark = 5
		}
		if n.Anchor != wantMark || n.Mark != wantMark {
			t.Errorf("note %d anchor/mark = %d/%d, want %d/%d", i, n.Anchor, n.Mark, wantMark, wantMark)
		}
	}
}

func TestPublisherHeadingOriginRequiresItsBeforeVerseInSelection(t *testing.T) {
	bd := xrefBible(map[string]map[int]int{"Genesis": {1: 5}, "Exodus": {1: 1}})
	bd.Headings = map[string]map[int][]Heading{"Genesis": {1: {
		{Text: "A fixture section", Style: "heading", BeforeVerse: 3, Footnotes: []Footnote{
			{Text: "Exodus 1:1", Kind: footnoteKindCrossref, Caller: "-"},
		}},
	}}}
	for _, span := range [][2]int{{1, 2}, {4, 4}, {4, 5}} {
		notes := publisherCrossRefsFor(bd, "Genesis", 1, versesBetween(bd, "Genesis", 1, span[0], span[1]))
		if len(notes) != 0 {
			t.Errorf("selection %d–%d inherited %d notes from an unselected heading position", span[0], span[1], len(notes))
		}
	}
	notes := publisherCrossRefsFor(bd, "Genesis", 1, versesBetween(bd, "Genesis", 1, 2, 4))
	if len(notes) != 1 || notes[0].Verse != 3 || notes[0].HeadingText != "A fixture section" || !notes[0].Heading {
		t.Fatalf("range containing the heading position did not retain its exact origin: %+v", notes)
	}
}
