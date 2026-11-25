package bibletext

// The AI study result panel: a modal popup that shows a spinner while the provider
// answers, then the response (or a friendly error with a retry). It reuses the
// chapter-picker modal approach — including hiding the native reading overlay
// while it's open, since that overlay floats above the Fyne canvas and would
// otherwise paint on top of the popup.

import (
	"context"
	"image/color"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// aiPassageDisclosureTheme keeps the glyph small inside a full touch target.
// Button measures its empty label too, so its text size follows the citation.
type aiPassageDisclosureTheme struct{ fyne.Theme }

func (t aiPassageDisclosureTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return t.Theme.Size(theme.SizeNameCaptionText)
	case theme.SizeNameInnerPadding:
		return 0
	case theme.SizeNameInlineIcon:
		return 14
	}
	return t.Theme.Size(name)
}

// aiPassageHeaderLayout places the quotation (when expanded) above the citation
// row with 12pt from quotation to citation and 12pt from citation to the header
// rule (5pt inside this block plus VBox's 7pt padding), while letting the 44pt
// tap band overlap surrounding air across the full width of the row.
type aiPassageHeaderLayout struct {
	box        *fyne.Container
	quote      fyne.CanvasObject
	citation   fyne.CanvasObject
	wash       *canvas.Rectangle
	hit        fyne.CanvasObject
	disclosure fyne.CanvasObject
}

const (
	aiQuoteToCitationGap = float32(12)
	aiCitationOuterAir   = float32(5) // + 7pt VBox padding = 12pt to title or header rule
)

func (l *aiPassageHeaderLayout) measureCitation(width float32) float32 {
	citeW := max(32, width-minTapTarget)
	if width > 0 && l.citation.Size().Width != citeW {
		l.citation.Resize(fyne.NewSize(citeW, l.citation.MinSize().Height))
	}
	return l.citation.MinSize().Height
}

func (l *aiPassageHeaderLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	w := float32(0)
	if l.box != nil {
		w = l.box.Size().Width
	}
	citeH := l.measureCitation(w)
	minW := l.citation.MinSize().Width + minTapTarget
	if !l.quote.Visible() {
		return fyne.NewSize(minW, aiCitationOuterAir+citeH+aiCitationOuterAir)
	}
	qs := l.quote.MinSize()
	return fyne.NewSize(max(qs.Width, minW), qs.Height+aiQuoteToCitationGap+citeH+aiCitationOuterAir)
}

func (l *aiPassageHeaderLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	citeW := max(0, size.Width-minTapTarget)
	citeH := l.measureCitation(size.Width)
	citeY := aiCitationOuterAir
	if l.quote.Visible() {
		qh := l.quote.MinSize().Height
		l.quote.Move(fyne.NewPos(0, 0))
		l.quote.Resize(fyne.NewSize(size.Width, qh))
		citeY = qh + aiQuoteToCitationGap
	}
	l.citation.Move(fyne.NewPos(0, citeY))
	l.citation.Resize(fyne.NewSize(citeW, citeH))

	hitH := max(minTapTarget, citeH)
	hitY := citeY + citeH/2 - hitH/2
	if l.box != nil && l.box.Position().Y < 11 && hitY < 0 {
		hitY = 0
	}
	l.wash.Move(fyne.NewPos(0, hitY))
	l.wash.Resize(fyne.NewSize(size.Width, hitH))
	l.hit.Move(fyne.NewPos(0, hitY))
	l.hit.Resize(fyne.NewSize(citeW, hitH))
	l.disclosure.Move(fyne.NewPos(citeW, hitY))
	l.disclosure.Resize(fyne.NewSize(minTapTarget, hitH))
}

// aiCitationTapArea extends the passage disclosure's tap and hover target
// across the citation text without adding a second keyboard tab stop.
type aiCitationTapArea struct {
	widget.BaseWidget
	state   *AppState
	wash    *canvas.Rectangle
	onTap   func()
	hovered bool
	pressed bool
}

func newAICitationTapArea(state *AppState, wash *canvas.Rectangle, onTap func()) *aiCitationTapArea {
	a := &aiCitationTapArea{state: state, wash: wash, onTap: onTap}
	a.ExtendBaseWidget(a)
	return a
}

func (a *aiCitationTapArea) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (a *aiCitationTapArea) syncWash() {
	pal := lightPalette
	if a.state != nil {
		pal = a.state.pal()
	} else if isDark() {
		pal = darkPalette
	}
	switch {
	case a.pressed:
		a.wash.FillColor = theme.Color(theme.ColorNamePressed)
	case a.hovered:
		a.wash.FillColor = paletteHover(pal)
	default:
		a.wash.FillColor = color.Transparent
	}
	a.wash.Refresh()
}

func (a *aiCitationTapArea) Tapped(*fyne.PointEvent) {
	a.pressed = false
	a.syncWash()
	if a.onTap != nil {
		a.onTap()
	}
}

func (a *aiCitationTapArea) MouseIn(*desktop.MouseEvent) {
	a.hovered = true
	a.syncWash()
}

func (a *aiCitationTapArea) MouseMoved(*desktop.MouseEvent) {}

func (a *aiCitationTapArea) MouseOut() {
	a.hovered = false
	a.pressed = false
	a.syncWash()
}

func (a *aiCitationTapArea) MouseDown(*desktop.MouseEvent) {
	a.pressed = true
	a.syncWash()
}

func (a *aiCitationTapArea) MouseUp(*desktop.MouseEvent) {
	if a.pressed {
		a.pressed = false
		a.syncWash()
	}
}

func (a *aiCitationTapArea) Cursor() desktop.Cursor { return desktop.PointerCursor }

