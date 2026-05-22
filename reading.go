package main

import (
	"fmt"
	"math"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// buildReadingPane returns the right-hand content: search results when a search
// is active, otherwise the chapter reading view.
func buildReadingPane(state *AppState) fyne.CanvasObject {
	if state.IsSearching {
		return buildSearchResultsView(state)
	}
	return buildReadingView(state)
}

func buildReadingView(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	chapterNumbers := state.Bible.GetChapterNumbersForBook(state.CurrentBook)
	normalizeCurrentChapter(state, chapterNumbers)
	verses := state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter)

	var paragraphs []fyne.CanvasObject
	var highlightTarget fyne.CanvasObject
	if len(verses) == 0 {
		msg := widget.NewLabel("No verses are available for this chapter yet.")
		msg.Wrapping = fyne.TextWrapWord
		paragraphs = append(paragraphs, msg)
	} else {
		for _, para := range groupVersesIntoParagraphs(verses) {
			objs, target := paragraphObjects(state, para)
			paragraphs = append(paragraphs, objs...)
			if target != nil {
				highlightTarget = target
			}
		}
	}

	reading := &readingLayout{maxWidth: 760, spacing: 16, target: highlightTarget}
	column := container.New(reading, paragraphs...)
	scroll := container.NewVScroll(column)
	reading.scroll = scroll // wired after creation so the layout can scroll to a verse

	paper := surface(container.NewPadded(scroll), pal.Surface, pal.Border, fyne.Size{})

	top := container.NewVBox()
	if bar := buildHistoryBar(state); bar != nil {
		top.Add(bar)
	}
	if state.CanReturnToSearchResults {
		top.Add(backToResultsBar(state))
	}
	top.Add(chapterHeader(state, chapterNumbers))

	// One uniform pad around the whole pane keeps the header and the page on the
	// same left/right margin.
	return container.NewPadded(container.NewBorder(top, nil, nil, nil, paper))
}

// chapterHeader renders the book title, chapter indicator/picker and the
// copy/previous/next controls, followed by a divider.
func chapterHeader(state *AppState, chapterNumbers []int) fyne.CanvasObject {
	pal := state.pal()
	total := len(chapterNumbers)

	title := canvas.NewText(state.CurrentBook, pal.Text)
	title.TextSize = 28
	title.TextStyle = fyne.TextStyle{Bold: true}

	var chapterLine fyne.CanvasObject
	if total > 1 {
		var pick *widget.Button
		pick = widget.NewButtonWithIcon(fmt.Sprintf("Chapter %d of %d", state.CurrentChapter, total), theme.MenuDropDownIcon(), func() {
			showChapterPicker(pick, state)
		})
		pick.Importance = widget.LowImportance
		chapterLine = container.NewHBox(pick)
	} else {
		lbl := canvas.NewText(fmt.Sprintf("Chapter %d", state.CurrentChapter), pal.TextMuted)
		lbl.TextSize = 13
		chapterLine = container.NewHBox(lbl)
	}

	idx := indexOf(chapterNumbers, state.CurrentChapter)

	copyBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() { copyChapter(state) })
	copyBtn.Importance = widget.LowImportance

	prev := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		if moveChapter(state, -1) {
			state.refresh()
		}
	})
	prev.Importance = widget.LowImportance
	if idx <= 0 {
		prev.Disable()
	}

	next := widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
		if moveChapter(state, 1) {
			state.refresh()
		}
	})
	next.Importance = widget.LowImportance
	if idx < 0 || idx >= total-1 {
		next.Disable()
	}

	nav := container.NewHBox(copyBtn, prev, next)
	left := container.NewVBox(title, chapterLine)
	row := container.NewBorder(nil, nil, left, container.NewVBox(layout.NewSpacer(), nav, layout.NewSpacer()), nil)

	rule := canvas.NewLine(pal.Border)
	rule.StrokeWidth = 1
	return container.NewVBox(row, rule)
}

func backToResultsBar(state *AppState) fyne.CanvasObject {
	pal := state.pal()
	label := state.ActiveSearchQuery
	if label == "" {
		label = "results"
	}
	back := widget.NewButtonWithIcon(fmt.Sprintf("Back to results for %q", label), theme.NavigateBackIcon(), func() {
		state.IsSearching = true
		clearHighlightedVerse(state)
		state.refreshReadingOnly()
	})
	back.Importance = widget.LowImportance
	return surface(container.NewHBox(back), pal.SurfaceAlt, pal.Border, fyne.Size{})
}

