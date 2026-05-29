package holybible

import (
	"fmt"
	"image/color"
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

	col := &readingColumn{maxWidth: 760}
	var child fyne.CanvasObject
	var chapter *chapterText
	if len(verses) == 0 {
		msg := widget.NewLabel("No verses are available for this chapter yet.")
		msg.Wrapping = fyne.TextWrapWord
		child = msg
	} else {
		// One widget for the whole chapter, so selection and copy span the entire
		// passage, not just a single paragraph.
		chapter = newChapterText(state, verses)
		col.chapter = chapter
		child = chapter
	}

	scroll := container.NewVScroll(container.New(col, child))
	col.scroll = scroll
	if chapter != nil {
		chapter.parentScroll = scroll
	}

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

// chapterText renders an entire chapter as one read-only, selectable text block.
// A single widget means selection (and copy) spans the whole chapter, not just a
// paragraph. It uses Wrapping=Off + Scroll=None so Fyne creates no inner scroll
// area: the block grows to its full height and reads like a printed page, while
// the surrounding page scroll handles movement. Wrapping is performed manually
// and redone on resize, so it stays responsive.
type chapterText struct {
	widget.Entry

	paragraphs   [][]Verse
	highlightRef VerseRef
	hasHighlight bool
	clipboard    fyne.Clipboard
	parentScroll *container.Scroll

	lastWidth     float32
	highlightLine int // line of the highlighted verse after wrapping (-1 = none)
	totalLines    int
}

// entryScrollNone is widget.ScrollNone, assignable to Entry.Scroll as an untyped
// constant (the field's type lives in an internal package).
const entryScrollNone = 3

func newChapterText(state *AppState, verses []Verse) *chapterText {
	c := &chapterText{
		paragraphs:    groupVersesIntoParagraphs(verses),
		highlightLine: -1,
	}
	if state.HasHighlightedVerse {
		c.hasHighlight = true
		c.highlightRef = VerseRef{Book: state.HighlightedBook, Chapter: state.HighlightedChapter, Verse: state.HighlightedVerse}
	}
	if state.window != nil {
		c.clipboard = state.window.Clipboard()
	}
	c.ExtendBaseWidget(c)
	c.MultiLine = true
	c.Wrapping = fyne.TextWrapOff
	c.Scroll = entryScrollNone // no internal scroll area is created
	c.rewrap(720)              // initial; corrected once the real width is known
	return c
}

// rewrap lays the chapter out to the given width by inserting line breaks: a
// single newline for a soft wrap and a blank line between paragraphs. It records
// the line where the highlighted verse begins so it can be scrolled into view.
func (c *chapterText) rewrap(width float32) {
	avail := width - 4*theme.InnerPadding()
	if avail < 80 {
		avail = 80
	}
	textSize := theme.TextSize()
	var style fyne.TextStyle
	spaceW := fyne.MeasureText(" ", textSize, style).Width

	c.highlightLine = -1
	lineNo := 0
	paras := make([]string, 0, len(c.paragraphs))

	for pi, para := range c.paragraphs {
		if pi > 0 {
			lineNo++ // the blank line produced by joining paragraphs with "\n\n"
		}
		var lines []string
		var cur strings.Builder
		curW := float32(0)
		for _, v := range para {
			if c.hasHighlight && refOf(v) == c.highlightRef {
				c.highlightLine = lineNo + len(lines)
			}
			for _, w := range verseTokens(v) {
				ww := fyne.MeasureText(w, textSize, style).Width
				add := ww
				if cur.Len() > 0 {
					add += spaceW
				}
				if cur.Len() > 0 && curW+add > avail {
					lines = append(lines, cur.String())
					cur.Reset()
					cur.WriteString(w)
					curW = ww
				} else {
					if cur.Len() > 0 {
						cur.WriteString(" ")
					}
					cur.WriteString(w)
					curW += add
				}
			}
		}
		if cur.Len() > 0 {
			lines = append(lines, cur.String())
		}
		lineNo += len(lines)
		paras = append(paras, strings.Join(lines, "\n"))
	}

	c.totalLines = lineNo + 1
	c.Entry.SetText(strings.Join(paras, "\n\n"))
}

// verseTokens splits a verse into wrap tokens, keeping the superscript number
// attached to the first word so a number never wraps onto its own line.
func verseTokens(v Verse) []string {
	words := strings.Fields(strings.TrimSpace(v.Text))
	num := superscriptNumber(v.Verse)
	if num == "" {
		return words
	}
	if len(words) == 0 {
		return []string{num}
	}
	words[0] = num + " " + words[0]
	return words
}

// Resize re-wraps to the new width (responsive) before laying out.
func (c *chapterText) Resize(size fyne.Size) {
	if size.Width > 1 && size.Width != c.lastWidth {
		c.lastWidth = size.Width
		c.rewrap(size.Width)
	}
	c.Entry.Resize(size)
}

// highlightY is the approximate Y of the highlighted verse, for scroll-to.
func (c *chapterText) highlightY() float32 {
	if c.highlightLine < 0 || c.totalLines <= 0 {
		return 0
	}
	return float32(c.highlightLine) / float32(c.totalLines) * c.MinSize().Height
}

// Read-only: ignore typed input but keep cursor movement, selection and copy.
func (c *chapterText) TypedRune(rune) {}

func (c *chapterText) TypedKey(key *fyne.KeyEvent) {
	switch key.Name {
	case fyne.KeyLeft, fyne.KeyRight, fyne.KeyUp, fyne.KeyDown,
		fyne.KeyHome, fyne.KeyEnd, fyne.KeyPageUp, fyne.KeyPageDown:
		c.Entry.TypedKey(key)
	}
}

func (c *chapterText) TypedShortcut(sc fyne.Shortcut) {
	switch sc.(type) {
	case *fyne.ShortcutCopy:
		// Copy clean text: drop the soft wraps we inserted, keep paragraph breaks.
		if c.clipboard != nil {
			c.clipboard.SetContent(cleanCopy(c.SelectedText()))
			return
		}
		c.Entry.TypedShortcut(sc)
	case *fyne.ShortcutSelectAll:
		c.Entry.TypedShortcut(sc)
	}
}

// cleanCopy turns soft-wrap newlines back into spaces while preserving the blank
// line between paragraphs, so copied passages read naturally.
func cleanCopy(s string) string {
	const para = "\x00"
	s = strings.ReplaceAll(s, "\n\n", para)
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, para, "\n\n")
}