var (
	_ fyne.Tappable      = (*aiCitationTapArea)(nil)
	_ desktop.Hoverable  = (*aiCitationTapArea)(nil)
	_ desktop.Mouseable  = (*aiCitationTapArea)(nil)
	_ desktop.Cursorable = (*aiCitationTapArea)(nil)
)

// aiErrorViewportLayout lets the message scroll use less than Fyne's 32pt
// scroll minimum when the pinned action and the sheet's chrome need the room.
// The scroll keeps its direction and its full content remains reachable.
type aiErrorViewportLayout struct {
	scroll *container.Scroll
	height float32
}

func (l *aiErrorViewportLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(l.scroll.MinSize().Width, l.height)
}

func (l *aiErrorViewportLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(size)
	}
}

// showAIPanel resolves the selection's passage once, here on the UI
// goroutine, and the panel keeps it for every action: the request, its cache
// key, the header, every Try again, the faster-model switch, the Report mail
// and the reopen after a light/dark change all read that one capture. Only
// Ask used to capture; the fixed actions resolved the passage again on each
// request, inside the request goroutine, against whatever chapter the reader
// was then on, so once a link or the narration had moved the chapter under
// the open panel they cited the old selection's verse numbers in the new
// chapter (Genesis 1:1 selected, "John 1:1" asked about after a link), while
// the header and the Report mail still named the passage selected.
func showAIPanel(state *AppState, action, selectedText, question string, span selSpan) {
	if state == nil || state.window == nil || state.window.Canvas() == nil {
		return
	}
	showAIPanelForPassage(state, action, aiPassageFor(state, selectedText, span), question, span)
}

// showAIPanelForPassage keeps the composer or selection's captured page through
// retries and appearance rebuilds, even if the reader has since navigated.
func showAIPanelForPassage(state *AppState, action string, passage aiPassage, question string, span selSpan) {
	showAIPanelWithSpeed(state, action, passage, question, span, aiSpeedChoice{})
}