// paragraphObjects renders a paragraph as flowing scripture. Each block is a
// RichText with a SINGLE text segment: Fyne mis-wraps RichText when a line break
// lands amid multiple inline segments (it then breaks one character per line), so
// one segment per block keeps wrapping reliable. When the paragraph contains the
// highlighted verse, that verse becomes its own accent-coloured block (also the
// scroll-to target); the verses around it stay in plain blocks.
func paragraphObjects(state *AppState, verses []Verse) (objs []fyne.CanvasObject, target fyne.CanvasObject) {
	hiIdx := -1
	for i, v := range verses {
		if isVerseHighlighted(state, v) {
			hiIdx = i
			break
		}
	}

	if hiIdx == -1 {
		return []fyne.CanvasObject{verseBlock(verses, false)}, nil
	}

	if hiIdx > 0 {
		objs = append(objs, verseBlock(verses[:hiIdx], false))
	}
	target = verseBlock(verses[hiIdx:hiIdx+1], true)
	objs = append(objs, target)
	if hiIdx < len(verses)-1 {
		objs = append(objs, verseBlock(verses[hiIdx+1:], false))
	}
	return objs, target
}

func verseBlock(verses []Verse, highlight bool) fyne.CanvasObject {
	var b strings.Builder
	for i, v := range verses {
		if i > 0 {
			b.WriteString("  ")
		}
		if n := superscriptNumber(v.Verse); n != "" {
			b.WriteString(n)
			b.WriteString(" ")
		}
		b.WriteString(strings.TrimSpace(v.Text))
	}

	style := widget.RichTextStyle{SizeName: sizeNameReading, ColorName: colorNameVerseText}
	if highlight {
		style.ColorName = colorNameHighlightHi
		style.TextStyle = fyne.TextStyle{Bold: true}
	}

	rt := widget.NewRichText(&widget.TextSegment{Text: b.String(), Style: style})
	rt.Wrapping = fyne.TextWrapWord
	return rt
}

func copyChapter(state *AppState) {
	if state.window == nil {
		return
	}
	verses := state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter)
	if len(verses) == 0 {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %d\n\n", state.CurrentBook, state.CurrentChapter)
	for _, v := range verses {
		fmt.Fprintf(&b, "%d %s\n", v.Verse, strings.TrimSpace(v.Text))
	}
	state.window.Clipboard().SetContent(b.String())
}

// readingLayout stacks paragraphs vertically, centred and capped at a
// comfortable line length. When a target paragraph and scroll are set it scrolls
// that verse into view during layout — on the render thread, so no goroutine and
// no data race.
type readingLayout struct {
	maxWidth float32
	spacing  float32
	target   fyne.CanvasObject
	scroll   *container.Scroll
	scrolled bool
}

func (l *readingLayout) columnWidth(available float32) float32 {
	if available > l.maxWidth {
		return l.maxWidth
	}
	if available < 0 {
		return 0
	}
	return available
}

func (l *readingLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	width := l.columnWidth(size.Width)
	x := (size.Width - width) / 2
	if x < 0 {
		x = 0
	}

	y := float32(0)
	targetY := float32(-1)
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		// Resize to the column width first so wrapping content reflows, then
		// read the resulting height.
		o.Resize(fyne.NewSize(width, o.MinSize().Height))
		h := o.MinSize().Height
		o.Resize(fyne.NewSize(width, h))
		o.Move(fyne.NewPos(x, y))
		if o == l.target {
			targetY = y
		}
		y += h + l.spacing
	}

	if l.scroll != nil && l.target != nil && targetY >= 0 && !l.scrolled {
		offset := targetY - 32
		if offset < 0 {
			offset = 0
		}
		l.scroll.Offset = fyne.NewPos(0, offset)
		l.scrolled = true
		l.scroll.Refresh()
	}
}

func (l *readingLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var width, height float32
	visible := 0
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		m := o.MinSize()
		if m.Width > width {
			width = m.Width
		}
		height += m.Height
		visible++
	}
	if visible > 1 {
		height += float32(visible-1) * l.spacing
	}
	return fyne.NewSize(width, height)
}

func indexOf(values []int, target int) int {
	for i, v := range values {
		if v == target {
			return i
		}
	}
	return -1
}

// --- Paragraph grouping -----------------------------------------------------

