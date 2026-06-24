package bibletext

// The AI study result panel: a modal popup that shows a spinner while Gemini
// answers, then the response (or a friendly error with a retry). It reuses the
// chapter-picker modal approach — including hiding the native reading overlay
// while it's open, since that overlay floats above the Fyne canvas and would
// otherwise paint on top of the popup.

import (
	"context"
	"fmt"
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
		Text: "“" + oneLinePreview(selectedText, 300) + "”",
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

	// A running ProgressBarInfinite ticks an animation every frame, which keeps the
	// whole canvas marked dirty and forces a full-tree repaint at ~20fps — that
	// competes with scrolling and lingers (until renderer-cache expiry) if the panel
	// is dismissed mid-spin. Track the live spinner and Stop() it on every exit from
	// the thinking state so the animation never outlives the panel.
	var thinkingBar *widget.ProgressBarInfinite
	stopThinking := func() {
		if thinkingBar != nil {
			thinkingBar.Stop()
			thinkingBar = nil
		}
	}

	copyBtn := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() {
		if current != "" {
			state.window.Clipboard().SetContent(current)
		}
	})
	copyBtn.Importance = widget.LowImportance
	copyBtn.Disable()

	closeBtn := widget.NewButton("Close", func() {
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
		container.NewHBox(layout.NewSpacer(), copyBtn, closeBtn),
	)

	// --- State transitions. setThinking/setError layer their content on top of
	// the (empty) answer scroll; setResult fills the scroll and drops the overlay.
	setThinking := func() {
		bar := widget.NewProgressBarInfinite()
		thinkingBar = bar
		msg := widget.NewLabel("Reading the passage…")
		msg.Alignment = fyne.TextAlignCenter
		body.Objects = []fyne.CanvasObject{
			answerScroll,
			container.NewVBox(layout.NewSpacer(), msg, bar, layout.NewSpacer()),
		}
		body.Refresh()
	}
	setResult := func(text string) {
		stopThinking()
		current = text
		copyBtn.Enable()
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

	var startFetch func()
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
		body.Objects = []fyne.CanvasObject{
			container.NewVBox(layout.NewSpacer(), lbl, container.NewCenter(actBtn), layout.NewSpacer()),
		}
		body.Refresh()
	}

	startFetch = func() {
		setThinking()
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
			defer cancel()
			result, err := runAIAction(ctx, state, action, selectedText, question)
			fyne.Do(func() {
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

// oneLinePreview collapses whitespace and truncates to a single short line.
func oneLinePreview(s string, maxRunes int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > maxRunes {
		return string(r[:maxRunes]) + "…"
	}
	return s
}