func showAIPanelWithSpeed(state *AppState, action string, passage aiPassage, question string, span selSpan, choice aiSpeedChoice) {
	if state == nil || state.window == nil {
		return
	}
	cnv := state.window.Canvas()
	if cnv == nil {
		return
	}
	panelID := devDiagnosticID()
	diagnose := func(event string, metrics ...devDiagnosticMetric) {
		if !devDiagnosticsEnabled {
			return
		}
		metrics = append(metrics, devDiagnosticMetric{Key: "panel_id", Value: int64(panelID)},
			devDiagnosticMetric{Key: "action", Value: devDiagnosticAction(action)})
		devDiagnosticEvent(event, metrics...)
	}
	diagnose("panel_open", devDiagnosticMetric{Key: "question_bytes", Value: int64(len(question))},
		devDiagnosticMetric{Key: "passage_bytes", Value: int64(len(passage.Text))},
		devDiagnosticMetric{Key: "width", Value: int64(cnv.Size().Width)},
		devDiagnosticMetric{Key: "height", Value: int64(cnv.Size().Height)})
	pal := state.pal()
	// The reopen keeps the captured passage, so a light/dark change rebuilds
	// the panel about the passage selected, wherever the reader now is.
	reopen := func() {
		showAIPanelWithSpeed(state, action, passage, question, span, choice)
	}

	// The native overlay floats above the canvas; hide it while the modal is up.
	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	restore := func() {
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	}

	// --- Header: action title and grounding passage.
	title := canvas.NewText(aiActionTitle(action), pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 22

	// Ask's question introduces the reading area below the passage citation.
	// It scrolls with the response, so a long question cannot crowd it out.
	headerItems := []fyne.CanvasObject{title}
	var questionText *widget.RichText
	if action == aiActionAsk {
		if q := strings.TrimSpace(question); q != "" {
			ql := widget.NewRichText(aiAnswerLayoutSegments([]widget.RichTextSegment{&widget.TextSegment{
				Text:  q,
				Style: widget.RichTextStyle{SizeName: theme.SizeNameCaptionText},
			}})...)
			ql.Wrapping = fyne.TextWrapWord
			questionText = ql
		}
	}
	boundedContext := strings.TrimSpace(passage.previewText()) != ""
	var passageExcerpt *noteExcerpt
	var detailsScroll *container.Scroll
	var detailsBox *fyne.Container
	var detailsButton *widget.Button
	var detailsChoice bool
	detailsExpanded := true
	var refitContext func()
	preview, excerpt := newPassageExcerptPreview(passage.Ref, passage.previewVersion(), passage.previewText(),
		noteExcerptMaxLinesFor(fyne.CurrentDevice().IsMobile(), cnv.Size().Height), pal)
	passageExcerpt = excerpt
	if boundedContext {
		// The citation stays visible when the quote collapses.
		// Expanded details retain the complete selected words in a scroll.
		block := preview.Objects[0].(*fyne.Container)
		quoteBox := block.Objects[0]
		block.Objects = block.Objects[1:]
		excerpt.setMaxLines(len([]rune(passage.previewText())) + 1)
		detailsScroll = container.NewVScroll(container.New(squeezeWidthLayout{}, quoteBox))
		detailsBox = container.NewStack(detailsScroll)
		detailsButton = widget.NewButtonWithIcon("", iconDisclosureCollapse, func() {
			detailsChoice = true
			detailsExpanded = !detailsExpanded
			diagnose("panel_details")
			if refitContext != nil {
				refitContext()
			}
		})
		detailsButton.Importance = widget.LowImportance
		compactButton := container.NewThemeOverride(detailsButton,
			aiPassageDisclosureTheme{fyne.CurrentApp().Settings().Theme()})
		setPassagePreviewCompact(preview, true)
		citeWash := canvas.NewRectangle(color.Transparent)
		citeWash.CornerRadius = 8
		citeHit := newAICitationTapArea(state, citeWash, func() {
			if detailsButton.OnTapped != nil {
				detailsButton.OnTapped()
			}
		})
		// Keep the title on its own row: long action titles and the disclosure
		// would otherwise widen the popup beyond a narrow phone's canvas.
		headerLay := &aiPassageHeaderLayout{
			quote:      detailsBox,
			citation:   preview,
			wash:       citeWash,
			hit:        citeHit,
			disclosure: compactButton,
		}
		passageHeader := container.New(headerLay, citeWash, detailsBox, preview, citeHit, compactButton)
		headerLay.box = passageHeader
		headerItems = append(headerItems, passageHeader)
	} else {
		headerItems = append(headerItems, preview)
	}
	headerItems = append(headerItems, widget.NewSeparator())
	header := container.NewVBox(headerItems...)

	// --- Body: a vertical scroll holds the answer, with the thinking and error
	// states layered on top of it. The panel grows to fit the answer (capped at
	// maxBodyH) so short answers show in full with no empty space, and only very
	// long ones need to scroll.
	ps := sheetPanelSize(state, cnv)
	bodyW := ps.Width - 36
	// A floor for the states that size themselves before the popup exists. The
	// real cap is worked out per-state in fitBody, from the chrome the panel
	// actually has rather than a guess at it.
	maxBodyH := ps.Height - 168
	answer := widget.NewRichTextFromMarkdown("")
	answer.Wrapping = fyne.TextWrapWord
	var markdownAnswer *compareAnswer
	var answerView fyne.CanvasObject = answer
	if action == aiActionCompare || action == aiActionAsk {
		markdownAnswer = newCompareAnswer()
		answerView = markdownAnswer
	}
	var questionView fyne.CanvasObject
	if questionText != nil {
		questionText.Resize(fyne.NewSize(bodyW-16, questionText.MinSize().Height))
		questionView = container.New(layout.NewCustomPaddedLayout(0, 0, 8, 8),
			container.NewThemeOverride(questionText, passageCitationTheme{fyne.CurrentApp().Settings().Theme()}))
	}
	withQuestion := func(content fyne.CanvasObject) fyne.CanvasObject {
		if questionView == nil {
			return content
		}
		return container.New(squeezeWidthLayout{}, container.NewVBox(questionView, content))
	}
	answerContent := withQuestion(answerView)
	answerScroll := container.NewVScroll(answerContent)
	answerScroll.SetMinSize(fyne.NewSize(bodyW, maxBodyH))
	body := container.NewStack(answerScroll)

	// --- Footer: copy, close, and an honesty note.
	var current string
	var popup *widget.PopUp
	var content, footer *fyne.Container
	// userClosed distinguishes the reader's own Close from a rebuildWindow
	// drain-eviction (theme-variant flip): only the latter reopens the panel
	// when an in-flight answer lands.
	var userClosed bool

	// A running ProgressBarInfinite ticks an animation every frame, which keeps the
	// whole canvas marked dirty and forces a full-tree repaint on each frame — that
	// competes with scrolling and lingers (until renderer-cache expiry) if the panel
	// is dismissed mid-spin. Track the live spinner and Stop() it on every exit from
	// the thinking state so the animation never outlives the panel.
	var thinkingBar *widget.ProgressBarInfinite
	// cancelFetch abandons the in-flight study request. Stopping the spinner is
	// not enough: the request keeps running for the whole aiRequestBudget and
	// keeps billing the reader's key for an answer nobody will read. Every
	// exit — Close, Cancel, a re-run — goes through stopThinking.
	var cancelFetch func()
	var speed *aiSpeedControl
	stopThinking := func() {
		if speed != nil {
			speed.close()
			speed = nil
		}
		if thinkingBar != nil {
			thinkingBar.Stop()
			thinkingBar = nil
		}
		if cancelFetch != nil {
			cancelFetch()
			cancelFetch = nil
		}
	}

	copyBtn := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() {
		if current != "" {
			state.window.Clipboard().SetContent(current)
		}
	})
	copyBtn.Importance = widget.LowImportance
	copyBtn.Disable()

	// Report lets a reader flag AI output (Guideline 1.2): it opens a pre-filled
	// email to support naming the passage and the study action, with the
	// question and the generated answer. Enabled once an answer is shown. Only
	// this panel renders free-form AI prose — "Find" shows canonical scripture
	// only — so this is the one surface that needs it.
	reportBtn := widget.NewButton("Report", func() {
		if current == "" {
			return
		}
		mu := &url.URL{
			Scheme: "mailto",
			Opaque: SupportMailtoRecipient(),
			RawQuery: url.Values{
				"subject": {"BibleText: report AI content"},
				"body":    {aiReportMailBodyForPassage(action, passage, question, current)},
			}.Encode(),
		}
		openExternalURL(mu)
	})
	reportBtn.Importance = widget.LowImportance
	reportBtn.Disable()

	// fitBody sizes a body scroll to want, then grows or shrinks the panel to
	// exactly what that needs.
	//
	// The chrome — header, footer, padding, border — is MEASURED, not guessed.
	// Two hardcoded guesses used to stand in for it, 168 when capping the body
	// and 158 when resizing the panel. They disagreed with each other, and both
	// under-reserved by about 54pt against a header that grows with the quote
	// and the question. That shortfall is what clipped "Try again" out of the
	// error state and put a scrollbar on a panel with room to spare.
	//
	// least, where a state gives one, is the height under which the body
	// would cut what the state must show whole (wholeThrough): the waiting
	// line, or the error's message. The body keeps it even
	// where the room below the header is less, and the panel then covers the
	// header, as a sheet that cannot clear it does; it is held to what the
	// window gives a sheet at most (sheetMostHeight).
	//
	// It keeps the body, the height it wanted and its least, so a window
	// resize can size the panel again, whichever state it is in, from the
	// height the window now gives it (sheet_refit.go).
	var fittedBody *container.Scroll
	var fittedWant float32
	var fittedLeast func() float32
	var errorViewport *aiErrorViewportLayout
	var errorOverflow *aiErrorOverflow
	var waitColumn *aiWaitColumn
	var disclaimerText *widget.RichText
	var disclaimer fyne.CanvasObject
	fitBody := func(sc *container.Scroll, want float32, least func() float32) {
		if errorOverflow != nil {
			errorOverflow.restore(content, header, footer, body)
		}
		if want < 1 {
			want = 1
		}
		if disclaimer != nil && disclaimer.Visible() && disclaimerText != nil {
			disclaimerText.Resize(fyne.NewSize(bodyW, disclaimerText.MinSize().Height))
		}
		fittedBody, fittedWant, fittedLeast = sc, want, least
		var leastH float32
		if least != nil {
			leastH = least()
		}
		setBodyHeight := func(height float32) {
			if errorViewport != nil && errorViewport.scroll == sc {
				errorViewport.height = height
			}
			sc.SetMinSize(fyne.NewSize(bodyW, height))
		}
		setBodyHeight(want)
		if popup == nil {
			return
		}
		if boundedContext {
			// Measure expanded context before deciding whether it leaves several
			// answer rows and the waiting or error state's required column.
			// Explicit disclosure choices survive refits and state transitions.
			detailsBox.Show()
			contextW := header.Size().Width
			if contextW <= 0 {
				contextW = bodyW
			}
			// Wrapped text's minimum height settles after its actual width lands.
			for range 2 {
				detailsScroll.Content.Resize(fyne.NewSize(contextW, detailsScroll.Content.MinSize().Height))
			}
			contextH := detailsScroll.Content.MinSize().Height
			setContextHeight := func(height float32) {
				direction := fyne.ScrollVerticalOnly
				if contextH <= height {
					// A fitting quote needs no scroller's 32-point minimum height.
					direction = fyne.ScrollNone
				}
				changed := detailsScroll.Direction != direction
				detailsScroll.Direction = direction
				detailsScroll.SetMinSize(fyne.NewSize(0, height))
				if changed {
					detailsScroll.Refresh()
				}
			}
			setContextHeight(minF(contextH, ps.Height/3))
			need := fyne.Max(minF(want, 96), leastH)
			if thinkingBar != nil {
				baseWant := want
				if waitColumn != nil {
					curH, closedH, _ := waitColumn.lay.boundsAt(bodyW)
					if curH > closedH {
						baseWant = fyne.Max(1, want-(curH-closedH))
					}
				}
				// While waiting, automatic collapse yields the quote before
				// clipping the closed waiting column on short portrait screens,
				// without collapsing an open quote mid-tap when Faster options opens.
				need = fyne.Max(baseWant, leastH)
			}
			available := func() float32 {
				return ps.Height - (popup.MinSize().Height - sc.MinSize().Height)
			}
			if !detailsChoice {
				detailsExpanded = available() >= need
			}
			if detailsExpanded {
				detailsButton.SetIcon(iconDisclosureCollapse)
				if available() < need {
					h := detailsScroll.MinSize().Height - (need - available())
					setContextHeight(fyne.Max(32, h))
				}
			} else {
				detailsBox.Hide()
				detailsButton.SetIcon(iconDisclosureExpand)
			}
		}
		// Everything the panel needs that is NOT the body. Read after the
		// SetMinSize above, so it is this panel's real chrome.
		chrome := popup.MinSize().Height - want
		// The panel's height with the room to show the whole body.
		preferred := want + chrome
		// When the fixed header, recovery rows and footer leave less than one
		// readable message line, move their variable content into one scroll.
		// The title and Close stay fixed, with their ordinary dimensions.
		if errorOverflow != nil && errorOverflow.message == sc && chrome+errorOverflow.readable > sheetMostHeight(state, cnv) {
			errorOverflow.show(content)
			errorOverflow.scroll.SetMinSize(fyne.NewSize(bodyW, 32))
			chrome := popup.MinSize().Height - errorOverflow.scroll.MinSize().Height
			height := max(float32(32), sheetMostHeight(state, cnv)-chrome)
			errorOverflow.scroll.SetMinSize(fyne.NewSize(bodyW, height))
			popup.Resize(fyne.NewSize(ps.Width, height+chrome))
			coverHeaderUnlessClear(state, popup, height+chrome)
			return
		}
		if max := ps.Height - chrome; want > max {
			want = fyne.Max(max, minF(leastH, sheetMostHeight(state, cnv)-chrome))
			if want < 1 {
				want = 1
			}
			setBodyHeight(want)
		}
		popup.Resize(fyne.NewSize(ps.Width, want+chrome))
		// The chrome alone, or the chrome and the body's least, can be taller
		// than the room below the header on a short desktop window; the panel
		// then covers the header, as tall as the whole body needs, capped at
		// the room, and never shorter than the chrome and the body's least.
		coverHeaderUnlessClear(state, popup, preferred)
	}
	refitContext = func() {
		if fittedBody != nil {
			fitBody(fittedBody, fittedWant, fittedLeast)
		}
	}

	exitBtn := widget.NewButton("Close", func() {
		if thinkingBar != nil {
			diagnose("panel_cancel")
		} else {
			diagnose("panel_close")
		}
		userClosed = true
		stopThinking()
		if popup != nil {
			popup.Hide()
		}
		restore()
	})
	exitOutline := canvas.NewRectangle(color.Transparent)
	exitOutline.StrokeColor = pal.Border
	exitOutline.StrokeWidth = 1
	exitOutline.CornerRadius = 10
	exitSlot := container.NewStack(exitBtn, exitOutline)

	disclaimerText = widget.NewRichText(&widget.TextSegment{
		Text: "AI-generated — may be imperfect. Verify important details.",
		Style: widget.RichTextStyle{
			ColorName: colorNameMuted,
			SizeName:  theme.SizeNameCaptionText,
		},
	})
	disclaimerText.Wrapping = fyne.TextWrapWord
	disclaimer = container.NewThemeOverride(disclaimerText,
		passageCitationTheme{fyne.CurrentApp().Settings().Theme()})

	footer = container.NewVBox(
		widget.NewSeparator(),
		disclaimer,
		container.NewHBox(reportBtn, layout.NewSpacer(), copyBtn, exitSlot),
	)

	setFooterState := func(mode string) {
		answered := mode == "answer"
		if answered {
			disclaimerText.Resize(fyne.NewSize(bodyW, disclaimerText.MinSize().Height))
			disclaimer.Show()
			reportBtn.Show()
			copyBtn.Show()
		} else {
			disclaimer.Hide()
			reportBtn.Hide()
			copyBtn.Hide()
		}
		if mode == "thinking" {
			exitBtn.SetText("Cancel")
			exitOutline.Show()
		} else {
			exitBtn.SetText("Close")
			exitOutline.Hide()
		}
		exitOutline.Refresh()
		exitSlot.Refresh()
		footer.Refresh()
	}

	clearErrorOverflow := func() {
		if errorOverflow != nil {
			errorOverflow.restore(content, header, footer, body)
			exitSlot.Refresh()
			footer.Refresh()
			errorOverflow = nil
		}
	}

	// --- State transitions. Each of the three swaps the body for its own
	// content and then sizes the panel to it; none inherits the height another
	// left behind.
	// Declared before setThinking so the waiting state's speed control can
	// re-run the request; assigned below.
	var startFetch func()
	// fetchGen identifies the request the panel currently cares about (Find's
	// aiSearchSession, in miniature). Every startFetch bumps it; a completion
	// compares its captured value and bails when superseded. Without this, a
	// request abandoned by a speed re-run would still settle here — nil-ing
	// cancelFetch (disarming the NEW request's cancel) and painting its stale
	// answer or cancellation error over the state the reader moved on to.
	var fetchGen int
	// modelLine names the model the request in flight is sending to, as that
	// request reports it (ai_model_in_use.go). It belongs to the waiting state
	// setThinking drew last; a report from any other request is dropped.
	var modelLine *aiModelLine
	var lastReport aiModelInUse
	var restarted bool

	setThinking := func() {
		clearErrorOverflow()
		// Same reason as setError: a re-run (a speed choice, Try again) leaves
		// the previous answer on neither the screen nor the clipboard.
		current = ""
		copyBtn.Disable()
		reportBtn.Disable()
		setFooterState("thinking")

		bar := widget.NewProgressBarInfinite()
		thinkingBar = bar
		seed := seedAISpeedReport(state, choice)
		lastReport = seed
		// Empty until the request says which model it is sending to, and new
		// with every request, so a re-run never shows the model before it.
		modelLine = newAIModelLine(aiModelInUse{})
		var waitScroll *container.Scroll
		activeSpeed := newAISpeedControlForChoice(state, func(next aiSpeedChoice) {
			choice = next
			restarted = true
			startFetch()
		}, choice)
		activeSpeed.active = func() bool { return sheetShowing(state, popup) }
		activeSpeed.onExpand = func() { scrollExpandedChoicesIntoView(waitScroll, activeSpeed) }
		speed = activeSpeed

		status := "Reading the passage…"
		if restarted {
			status = "Reading the passage again…"
		}
		restarted = false

		col := newAIWaitColumn(pal, status, bar, modelLine, activeSpeed, nil, bodyW, seed)
		waitColumn = col
		// The waiting column sits in a scroll so a short landscape viewport
		// keeps the waiting line whole while the footer's single Cancel stays
		// pinned below; squeezeWidthLayout stops the scroll widening the column
		// sideways (sheet_fit.go).
		waitCol := container.New(squeezeWidthLayout{}, col.box)
		waitContent := withQuestion(waitCol)
		waitScroll = container.NewVScroll(waitContent)
		least := func() float32 {
			return waitContent.MinSize().Height - waitCol.MinSize().Height + col.statusFoot()
		}
		modelLine.relayout = func() { fitBody(waitScroll, waitContent.MinSize().Height+10, least) }
		activeSpeed.relayout = modelLine.relayout
		body.Objects = []fyne.CanvasObject{waitScroll}
		body.Refresh()
		fitBody(waitScroll, waitContent.MinSize().Height+10, least)
		state.consumeWaitFocusRestore(popup)
	}
	setResult := func(text string) {
		clearErrorOverflow()
		renderStarted := devDiagnosticNow()
		diagnose("render_start", devDiagnosticMetric{Key: "bytes", Value: int64(len(text))})
		stopThinking()
		waitColumn = nil
		restarted = false
		current = text
		copyBtn.Enable()
		reportBtn.Enable()
		setFooterState("answer")
		// The answer has landed, so a light/dark rebuild may bring the panel
		// back: the reopen runs the same action, which aiCache answers at once
		// without a second request. Only now — while a request is in flight the
		// request belongs to this panel, which reopens itself when it lands
		// (below), and a second reopen would stack two panels (sheet_reopen.go).
		registerSheetReopen(state, popup, reopen)
		if markdownAnswer != nil {
			markdownAnswer.setText(text)
		} else {
			setAIAnswerText(answer, text)
		}
		diagnose("render_parsed", devDiagnosticSince(renderStarted))
		// Wrapped answer text reports its true height at a known width.
		// Pre-wrap at the real body width so the height is right, then
		// fit the panel to the answer (capped at maxBodyH).
		answerView.Resize(fyne.NewSize(bodyW, answerView.MinSize().Height))
		if questionText != nil {
			questionText.Resize(fyne.NewSize(bodyW-16, questionText.MinSize().Height))
		}
		diagnose("render_wrapped", devDiagnosticSince(renderStarted))
		answerScroll.ScrollToTop()
		body.Objects = []fyne.CanvasObject{answerScroll}
		body.Refresh()
		fitBody(answerScroll, answerContent.MinSize().Height+10, nil)
		diagnose("render_fit_done", devDiagnosticSince(renderStarted))
		// Re-measure once the real layout has landed so the height is exact.
		studyRemeasure(func() {
			if userClosed || !sheetShowing(state, popup) {
				diagnose("remeasure_skipped")
				return
			}
			started := devDiagnosticNow()
			diagnose("remeasure_start")
			answerView.Refresh()
			answerScroll.Refresh()
			fitBody(answerScroll, answerContent.MinSize().Height+10, nil)
			diagnose("remeasure_done", devDiagnosticSince(started))
		})
	}

	setError := func(err error) {
		clearErrorOverflow()
		diagnose("panel_error")
		msg, needsSettings := friendlyAIError(err), isNoKeyError(err)
		presentation, special := aiSpecialError(err)
		stopThinking()
		waitColumn = nil
		restarted = false
		// An error is not an answer aiCache holds, so reopening would ask
		// again, on the reader's key: the error state never comes back after a
		// light/dark rebuild. Only setResult registers, and nothing leads from
		// an answer back to a request today, so no registration is live here;
		// this forget is the belt to that, should a path ever be added.
		forgetSheetReopen(state, popup)
		// Drop the previous answer with its buttons, and hide the answer-only
		// footer furniture while no generated text is shown.
		current = ""
		copyBtn.Disable()
		reportBtn.Disable()
		setFooterState("error")
		if markdownAnswer != nil {
			markdownAnswer.setText("")
		} else {
			answer.ParseMarkdown("")
		}
		gen := fetchGen
		var action fyne.CanvasObject
		if needsSettings {
			actBtn := widget.NewButton("Open AI settings", func() {
				diagnose("panel_settings")
				userClosed = true
				stopThinking()
				if popup != nil {
					popup.Hide()
				}
				restore()
				showAISettings(state)
			})
			actBtn.Importance = widget.HighImportance
			action = container.NewCenter(actBtn)
		} else {
			// Each recovery option has a row of its own: a framed Try again
			// preserving any valid one-time effort, then the billing link or
			// the open timeout speed rows from the attempt's last report.
			control := newAITimeoutSpeedControl(state, func(next aiSpeedChoice) {
				choice = next
				startFetch()
			}, choice, lastReport)
			control.setWidth(min(aiTimeoutChoicesMeasure, bodyW))
			control.active = func() bool { return gen == fetchGen && !userClosed && sheetShowing(state, popup) }
			shown, actions := aiErrorRecovery(presentation, control)
			if special {
				msg = shown
			}
			action = container.New(squeezeWidthLayout{}, actions)
		}
		lbl := widget.NewLabel(msg)
		lbl.Wrapping = fyne.TextWrapWord
		lbl.Alignment = fyne.TextAlignCenter
		// The action stays below the message's scroll: even a landscape phone
		// too short for the whole error must show the way to retry or add a key.
		// The message and Ask's question keep their natural height wherever a
		// sheet has room for it, and scroll only once that room is exhausted.
		errItems := []fyne.CanvasObject{spacer(8), lbl}
		var detailObj fyne.CanvasObject
		if special && presentation.detail != "" {
			detailObj = aiErrorDetail(presentation)
			errItems = append(errItems, detailObj)
		}
		errCol := container.New(squeezeWidthLayout{}, container.NewVBox(errItems...))
		errContent := withQuestion(errCol)
		errScroll := container.NewVScroll(errContent)
		errorViewport = &aiErrorViewportLayout{scroll: errScroll, height: errScroll.MinSize().Height}
		body.Objects = []fyne.CanvasObject{container.NewBorder(nil, action, nil, nil,
			container.New(errorViewport, errScroll))}
		body.Refresh()
		errorOverflow = newAIErrorOverflow(title, headerItems[1:], errContent, action,
			exitSlot, errScroll, body.Objects[0])
		// Pre-wrap the message at the real body width before measuring, for the
		// same reason the answer is pre-wrapped: a word-wrapped label reports
		// its true height only once it has wrapped at a known width.
		lbl.Resize(fyne.NewSize(bodyW, lbl.MinSize().Height))
		if detailObj != nil {
			detailObj.Resize(fyne.NewSize(bodyW, detailObj.MinSize().Height))
		}
		if questionText != nil {
			questionText.Resize(fyne.NewSize(bodyW-16, questionText.MinSize().Height))
		}
		errContent.Resize(fyne.NewSize(bodyW, errContent.MinSize().Height))
		least := wholeThrough(errContent, errCol, errCol)
		fitBody(errScroll, errContent.MinSize().Height+10, least)
		lbl.Refresh()
		errScroll.Refresh()
		fitBody(errScroll, errContent.MinSize().Height+10, least)
	}

	startFetch = func() {
		// Nothing leaves the device for a provider the reader has not allowed
		// (ai_permission.go). The panel says nothing was sent before the
		// question opens over it, so that is what is left after Not now, or
		// after the question closes any other way; Try again asks again, and
		// Allow records the permission and comes back here to ask the provider.
		// A light/dark rebuild or a rotation drains the question with this
		// panel, and the question's reopen is this panel's: a new panel about
		// the same passage, with Ask's question as typed, which asks again
		// and sends nothing until Allow (sheet_reopen.go).
		if info, pending := aiPermissionPending(state); pending {
			stopThinking()
			fetchGen++ // nothing still on its way may land in this panel
			setError(&aiPermissionError{provider: info})
			promptAIPermission(state, info, func() {
				if !userClosed && sheetShowing(state, popup) {
					startFetch()
				}
			}, nil, reopen)
			return
		}
		// Abandon any request already in flight FIRST (the faster-model switch,
		// Try again): cancel its context and stop its spinner, or the
		// superseded request would keep billing the reader's key for the whole
		// aiRequestBudget while its detached ProgressBarInfinite keeps
		// repainting the canvas — the exact leak documented above cancelFetch.
		stopThinking()
		// Back in flight: not reopenable until the new answer lands. The two
		// ways in (Try again from an error, the faster model while thinking)
		// both start from a state that holds no registration, so this is the
		// same belt as setError's.
		forgetSheetReopen(state, popup)
		fetchGen++
		gen := fetchGen
		requestID := devDiagnosticID()
		requestStarted := devDiagnosticNow()
		diagnose("request_start", devDiagnosticMetric{Key: "generation", Value: int64(gen)},
			devDiagnosticMetric{Key: "request_id", Value: int64(requestID)})
		setThinking()
		ctx, cancel := context.WithCancel(context.Background())
		ctx, cancelTimeout := context.WithTimeout(ctx, aiRequestBudget)
		ctx = devDiagnosticRequestContext(ctx, requestID)
		ctx = withAIPassage(ctx, passage)
		ctx = withAISpeedChoice(ctx, choice)
		// The request names its model on the waiting state it started, and
		// only there: a request the faster-model switch or Try again has
		// replaced (a newer gen), or one whose wait is over (Close, Cancel,
		// an answer — every exit stops the bar), can still report its retry.
		ctx = showAIModelOnUI(ctx, func(m aiModelInUse) {
			if gen != fetchGen || thinkingBar == nil || !sheetShowing(state, popup) {
				return
			}
			lastReport = m
			if waitColumn != nil {
				waitColumn.show(m)
				return
			}
			modelLine.show(m)
			speed.show(m)
		})
		cancelFetch = cancel // so Close / Cancel can abandon THIS request
		requestProvider := activeAIProvider(state)
		go func() {
			defer cancelTimeout()
			result, err := aiActionRun(ctx, state, action, passage.Text, question, span)
			err = withAIErrorProvider(err, requestProvider)
			completed := devDiagnosticNow()
			var completionMetrics []devDiagnosticMetric
			if devDiagnosticsEnabled {
				completionMetrics = []devDiagnosticMetric{
					{Key: "generation", Value: int64(gen)}, {Key: "request_id", Value: int64(requestID)},
					{Key: "bytes", Value: int64(len(result))}, {Key: "error_class", Value: devDiagnosticErrorClass(err)},
					{Key: "http_status", Value: devDiagnosticHTTPStatus(err)},
					devDiagnosticSince(requestStarted),
				}
			}
			diagnose("request_complete", completionMetrics...)
			fyne.Do(func() {
				queueWait := devDiagnosticSince(completed)
				queueWait.Key = "queue_ms"
				diagnose("request_ui_delivery", append(completionMetrics, queueWait)...)
				if gen != fetchGen {
					diagnose("request_drop_superseded", devDiagnosticMetric{Key: "generation", Value: int64(gen)})
					return // superseded — a newer request owns the panel now
				}
				cancelFetch = nil // settled: nothing left to abandon
				// The reader dismissed this panel (Close / Cancel). Painting a
				// late answer into it would repaint a hidden, detached popup —
				// and with the generous aiRequestBudget that answer can arrive long
				// after they moved on. The reply is in aiCache, so reopening
				// the same action shows it instantly.
				if userClosed {
					diagnose("request_drop_closed", devDiagnosticMetric{Key: "generation", Value: int64(gen)})
					return
				}
				// The panel may have been EVICTED (not user-closed) by a
				// rebuildWindow drain — a theme-variant flip — while the
				// request ran. Don't deliver the answer into the hidden,
				// detached popup: reopen a fresh panel instead. The re-run
				// hits aiCache, so the already-paid answer shows instantly
				// in the new palette; an errored fetch just ends quietly.
				if !userClosed && popup != nil && !popup.Visible() {
					diagnose("panel_evicted", devDiagnosticMetric{Key: "generation", Value: int64(gen)})
					stopThinking()
					if err == nil {
						reopen()
					}
					return
				}
				if err != nil {
					setError(err)
					return
				}
				setResult(result)
			})
		}()
	}

	content = container.NewBorder(header, footer, nil, nil, body)
	popup = widget.NewModalPopUp(
		surface(container.NewPadded(content), pal.SurfaceAlt, pal.Border, fyne.Size{}),
		cnv,
	)
	popup.Show()
	// A modest starting size for the thinking state; setResult grows or shrinks
	// the panel to fit the answer once it arrives.
	popup.Resize(fyne.NewSize(ps.Width, minF(ps.Height, 320)))
	lastCanvas := cnv.Size()
	registerCanvasSheetRefit(state, popup, func() {
		now := cnv.Size()
		// Mobile roots also relayout for the keyboard. Only a canvas resize
		// changes this card's margins; it never replaces the answer or fetch.
		if fyne.CurrentDevice().IsMobile() && now == lastCanvas {
			return
		}
		lastCanvas = now
		if passageExcerpt != nil && !boundedContext {
			passageExcerpt.setMaxLines(noteExcerptMaxLinesFor(fyne.CurrentDevice().IsMobile(), cnv.Size().Height))
		}
		ps = sheetPanelSize(state, cnv)
		bodyW = ps.Width - 36
		if waitColumn != nil {
			waitColumn.lay.width = bodyW
		}
		if fittedBody != nil {
			offset := fittedBody.Offset
			var contextOffset fyne.Position
			if detailsScroll != nil {
				contextOffset = detailsScroll.Offset
			}
			// Release the old width before laying out the citation and quote.
			// Their wrapped heights settle only after the new width lands.
			fittedBody.SetMinSize(fyne.NewSize(bodyW, fittedBody.MinSize().Height))
			popup.Resize(fyne.NewSize(ps.Width, minF(popup.Content.Size().Height, ps.Height)))
			for range 2 {
				fittedBody.Content.Resize(fyne.NewSize(fittedBody.Size().Width, fittedBody.Content.MinSize().Height))
				fitBody(fittedBody, fittedBody.Content.MinSize().Height+10, fittedLeast)
			}
			fittedBody.ScrollToOffset(offset)
			if detailsScroll != nil {
				detailsScroll.ScrollToOffset(contextOffset)
			}
		}
	})
	startFetch()
}