func groupVersesIntoParagraphs(verses []Verse) [][]Verse {
	if len(verses) == 0 {
		return nil
	}

	paragraphs := make([][]Verse, 0, len(verses)/4+1)
	current := make([]Verse, 0, 6)
	charCount := 0

	for i, verse := range verses {
		if len(current) > 0 {
			prev := current[len(current)-1]
			if shouldBreakParagraph(prev.Text, charCount) {
				paragraphs = append(paragraphs, current)
				current = make([]Verse, 0, 6)
				charCount = 0
			}
		}
		current = append(current, verse)
		charCount += len([]rune(verse.Text)) + 1

		if i == len(verses)-1 && len(current) > 0 {
			paragraphs = append(paragraphs, current)
		}
	}
	return paragraphs
}

func shouldBreakParagraph(prevVerseText string, currentParagraphChars int) bool {
	if currentParagraphChars < 320 {
		return false
	}
	trimmed := strings.TrimSpace(prevVerseText)
	return strings.HasSuffix(trimmed, ".") ||
		strings.HasSuffix(trimmed, "!") ||
		strings.HasSuffix(trimmed, "?") ||
		strings.HasSuffix(trimmed, "\"") ||
		strings.HasSuffix(trimmed, "'")
}

func superscriptNumber(n int) string {
	if n <= 0 {
		return ""
	}
	mapper := map[rune]rune{
		'0': '⁰', '1': '¹', '2': '²', '3': '³', '4': '⁴',
		'5': '⁵', '6': '⁶', '7': '⁷', '8': '⁸', '9': '⁹',
	}
	var b strings.Builder
	for _, d := range fmt.Sprintf("%d", n) {
		if s, ok := mapper[d]; ok {
			b.WriteRune(s)
		}
	}
	return b.String()
}

// --- Chapter picker ---------------------------------------------------------

func showChapterPicker(anchor fyne.CanvasObject, state *AppState) {
	chapterNumbers := state.Bible.GetChapterNumbersForBook(state.CurrentBook)
	if len(chapterNumbers) == 0 {
		return
	}
	cnv := canvasForObject(anchor)
	if cnv == nil {
		return
	}
	pal := state.pal()

	var popup *widget.PopUp
	columns := chapterPickerColumns(len(chapterNumbers))
	grid := container.NewGridWithColumns(columns)
	for _, chapter := range chapterNumbers {
		ch := chapter
		btn := widget.NewButton(fmt.Sprintf("%d", ch), func() {
			state.CurrentChapter = ch
			clearHighlightedVerse(state)
			addRecentChapter(state, state.CurrentBook, state.CurrentChapter)
			if popup != nil {
				popup.Hide()
			}
			state.refresh()
		})
		if ch == state.CurrentChapter {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.LowImportance
		}
		grid.Add(btn)
	}

	titleText := canvas.NewText(state.CurrentBook, pal.Text)
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	titleText.TextSize = 18
	subText := canvas.NewText(fmt.Sprintf("%d chapters", len(chapterNumbers)), pal.TextMuted)
	subText.TextSize = 12

	closeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		if popup != nil {
			popup.Hide()
		}
	})
	closeBtn.Importance = widget.LowImportance

	head := container.NewBorder(nil, nil,
		container.NewVBox(titleText, subText),
		container.NewVBox(closeBtn, layout.NewSpacer()),
	)
	body := container.NewVBox(head, widget.NewSeparator())

	rows := int(math.Ceil(float64(len(chapterNumbers)) / float64(columns)))
	maxHeight := float32(520)
	if _, size := cnv.InteractiveArea(); size.Height > 0 {
		maxHeight = size.Height * 0.7
	}
	if gridHeight := float32(rows) * 46; gridHeight <= maxHeight {
		body.Add(grid)
	} else {
		gs := container.NewVScroll(grid)
		gs.SetMinSize(fyne.NewSize(float32(columns)*52, maxHeight))
		body.Add(gs)
	}

	popup = widget.NewModalPopUp(surface(container.NewPadded(body), pal.Surface, pal.Border, fyne.Size{}), cnv)
	popup.Show()
}

func chapterPickerColumns(total int) int {
	if total <= 0 {
		return 1
	}
	columns := int(math.Ceil(math.Sqrt(float64(total))))
	if columns < 2 {
		columns = 2
	}
	if columns > 8 {
		columns = 8
	}
	return columns
}

func canvasForObject(obj fyne.CanvasObject) fyne.Canvas {
	driver := fyne.CurrentApp().Driver()
	if driver == nil {
		return nil
	}
	if cnv := driver.CanvasForObject(obj); cnv != nil {
		return cnv
	}
	if windows := driver.AllWindows(); len(windows) > 0 {
		return windows[0].Canvas()
	}
	return nil
}
