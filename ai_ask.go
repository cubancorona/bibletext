package bibletext

import (
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// promptAskQuestion captures the passage before collecting a question. A link
// can move the reader while the composer is open; the request still belongs
// to the selection and translation the card showed when it opened.
func promptAskQuestion(state *AppState, selectedText string, span selSpan) {
	promptAskQuestionWith(state, selectedText, "", span)
}

func promptAskQuestionWith(state *AppState, selectedText, question string, span selSpan) {
	if state == nil || state.window == nil {
		return
	}
	selectedText = strings.TrimSpace(selectedText)
	if selectedText == "" {
		return
	}
	promptAskQuestionForPassage(state, aiPassageFor(state, selectedText, span), question, span)
}

// promptAskQuestionForPassage reopens with the same passage and the question
// typed so far. iOS uses the shared native multiline editor; other platforms
// keep the Fyne field. Ask produces a prose answer, rather than Search's list
// of matching verses.
func promptAskQuestionForPassage(state *AppState, passage aiPassage, question string, span selSpan) {
	if state == nil || state.window == nil || strings.TrimSpace(passage.Text) == "" {
		return
	}
	cnv := pickerCanvas(state)
	if cnv == nil {
		return
	}
	pal := state.pal()
	mobile := fyne.CurrentDevice().IsMobile()
	useNative := askQuestionEnabled && mobile && noteEntryNative()
	composerID := devDiagnosticID()
	nativeMetric := int64(0)
	if useNative {
		nativeMetric = 1
	}
	devDiagnosticEvent("composer_open", devDiagnosticMetric{Key: "panel_id", Value: int64(composerID)},
		devDiagnosticMetric{Key: "native", Value: nativeMetric})
	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}

	var popup *widget.PopUp
	closed, sheetOpen := false, true
	noteEntryOwner++
	owner := noteEntryOwner
	closeAsk := func() {
		if closed {
			return
		}
		closed, sheetOpen = true, false
		devDiagnosticEvent("composer_close", devDiagnosticMetric{Key: "panel_id", Value: int64(composerID)})
		if owner == noteEntryOwner {
			noteEntryOnChanged = nil
			noteEntryHide()
		}
		if popup != nil {
			popup.Hide()
		}
		if state.showReadingOverlay != nil && cnv.Overlays().Top() == nil {
			state.showReadingOverlay()
		}
	}

	title := canvas.NewText("Ask about this passage", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 20
	passageView, excerpt := newPassageExcerptPreview(passage.Ref, passage.previewVersion(), passage.previewText(),
		noteExcerptMaxLinesFor(mobile, cnv.Size().Height), pal)

	entry := newSearchEntry()
	const placeholder = "Ask a question about this passage…"
	entry.SetPlaceHolder(placeholder)
	entry.SetText(question)
	questionText := func() string {
		if useNative {
			return noteEntryTyped()
		}
		return entry.Text
	}
	doAsk := func() {
		if closed || owner != noteEntryOwner {
			return
		}
		q := strings.TrimSpace(questionText())
		if q == "" {
			return
		}
		devDiagnosticEvent("composer_submit", devDiagnosticMetric{Key: "panel_id", Value: int64(composerID)},
			devDiagnosticMetric{Key: "question_bytes", Value: int64(len(q))},
			devDiagnosticMetric{Key: "passage_bytes", Value: int64(len(passage.Text))})
		closeAsk()
		showAIPanelForPassage(state, aiActionAsk, passage, q, span)
	}
	// The Fyne field retains Return-to-submit. The native UITextView is
	// multiline: Return inserts a newline and the Ask button submits it.
	entry.OnSubmitted = func(string) { doAsk() }
	askBtn := widget.NewButton("Ask", doAsk)
	askBtn.Importance = widget.HighImportance
	cancelBtn := widget.NewButton("Cancel", closeAsk)
	actions := container.NewBorder(nil, nil, nil, container.NewHBox(cancelBtn, askBtn))

	var entrySlot fyne.CanvasObject
	if useNative {
		entrySlot = newNoteEntrySlot(&sheetOpen)
	} else {
		entrySlot = inputFrame(withCaret(state, entry), pal.Border)
	}
	hint := widget.NewRichText(&widget.TextSegment{
		Text:  "Your question and selected passage will be sent to your chosen AI provider.",
		Style: widget.RichTextStyle{ColorName: colorNameMuted, SizeName: theme.SizeNameCaptionText},
	})
	hint.Wrapping = fyne.TextWrapWord
	form := container.NewVBox(title, passageView, widget.NewSeparator(), entrySlot, actions, hint)
	focusEntry := func() { cnv.Focus(entry) }
	registerReopen := func() {
		registerSheetReopenCapture(state, popup, func() func() {
			q := questionText()
			return func() { promptAskQuestionForPassage(state, passage, q, span) }
		})
	}

	if !mobile {
		card := surface(container.NewPadded(form), pal.SurfaceAlt, pal.Border, fyne.Size{})
		popup = widget.NewModalPopUp(card, cnv)
		popup.Show()
		rows := excerpt.maxLines
		fit := func() {
			w := min(float32(460), max(float32(280), cnv.Size().Width-80))
			excerpt.setMaxLines(rows)
			setPassagePreviewCompact(passageView, false)
			popup.Resize(fyne.NewSize(w, card.MinSize().Height))
			popup.Resize(fyne.NewSize(w, card.MinSize().Height))
			room := clearOfHeader(cnv.Size().Height, cnv.Size().Height, headerClearance(state))
			for n := rows; n > 1 && popup.MinSize().Height > room; n-- {
				excerpt.setMaxLines(n - 1)
				popup.Resize(fyne.NewSize(w, card.MinSize().Height))
			}
			if popup.MinSize().Height > room {
				setPassagePreviewCompact(passageView, true)
				popup.Resize(fyne.NewSize(w, card.MinSize().Height))
			}
		}
		fit()
		registerReopen()
		registerSheetRefit(state, popup, fit)
		focusEntry()
		return
	}

	// The top-anchored non-modal card keeps its keyboard-free foot gap
	// across refits, rather than staying short after the keyboard goes away.
	compactTitle := canvas.NewText("Ask a question", pal.Text)
	compactTitle.TextStyle, compactTitle.TextSize = title.TextStyle, 18
	body := newComposeViewport(cnv, form, compactTitle, passageView, entrySlot, actions, excerpt,
		passagePreviewCitation(passage.Ref, passage.previewVersion()), useNative, false, focusEntry)
	card := surface(container.NewPadded(body), pal.SurfaceAlt, pal.Border, fyne.Size{})
	cw, ch := cnv.Size().Width, cnv.Size().Height
	topY := float32(0)
	pos, sz := noteSheetArea(cnv)
	if sz.Height > 0 {
		topY, ch = pos.Y, sz.Height
	}
	sideL, sideR := sideInsets(cw, pos, sz)
	sheet, refitSides := clearOfSideInsets(card, sideL, sideR)
	popup = widget.NewPopUp(sheet, cnv)
	footGap := cnv.Size().Height - topY - ch
	popup.Resize(fyne.NewSize(cw, ch))
	popup.ShowAtPosition(fyne.NewPos(0, topY))
	body.Refresh()
	registerReopen()
	lastCanvas := cnv.Size()
	lastAreaPos, lastAreaW := pos, sz.Width
	lastInputPos, lastInputSize := canvasArea(cnv)
	refit := func() {
		now := cnv.Size()
		relayout := func() {
			excerpt.setMaxLines(noteExcerptMaxLinesFor(true, now.Height))
			apos, asz := noteSheetArea(cnv)
			refitSides(sideInsets(now.Width, apos, asz))
			popup.Resize(fyne.NewSize(now.Width, now.Height-topY-footGap))
			body.Refresh()
		}
		if slot, ok := entrySlot.(*noteEntrySlot); ok {
			slot.settle(relayout)
		} else {
			relayout()
		}
	}
	var watch func()
	watch = func() {
		if popup == nil || !popup.Visible() {
			closeAsk()
			return
		}
		now := cnv.Size()
		apos, asz := noteSheetArea(cnv)
		inputPos, inputSize := canvasArea(cnv)
		if now != lastCanvas || apos != lastAreaPos || asz.Width != lastAreaW || inputPos != lastInputPos || inputSize != lastInputSize {
			lastCanvas, lastAreaPos, lastAreaW = now, apos, asz.Width
			lastInputPos, lastInputSize = inputPos, inputSize
			refit()
		}
		noteSheetAfter(150*time.Millisecond, watch)
	}
	noteSheetAfter(150*time.Millisecond, watch)
	if useNative {
		noteEntryShow(question, placeholder, pal)
		// Ask has no note character limit or live counter. Its delegate
		// must not update a former note composer's counter.
		noteEntryOnChanged = nil
		entrySlot.(*noteEntrySlot).push()
		return
	}
	focusEntry()
}
