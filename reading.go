package bibletext

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
// is active, otherwise the chapter reading view. On macOS the reading text is a
// native NSTextView overlay; setReadingOverlayVisible hides it while the search
// results are shown (no-op on other platforms).
func buildReadingPane(state *AppState) fyne.CanvasObject {
	if state.IsSearching {
		setReadingOverlayVisible(false)
		return buildSearchResultsView(state)
	}
	setReadingOverlayVisible(true)
	return buildReadingView(state)
}

func buildReadingView(state *AppState) fyne.CanvasObject {
	chapterNumbers := state.Bible.GetChapterNumbersForBook(state.CurrentBook)
	normalizeCurrentChapter(state, chapterNumbers)
	verses := state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter)

	// The scrollable text area is platform-specific: a Fyne chapterText (with
	// drag-selection) on Linux/Windows, and a native NSTextView overlay (with
	// the system selection menu) on macOS — see reading_fyne.go / reading_macos.go.
	paper := readingScrollArea(state, verses, state.pal())

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

// chapterHeader renders the book + chapter heading, a small inline copy icon,
// the chapter picker with prev/next arrows clustered beside it, and a focus
// (distraction-free) toggle on the right, followed by a divider. It mirrors the
// iOS chapter toolbar, adapted to desktop sizing.
//
//	┌─────────────────────────────────────────────────────┐
//	│ Genesis 1 ⧉                                    ⤢    │
//	│ Chapter 1 of 50 ▾   ←  →                            │
//	└─────────────────────────────────────────────────────┘
func chapterHeader(state *AppState, chapterNumbers []int) fyne.CanvasObject {
	pal := state.pal()
	total := len(chapterNumbers)

	// Heading reflects the chapter: "Genesis 1".
	title := canvas.NewText(fmt.Sprintf("%s %d", state.CurrentBook, state.CurrentChapter), pal.Text)
	title.TextSize = headingTextSize
	title.TextStyle = fyne.TextStyle{Bold: true}

	// Small copy icon tucked beside the heading — close to the text it copies.
	copyBtn := newIconTapButton(state, theme.ContentCopyIcon(), 17, 36, func() {
		copyChapter(state)
	})
	titleRow := container.NewHBox(title, hgap(6), copyBtn)

	const navBoxH = 24

	var chapterLine fyne.CanvasObject
	if total > 1 {
		chapterLine = newChapterPickerAnchor(state,
			fmt.Sprintf("Chapter %d of %d  ▾", state.CurrentChapter, total),
			pal.TextMuted, subheadingTextSize, navBoxH)
	} else {
		lbl := canvas.NewText(fmt.Sprintf("Chapter %d", state.CurrentChapter), pal.TextMuted)
		lbl.TextSize = subheadingTextSize
		chapterLine = container.NewCenter(lbl)
	}

	idx := indexOf(chapterNumbers, state.CurrentChapter)

	prev := newIconTapButton(state, theme.NavigateBackIcon(), 20, navBoxH, func() {
		if moveChapter(state, -1) {
			state.refresh()
		}
	})
	prev.disabled = idx <= 0

	next := newIconTapButton(state, theme.NavigateNextIcon(), 20, navBoxH, func() {
		if moveChapter(state, 1) {
			state.refresh()
		}
	})
	next.disabled = idx < 0 || idx >= total-1

	// The chapter line and arrows sit directly in the HBox (no spacer-VBox
	// wrapper): each control carries its own boxH so they share a baseline, and
	// the picker anchor needs a first-class hit box rather than a nested one.
	chapterRow := container.NewHBox(chapterLine, hgap(12), prev, next)

	// Focus toggle on the right: enter distraction-free reading (hide the
	// sidebar + app header) or, when already in it, restore the full layout.
	focusIcon := theme.ViewFullScreenIcon()
	if state.IsFullScreen {
		focusIcon = theme.ViewRestoreIcon()
	}
	focusBtn := widget.NewButtonWithIcon("", focusIcon, func() {
		state.IsFullScreen = !state.IsFullScreen
		rebuildWindow(state)
	})
	focusBtn.Importance = widget.LowImportance

	left := container.NewVBox(titleRow, chapterRow)
	right := container.NewVBox(layout.NewSpacer(), focusBtn, layout.NewSpacer())
	row := container.NewBorder(nil, nil, left, right, nil)

	rule := canvas.NewLine(pal.Border)
	rule.StrokeWidth = 1
	return container.NewVBox(row, rule)
}