// Scrolled forwards the wheel to the page so the whole chapter scrolls.
func (c *chapterText) Scrolled(ev *fyne.ScrollEvent) {
	if c.parentScroll != nil {
		c.parentScroll.Scrolled(ev)
	}
}

func (c *chapterText) CreateRenderer() fyne.WidgetRenderer {
	return &plainEntryRenderer{base: c.Entry.CreateRenderer()}
}

// plainEntryRenderer strips the entry's box and border so the text reads as prose.
type plainEntryRenderer struct{ base fyne.WidgetRenderer }

func (r *plainEntryRenderer) Destroy()                     { r.base.Destroy() }
func (r *plainEntryRenderer) Objects() []fyne.CanvasObject { return r.base.Objects() }
func (r *plainEntryRenderer) Layout(size fyne.Size)        { r.base.Layout(size); r.makePlain() }
func (r *plainEntryRenderer) Refresh()                     { r.base.Refresh(); r.makePlain() }

func (r *plainEntryRenderer) MinSize() fyne.Size {
	m := r.base.MinSize()
	if trim := theme.InputBorderSize() * 2; m.Height > trim {
		m.Height -= trim
	}
	return m
}

func (r *plainEntryRenderer) makePlain() {
	objs := r.base.Objects()
	if len(objs) < 2 {
		return
	}
	if box, ok := objs[0].(*canvas.Rectangle); ok {
		box.FillColor = color.Transparent
		box.CornerRadius = 0
		canvas.Refresh(box)
	}
	if border, ok := objs[1].(*canvas.Rectangle); ok {
		border.StrokeColor = color.Transparent
		border.StrokeWidth = 0
		border.CornerRadius = 0
		canvas.Refresh(border)
	}
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

// readingColumn centres its single child and caps the line length for
// comfortable reading. When the child is a chapterText with a highlighted verse,
// it scrolls that verse into view during layout — on the render thread, so there
// is no goroutine and no data race.
type readingColumn struct {
	maxWidth float32
	scroll   *container.Scroll
	chapter  *chapterText
	scrolled bool
}

func (l *readingColumn) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	child := objects[0]

	w := size.Width
	if w > l.maxWidth {
		w = l.maxWidth
	}
	if w < 0 {
		w = 0
	}
	x := (size.Width - w) / 2
	if x < 0 {
		x = 0
	}

	// First resize sets the width so wrapping content reflows; then size to the
	// resulting height.
	child.Resize(fyne.NewSize(w, child.MinSize().Height))
	child.Resize(fyne.NewSize(w, child.MinSize().Height))
	child.Move(fyne.NewPos(x, 0))

	if l.scroll != nil && l.chapter != nil && l.chapter.highlightLine >= 0 && !l.scrolled {
		y := l.chapter.highlightY() - 24
		if y < 0 {
			y = 0
		}
		l.scroll.Offset = fyne.NewPos(0, y)
		l.scrolled = true
		l.scroll.Refresh()
	}
}

func (l *readingColumn) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.Size{}
	}
	// Report height only. The width is deliberately 0: the child is a Wrapping=Off
	// text whose MinSize width is its longest line. If that fed upward, the
	// enclosing HSplit would size its divider from it (split.go computeSplitLengths
	// clamps to leading/trailing minimums), and a transient narrow layout could
	// feed back and let the sidebar expand to fill the window. The real layout
	// width comes from the parent in Layout, not from here.
	return fyne.NewSize(0, objects[0].MinSize().Height)
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

	// On iOS the reading view is a native UITextView overlay that floats above
	// the Fyne canvas, so it would render on top of (and steal touches from)
	// this popup. Hide it while the picker is open; restore it on dismiss.
	// No-op on desktop/Android.
	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	restoreOverlay := func() {
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	}

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
			restoreOverlay()
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
		restoreOverlay()
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
