package bibletext

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func buildSearchResultsView(state *AppState) fyne.CanvasObject {
	pal := state.pal()
	trimmed := strings.TrimSpace(state.ActiveSearchQuery)

	// Echo the query as a heading, but truncate-to-fit with an ellipsis: a
	// width-aware Label keeps an unusually long query (a big paste, or repeated
	// characters) from rendering as one unbroken bar that runs off the edge of
	// the results header — on both the narrow phone column and the wide desktop
	// pane. Colour comes from the theme foreground (= pal.Text); SizeNameHeadingText
	// matches the other page headings.
	title := widget.NewLabel(fmt.Sprintf("Results for %q", trimmed))
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.SizeName = theme.SizeNameHeadingText
	title.Truncation = fyne.TextTruncateEllipsis

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
	subLabel.TextSize = subheadingTextSize

	terms := strings.Fields(strings.ToLower(trimmed))

	rows := make([]fyne.CanvasObject, 0, len(state.SearchResults))
	for _, verse := range state.SearchResults {
		rows = append(rows, searchResultRow(state, verse, terms, pal))
	}

	column := container.New(&readingColumn{maxWidth: 820}, container.NewVBox(rows...))
	scroll := container.NewVScroll(column)
	paper := surface(container.NewPadded(scroll), pal.Surface, pal.Border, fyne.Size{})

	head := container.NewVBox(title, subLabel, widget.NewSeparator())
	return container.NewPadded(container.NewBorder(head, nil, nil, nil, paper))
}

func searchResultRow(state *AppState, verse Verse, terms []string, pal palette) fyne.CanvasObject {
	ref := canvas.NewText(fmt.Sprintf("%s %d:%d", verse.BookName, verse.Chapter, verse.Verse), pal.Accent)
	ref.TextStyle = fyne.TextStyle{Bold: true}
	ref.TextSize = 18

	segs := termHighlightSegments(strings.TrimSpace(verse.Text), terms, colorNameVerseText, colorNameHighlightHi)
	text := widget.NewRichText(segs...)
	text.Wrapping = fyne.TextWrapWord

	// The whole card is one tap target — reference, verse text, and surrounding
	// padding — not just the reference heading.
	inner := container.NewPadded(container.NewVBox(ref, text))
	card := newSearchResultCard(state, verse, inner, pal)

	return container.NewVBox(card, widget.NewSeparator())
}

// searchResultCard makes an entire result row tappable. Previously only the
// small "Book C:V" heading opened the verse, which is an awkward target —
// especially on touch, where the rest of the row looks tappable but isn't.
// Tapping anywhere on the card jumps to that verse; on desktop it also shows a
// pointer cursor and a faint hover wash so the row reads as clickable.
type searchResultCard struct {
	widget.BaseWidget
	state   *AppState
	verse   Verse
	content fyne.CanvasObject
	hoverBg color.NRGBA
	bg      *canvas.Rectangle
}

func newSearchResultCard(state *AppState, verse Verse, content fyne.CanvasObject, pal palette) *searchResultCard {
	c := &searchResultCard{state: state, verse: verse, content: content, hoverBg: pal.SurfaceAlt}
	c.ExtendBaseWidget(c)
	return c
}

func (c *searchResultCard) CreateRenderer() fyne.WidgetRenderer {
	c.bg = canvas.NewRectangle(color.Transparent)
	c.bg.CornerRadius = 8
	return widget.NewSimpleRenderer(container.NewStack(c.bg, c.content))
}

func (c *searchResultCard) Tapped(*fyne.PointEvent) {
	openSearchResult(c.state, c.verse)
}

func (c *searchResultCard) MouseIn(*desktop.MouseEvent) {
	if c.bg != nil {
		c.bg.FillColor = c.hoverBg
		c.bg.Refresh()
	}
}

func (c *searchResultCard) MouseMoved(*desktop.MouseEvent) {}

func (c *searchResultCard) MouseOut() {
	if c.bg != nil {
		c.bg.FillColor = color.Transparent
		c.bg.Refresh()
	}
}

func (c *searchResultCard) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

var (
	_ fyne.Tappable      = (*searchResultCard)(nil)
	_ desktop.Hoverable  = (*searchResultCard)(nil)
	_ desktop.Cursorable = (*searchResultCard)(nil)
)

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
			SizeName:  theme.SizeNameText,
			ColorName: colorName,
			TextStyle: fyne.TextStyle{Bold: bold},
		},
	}
}