// --- Shared compact toolbar controls -----------------------------------------
//
// iconTapButton and chapterPickerAnchor were first written for the iOS reading
// header; they're shared here so the desktop chapter toolbar can use the same
// small, low-chrome controls.

// iconTapButton is a small, low-chrome tappable icon — lighter than
// widget.Button (no background, no fixed padding). The icon is rendered at
// iconSize, centred inside a box of boxH height so it can line up vertically
// with adjacent text of a different size. A disabled button renders faint and
// ignores taps.
type iconTapButton struct {
	widget.BaseWidget
	state    *AppState
	icon     fyne.Resource
	iconSize float32
	boxH     float32
	disabled bool
	onTapped func()
}

func newIconTapButton(state *AppState, icon fyne.Resource, iconSize, boxH float32, onTapped func()) *iconTapButton {
	b := &iconTapButton{state: state, icon: icon, iconSize: iconSize, boxH: boxH, onTapped: onTapped}
	b.ExtendBaseWidget(b)
	return b
}

func (b *iconTapButton) Tapped(*fyne.PointEvent) {
	if b.disabled || b.onTapped == nil {
		return
	}
	b.onTapped()
}

func (b *iconTapButton) CreateRenderer() fyne.WidgetRenderer {
	img := canvas.NewImageFromResource(theme.NewColoredResource(b.icon, colorNameMuted))
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(b.iconSize, b.iconSize))
	if b.disabled {
		img.Translucency = 0.6 // faint when there's no chapter to move to
	}
	// GridWrap pins the cell to a fixed size; NewCenter vertically centres the
	// smaller icon within that box so it aligns with neighbouring text.
	box := container.NewGridWrap(fyne.NewSize(b.iconSize+8, b.boxH), container.NewCenter(img))
	return widget.NewSimpleRenderer(box)
}

var _ fyne.Tappable = (*iconTapButton)(nil)

// chapterPickerAnchor is a small tappable bit of muted text (e.g.
// "Chapter 1 of 21 ▾") that opens the chapter picker on tap. It avoids
// widget.Button's relatively heavy padding so the chapter line stays as
// quiet as the "World English Bible · Public Domain" subtitle.
type chapterPickerAnchor struct {
	widget.BaseWidget
	state *AppState
	text  string
	tint  color.NRGBA
	size  float32
	boxH  float32
	lbl   *canvas.Text
}

func newChapterPickerAnchor(state *AppState, text string, tint color.NRGBA, size, boxH float32) *chapterPickerAnchor {
	a := &chapterPickerAnchor{state: state, text: text, tint: tint, size: size, boxH: boxH}
	a.ExtendBaseWidget(a)
	return a
}

func (a *chapterPickerAnchor) CreateRenderer() fyne.WidgetRenderer {
	a.lbl = canvas.NewText(a.text, a.tint)
	a.lbl.TextSize = a.size
	a.lbl.TextStyle = fyne.TextStyle{Bold: true}
	// Mirror iconTapButton: pin the text inside a fixed-size GridWrap cell so the
	// widget has a solid, full-height hit rectangle. A bare canvas.Text renderer
	// is not reliably matched by Fyne's mobile-driver tap hit-test, which left the
	// chapter picker unresponsive on iOS; the explicit box fixes it and also
	// vertically centres the small text against the taller nav arrows.
	w := fyne.MeasureText(a.text, a.size, a.lbl.TextStyle).Width
	box := container.NewGridWrap(fyne.NewSize(w, a.boxH), container.NewCenter(a.lbl))
	return widget.NewSimpleRenderer(box)
}

