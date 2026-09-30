//go:build !ios && !android

package bibletext

// THE DESKTOP SHARE CONFIRMATION: the sheet every Windows and Linux share
// ends in (share_fallback.go), and the recorded counterpart there of the
// system share sheet the other platforms open (docs/PLATFORM_MATRIX.md,
// Sharing).
//
// Neither platform has a system share sheet to call: Linux has none (the
// portal request has been open since 2016), and the Windows one is not yet
// built. What the reader gets instead is the clipboard, and until this sheet
// the only sign of it was a 13pt "Copied to the clipboard" pill at the
// window's foot for 1.4 seconds, barely lighter than the page, gone at the
// next click, and on Share with note drawn in the frame the composer closed
// and the note card appeared, so the eye was elsewhere. A reader pressed
// Share, saw the menu or the composer close, and concluded nothing had
// happened, while the note they wrote waited on a clipboard they did not
// know about.
//
// The sheet says what happened and what to do next, shows the exact text
// that is on the clipboard, and stays until the reader is done with it:
//
//   - the heading and one line by verb, the words recorded in
//     docs/BACKLOG.md ("Linux and Windows: an in-app share sheet");
//   - a read-only box with the clipboard's contents, wrapping, and
//     scrolling when the text is long;
//   - Email…, when the desktop has a mail client to hand the text to
//     (share_email.go); Copy again, for a clipboard something else has taken
//     since; and Done. Escape and Return are Done too, wherever the caret is.
//
// It is modal, so a stray click cannot lose it, and a real sheet: it
// registers a reopen, so a light/dark rebuild brings it back with the same
// text (sheet_reopen.go), a refit, so a window resize sizes it again
// (sheet_refit.go), and it opens below the header (sheet_fit.go). Opened
// from the verse-of-the-day card it stacks over the card, and its reopen
// brings the card back beneath it.
//
// Share as image keeps its save to ~/Downloads and the file-manager reveal
// and ends in the same sheet with no clipboard box: the line says where the
// picture went, and Email… attaches it where the platform can.
//
// UI goroutine only, like every sheet.

import (
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The sheet's words. The heading and the three verb lines are the approved
// wording; the image share, which copies nothing, has a heading of its own.
const (
	shareSheetHeading      = "Copied — ready to paste"
	shareSheetImageHeading = "Picture saved"
	shareLineNote          = "Your note and the link are on the clipboard. Paste them into a message or email to send your note."
	shareLineCitation      = "The verse and its citation are on the clipboard. Paste them into a message, email or document."
	shareLineLink          = "The link is on the clipboard. Paste it into a message or email."
	shareLineImage         = "The picture is saved in Downloads and shown in your file manager."
	shareLineImageTemp     = "The picture is shown in your file manager."
	shareLineCopiedAgain   = "Copied again."
	shareButtonCopyAgain   = "Copy again"
	shareButtonEmail       = "Email…"
	shareButtonDone        = "Done"
)

// shareCopiedAgainFor is how long the line reads "Copied again." after Copy
// again before the verb's own line returns.
const shareCopiedAgainFor = 1500 * time.Millisecond

// The clipboard box's height: enough for a note and its link without
// scrolling, never so tall that the sheet cannot clear the header, and the
// least it gives up to, where the scroll does the rest.
const (
	shareBoxMaxHeight  = 190
	shareBoxMinHeight  = 56
	shareBoxHeightStep = 24
)

// shareVerb is which text share a message came from.
type shareVerb int

const (
	shareVerbCitation shareVerb = iota
	shareVerbLink
	shareVerbNote
)

// shareDone describes one finished share to the sheet: what to say, what is
// on the clipboard, and what a mail would carry.
type shareDone struct {
	line       string // the sentence under the heading
	text       string // what is on the clipboard; "" for the image share, which has no box
	subject    string // the mail's subject: the citation
	body       string // the mail's body
	attachment string // a file the mail attaches: the image share's PNG
}

// shareVerbOf tells which verb composed a text share from the message's own
// shape, which is all the platform seam carries (nativeShareText takes the
// string alone, on every platform). The shapes are the composers' in
// share.go and cannot be confused: a link share is the citation line and the
// link on the line below it; a note share puts the note and a blank line
// before those two; a citation share ends in the citation line and carries
// no link. A note share whose note was left empty is byte for byte the link
// share (shareVerseLinkWithNote), and is told as one, which is what it is.
func shareVerbOf(s string) shareVerb {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if strings.HasPrefix(lines[len(lines)-1], shareLinkBase+"/") {
		if len(lines) > 2 {
			return shareVerbNote
		}
		return shareVerbLink
	}
	return shareVerbCitation
}

// shareSubjectOf is the citation of a text share, for a mail's subject:
// "John 3:16 (World English Bible)". A citation share ends in that line
// behind its em dash (citationLine); the link shares carry it on the line
// before the link.
func shareSubjectOf(s string, verb shareVerb) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	line := lines[len(lines)-1]
	if verb != shareVerbCitation && len(lines) >= 2 {
		line = lines[len(lines)-2]
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "— "))
}