// studyRemeasure schedules the answer's second fit, once the real layout has
// landed, so the panel fits the answer exactly. A variable for the same
// reason as votdRemeasure: under the test driver fyne.Do runs on the timer's
// own goroutine, and its font measurement would race the test's, so a test
// that lands an answer runs the fit synchronously instead.
var studyRemeasure = func(fit func()) {
	time.AfterFunc(40*time.Millisecond, func() { fyne.Do(fit) })
}

// wholeThrough is the one rule the study panel's waiting and error states
// keep their body by on a short window (fitBody's least): the body scroll is
// never shorter than its content down to the foot of head, the part at the
// top of col that the state must show whole. content is col with whatever
// stands above it in the scroll, Ask's question. The waiting state's head is
// its waiting line; the error state's is its message column, with recovery
// controls pinned below the scroll. Measured when the panel is fitted, so a
// message, a question or a model name that wraps again is measured as it
// now wraps.
func wholeThrough(content, col, head fyne.CanvasObject) func() float32 {
	return func() float32 {
		return content.MinSize().Height - col.MinSize().Height + head.MinSize().Height
	}
}

// aiReportMailBody is the body of the Report mail: the passage as the panel
// cited it (verses included, so the report names what was shown), the study
// action, the reader's question when the action took one, and the answer.
// A fixed action carries no question line at all, rather than an empty one.
func aiReportMailBody(action, ref, question, answer string) string {
	return aiReportMailBodyForPassage(action, aiPassage{Ref: ref}, question, answer)
}