func (a *chapterPickerAnchor) Tapped(*fyne.PointEvent) {
	showChapterPicker(a, a.state)
}

// Make sure Fyne dispatches taps to us.
var _ fyne.Tappable = (*chapterPickerAnchor)(nil)

// rebuildWindow swaps in a fresh CreateMainUI tree. Use this (rather than
// state.refresh(), which only repaints the reading pane) when a change affects
// the whole window chrome — e.g. entering or leaving the distraction-free
// reading mode, which hides/shows the sidebar (desktop) or bottom tabs and
// header (mobile). afterRebuild is a build-tagged hook: a no-op on desktop,
// and an overlay re-pin on iOS.
func rebuildWindow(state *AppState) {
	if state.app == nil || state.window == nil {
		return
	}
	state.window.SetContent(CreateMainUI(state.app, state, state.window))
	afterRebuild(state)
}

// --- Native-overlay chapter HTML (iOS UITextView + macOS NSTextView) ---------
//
// buildChapterHTML emits an HTML document that the AppKit/UIKit HTML importer
// turns into a richly-styled attributed string for the native text overlay
// (shared by the iOS and macOS reading views). All colours are inlined so
// light/dark mode tracks the active palette on every rebuild.
//
// The font stack leads with Georgia — a warm, screen-optimised book serif that
// is present on both macOS and iOS and matches the desktop chrome — with Iowan
// Old Style and Times as fallbacks. Generous line-height + paragraph spacing
// give an unhurried, page-of-a-book feel; kerning + ligatures + old-style
// numerals add a faint warmth.
func buildChapterHTML(state *AppState, verses []Verse) string {
	pal := state.pal()
	textHex := nrgbaToHex(pal.Text)
	numHex := nrgbaToHex(pal.VerseNumber)
	highlightTextHex := nrgbaToHex(pal.HighlightText)
	highlightBgHex := nrgbaToHex(pal.Highlight)

	var b strings.Builder
	b.WriteString("<html><head><style>")
	fmt.Fprintf(&b, `body {
		font-family: Georgia, "Iowan Old Style", "Times New Roman", serif;
		font-size: 19px;
		color: %s;
		line-height: 1.72;
		letter-spacing: 0.004em;
		margin: 0; padding: 0;
		-webkit-text-size-adjust: 100%%;
		-webkit-font-smoothing: antialiased;
		font-feature-settings: "kern" 1, "liga" 1, "calt" 1, "onum" 1;
	}`, textHex)
	fmt.Fprintf(&b, `p {
		margin: 0 0 24px 0;
		text-align: left;
		hyphens: auto;
		-webkit-hyphens: auto;
	}`)
	fmt.Fprintf(&b, `sup.v {
		color: %s;
		font-weight: 600;
		font-size: 0.66em;
		letter-spacing: 0;
		margin-right: 2px;
	}`, numHex)
	fmt.Fprintf(&b, `.hl {
		color: %s;
		background-color: %s;
		font-weight: 600;
		padding: 0 2px;
		border-radius: 2px;
	}`, highlightTextHex, highlightBgHex)
	b.WriteString("</style></head><body>")

	for _, para := range groupVersesIntoParagraphs(verses) {
		b.WriteString("<p>")
		for i, v := range para {
			if i > 0 {
				b.WriteByte(' ')
			}
			fmt.Fprintf(&b, `<sup class="v">%d</sup>&nbsp;`, v.Verse)
			body := htmlEscape(strings.TrimSpace(strings.ReplaceAll(v.Text, "\n", " ")))
			if isVerseHighlighted(state, v) {
				fmt.Fprintf(&b, `<span class="hl">%s</span>`, body)
			} else {
				b.WriteString(body)
			}
		}
		b.WriteString("</p>")
	}
	b.WriteString("</body></html>")
	return b.String()
}

// nrgbaToHex formats an image/color.NRGBA as a #RRGGBB string for CSS.
func nrgbaToHex(c color.NRGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// htmlEscape escapes the characters that would break out of a content span.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
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
