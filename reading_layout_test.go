//go:build !race

package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func collectRichText(o fyne.CanvasObject, out *[]*widget.RichText) {
	switch v := o.(type) {
	case *widget.RichText:
		*out = append(*out, v)
	case *container.Scroll:
		collectRichText(v.Content, out)
	case *container.Split:
		collectRichText(v.Leading, out)
		collectRichText(v.Trailing, out)
	case *fyne.Container:
		for _, c := range v.Objects {
			collectRichText(c, out)
		}
	}
}

// TestReadingParagraphsUseSingleSegment guards the fix for the "one character
// per line" bug: Fyne mis-wraps a RichText when a line break falls amid multiple
// inline segments, so every reading block must contain exactly one text segment.
func TestReadingParagraphsUseSingleSegment(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := sampleState() // John 1 has verses 1-3 in the sample data
	var rts []*widget.RichText
	collectRichText(buildReadingView(state), &rts)
	if len(rts) == 0 {
		t.Fatal("found no reading paragraphs")
	}
	for i, rt := range rts {
		if len(rt.Segments) != 1 {
			t.Errorf("reading block %d has %d segments; must be 1 to wrap correctly", i, len(rt.Segments))
		}
	}
}

// TestHighlightedVerseIsOwnBoldBlock verifies a highlighted verse splits out into
// its own single-segment, bold block (the scroll-to target), with the verses
// before and after kept in their own plain blocks.
func TestHighlightedVerseIsOwnBoldBlock(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := sampleState()
	state.HighlightedBook = "John"
	state.HighlightedChapter = 1
	state.HighlightedVerse = 2
	state.HasHighlightedVerse = true

	var rts []*widget.RichText
	collectRichText(buildReadingView(state), &rts)
	if len(rts) != 3 {
		t.Fatalf("expected 3 blocks (before / highlighted / after), got %d", len(rts))
	}
	for i, rt := range rts {
		if len(rt.Segments) != 1 {
			t.Errorf("block %d has %d segments; must be 1", i, len(rt.Segments))
		}
	}
	mid, ok := rts[1].Segments[0].(*widget.TextSegment)
	if !ok || !mid.Style.TextStyle.Bold {
		t.Error("the highlighted verse block should be bold")
	}
	if !ok || mid.Style.ColorName != colorNameHighlightHi {
		t.Error("the highlighted verse block should use the highlight colour")
	}
}
