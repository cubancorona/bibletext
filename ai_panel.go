package holybible

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

func showAIPanel(state *AppState, action, selectedText string) {
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

	quote := canvas.NewText("“"+oneLinePreview(selectedText, 80)+"”", pal.TextMuted)
	quote.TextStyle = fyne.TextStyle{Italic: true}
	quote.TextSize = subheadingTextSize

	header := container.NewVBox(title, ref, quote, widget.NewSeparator())

	// --- Body: swaps between thinking / result / error.
	body := container.NewStack()

	// --- Footer: copy, close, and an honesty note.
	var current string
	var popup *widget.PopUp

	copyBtn := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() {
		if current != "" {
			state.window.Clipboard().SetContent(current)
		}
	})
	copyBtn.Importance = widget.LowImportance
	copyBtn.Disable()

	closeBtn := widget.NewButton("Close", func() {
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

	// --- State transitions.
	setThinking := func() {
		bar := widget.NewProgressBarInfinite()
		msg := widget.NewLabel("Reading the passage…")
		msg.Alignment = fyne.TextAlignCenter
		body.Objects = []fyne.CanvasObject{
			container.NewVBox(layout.NewSpacer(), msg, bar, layout.NewSpacer()),
		}
		body.Refresh()
	}
	setResult := func(text string) {
		current = text
		copyBtn.Enable()
		rt := widget.NewRichTextFromMarkdown(text)
		rt.Wrapping = fyne.TextWrapWord
		body.Objects = []fyne.CanvasObject{container.NewVScroll(rt)}
		body.Refresh()
	}

	var startFetch func()
	setError := func(msg string, needsSettings bool) {
		copyBtn.Disable()
		lbl := widget.NewLabel(msg)
		lbl.Wrapping = fyne.TextWrapWord
		lbl.Alignment = fyne.TextAlignCenter
		var actBtn *widget.Button
		if needsSettings {
			actBtn = widget.NewButton("Open AI settings", func() {
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
			result, err := runAIAction(ctx, state, action, selectedText)
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
		surface(container.NewPadded(content), pal.Surface, pal.Border, fyne.Size{}),
		cnv,
	)
	popup.Show()
	popup.Resize(aiPanelSize(cnv.Size()))
	startFetch()
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
	h := canvasSize.Height - 96
	if h > 620 {
		h = 620
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
