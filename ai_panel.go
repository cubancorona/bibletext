package bibletext

// The AI study result panel: a modal popup that shows a spinner while Gemini
// answers, then the response (or a friendly error with a retry). It reuses the
// chapter-picker modal approach — including hiding the native reading overlay
// while it's open, since that overlay floats above the Fyne canvas and would
// otherwise paint on top of the popup.

import (
	"context"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
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

// aiPassageHeaderLayout lets the touch target use the citation's surrounding
// space while retaining at least twelve points between quotation and citation.
type aiPassageHeaderLayout struct{ citation fyne.CanvasObject }

func (l aiPassageHeaderLayout) box(objects []fyne.CanvasObject) fyne.Layout {
	gap := float32(12)
	if len(objects) > 1 {
		gap = fyne.Max(0, gap-(objects[1].MinSize().Height-l.citation.MinSize().Height)/2)
	}
	return layout.NewCustomPaddedVBoxLayout(gap)
}

func (l aiPassageHeaderLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return l.box(objects).MinSize(objects)
}

func (l aiPassageHeaderLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	l.box(objects).Layout(objects, size)
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
		showAIPanelForPassage(state, action, passage, question, span)
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
		disclosure := container.NewGridWrap(fyne.NewSize(minTapTarget, minTapTarget), compactButton)
		setPassagePreviewCompact(preview, true)
		// Keep the title on its own row: long action titles and the disclosure
		// would otherwise widen the popup beyond a narrow phone's canvas.
		citationRow := container.NewBorder(nil, nil, nil, container.New(vCenterLayout{}, disclosure), container.New(vCenterLayout{}, preview))
		headerItems = append(headerItems, container.New(aiPassageHeaderLayout{citation: preview}, detailsBox, citationRow))
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
	bodyW := ps.Width - 44
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
	// userClosed distinguishes the reader's own Close from a rebuildWindow
	// drain-eviction (theme-variant flip): only the latter reopens the panel
	// when an in-flight answer lands.
	var userClosed bool

	// A running ProgressBarInfinite ticks an animation every frame, which keeps the
	// whole canvas marked dirty and forces a full-tree repaint at ~20fps — that
	// competes with scrolling and lingers (until renderer-cache expiry) if the panel
	// is dismissed mid-spin. Track the live spinner and Stop() it on every exit from
	// the thinking state so the animation never outlives the panel.
	var thinkingBar *widget.ProgressBarInfinite
	// cancelFetch abandons the in-flight study request. Stopping the spinner is
	// not enough: the request keeps running for the whole aiRequestBudget and
	// keeps billing the reader's key for an answer nobody will read. Every
	// exit — Close, Cancel, a re-run — goes through stopThinking.
	var cancelFetch func()
	stopThinking := func() {
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
	// It keeps the body and the height it wanted, so a window resize can size
	// the panel again, whichever state it is in, from the height the window
	// now gives it (sheet_refit.go).
	var fittedBody *container.Scroll
	var fittedWant float32
	fitBody := func(sc *container.Scroll, want float32) {
		if want < 1 {
			want = 1
		}
		fittedBody, fittedWant = sc, want
		sc.SetMinSize(fyne.NewSize(bodyW, want))
		if popup == nil {
			return
		}
		if boundedContext {
			// Measure expanded context before deciding whether it leaves several
			// answer rows. An explicit disclosure choice survives window resizing.
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
			need := minF(want, 96)
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
		if max := ps.Height - chrome; want > max {
			want = max
			if want < 1 {
				want = 1
			}
			sc.SetMinSize(fyne.NewSize(bodyW, want))
		}
		popup.Resize(fyne.NewSize(ps.Width, want+chrome))
	}
	refitContext = func() {
		if fittedBody != nil {
			fitBody(fittedBody, fittedWant)
		}
	}

	closeBtn := widget.NewButton("Close", func() {
		diagnose("panel_close")
		userClosed = true
		stopThinking()
		if popup != nil {
			popup.Hide()
		}
		restore()
	})

	disclaimer := canvas.NewText("AI-generated — may be imperfect. Verify important details.", pal.TextMuted)
	disclaimer.TextSize = 11

	footer := container.NewVBox(
		widget.NewSeparator(),
		disclaimer,
		container.NewHBox(reportBtn, layout.NewSpacer(), copyBtn, closeBtn),
	)

	// --- State transitions. Each of the three swaps the body for its own
	// content and then sizes the panel to it; none inherits the height another
	// left behind.
	// Declared before setThinking so the waiting state's "faster model" offer
	// can re-run the request; assigned below.
	var startFetch func()
	// fetchGen identifies the request the panel currently cares about (Find's
	// aiSearchSession, in miniature). Every startFetch bumps it; a completion
	// compares its captured value and bails when superseded. Without this, a
	// request abandoned by the faster-model re-run would still settle here —
	// nil-ing cancelFetch (disarming the NEW request's cancel) and painting its
	// stale answer or cancellation error over the state the reader moved on to.
	var fetchGen int
	// modelLine names the model the request in flight is sending to, as that
	// request reports it (ai_model_in_use.go). It belongs to the waiting state
	// setThinking drew last; a report from any other request is dropped.
	var modelLine *aiModelLine

	setThinking := func() {
		// Same reason as setError: a re-run (the faster-model switch, Try again)
		// leaves the previous answer on neither the screen nor the clipboard.
		current = ""
		copyBtn.Disable()
		reportBtn.Disable()
		bar := widget.NewProgressBarInfinite()
		thinkingBar = bar
		msg := widget.NewLabel("Reading the passage…")
		msg.Alignment = fyne.TextAlignCenter
		// The budget is generous enough for a thinking model (aiRequestBudget),
		// so say so — otherwise a minute of spinner reads as a hang. Close is
		// the way out here (it calls stopThinking, so the ProgressBarInfinite
		// stops repainting the canvas); the Find surface has its own Cancel.
		hint := container.NewGridWrap(fyne.NewSize(260, captionHeightFor(2)),
			centeredCaption("Capable models can take a minute or more."))
		// Empty until the request says which model it is sending to, and new
		// with every request, so a re-run never shows the model before it.
		modelLine = newAIModelLine(aiModelInUse{})
		// Reads the field at TAP time (not the value at build time), so it
		// always abandons the request that is actually running — the mistake
		// the Find surface's first Cancel made.
		var fasterRow fyne.CanvasObject = spacer(0)
		if pid, fm, label, ok := fasterModelOffer(state); ok {
			fasterRow = container.NewVBox(spacer(6), fasterModelControl(label, func() {
				applyFasterModel(state, pid, fm)
				startFetch() // startFetch abandons the slow request before starting this one
			}))
		}
		cancelBtn := widget.NewButton("Cancel", func() {
			diagnose("panel_cancel")
			userClosed = true
			stopThinking() // cancels the request, not just the spinner
			if popup != nil {
				popup.Hide()
			}
			restore()
		})
		// The whole waiting column sits in a scroll. Its natural height (~290pt)
		// is FIXED while the panel's is not: on a landscape phone the body region
		// can be half that, and the column painted its tail — Cancel included —
		// over the footer or past the card. In the common case
		// the scroll has room and never engages. Spacers are gone (inside a
		// scroll they collapse to nothing anyway); squeezeWidthLayout stops the
		// scroll widening the column sideways (sheet_fit.go).
		waitCol := container.New(squeezeWidthLayout{}, container.NewVBox(
			spacer(8),
			container.NewCenter(msg), spacer(10),
			// Bounded, not full-bleed: a panel-wide bar reads as a banner
			// rather than a quiet progress hint, and it dwarfed the text.
			container.NewCenter(container.NewGridWrap(fyne.NewSize(240, bar.MinSize().Height), bar)),
			// The model, under the bar and over the hint, in the hint's style.
			spacer(10), container.NewCenter(modelLine.box), container.NewCenter(hint),
			// inputFrame: the theme's button fill IS this panel's card
			// colour (SurfaceAlt), so a bare Cancel here had no visible
			// box at all. The outline restores one.
			spacer(4), container.NewCenter(inputFrame(cancelBtn, pal.Border)),
			fasterRow,
		))
		waitContent := withQuestion(waitCol)
		waitScroll := container.NewVScroll(waitContent)
		// A model name that wraps makes the column taller: fit the panel to
		// it again, as below, so Cancel stays inside the scroll's reach.
		modelLine.relayout = func() { fitBody(waitScroll, waitContent.MinSize().Height+10) }
		// Replace, don't layer. The answer scroll underneath is empty in this
		// state, but it still carries the answer cap as its minimum size, and in
		// a stack that minimum wins — so the panel was sized for an answer that
		// is not there yet and the fit below could not shrink it. setError
		// already replaced; this is the same swap.
		body.Objects = []fyne.CanvasObject{waitScroll}
		body.Refresh()
		// Size the panel to the waiting column, as the answer and error states
		// size it to theirs. Re-entering this state from an error — Try again,
		// or the faster-model switch — used to leave the panel at the height the
		// previous state chose, which is what put Cancel under the footer.
		fitBody(waitScroll, waitContent.MinSize().Height+10)
	}
	setResult := func(text string) {
		renderStarted := devDiagnosticNow()
		diagnose("render_start", devDiagnosticMetric{Key: "bytes", Value: int64(len(text))})
		stopThinking()
		current = text
		copyBtn.Enable()
		reportBtn.Enable()
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
		// Pre-wrap at the body width so the height is right, then
		// fit the panel to the answer (capped at maxBodyH).
		answerView.Resize(fyne.NewSize(bodyW-16, answerView.MinSize().Height))
		if questionText != nil {
			questionText.Resize(fyne.NewSize(bodyW-16, questionText.MinSize().Height))
		}
		diagnose("render_wrapped", devDiagnosticSince(renderStarted))
		answerScroll.ScrollToTop()
		body.Objects = []fyne.CanvasObject{answerScroll}
		body.Refresh()
		fitBody(answerScroll, answerContent.MinSize().Height+10)
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
			fitBody(answerScroll, answerContent.MinSize().Height+10)
			diagnose("remeasure_done", devDiagnosticSince(started))
		})
	}

	setError := func(msg string, needsSettings bool) {
		diagnose("panel_error")
		stopThinking()
		// An error is not an answer aiCache holds, so reopening would ask
		// again, on the reader's key: the error state never comes back after a
		// light/dark rebuild. Only setResult registers, and nothing leads from
		// an answer back to a request today, so no registration is live here;
		// this forget is the belt to that, should a path ever be added.
		forgetSheetReopen(state, popup)
		// Drop the previous answer with its buttons. Copy was already disabled
		// here; Report was not, and nothing disabled it anywhere, so it kept
		// pointing at whatever `current` still held. Report mails the answer to
		// support as the flagging surface for AI content — mailing a stale one
		// from under an error message is the worst version of that.
		current = ""
		copyBtn.Disable()
		reportBtn.Disable()
		if markdownAnswer != nil {
			markdownAnswer.setText("")
		} else {
			answer.ParseMarkdown("")
		}
		lbl := widget.NewLabel(msg)
		lbl.Wrapping = fyne.TextWrapWord
		lbl.Alignment = fyne.TextAlignCenter
		var actBtn *widget.Button
		if needsSettings {
			actBtn = widget.NewButton("Open AI settings", func() {
				diagnose("panel_settings")
				// Every dismiss path undoes what opening the panel did: mark it
				// closed so a late reply cannot reopen it, and put the native
				// reading overlay back. Close and Cancel already did both; this
				// path did neither, and was correct only because the settings
				// sheet restores the overlay itself and rebuildWindow clears the
				// latch as a backstop. Two distant invariants is not a guarantee
				// — the backstop exists because this latch has been left on
				// before, and a blank verse pane is what that looks like.
				userClosed = true
				stopThinking()
				if popup != nil {
					popup.Hide()
				}
				restore()
				showAISettings(state)
			})
			actBtn.Importance = widget.HighImportance
		} else {
			actBtn = widget.NewButton("Try again", func() { startFetch() })
		}
		// Scrolled for the same reason as the waiting column: a long provider
		// error on a short canvas pushed the one actionable button into the
		// footer.
		errCol := container.New(squeezeWidthLayout{}, container.NewVBox(
			spacer(8), lbl, container.NewCenter(actBtn),
		))
		errContent := withQuestion(errCol)
		errScroll := container.NewVScroll(errContent)
		body.Objects = []fyne.CanvasObject{errScroll}
		body.Refresh()
		// Pre-wrap the message at the body width before measuring, for the same
		// reason the answer is pre-wrapped: a word-wrapped label reports its
		// true height only once it has wrapped at a known width.
		lbl.Resize(fyne.NewSize(bodyW-16, lbl.MinSize().Height))
		fitBody(errScroll, errContent.MinSize().Height+10)
	}

	startFetch = func() {
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
		// The request names its model on the waiting state it started, and
		// only there: a request the faster-model switch or Try again has
		// replaced (a newer gen), or one whose wait is over (Close, Cancel,
		// an answer — every exit stops the bar), can still report its retry.
		ctx = showAIModelOnUI(ctx, func(m aiModelInUse) {
			if gen != fetchGen || thinkingBar == nil {
				return
			}
			modelLine.show(m)
		})
		cancelFetch = cancel // so Close / Cancel can abandon THIS request
		go func() {
			defer cancelTimeout()
			result, err := aiActionRun(ctx, state, action, passage.Text, question, span)
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
					setError(friendlyAIError(err), isNoKeyError(err))
					return
				}
				setResult(result)
			})
		}()
	}

	content := container.NewBorder(header, footer, nil, nil, body)
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
		bodyW = ps.Width - 44
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
				fitBody(fittedBody, fittedBody.Content.MinSize().Height+10)
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
