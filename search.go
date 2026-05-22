package main

import (
	"fmt"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func buildSearchResultsView(state *AppState) fyne.CanvasObject {
	pal := state.pal()
	trimmed := strings.TrimSpace(state.ActiveSearchQuery)

	title := canvas.NewText(fmt.Sprintf("Results for %q", trimmed), pal.Text)
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}

	var sub string
	switch {
	case len([]rune(trimmed)) < 2:
		sub = "Type at least 2 characters to search."
	case len(state.SearchResults) == 0:
		sub = "No verses matched your search."
	case state.SearchTruncated:
		sub = fmt.Sprintf("Showing the first %d matches — refine your search to narrow it down.", len(state.SearchResults))
	default:
		sub = fmt.Sprintf("%d matches", len(state.SearchResults))
	}
	subLabel := canvas.NewText(sub, pal.TextMuted)
	subLabel.TextSize = 13

	terms := strings.Fields(strings.ToLower(trimmed))

	rows := make([]fyne.CanvasObject, 0, len(state.SearchResults))
	for _, verse := range state.SearchResults {
		rows = append(rows, searchResultRow(state, verse, terms, pal))
	}

	column := container.New(&readingLayout{maxWidth: 820, spacing: 6}, rows...)
	scroll := container.NewVScroll(column)
	paper := surface(container.NewPadded(scroll), pal.Surface, pal.Border, fyne.Size{})

	head := container.NewVBox(title, subLabel, widget.NewSeparator())
	return container.NewPadded(container.NewBorder(head, nil, nil, nil, paper))
}

func searchResultRow(state *AppState, verse Verse, terms []string, pal palette) fyne.CanvasObject {
	v := verse
	ref := widget.NewButton(fmt.Sprintf("%s %d:%d", verse.BookName, verse.Chapter, verse.Verse), func() {
		openSearchResult(state, v)
	})
	ref.Importance = widget.LowImportance

	segs := termHighlightSegments(strings.TrimSpace(verse.Text), terms, colorNameVerseText, colorNameHighlightHi)
	text := widget.NewRichText(segs...)
	text.Wrapping = fyne.TextWrapWord

	return container.NewVBox(
		container.NewBorder(nil, nil, ref, nil, nil),
		text,
		widget.NewSeparator(),
	)
}

type matchRange struct {
	start int
	end   int
}

// termHighlightSegments splits text into RichText segments, emphasising every
// occurrence of the search terms. Matching is case-insensitive.
func termHighlightSegments(text string, terms []string, base, highlight fyne.ThemeColorName) []widget.RichTextSegment {
	ranges := matchRanges(text, terms)
	if len(ranges) == 0 {
		return []widget.RichTextSegment{resultSegment(text, base, false)}
	}

	segs := make([]widget.RichTextSegment, 0, len(ranges)*2+1)
	pos := 0
	for _, r := range ranges {
		if r.start > pos {
			segs = append(segs, resultSegment(text[pos:r.start], base, false))
		}
		segs = append(segs, resultSegment(text[r.start:r.end], highlight, true))
		pos = r.end
	}
	if pos < len(text) {
		segs = append(segs, resultSegment(text[pos:], base, false))
	}
	return segs
}

// matchRanges returns merged, ordered byte ranges where any term occurs. It
// bails out on multi-byte text to keep byte offsets aligned with the original.
func matchRanges(text string, terms []string) []matchRange {
	lower := strings.ToLower(text)
	if len(lower) != len(text) {
		return nil
	}

	var ranges []matchRange
	for _, term := range terms {
		term = strings.TrimSpace(strings.ToLower(term))
		if len([]rune(term)) < 2 {
			continue
		}
		from := 0
		for {
			i := strings.Index(lower[from:], term)
			if i < 0 {
				break
			}
			start := from + i
			ranges = append(ranges, matchRange{start: start, end: start + len(term)})
			from = start + len(term)
		}
	}
	if len(ranges) == 0 {
		return nil
	}

	sort.Slice(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })
	merged := ranges[:1]
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if r.start <= last.end {
			if r.end > last.end {
				last.end = r.end
			}
			continue
		}
		merged = append(merged, r)
	}
	return merged
}

func resultSegment(s string, colorName fyne.ThemeColorName, bold bool) widget.RichTextSegment {
	return &widget.TextSegment{
		Text: s,
		Style: widget.RichTextStyle{
			Inline:    true,
			SizeName:  sizeNameReading,
			ColorName: colorName,
			TextStyle: fyne.TextStyle{Bold: bold},
		},
	}
}