// shareDoneForText is the sheet's brief for a text share that is now on the
// clipboard.
func shareDoneForText(s string) shareDone {
	verb := shareVerbOf(s)
	line := shareLineCitation
	switch verb {
	case shareVerbLink:
		line = shareLineLink
	case shareVerbNote:
		line = shareLineNote
	}
	return shareDone{line: line, text: s, subject: shareSubjectOf(s, verb), body: s}
}

// setShareClipboard puts s on the system clipboard, through the app's
// clipboard rather than the window's: the same clipboard on a real driver,
// and under the test driver the one that can be read back.
func setShareClipboard(s string) {
	app := fyne.CurrentApp()
	if app == nil {
		return
	}
	if cb := app.Clipboard(); cb != nil {
		cb.SetContent(s)
	}
}

// showShareCopiedSheet opens the confirmation sheet for d over whatever is
// on the canvas. It returns at once; the sheet stays until Done, Escape or
// Return.
func showShareCopiedSheet(state *AppState, d shareDone) {
	if state == nil || state.window == nil {
		return
	}
	cnv := pickerCanvas(state)
	if cnv == nil {
		return
	}
	pal := state.pal()

	// The native reading overlay floats above the canvas where there is one
	// (the macOS mimic of this path); on Windows and Linux the hide is nil
	// and the show is the sheet-close consume point (installSheetCloseConsume).
	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}

	heading := shareSheetHeading
	if d.text == "" {
		heading = shareSheetImageHeading
	}
	title := canvas.NewText(heading, pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 20

	line := widget.NewLabel(d.line)
	line.Wrapping = fyne.TextWrapWord
	// The line's slot keeps the height the verb's own line wraps to while it
	// reads "Copied again.", which takes one line where the verb's takes two:
	// a slot that shrank would lift the box and the buttons for the moment and
	// drop them back, under a pointer on its way to Done or to a second Copy
	// again (heldLineLayout).
	held := widget.NewLabel(d.line)
	held.Wrapping = fyne.TextWrapWord
	lineSlot := container.New(heldLineLayout{held: held}, line)

	// The clipboard's contents, exactly, inside a scroll so a long note or a
	// block quotation cannot push Done off the sheet. The scroll's height is
	// set by fit below, from the text's wrapped height. The label is not
	// selectable: a selection in a Fyne label holds the caret, the desktop
	// driver then hands every key to it and never to the canvas's handler,
	// and the label answers none, so after a drag across the box Escape and
	// Return did nothing until the reader clicked elsewhere. The whole text
	// is on the clipboard already, and Copy again puts it back.
	var boxText *widget.Label
	var boxScroll *container.Scroll
	var box fyne.CanvasObject
	if d.text != "" {
		boxText = widget.NewLabel(d.text)
		boxText.Wrapping = fyne.TextWrapWord
		boxScroll = container.NewVScroll(container.New(squeezeWidthLayout{}, boxText))
		box = inputFrame(container.NewPadded(boxScroll), pal.Border)
	}

	var popup *widget.PopUp
	closed := false
	// Escape is routed through state.dismissSheet by the desktop canvas
	// (installShortcuts). The sheet beneath, when there is one, put its own
	// there — the verse-of-the-day card — and gets it back when this closes,
	// so Escape then closes the card as it did before the share.
	prevDismiss := state.dismissSheet
	// Return closes as Done does, through the canvas's own key handler,
	// wrapped while the sheet shows and put back as it closes (below).
	prevKeys := cnv.OnTypedKey()
	closeSheet := func() {
		if closed {
			return
		}
		closed = true
		if popup != nil {
			popup.Hide()
		}
		state.dismissSheet = prevDismiss
		cnv.SetOnTypedKey(prevKeys)
		// Restore only when nothing else owns the canvas: over the
		// verse-of-the-day card the card is still up, and its own close
		// restores.
		if state.showReadingOverlay != nil && cnv.Overlays().Top() == nil {
			state.showReadingOverlay()
		}
	}

	// Copy again: the line says so for a moment, then reads as before. A
	// second press within the moment starts the moment again, and the line
	// is left alone if the sheet has closed by the time it ends.
	copies := 0
	copyAgain := newShareSheetButton(shareButtonCopyAgain, closeSheet, func() {
		setShareClipboard(d.text)
		copies++
		n := copies
		line.SetText(shareLineCopiedAgain)
		sheetAfter(shareCopiedAgainFor, func() {
			if n == copies && !closed {
				line.SetText(d.line)
			}
		})
	})

	// Email… hands the text, or the picture, to the desktop's mail client
	// (composeShareEmail). It is shown only once the platform has answered
	// that there is one to hand it to, asked off the UI goroutine so the
	// sheet never waits on the desktop bus (shareEmailProbe); a compose runs
	// there too, and reports nothing back but a log line, the copy having
	// already succeeded.
	subject, body, attachment := d.subject, d.body, d.attachment
	email := newShareSheetButton(shareButtonEmail, closeSheet, func() {
		go func() {
			if err := shareEmailCompose(subject, body, attachment); err != nil {
				fyne.LogError("could not hand the share to a mail client", err)
			}
		}()
	})
	email.Hide()

	done := newShareSheetButton(shareButtonDone, closeSheet, closeSheet)
	done.Importance = widget.HighImportance

	// Email… comes first in the row, which is right-aligned: it appears a
	// moment after the sheet, when the probe answers, and an item appearing
	// at the row's left end moves nothing to its right. In the middle it
	// pushed Copy again left by its own width and took its place, under a
	// pointer already on its way there.
	buttons := container.NewHBox(email, done)
	if d.text != "" {
		buttons = container.NewHBox(email, copyAgain, done)
	}
	actions := container.NewBorder(nil, nil, nil, buttons)

	parts := []fyne.CanvasObject{title, lineSlot}
	if box != nil {
		parts = append(parts, box)
	}
	parts = append(parts, actions)
	form := container.NewVBox(parts...)

	card := surface(container.NewPadded(form), pal.SurfaceAlt, pal.Border, fyne.Size{})
	popup = widget.NewModalPopUp(card, cnv)
	popup.Show()

	// Return closes as Done does. The desktop driver hands a key to the
	// focused widget, or else to the canvas's handler, and the sheet's
	// resting state is nothing focused: it opens with no caret, a tapped
	// button gives the caret up, and nothing else on it can take the caret
	// (the box, above). So the canvas's handler — the desktop's
	// Escape route (installShortcuts) — is wrapped while the sheet is on top
	// and put back as it closes. A rebuild's drain closes the sheet without
	// closeSheet and installs the canvas's handler afresh, which replaces
	// the wrapper along with everything else the canvas had. Tab is the one
	// way the caret reaches the sheet, onto a button, and the buttons answer
	// Escape and Return themselves (shareSheetButton).
	cnv.SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if (ev.Name == fyne.KeyReturn || ev.Name == fyne.KeyEnter) &&
			popup != nil && popup.Visible() && cnv.Overlays().Top() == popup {
			closeSheet()
			return
		}
		if prevKeys != nil {
			prevKeys(ev)
		}
	})

	w := float32(460)
	if cw := cnv.Size().Width - 80; cw > 280 && w > cw {
		w = cw
	}
	// Two passes, as the composer takes: the line and the box wrap, so their
	// heights depend on the width they are laid out at. The first pass lays
	// the form out at the sheet's width; the box is then given the height its
	// text wraps to, capped, and the second pass takes the height that gives.
	// Where the sheet would still start inside the header (headerClearance)
	// the box gives up height, a step at a time down to its least, and the
	// scroll shows the rest.
	fit := func() {
		popup.Resize(fyne.NewSize(w, card.MinSize().Height))
		if boxScroll == nil {
			popup.Resize(fyne.NewSize(w, card.MinSize().Height))
			return
		}
		want := boxText.MinSize().Height + 2*theme.Padding()
		if want > shareBoxMaxHeight {
			want = shareBoxMaxHeight
		}
		if want < shareBoxMinHeight {
			want = shareBoxMinHeight
		}
		boxScroll.SetMinSize(fyne.NewSize(0, want))
		popup.Resize(fyne.NewSize(w, card.MinSize().Height))
		room := clearOfHeader(cnv.Size().Height, cnv.Size().Height, headerClearance(state))
		for want > shareBoxMinHeight && popup.MinSize().Height > room {
			want -= shareBoxHeightStep
			if want < shareBoxMinHeight {
				want = shareBoxMinHeight
			}
			boxScroll.SetMinSize(fyne.NewSize(0, want))
			popup.Resize(fyne.NewSize(w, card.MinSize().Height))
		}
	}
	fit()

	state.dismissSheet = func() {
		if popup != nil && popup.Visible() {
			closeSheet()
		}
	}
	// A light/dark rebuild brings the sheet back showing the same thing, and
	// the sheet it was opened over beneath it (takeReopenBeneath): the card
	// is where the reader was, and Done returns them to it.
	registerSheetReopenCapture(state, popup, func() func() {
		under := takeReopenBeneath(state, popup)
		return func() {
			if under != nil {
				under()
			}
			showShareCopiedSheet(state, d)
		}
	})
	registerSheetRefit(state, popup, fit)

	shareEmailProbe(attachment != "", func(ok bool) {
		if !ok || closed || popup == nil || !popup.Visible() {
			return
		}
		email.Show()
		buttons.Refresh()
		fit()
	})
}

