package bibletext

// The cross-references panel: a modal that lists the passages related to the
// selected verse(s). It mirrors the AI panel (spinner while the dataset loads on
// first use, then content), and reuses the chapter-picker overlay hide/restore
// dance. Each row is a full tap target that jumps to the passage in context.

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func showCrossRefs(state *AppState, text string, span selSpan) {
	if state == nil || state.window == nil {
		return
	}
	cnv := state.window.Canvas()
	if cnv == nil {
		return
	}
	pal := state.pal()

	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	restore := func() {
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	}

	title := canvas.NewText("Cross-references", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 22
	src := canvas.NewText(citationForSelection(state, text, span), pal.Accent)
	src.TextStyle = fyne.TextStyle{Bold: true}
	src.TextSize = subheadingTextSize
	header := container.NewVBox(title, src, widget.NewSeparator())

	ps := sheetPanelSize(state, cnv)
	bodyW := ps.Width - 44
	listBox := container.NewVBox()
	scroll := container.NewVScroll(listBox)
	body := container.NewStack(scroll)

	var popup *widget.PopUp
	// Stop the spinner on every exit from the thinking state — a running
	// ProgressBarInfinite pins the canvas dirty and repaints the whole tree every
	// frame, which lingers past dismissal and competes with scrolling.
	var thinkingBar *widget.ProgressBarInfinite
	stopThinking := func() {
		if thinkingBar != nil {
			thinkingBar.Stop()
			thinkingBar = nil
		}
	}
	closePanel := func() {
		stopThinking()
		if popup != nil {
			popup.Hide()
		}
		restore()
	}
	closeBtn := widget.NewButton("Close", closePanel)
	// The credit names the sources whose rows stand in the open, and the
	// on-screen edition's own notice when it is licensed: the preview under
	// every row is that edition's verse text, and it used to sit under a line
	// that credited only the reference dataset.
	sources, notice := crossRefFooterCredit(state)
	credit := canvas.NewText(sources, pal.TextMuted)
	credit.TextSize = 11
	footerLines := []fyne.CanvasObject{widget.NewSeparator(), credit}
	if notice != "" {
		footerLines = append(footerLines, crossRefCaption(notice))
	}
	footerLines = append(footerLines, container.NewHBox(layout.NewSpacer(), closeBtn))
	footer := container.NewVBox(footerLines...)

	setCentered := func(o fyne.CanvasObject) {
		body.Objects = []fyne.CanvasObject{container.NewVBox(layout.NewSpacer(), o, layout.NewSpacer())}
		body.Refresh()
	}
	setMessage := func(msg string) {
		stopThinking()
		lbl := widget.NewLabel(msg)
		lbl.Wrapping = fyne.TextWrapWord
		lbl.Alignment = fyne.TextAlignCenter
		setCentered(lbl)
	}
	setThinking := func() {
		// Stop any spinner already running before replacing it, as both sibling
		// transitions do. A ProgressBarInfinite left assigned-over keeps its
		// RepeatForever animation, repainting the whole canvas at ~20fps until
		// GC reclaims it; ui.go documents that as a real defect it once had.
		// Only one call site reaches here today, which is not a reason for the
		// transition to be the one that cannot be called twice.
		stopThinking()
		bar := widget.NewProgressBarInfinite()
		thinkingBar = bar
		msg := widget.NewLabel("Finding related passages…")
		msg.Alignment = fyne.TextAlignCenter
		setCentered(container.NewVBox(msg, bar))
	}
	follow := func(cc crossRef) {
		closePanel()
		followCrossRef(state, cc)
	}
	// fitList sizes the list so the panel is exactly as tall as its cap, the
	// list scrolling within it. The chrome around the list (header, footer,
	// padding, border) is MEASURED, as the AI panel's fitBody measures it: a
	// 150pt guess stood here, 25pt short of the chrome the panel has, and
	// the list stood the panel that much over its cap, which on a desktop
	// window put its top edge inside the header. Only while the list is the
	// body: the chrome is what the panel needs besides it.
	listShowing := func() bool { return len(body.Objects) == 1 && body.Objects[0] == scroll }
	fitList := func() {
		chrome := popup.MinSize().Height - scroll.MinSize().Height
		h := ps.Height - chrome
		if h < 1 {
			h = 1
		}
		scroll.SetMinSize(fyne.NewSize(bodyW, h))
	}
	showRefs := func(refs []crossRef, tskErr error) {
		stopThinking()
		lst := buildCrossRefList(state, selectionVerses(state, text, span), refs, tskErr, pal, follow)
		if len(lst.Objects) == 0 {
			msg := crossRefNoneLine
			if lst.CoverageNote != "" {
				msg += "\n" + lst.CoverageNote
			}
			setMessage(msg)
			return
		}
		listBox.Objects = lst.Objects
		listBox.Refresh()
		body.Objects = []fyne.CanvasObject{scroll}
		fitList()
		body.Refresh()
		scroll.ScrollToTop()
	}

	content := container.NewBorder(header, footer, nil, nil, body)
	popup = widget.NewModalPopUp(
		surface(container.NewPadded(content), pal.SurfaceAlt, pal.Border, fyne.Size{}),
		cnv,
	)
	popup.Show()
	popup.Resize(fyne.NewSize(ps.Width, minF(ps.Height, 460)))
	// Sized again from the height a desktop window now gives it whenever the
	// window changes size, in whichever state it is (sheet_refit.go).
	registerSheetRefit(state, popup, func() {
		ps.Height = sheetPanelSize(state, cnv).Height
		if listShowing() {
			fitList()
		}
		popup.Resize(fyne.NewSize(ps.Width, minF(ps.Height, 460)))
	})
	// The same selection again after a light/dark rebuild. The dataset loads
	// once and is guarded, so a reopen mid-load waits on the same load rather
	// than starting another (sheet_reopen.go).
	registerSheetReopen(state, popup, func() { showCrossRefs(state, text, span) })

	// A selection the cross-references cannot cover at all is answered
	// before anything loads: the answer cannot change, and a reader who is
	// offline must not be told to check the connection for it. The next
	// major release's (crossRefCoverage); the shipping build loads first.
	if nextRelease {
		book, _ := readerChapter(state)
		if note, none := crossRefCoverage(state.currentVersion().ID, book, selectionVerses(state, text, span)); none {
			setMessage(crossRefNoneLine + "\n" + note)
			return
		}
	}

	setThinking()
	crossRefsRun(func() {
		err := crossRefsLoad()
		// crossRefsForSelection always returns the embedded Gospel parallels (offline),
		// plus the TSK cross-references when they loaded — so a TSK fetch failure still
		// shows parallels, and we only surface the error when there's nothing at all.
		refs := crossRefsForSelection(state, text, span)
		fyne.Do(func() {
			// With the edition's own block in the list, a Treasury that failed
			// to load is reported in its own place (buildCrossRefList) and the
			// block still shows — it needs no network.
			if len(refs) == 0 && err != nil && !state.currentVersion().PublisherCrossRefs {
				setMessage("Couldn't load cross-references.\nCheck your connection and try again.")
				return
			}
			showRefs(refs, err)
		})
	})
}

// crossRefsRun runs the panel's load and its landing, on a goroutine of their
// own, and crossRefsLoad is the load: ensureCrossRefs, which may fetch the
// Treasury. Seams for tests, which keep the panel in the state it opened in by
// never running the work, or run it where they stand with the load answered,
// so nothing touches the panel from another goroutine while they read it.
var (
	crossRefsRun  = func(work func()) { go work() }
	crossRefsLoad = ensureCrossRefs
)

// followCrossRef takes the reader to a row's passage and washes what the row
// cites: the whole range, as Go to and the verse of the day wash theirs, not
// only its first verse. A wash lies within one chapter (VerseSpan), so a range
// that runs on into the next chapter or book is washed from its start to the
// end of the chapter it starts in.
func followCrossRef(state *AppState, c crossRef) {
	v := state.Bible.GetVerse(c.Book, c.Chapter, c.Verse)
	if v == nil {
		return
	}
	end := v.Verse
	switch {
	case c.EndV == 0:
	case !c.crossBook() && (c.EndCh == 0 || c.EndCh == c.Chapter):
		end = c.EndV
	default:
		if ch := state.Bible.GetChapter(v.BookName, v.Chapter); len(ch) > 0 {
			end = ch[len(ch)-1].Verse
		}
	}
	goToVerseRange(state, v.BookName, v.Chapter, v.Verse, end)
}

// crossRefPreviewRunes is how much of a row's first verse its preview shows.
const crossRefPreviewRunes = 90

// crossRefPreview is the opening of a row's verse, cut at a word: the cut used
// to fall wherever the 90th character did, mid-word in about half of all rows
// and on a space in another seventh, which put a space before the ellipsis.
func crossRefPreview(text string) string {
	r := []rune(collapseSpaces(text))
	if len(r) <= crossRefPreviewRunes {
		return string(r)
	}
	cut := string(r[:crossRefPreviewRunes])
	if r[crossRefPreviewRunes] != ' ' {
		// Back to the last whole word, unless that would leave too little.
		if i := strings.LastIndexByte(cut, ' '); i >= len(cut)/2 {
			cut = cut[:i]
		}
	}
	return strings.TrimRight(cut, " ,;:") + "…"
}

func crossRefRow(state *AppState, c crossRef, pal palette, onTap func(crossRef)) fyne.CanvasObject {
	ref := canvas.NewText(c.label(), pal.Accent)
	ref.TextStyle = fyne.TextStyle{Bold: true}
	ref.TextSize = 16

	// The reference line, with a "Parallel" tag for Gospel-synopsis entries.
	// A row of the same saying on another occasion is not the same event and
	// does not wear that tag: it carries its own label, on a line of its own
	// because the label is longer than the tag and a phone has no room for it
	// beside a long reference.
	var refLine fyne.CanvasObject = ref
	if c.Parallel && !c.otherOccasion() {
		refLine = container.NewHBox(container.NewCenter(ref), container.NewCenter(parallelBadge(pal)))
	}
	lines := []fyne.CanvasObject{refLine}
	if c.otherOccasion() {
		lines = append(lines, container.NewHBox(otherOccasionBadge(pal)))
	}

	// For a parallel, the pericope title ("The Beatitudes") is the useful context,
	// so show it above the verse preview; for another occasion, the saying's.
	if c.Parallel && c.Title != "" {
		t := canvas.NewText(c.Title, pal.TextMuted)
		t.TextSize = 12
		t.TextStyle = fyne.TextStyle{Italic: true}
		lines = append(lines, t)
	}

	// The preview carries the divine name's small capitals as the page draws
	// them (small_caps_draw.go): in the NKJV they are the whole difference
	// between "Lᴏʀᴅ", the Tetragrammaton, and "Lord", Adonai, and the raw text
	// the preview used to show has the same letters for both. They are
	// characters, so the 90-character cut is unchanged; the chrome face has no
	// small capitals and the toolkit draws them from its own fallback face, as
	// a shared verse's are drawn wherever it is pasted.
	snippet := ""
	if v := state.Bible.GetVerse(c.Book, c.Chapter, c.Verse); v != nil {
		snippet = crossRefPreview(smallCapsText(*v))
	}
	snip := widget.NewLabel(snippet)
	snip.Wrapping = fyne.TextWrapWord
	lines = append(lines, snip)

	inner := container.NewPadded(container.NewVBox(lines...))
	card := newTapCard(inner, pal.SurfaceAlt, func() { onTap(c) })
	return container.NewVBox(card, widget.NewSeparator())
}

// parallelBadge is the small accent pill that marks a Gospel-synopsis parallel row.
func parallelBadge(pal palette) fyne.CanvasObject {
	t := canvas.NewText("PARALLEL", pal.AccentText)
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.TextSize = 9
	bg := canvas.NewRectangle(pal.Accent)
	bg.CornerRadius = 5
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(2, 2, 7, 7), t))
}

