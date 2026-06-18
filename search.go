package bibletext

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// trackSearchScroll remembers a results list's scroll position in state and restores
// it once the list has laid out — so returning to the Search tab lands where you left
// off. A new search resets state.searchScrollY to 0, so live/typed results start at top.
func trackSearchScroll(state *AppState, scroll *container.Scroll) {
	scroll.OnScrolled = func(p fyne.Position) { state.searchScrollY = p.Y }
	if state.searchScrollY > 0 {
		target := state.searchScrollY
		time.AfterFunc(60*time.Millisecond, func() {
			fyne.Do(func() { scroll.ScrollToOffset(fyne.NewPos(0, target)) })
		})
	}
}

func buildSearchResultsView(state *AppState) fyne.CanvasObject {
	// When the current results context is the AI search, render its (state-held)
	// passages with the AI view. This is what makes "back to results" and the Read-tab
	// inline results work for Ask-AI as well as keyword search.
	if state.aiSearchActive {
		return aiResultsView(state, state.aiSearchQuery, state.aiSearchResults)
	}
	pal := state.pal()
	trimmed := strings.TrimSpace(state.ActiveSearchQuery)

	// Before a real query is entered, show a calm, centred prompt rather than an
	// empty `Results for ""` heading sitting over an empty bordered box.
	if len([]rune(trimmed)) < 2 {
		return searchPromptView(state)
	}

	// The query already shows in the search field above, so the results header is just
	// a compact muted count line (no big "Results for …" heading) — keeps the results
	// taking most of the pane.
	var sub string
	switch {
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
	trackSearchScroll(state, scroll)
	paper := surface(container.NewPadded(scroll), pal.Surface, pal.Border, fyne.Size{})

	head := container.NewVBox(subLabel, widget.NewSeparator())
	return container.NewPadded(container.NewBorder(head, nil, nil, nil, paper))
}

// searchPromptView is the calm, centred empty state shown before a query is
// entered — a muted search glyph and a one-line invitation. Clearer than echoing
// `Results for ""` over an empty results box.
func searchPromptView(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	icon := canvas.NewImageFromResource(theme.NewColoredResource(theme.SearchIcon(), colorNameMuted))
	icon.FillMode = canvas.ImageFillContain
	icon.SetMinSize(fyne.NewSize(44, 44))

	title := canvas.NewText("Search the Bible", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 20
	title.Alignment = fyne.TextAlignCenter

	hint := canvas.NewText("A word or phrase, or a reference like John 3:16.", pal.TextMuted)
	hint.TextSize = subheadingTextSize
	hint.Alignment = fyne.TextAlignCenter

	col := container.NewVBox(
		container.NewCenter(icon),
		spacer(12),
		container.NewCenter(title),
		spacer(4),
		container.NewCenter(hint),
	)
	return container.NewCenter(col)
}

// aiResultsView renders AI-found passages as the same tappable cards as keyword
// search (no term highlight), with an honesty note — the passages are AI-suggested,
// but the text shown is the real verse from our Bible.
func aiResultsView(state *AppState, query string, verses []Verse) fyne.CanvasObject {
	pal := state.pal()
	_ = query // the question shows in the Ask field above; no big heading here

	sub := fmt.Sprintf("%d passages found by AI", len(verses))
	switch len(verses) {
	case 0:
		sub = "AI didn’t find matching passages — try rephrasing."
	case 1:
		sub = "1 passage found by AI"
	}
	subLabel := canvas.NewText(sub, pal.TextMuted)
	subLabel.TextSize = subheadingTextSize

	rows := make([]fyne.CanvasObject, 0, len(verses))
	for _, v := range verses {
		rows = append(rows, searchResultRow(state, v, nil, pal))
	}
	column := container.New(&readingColumn{maxWidth: 820}, container.NewVBox(rows...))
	scroll := container.NewVScroll(column)
	trackSearchScroll(state, scroll)
	paper := surface(container.NewPadded(scroll), pal.Surface, pal.Border, fyne.Size{})

	note := canvas.NewText("AI-suggested passages — read each in context.", pal.TextMuted)
	note.TextSize = 11
	head := container.NewVBox(subLabel, note, widget.NewSeparator())
	return container.NewPadded(container.NewBorder(head, nil, nil, nil, paper))
}

// aiSearchPromptView is the calm empty state for Ask-AI mode, shown before a
// question is asked.
func aiSearchPromptView(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	icon := canvas.NewImageFromResource(theme.NewColoredResource(theme.SearchIcon(), colorNameMuted))
	icon.FillMode = canvas.ImageFillContain
	icon.SetMinSize(fyne.NewSize(44, 44))

	title := canvas.NewText("Ask in your own words", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 20
	title.Alignment = fyne.TextAlignCenter

	hint := canvas.NewText("e.g. “what did God say to Jonah?”", pal.TextMuted)
	hint.TextSize = subheadingTextSize
	hint.Alignment = fyne.TextAlignCenter

	col := container.NewVBox(
		container.NewCenter(icon), spacer(12),
		container.NewCenter(title), spacer(4),
		container.NewCenter(hint),
	)
	return container.NewCenter(col)
}

// aiNoKeyView is the clean, non-intrusive explanation shown in Ask-AI mode when no
// provider key is set: what it does, that it needs the reader's own key, and a quiet
// route into settings. No error styling.
func aiNoKeyView(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	title := canvas.NewText("AI search needs your own key", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 18
	title.Alignment = fyne.TextAlignCenter

	body := widget.NewLabel("Ask for passages in plain words and AI finds them. It uses your own AI provider key, stored only on this device.")
	body.Wrapping = fyne.TextWrapWord
	body.Alignment = fyne.TextAlignCenter

	setup := widget.NewButton("Set up AI", func() { showAISettings(state) })
	setup.Importance = widget.HighImportance

	col := container.NewVBox(
		container.NewCenter(title), spacer(6),
		container.NewGridWrap(fyne.NewSize(300, body.MinSize().Height), body), spacer(14),
		container.NewCenter(setup),
	)
	return container.NewCenter(col)
}

// aiSearchMessageView centres a short message with an optional action button —
// used for AI-search errors (with a Try again retry).
func aiSearchMessageView(msg, action string, onAction func()) fyne.CanvasObject {
	lbl := widget.NewLabel(msg)
	lbl.Wrapping = fyne.TextWrapWord
	lbl.Alignment = fyne.TextAlignCenter

	items := []fyne.CanvasObject{container.NewGridWrap(fyne.NewSize(300, lbl.MinSize().Height+8), lbl)}
	if action != "" && onAction != nil {
		btn := widget.NewButton(action, onAction)
		items = append(items, spacer(12), container.NewCenter(btn))
	}
	return container.NewCenter(container.NewVBox(items...))
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