func aiReportMailBodyForPassage(action string, passage aiPassage, question, answer string) string {
	var b strings.Builder
	b.WriteString("I'd like to report AI-generated content shown in BibleText.\n\n")
	b.WriteString("Reference: " + passage.Ref + "\n")
	b.WriteString("Study action: " + aiActionTitle(action) + "\n")
	if action == aiActionAsk {
		if passage.Version != "" {
			b.WriteString("Translation: " + passage.Version + "\n")
		}
		b.WriteString("Question: " + strings.TrimSpace(question) + "\n")
	}
	b.WriteString("\nResponse:\n" + answer + "\n\nMy concern:\n")
	return b.String()
}

// minF returns the smaller of two float32 values.
func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// sheetPanelSize is aiPanelSize for a sheet shown centred on the window: its
// height is also kept clear of the header on a desktop window, as every
// desktop sheet's is (headerClearance).
func sheetPanelSize(state *AppState, cnv fyne.Canvas) fyne.Size {
	ps := aiPanelSize(cnv.Size())
	ps.Height = clearOfHeader(ps.Height, cnv.Size().Height, headerClearance(state))
	return ps
}

// aiPanelSize fits the panel to the canvas: a comfortable reading width, capped,
// with room to breathe around the edges on both phone and desktop.
func aiPanelSize(canvasSize fyne.Size) fyne.Size {
	w := canvasSize.Width - 48
	if w > 560 {
		w = 560
	}
	if w < 280 {
		w = 280
	}
	h := canvasSize.Height - 80
	if h > 760 {
		h = 760
	}
	if h < 240 {
		h = 240
	}
	return fyne.NewSize(w, h)
}

