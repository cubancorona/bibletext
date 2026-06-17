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

	// "Book N ⌄" — one cohesive tap target (text + a clear dropdown chevron) that
	// opens the combined reference picker (book list + chapter grid).
	const titleBoxH = 38
	ref := newReferenceButton(fmt.Sprintf("%s %d", state.CurrentBook, state.CurrentChapter), pal.Text, headingTextSize, titleBoxH, func() {
		showReferencePicker(state)
	})

	// Small copy icon tucked beside the heading — close to the text it copies.
	copyBtn := newIconTapButton(state, theme.ContentCopyIcon(), 17, titleBoxH, func() {
		copyChapter(state)
	})
	titleRow := container.NewHBox(ref, hgap(8), copyBtn)

	const navBoxH = 34

	// Quiet chapter context below the heading — also a picker target, so the
	// whole "Chapter N of M" line opens the picker too.
	chapText := fmt.Sprintf("Chapter %d of %d", state.CurrentChapter, total)
	if total <= 1 {
		chapText = fmt.Sprintf("Chapter %d", state.CurrentChapter)
	}
	chapterLine := newTapTextStyled(chapText, pal.TextMuted, subheadingTextSize, navBoxH, false, func() {
		showReferencePicker(state)
	})

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
	chapterRow := container.NewHBox(chapterLine, hgap(8), prev, next)

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

// minTapTarget is the smallest comfortable touch target (Apple HIG ~44pt). The
// reading header passes it as the box height for the small icon buttons (the
// chapter arrows, the copy icon) so they're easy to hit on a phone; the picker
// text anchors get generous horizontal padding (tapTextHPad) for the same reason.
const (
	minTapTarget float32 = 44
	tapTextHPad  float32 = 18
)

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
	// smaller icon within that box so it aligns with neighbouring text. The cell
	// is at least as wide as it is tall, so a small glyph still gets a square,
	// finger-friendly hit area rather than a thin sliver.
	w := b.iconSize + 16
	if w < b.boxH {
		w = b.boxH
	}
	box := container.NewGridWrap(fyne.NewSize(w, b.boxH), container.NewCenter(img))
	return widget.NewSimpleRenderer(box)
}

var _ fyne.Tappable = (*iconTapButton)(nil)

// tapText is a small tappable bit of bold text with a solid GridWrap hit box.
// A bare canvas.Text renderer is not reliably matched by Fyne's mobile-driver
// tap hit-test (it once left the chapter picker unresponsive on iOS); pinning
// the text inside a fixed-size cell gives a full-height hit rectangle and
// vertically centres it against taller neighbouring controls. Used for the
// tappable book name and chapter number in the reading header.
type tapText struct {
	widget.BaseWidget
	text  string
	tint  color.NRGBA
	size  float32
	boxH  float32
	bold  bool
	onTap func()
}

// newTapText makes a bold tappable label (the book + chapter heading).
func newTapText(text string, tint color.NRGBA, size, boxH float32, onTap func()) *tapText {
	return newTapTextStyled(text, tint, size, boxH, true, onTap)
}