// shareSheetButton is a button on the confirmation that keeps Escape and
// Return with the sheet while it holds the caret. Tab gives a button the
// caret, and the desktop driver then hands it every key, the canvas's
// handler only ever hearing a key when nothing is focused; a widget.Button
// answers Space alone, so after a Tab, Escape and Return did nothing at all.
// On these, Escape and Return close the sheet as Done does, whichever button
// holds the caret — Return is Done on this sheet, as the sheet's rule has it,
// not "press the focused button" — and Space presses the button that holds
// it, as on any button.
type shareSheetButton struct {
	widget.Button
	closeSheet func()
}

func newShareSheetButton(label string, closeSheet, tapped func()) *shareSheetButton {
	b := &shareSheetButton{closeSheet: closeSheet}
	b.Text = label
	b.OnTapped = tapped
	b.ExtendBaseWidget(b)
	return b
}

// TypedKey closes the sheet on Escape and Return, and leaves every other
// key to the button.
func (b *shareSheetButton) TypedKey(ev *fyne.KeyEvent) {
	switch ev.Name {
	case fyne.KeyEscape, fyne.KeyReturn, fyne.KeyEnter:
		b.closeSheet()
	default:
		b.Button.TypedKey(ev)
	}
}

// heldLineLayout lays the sheet's line out over its slot and makes the slot
// as tall as the verb's own line wraps to at the slot's width, whatever the
// line reads for the moment. held is a copy of the verb's line kept off the
// canvas — never drawn, never read — and measured at that width.
type heldLineLayout struct{ held *widget.Label }

func (l heldLineLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	l.held.Resize(size)
	for _, o := range objects {
		o.Move(fyne.Position{})
		o.Resize(size)
	}
}

func (l heldLineLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	min := l.held.MinSize()
	for _, o := range objects {
		min = min.Max(o.MinSize())
	}
	return min
}
