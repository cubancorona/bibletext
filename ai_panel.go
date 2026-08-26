package bibletext

// The AI study result panel: a modal popup that shows a spinner while Gemini
// answers, then the response (or a friendly error with a retry). It reuses the
// chapter-picker modal approach — including hiding the native reading overlay
// while it's open, since that overlay floats above the Fyne canvas and would
// otherwise paint on top of the popup.

import (
	"context"
	"fmt"
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

func showAIPanel(state *AppState, action, selectedText, question string) {
	if state == nil || state.window == nil {
		return
	}
	cnv := state.window.Canvas()
	if cnv == nil {
		return
	}
	pal := state.pal()

	// The native overlay floats above the canvas; hide it while the modal is up.
	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	restore := func() {
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	}

	// --- Header: action title, reference, and a one-line preview of the selection.
	title := canvas.NewText(aiActionTitle(action), pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 22

	ref := canvas.NewText(fmt.Sprintf("%s %d", state.CurrentBook, state.CurrentChapter), pal.Accent)
	ref.TextStyle = fyne.TextStyle{Bold: true}
	ref.TextSize = subheadingTextSize

	// The quoted selection stays to one quiet line. A canvas.Text just clips at
	// the panel edge with no hint there's more, so use a RichText that truncates
	// with an ellipsis (it's given the panel width by the VBox). The char cap is
	// only a sanity bound; the visual truncation is what keeps it tidy.
	quote := widget.NewRichText(&widget.TextSegment{
		Text: quotedOneLine(selectedText, 300),
		Style: widget.RichTextStyle{
			ColorName: colorNameMuted,
			SizeName:  theme.SizeNameCaptionText,
			TextStyle: fyne.TextStyle{Italic: true},
			Inline:    true,
		},
	})
	quote.Wrapping = fyne.TextWrapOff
	quote.Truncation = fyne.TextTruncateEllipsis

	// For "Ask", the title is the generic "Answer", so show the reader's actual question
	// (word-wrapped, bold) above the grounding passage preview.
	headerItems := []fyne.CanvasObject{title, ref}
	if action == aiActionAsk {
		if q := strings.TrimSpace(question); q != "" {
			ql := widget.NewRichText(&widget.TextSegment{
				Text:  q,
				Style: widget.RichTextStyle{TextStyle: fyne.TextStyle{Bold: true}},
			})
			ql.Wrapping = fyne.TextWrapWord
			headerItems = append(headerItems, ql)
		}
	}
	headerItems = append(headerItems, quote, widget.NewSeparator())
	header := container.NewVBox(headerItems...)

	// --- Body: a vertical scroll holds the answer, with the thinking and error
	// states layered on top of it. The panel grows to fit the answer (capped at
	// maxBodyH) so short answers show in full with no empty space, and only very
	// long ones need to scroll.
	ps := aiPanelSize(cnv.Size())
	bodyW := ps.Width - 44
	maxBodyH := ps.Height - 168 // headroom for header, footer and padding
	answer := widget.NewRichTextFromMarkdown("")
	answer.Wrapping = fyne.TextWrapWord
	answerScroll := container.NewVScroll(answer)
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
	// email to support with the question + the generated answer. Enabled once an
	// answer is shown. Only this panel renders free-form AI prose — "Find" shows
	// canonical scripture only — so this is the one surface that needs it.
	reportBtn := widget.NewButton("Report", func() {
		if current == "" {
			return
		}
		mailBody := fmt.Sprintf(
			"I'd like to report AI-generated content shown in BibleText.\n\nReference: %s %d\nQuestion: %s\n\nResponse:\n%s\n\nMy concern:\n",
			state.CurrentBook, state.CurrentChapter, strings.TrimSpace(question), current)
		mu := &url.URL{
			Scheme:   "mailto",
			Opaque:   SupportMailtoRecipient(),
			RawQuery: url.Values{"subject": {"BibleText: report AI content"}, "body": {mailBody}}.Encode(),
		}
		openExternalURL(mu)
	})
	reportBtn.Importance = widget.LowImportance
	reportBtn.Disable()

	closeBtn := widget.NewButton("Close", func() {
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

	// --- State transitions. setThinking/setError layer their content on top of
	// the (empty) answer scroll; setResult fills the scroll and drops the overlay.
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

	setThinking := func() {
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
		body.Objects = []fyne.CanvasObject{
			answerScroll,
			container.NewVScroll(container.New(squeezeWidthLayout{}, container.NewVBox(
				spacer(8),
				container.NewCenter(msg), spacer(10),
				// Bounded, not full-bleed: a panel-wide bar reads as a banner
				// rather than a quiet progress hint, and it dwarfed the text.
				container.NewCenter(container.NewGridWrap(fyne.NewSize(240, bar.MinSize().Height), bar)),
				spacer(10), container.NewCenter(hint),
				// inputFrame: the theme's button fill IS this panel's card
				// colour (SurfaceAlt), so a bare Cancel here had no visible
				// box at all. The outline restores one.
				spacer(4), container.NewCenter(inputFrame(cancelBtn, pal.Border)),
				fasterRow,
			))),
		}
		body.Refresh()
	}
	setResult := func(text string) {
		stopThinking()
		current = text
		copyBtn.Enable()
		reportBtn.Enable()
		answer.ParseMarkdown(text)
		// A word-wrapped RichText only reports its true height once it has wrapped
		// at a known width. Pre-wrap at the body width so the height is right, then
		// fit the panel to the answer (capped at maxBodyH).
		answer.Resize(fyne.NewSize(bodyW-16, answer.MinSize().Height))
		fitH := answer.MinSize().Height + 10
		if fitH > maxBodyH {
			fitH = maxBodyH
		}
		answerScroll.SetMinSize(fyne.NewSize(bodyW, fitH))
		answerScroll.ScrollToTop()
		body.Objects = []fyne.CanvasObject{answerScroll}
		body.Refresh()
		if popup != nil {
			popup.Resize(fyne.NewSize(ps.Width, fitH+158))
		}
		// Re-measure once the real layout has landed so the height is exact.
		time.AfterFunc(40*time.Millisecond, func() {
			fyne.Do(func() {
				answer.Refresh()
				answerScroll.Refresh()
				if popup != nil {
					fit := answer.MinSize().Height + 10
					if fit > maxBodyH {
						fit = maxBodyH
					}
					answerScroll.SetMinSize(fyne.NewSize(bodyW, fit))
					popup.Resize(fyne.NewSize(ps.Width, fit+158))
				}
			})
		})
	}

	setError := func(msg string, needsSettings bool) {
		stopThinking()
		copyBtn.Disable()
		answer.ParseMarkdown("")
		lbl := widget.NewLabel(msg)
		lbl.Wrapping = fyne.TextWrapWord
		lbl.Alignment = fyne.TextAlignCenter
		var actBtn *widget.Button
		if needsSettings {
			actBtn = widget.NewButton("Open AI settings", func() {
				stopThinking()
				if popup != nil {
					popup.Hide()
				}
				showAISettings(state)
			})
			actBtn.Importance = widget.HighImportance
		} else {
			actBtn = widget.NewButton("Try again", func() { startFetch() })
		}
		// Scrolled for the same reason as the waiting column: a long provider
		// error on a short canvas pushed the one actionable button into the
		// footer.
		body.Objects = []fyne.CanvasObject{
			container.NewVScroll(container.New(squeezeWidthLayout{}, container.NewVBox(
				spacer(8), lbl, container.NewCenter(actBtn),
			))),
		}
		body.Refresh()
	}

	startFetch = func() {
		// Abandon any request already in flight FIRST (the faster-model switch,
		// Try again): cancel its context and stop its spinner, or the
		// superseded request would keep billing the reader's key for the whole
		// aiRequestBudget while its detached ProgressBarInfinite keeps
		// repainting the canvas — the exact leak documented above cancelFetch.
		stopThinking()
		fetchGen++
		gen := fetchGen
		setThinking()
		ctx, cancel := context.WithCancel(context.Background())
		ctx, cancelTimeout := context.WithTimeout(ctx, aiRequestBudget)
		cancelFetch = cancel // so Close / Cancel can abandon THIS request
		go func() {
			defer cancelTimeout()
			result, err := aiActionRun(ctx, state, action, selectedText, question)
			fyne.Do(func() {
				if gen != fetchGen {
					return // superseded — a newer request owns the panel now
				}
				cancelFetch = nil // settled: nothing left to abandon
				// The reader dismissed this panel (Close / Cancel). Painting a
				// late answer into it would repaint a hidden, detached popup —
				// and with the generous aiRequestBudget that answer can arrive long
				// after they moved on. The reply is in aiCache, so reopening
				// the same action shows it instantly.
				if userClosed {
					return
				}
				// The panel may have been EVICTED (not user-closed) by a
				// rebuildWindow drain — a theme-variant flip — while the
				// request ran. Don't deliver the answer into the hidden,
				// detached popup: reopen a fresh panel instead. The re-run
				// hits aiCache, so the already-paid answer shows instantly
				// in the new palette; an errored fetch just ends quietly.
				if !userClosed && popup != nil && !popup.Visible() {
					stopThinking()
					if err == nil {
						showAIPanel(state, action, selectedText, question)
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
	startFetch()
}

// minF returns the smaller of two float32 values.
func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
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