// newTapTextStyled is newTapText with control over the weight, so the quiet
// "Chapter N of M" line can be tappable without going bold.
func newTapTextStyled(text string, tint color.NRGBA, size, boxH float32, bold bool, onTap func()) *tapText {
	t := &tapText{text: text, tint: tint, size: size, boxH: boxH, bold: bold, onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tapText) CreateRenderer() fyne.WidgetRenderer {
	lbl := canvas.NewText(t.text, t.tint)
	lbl.TextSize = t.size
	lbl.TextStyle = fyne.TextStyle{Bold: t.bold}
	// Pad the hit box well beyond the glyphs so the heading is an easy phone
	// target, not a thin strip the width of the text.
	w := fyne.MeasureText(t.text, t.size, lbl.TextStyle).Width + tapTextHPad
	box := container.NewGridWrap(fyne.NewSize(w, t.boxH), container.NewCenter(lbl))
	return widget.NewSimpleRenderer(box)
}

func (t *tapText) Tapped(*fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

// Make sure Fyne dispatches taps to us.
var _ fyne.Tappable = (*tapText)(nil)

// referenceButton is the tappable book+chapter heading ("John 10 ⌄") that opens
// the combined reference picker. It renders the bold reference text followed
// immediately by a clear, full-size dropdown chevron — one cohesive, unambiguous
// affordance in a single comfortable hit box. (It replaces a heading + a separate
// tiny muted caret, which read as a small ambiguous mark floating a wide gap from
// the text.)
type referenceButton struct {
	widget.BaseWidget
	text  string
	tint  color.NRGBA
	size  float32
	boxH  float32
	onTap func()
}

func newReferenceButton(text string, tint color.NRGBA, size, boxH float32, onTap func()) *referenceButton {
	b := &referenceButton{text: text, tint: tint, size: size, boxH: boxH, onTap: onTap}
	b.ExtendBaseWidget(b)
	return b
}

func (b *referenceButton) Tapped(*fyne.PointEvent) {
	if b.onTap != nil {
		b.onTap()
	}
}

func (b *referenceButton) CreateRenderer() fyne.WidgetRenderer {
	style := fyne.TextStyle{Bold: true}
	lbl := canvas.NewText(b.text, b.tint)
	lbl.TextSize = b.size
	lbl.TextStyle = style
	textW := fyne.MeasureText(b.text, b.size, style).Width

	// A solid dropdown chevron in the heading colour — far clearer as a "tap to
	// change book/chapter" affordance than a tiny muted caret. Sized generously so
	// the visible glyph reads big (the icon SVG carries some internal whitespace).
	chevSize := b.size * 0.8
	chev := canvas.NewImageFromResource(theme.NewColoredResource(theme.MenuDropDownIcon(), theme.ColorNameForeground))
	chev.FillMode = canvas.ImageFillContain
	chev.SetMinSize(fyne.NewSize(chevSize, chevSize))

	// Text then chevron, tight (one theme pad between them — not a wide gap), with
	// just a little symmetric breathing room as the hit box.
	inner := container.NewHBox(container.NewCenter(lbl), container.NewCenter(chev))
	w := textW + chevSize + theme.Padding() + 8
	box := container.NewGridWrap(fyne.NewSize(w, b.boxH), container.NewCenter(inner))
	return widget.NewSimpleRenderer(box)
}

var _ fyne.Tappable = (*referenceButton)(nil)

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

// lastPushedChapterFP is the fingerprint of the chapter HTML currently held by
// the native reading overlay (iOS UITextView / macOS NSTextView). The native
// push paths skip rebuilding + re-importing the HTML when it hasn't changed —
// re-parsing the whole chapter through the NSAttributedString HTML importer is
// the single most frequent expensive op (it fires on every tab-return-to-Read
// and same-chapter refresh). Written only from the UI goroutine; unused on
// platforms without a native overlay (Linux/Windows/Android).
var lastPushedChapterFP string

// chapterRenderFingerprint captures everything buildChapterHTML's output depends
// on, so the native push can detect a no-op. It MUST include the theme variant
// (colours are inlined) and the highlight identity (arriving at the same chapter
// from a search hit vs. prev/next is the same book+chapter but renders the
// highlighted verse differently).
func chapterRenderFingerprint(state *AppState) string {
	var variant fyne.ThemeVariant
	if app := fyne.CurrentApp(); app != nil {
		variant = app.Settings().ThemeVariant()
	}
	red := 0
	if redLetterEnabled() {
		red = 1
	}
	hl := "0"
	if state.HasHighlightedVerse {
		hl = fmt.Sprintf("%s:%d:%d", state.HighlightedBook, state.HighlightedChapter, state.HighlightedVerse)
	}
	return fmt.Sprintf("%s|%s|%d|v%d|r%d|h%s",
		state.CurrentVersion, state.CurrentBook, state.CurrentChapter, variant, red, hl)
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
	redLetterHex := nrgbaToHex(pal.RedLetter)
	redLetter := redLetterEnabled()

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
	fmt.Fprintf(&b, `.wj { color: %s; }`, redLetterHex)
	b.WriteString("</style></head><body>")

	for _, para := range groupVersesIntoParagraphs(verses) {
		b.WriteString("<p>")
		for i, v := range para {
			if i > 0 {
				b.WriteByte(' ')
			}
			fmt.Fprintf(&b, `<sup class="v">%d</sup>&nbsp;`, v.Verse)
			body := htmlEscape(strings.TrimSpace(strings.ReplaceAll(v.Text, "\n", " ")))
			switch {
			case isVerseHighlighted(state, v):
				// A search highlight wins visually over red-letter.
				fmt.Fprintf(&b, `<span class="hl">%s</span>`, body)
			case redLetter && isWordsOfChrist(v.BookName, v.Chapter, v.Verse):
				fmt.Fprintf(&b, `<span class="wj">%s</span>`, body)
			default:
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

// --- Reference picker -------------------------------------------------------
//
// One combined book + chapter picker on a single screen, opened from the reading
// header (tapping the book name or the chapter number): a scrollable book list
// on the left and a calendar-style chapter-number grid on the right that updates
// as you select a book. Tapping a chapter navigates there. Shared by desktop and
// iOS via the same header.

// fixedWidthLayout pins its content to a fixed width while filling the available
// height — used for the picker's left-hand book column.
type fixedWidthLayout struct{ width float32 }

func (f fixedWidthLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	h := float32(0)
	for _, o := range objs {
		if m := o.MinSize(); m.Height > h {
			h = m.Height
		}
	}
	return fyne.NewSize(f.width, h)
}

func (f fixedWidthLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Resize(fyne.NewSize(f.width, size.Height))
		o.Move(fyne.NewPos(0, 0))
	}
}

func showReferencePicker(state *AppState) {
	cnv := pickerCanvas(state)
	if cnv == nil {
		return
	}
	pal := state.pal()

	// On iOS the reading view is a native UITextView overlay that floats above
	// the Fyne canvas, so it would cover (and steal touches from) this popup.
	// Hide it while the picker is open; restore on dismiss. No-op elsewhere.
	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	restore := func() {
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	}

	var popup *widget.PopUp
	closePicker := func() {
		if popup != nil {
			popup.Hide()
		}
		restore()
	}

	books := state.Bible.Books
	selected := state.CurrentBook

	// Right pane: the chapter grid for the currently-selected book.
	chapterPane := container.NewStack()
	renderChapters := func(book string) {
		chapterPane.Objects = []fyne.CanvasObject{referenceChapterGrid(state, pal, book, func(ch int) {
			navigateToReference(state, book, ch)
			closePicker()
		})}
		chapterPane.Refresh()
	}
	renderChapters(selected)

	// Left pane: a scrollable list of every book; selecting one swaps the right
	// pane to that book's chapters (without navigating yet).
	list := widget.NewList(
		func() int { return len(books) },
		func() fyne.CanvasObject {
			lbl := widget.NewLabel("")
			lbl.Truncation = fyne.TextTruncateEllipsis
			return lbl
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			lbl := o.(*widget.Label)
			lbl.SetText(books[i])
			if books[i] == selected {
				lbl.TextStyle = fyne.TextStyle{Bold: true}
			} else {
				lbl.TextStyle = fyne.TextStyle{}
			}
			lbl.Refresh()
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(books) {
			return
		}
		selected = books[id]
		renderChapters(selected)
		list.Refresh()
	}

	title := canvas.NewText("Go to", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 18
	header := pickerHeader(title, closePicker)

	divider := canvas.NewRectangle(pal.Border)
	divider.SetMinSize(fyne.NewSize(1, 0))
	left := container.New(fixedWidthLayout{width: 152},
		container.NewBorder(nil, nil, nil, divider, list))
	body := container.NewBorder(header, nil, left, nil, container.NewPadded(chapterPane))

	popup = widget.NewModalPopUp(surface(container.NewPadded(body), pal.Surface, pal.Border, fyne.Size{}), cnv)
	popup.Show()
	w, h := pickerSplitSize(cnv)
	popup.Resize(fyne.NewSize(w, h))

	// Highlight + reveal the current book (its OnSelected refreshes the chapters).
	for i, b := range books {
		if b == selected {
			list.Select(i)
			list.ScrollTo(i)
			break
		}
	}
}

// referenceChapterGrid is the right pane: the chapter-number grid for one book,
// with the book name + chapter count above it. onPick fires with the chapter.
func referenceChapterGrid(state *AppState, pal palette, book string, onPick func(int)) fyne.CanvasObject {
	nums := state.Bible.GetChapterNumbersForBook(book)

	head := canvas.NewText(fmt.Sprintf("%s · %d chapters", book, len(nums)), pal.TextMuted)
	head.TextSize = 12

	// Fixed-size cells that wrap to as many columns as the pane is wide — so the
	// grid fits without clipping on a narrow phone pane and fills out on desktop.
	grid := container.NewGridWrap(fyne.NewSize(46, 40))
	for _, c := range nums {
		ch := c
		btn := widget.NewButton(fmt.Sprintf("%d", ch), func() { onPick(ch) })
		if book == state.CurrentBook && ch == state.CurrentChapter {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.LowImportance
		}
		grid.Add(btn)
	}

	return container.NewBorder(container.NewPadded(head), nil, nil, nil,
		container.NewVScroll(container.NewPadded(grid)))
}

// pickerHeader is a stage's top row: a leading element (title, or back+title) on
// the left and a close button on the right, above a separator.
func pickerHeader(leading fyne.CanvasObject, onClose func()) fyne.CanvasObject {
	closeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), onClose)
	closeBtn.Importance = widget.LowImportance
	bar := container.NewBorder(nil, nil, leading, container.NewVBox(closeBtn, layout.NewSpacer()), nil)
	return container.NewVBox(bar, widget.NewSeparator())
}

// navigateToReference jumps to a specific book + chapter and records the visit.
func navigateToReference(state *AppState, book string, chapter int) {
	selectBook(state, book, false)
	state.CurrentChapter = chapter
	clearHighlightedVerse(state)
	addRecentChapter(state, book, chapter)
	state.refresh()
}

// pickerCanvas returns the canvas to host a picker modal.
func pickerCanvas(state *AppState) fyne.Canvas {
	if state.window != nil {
		return state.window.Canvas()
	}
	if d := fyne.CurrentApp().Driver(); d != nil {
		if ws := d.AllWindows(); len(ws) > 0 {
			return ws[0].Canvas()
		}
	}
	return nil
}

// pickerSplitSize gives the split picker a roomy size (it needs width for the
// book column plus the chapter grid) capped to the screen.
func pickerSplitSize(cnv fyne.Canvas) (float32, float32) {
	w, h := float32(560), float32(560)
	if _, sz := cnv.InteractiveArea(); sz.Width > 0 {
		w = sz.Width - 24
		if w > 640 {
			w = 640
		}
		if w < 320 {
			w = 320
		}
		h = sz.Height * 0.8
		if h > 680 {
			h = 680
		}
		if h < 300 {
			h = 300
		}
	}
	return w, h
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
