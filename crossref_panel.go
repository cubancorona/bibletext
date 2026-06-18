package bibletext

// The cross-references panel: a modal that lists the passages related to the
// selected verse(s). It mirrors the AI panel (spinner while the dataset loads on
// first use, then content), and reuses the chapter-picker overlay hide/restore
// dance. Each row is a full tap target that jumps to the passage in context.

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func showCrossRefs(state *AppState, text string) {
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
	src := canvas.NewText(citationForSelection(state, text), pal.Accent)
	src.TextStyle = fyne.TextStyle{Bold: true}
	src.TextSize = subheadingTextSize
	header := container.NewVBox(title, src, widget.NewSeparator())

	ps := aiPanelSize(cnv.Size())
	bodyW := ps.Width - 44
	maxBodyH := ps.Height - 150
	listBox := container.NewVBox()
	scroll := container.NewVScroll(listBox)
	scroll.SetMinSize(fyne.NewSize(bodyW, maxBodyH))
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
	credit := canvas.NewText("Cross-references: OpenBible.info (CC-BY)", pal.TextMuted)
	credit.TextSize = 11
	footer := container.NewVBox(
		widget.NewSeparator(),
		credit,
		container.NewHBox(layout.NewSpacer(), closeBtn),
	)

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
		bar := widget.NewProgressBarInfinite()
		thinkingBar = bar
		msg := widget.NewLabel("Finding related passages…")
		msg.Alignment = fyne.TextAlignCenter
		setCentered(container.NewVBox(msg, bar))
	}
	showRefs := func(refs []crossRef) {
		stopThinking()
		if len(refs) == 0 {
			setMessage("No cross-references for this selection.")
			return
		}
		rows := make([]fyne.CanvasObject, 0, len(refs))
		for _, c := range refs {
			rows = append(rows, crossRefRow(state, c, pal, func(cc crossRef) {
				closePanel()
				if v := state.Bible.GetVerse(cc.Book, cc.Chapter, cc.Verse); v != nil {
					goToVerse(state, *v)
				}
			}))
		}
		listBox.Objects = rows
		listBox.Refresh()
		body.Objects = []fyne.CanvasObject{scroll}
		body.Refresh()
		scroll.ScrollToTop()
	}

	content := container.NewBorder(header, footer, nil, nil, body)
	popup = widget.NewModalPopUp(
		surface(container.NewPadded(content), pal.Surface, pal.Border, fyne.Size{}),
		cnv,
	)
	popup.Show()
	popup.Resize(fyne.NewSize(ps.Width, minF(ps.Height, 460)))

	setThinking()
	go func() {
		err := ensureCrossRefs()
		var refs []crossRef
		if err == nil {
			refs = crossRefsForSelection(state, text)
		}
		fyne.Do(func() {
			if err != nil {
				setMessage("Couldn't load cross-references.\nCheck your connection and try again.")
				return
			}
			showRefs(refs)
		})
	}()
}

func crossRefRow(state *AppState, c crossRef, pal palette, onTap func(crossRef)) fyne.CanvasObject {
	ref := canvas.NewText(c.label(), pal.Accent)
	ref.TextStyle = fyne.TextStyle{Bold: true}
	ref.TextSize = 16

	snippet := ""
	if v := state.Bible.GetVerse(c.Book, c.Chapter, c.Verse); v != nil {
		full := collapseSpaces(v.Text)
		snippet = firstRunes(full, 90)
		if len([]rune(full)) > 90 {
			snippet += "…"
		}
	}
	snip := widget.NewLabel(snippet)
	snip.Wrapping = fyne.TextWrapWord

	inner := container.NewPadded(container.NewVBox(ref, snip))
	card := newTapCard(inner, pal.SurfaceAlt, func() { onTap(c) })
	return container.NewVBox(card, widget.NewSeparator())
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
