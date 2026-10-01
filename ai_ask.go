package bibletext

// The reading selection menu's "Ask": collect a free-form question about the selected
// passage, then open the AI answer panel for a narrative reply. This is deliberately
// distinct from the Search page's "Find", which returns matching verses — Ask returns
// prose grounded in the selection (see buildAskPrompt in ai.go).

import (
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

// promptAskQuestion shows a small input sheet for the reader's question, then hands off to
// showAIPanel(aiActionAsk, …).
//
// iOS keyboard handling: a centered modal would put the field mid-screen, under the soft
// keyboard (which a modal can't dodge — see the fyne-modal-keyboard notes). So on mobile we
// use a NON-modal popup whose card fills the canvas and is anchored at the top, with the
// field + buttons near the top, comfortably above the keyboard. A full-canvas card also
// means no tap ever lands "outside" it, so the popup can't self-dismiss and leave the
// native reading overlay hidden — every exit runs through closeAsk, which restores it. On
// desktop there's no soft keyboard, so a plain centered modal is cleaner.
func promptAskQuestion(state *AppState, selectedText string) {
	promptAskQuestionWith(state, selectedText, "")
}

// promptAskQuestionWith opens the sheet with question already in the field —
// "" for every ordinary open. The light/dark reopen passes what the reader had
// typed when the rebuild drained the sheet (sheet_reopen.go).
func promptAskQuestionWith(state *AppState, selectedText, question string) {
	if state == nil || state.window == nil {
		return
	}
	cnv := pickerCanvas(state)
	if cnv == nil {
		return
	}
	selectedText = strings.TrimSpace(selectedText)
	if selectedText == "" {
		return
	}
	pal := state.pal()
	mobile := fyne.CurrentDevice().IsMobile()

	// The native overlay floats above the canvas; hide it while the sheet is up.
	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	var popup *widget.PopUp
	closed := false
	closeAsk := func() {
		if closed {
			return
		}
		closed = true
		if popup != nil {
			popup.Hide()
		}
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	}

	// --- Header: prompt, reference, one-line selection preview. ---
	title := canvas.NewText("Ask about this passage", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 20

	ref := canvas.NewText(fmt.Sprintf("%s %d", state.CurrentBook, state.CurrentChapter), pal.Accent)
	ref.TextStyle = fyne.TextStyle{Bold: true}
	ref.TextSize = subheadingTextSize

	quote := widget.NewRichText(&widget.TextSegment{
		Text: quotedOneLine(selectedText, 240),
		Style: widget.RichTextStyle{
			ColorName: colorNameMuted,
			SizeName:  theme.SizeNameCaptionText,
			TextStyle: fyne.TextStyle{Italic: true},
			Inline:    true,
		},
	})
	quote.Wrapping = fyne.TextWrapOff
	quote.Truncation = fyne.TextTruncateEllipsis

	// --- Question field + actions. ---
	entry := newSearchEntry() // Return submits on iOS (see searchKeyEntry)
	entry.SetPlaceHolder("Ask a question about this passage…")
	entry.SetText(question)
	// Read when the rebuild comes, not now: the question as typed so far.
	reopen := func() { promptAskQuestionWith(state, selectedText, entry.Text) }

	doAsk := func() {
		q := strings.TrimSpace(entry.Text)
		if q == "" {
			return
		}
		closeAsk()
		showAIPanel(state, aiActionAsk, selectedText, q)
	}
	entry.OnSubmitted = func(string) { doAsk() }

	askBtn := widget.NewButton("Ask", doAsk)
	askBtn.Importance = widget.HighImportance
	cancelBtn := widget.NewButton("Cancel", closeAsk)
	actions := container.NewBorder(nil, nil, nil, container.NewHBox(cancelBtn, askBtn))

	hint := canvas.NewText("AI answers in its own words, grounded in this passage.", pal.TextMuted)
	hint.TextSize = 11

	form := container.NewVBox(
		title, ref, quote,
		widget.NewSeparator(),
		inputFrame(withCaret(state, entry), pal.Border),
		actions,
		hint,
	)

	focusEntry := func() {
		if state.window != nil {
			state.window.Canvas().Focus(entry)
		}
	}

	if !mobile {
		// Desktop: a compact centered modal.
		card := surface(container.NewPadded(form), pal.SurfaceAlt, pal.Border, fyne.Size{})
		popup = widget.NewModalPopUp(card, cnv)
		popup.Show()
		registerSheetReopen(state, popup, reopen)
		w := float32(460)
		if cw := cnv.Size().Width - 80; cw > 280 && w > cw {
			w = cw
		}
		popup.Resize(fyne.NewSize(w, card.MinSize().Height))
		focusEntry()
		return
	}

	// Mobile: a full-canvas, top-anchored, NON-modal sheet (see the doc comment). The
	// trailing spacer pins the form to the top, above the soft keyboard. The card
	// keeps clear of the side safe insets of a phone held sideways (clearOfSideInsets).
	body := container.NewVBox(form, layout.NewSpacer())
	card := surface(container.NewPadded(body), pal.SurfaceAlt, pal.Border, fyne.Size{})

	cw := cnv.Size().Width
	ch := cnv.Size().Height
	topY := float32(0)
	pos, sz := sheetArea(cnv)
	if sz.Height > 0 {
		topY = pos.Y
		ch = sz.Height
	}
	sideL, sideR := sideInsets(cw, pos, sz)
	sheet, _ := clearOfSideInsets(card, sideL, sideR)
	popup = widget.NewPopUp(sheet, cnv)
	popup.Resize(fyne.NewSize(cw, ch))
	popup.ShowAtPosition(fyne.NewPos(0, topY))
	registerSheetReopen(state, popup, reopen)
	focusEntry()

	// Catch an outside-tap close. The sheet is non-modal and sized to the
	// interactive area, so the safe-inset bands (status bar / home indicator —
	// and after a rotation, much more) are still tappable canvas: Fyne's
	// PopUp.Tapped runs Hide() directly there, bypassing closeAsk, which would
	// leave the reading overlay latched hidden (blank verse pane). Same 150ms
	// Visible() poll as the audio source menu, armed through the same seam
	// (sheet_timers.go); closeAsk is idempotent.
	var watch func()
	watch = func() {
		if popup == nil || !popup.Visible() {
			// Skip when another overlay owns the canvas (a sheet reopened
			// within one tick of a rebuild drain) — see goto.go's watchdog.
			if cnv.Overlays().Top() == nil {
				closeAsk()
			}
			return
		}
		sheetAfter(150*time.Millisecond, watch)
	}
	sheetAfter(150*time.Millisecond, watch)
}