// quotedOneLine renders a selection as the single quoted line the AI sheets
// show under their title.
//
// The line is wrapped in curly double marks — but a verse that OPENS a
// quotation carries its own opening mark (Psalm 46:10 begins “Be still, and
// know that I am God.), and wrapping that verbatim printed a doubled ““. Outer
// double marks the selection already carries are dropped before the display
// pair is added. Marks INSIDE the line are left alone: this is a preview of
// what the reader highlighted, not the share pipeline, which nests them to
// Bluebook depth (formatBibleQuote).
func quotedOneLine(s string, maxRunes int) string {
	return "“" + oneLinePreview(trimOuterDoubleQuotes(s), maxRunes) + "”"
}

// trimOuterDoubleQuotes strips double quotation marks (curly or straight) from
// the very start and end of a selection. Single marks are deliberately left:
// the display pair is a double, so a stray ‘ cannot double up — and a trailing
// ’ is far more often a possessive ("the disciples’") than a quotation.
func trimOuterDoubleQuotes(s string) string {
	return strings.Trim(strings.TrimSpace(s), "“”\" \t\n")
}

// oneLinePreview collapses whitespace and truncates to a single short line.
func oneLinePreview(s string, maxRunes int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > maxRunes {
		return string(r[:maxRunes]) + "…"
	}
	return s
}