// otherOccasionLabel is the label on a row of the second kind of Gospel
// parallel (parallels.go): the same saying, recorded on an occasion other
// than the verse's. Reader-facing wording, in one place. The next major
// release's (docs/NEXT.md), where its wording is an open decision.
const otherOccasionLabel = "Same saying, another occasion"

// otherOccasionBadge marks such a row: the accent's outline where the
// parallel tag is filled, so the two kinds read as related and different.
func otherOccasionBadge(pal palette) fyne.CanvasObject {
	t := canvas.NewText(otherOccasionLabel, pal.Accent)
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.TextSize = 10
	bg := canvas.NewRectangle(color.Transparent)
	bg.StrokeColor = pal.Accent
	bg.StrokeWidth = 1
	bg.CornerRadius = 5
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(2, 2, 7, 7), t))
}

// tapCard makes an arbitrary content block one tap target, with a desktop hover
// wash and pointer cursor — the generic form of search.go's result card.
type tapCard struct {
	widget.BaseWidget
	content fyne.CanvasObject
	hoverBg color.NRGBA
	onTap   func()
	bg      *canvas.Rectangle
}

func newTapCard(content fyne.CanvasObject, hoverBg color.NRGBA, onTap func()) *tapCard {
	c := &tapCard{content: content, hoverBg: hoverBg, onTap: onTap}
	c.ExtendBaseWidget(c)
	return c
}

func (c *tapCard) CreateRenderer() fyne.WidgetRenderer {
	c.bg = canvas.NewRectangle(color.Transparent)
	c.bg.CornerRadius = 8
	return widget.NewSimpleRenderer(container.NewStack(c.bg, c.content))
}

func (c *tapCard) Tapped(*fyne.PointEvent) {
	if c.onTap != nil {
		c.onTap()
	}
}

func (c *tapCard) MouseIn(*desktop.MouseEvent) {
	if c.bg != nil {
		c.bg.FillColor = c.hoverBg
		c.bg.Refresh()
	}
}
func (c *tapCard) MouseMoved(*desktop.MouseEvent) {}
func (c *tapCard) MouseOut() {
	if c.bg != nil {
		c.bg.FillColor = color.Transparent
		c.bg.Refresh()
	}
}
func (c *tapCard) Cursor() desktop.Cursor { return desktop.PointerCursor }

var (
	_ fyne.Tappable      = (*tapCard)(nil)
	_ desktop.Hoverable  = (*tapCard)(nil)
	_ desktop.Cursorable = (*tapCard)(nil)
)
