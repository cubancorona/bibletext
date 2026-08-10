package bibletext

// What happens when a link carrying a note arrives with notes switched off.
//
// The first cut handed it straight to the browser. That is the wrong default:
// the reader tapped a link expecting BibleText, and being thrown into Safari
// with no explanation reads as a fault rather than as a decision. It also took
// the choice away — somebody sent them a message, and whether to read it is
// theirs to decide, not a setting's to decide permanently.
//
// So the app asks, once, at the moment it matters, and offers all three of the
// things a reader could reasonably want:
//
//   - read the note, without changing any setting → the browser
//   - ignore it and read the scripture → the passage, note dropped
//   - they have changed their mind about notes → turn them on and read it here
//
// The third is a quiet link rather than a third button, in the same style as the
// "Switch to a faster model" offer: it is the one option that changes a setting,
// so it should not compete with the two that do not.

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// offerNoteLinkChoice presents the choice and acts on it. It is only reached
// when the link has a note AND notes are off; every other link goes straight
// through.
//
// With no window to ask in — a cold start that has not built one yet — it falls
// back to the browser, because the note is the thing the link was sent for.
func offerNoteLinkChoice(state *AppState, rawURL string, t ShareTarget) {
	if state == nil || state.window == nil {
		openLinkInBrowser(rawURL)
		return
	}
	cnv := pickerCanvas(state)
	if cnv == nil {
		openLinkInBrowser(rawURL)
		return
	}
	pal := state.pal()

	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	var popup *widget.PopUp
	closed := false
	closeIt := func() {
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

	title := canvas.NewText("Someone added a note", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 18

	ref := canvas.NewText(shareTargetReference(t), pal.Accent)
	ref.TextStyle = fyne.TextStyle{Bold: true}
	ref.TextSize = subheadingTextSize

	body := widget.NewLabel("This link came with a message, and notes are turned off in BibleText. " +
		"You can read it in your browser, or open the passage on its own.")
	body.Wrapping = fyne.TextWrapWord

	// Reading the note leaves for the browser; reading the passage stays here.
	// Neither touches the setting — that is what the link underneath is for.
	inBrowser := widget.NewButton("Read it in the browser", func() {
		closeIt()
		openLinkInBrowser(rawURL)
	})
	inBrowser.Importance = widget.HighImportance

	passageOnly := widget.NewButton("Just the passage", func() {
		closeIt()
		bare := t
		bare.Note = "" // the note is dropped, not stored
		applyShareTarget(state, bare)
	})

	turnOn := fasterModelControl("Turn notes back on and read it here", func() {
		closeIt()
		setNotesEnabled(true)
		applyShareTarget(state, t)
	})

	form := container.NewVBox(
		title, ref,
		widget.NewSeparator(),
		body,
		container.NewBorder(nil, nil, passageOnly, container.NewHBox(inBrowser)),
		turnOn,
	)

	card := surface(container.NewPadded(form), pal.SurfaceAlt, pal.Border, fyne.Size{})
	popup = widget.NewModalPopUp(card, cnv)
	popup.Show()
	w := float32(440)
	if cw := cnv.Size().Width - 40; cw > 260 && w > cw {
		w = cw
	}
	popup.Resize(fyne.NewSize(w, card.MinSize().Height))
}

// shareTargetReference is the passage a link names, for the offer's subtitle —
// "John 3:16-18". Built from the target rather than app state because the app
// has not navigated there yet, and may never.
func shareTargetReference(t ShareTarget) string {
	ref := t.Book + " " + strconv.Itoa(t.Chapter)
	switch {
	case t.VerseLo > 0 && t.VerseHi > t.VerseLo:
		ref += ":" + strconv.Itoa(t.VerseLo) + "-" + strconv.Itoa(t.VerseHi)
	case t.VerseLo > 0:
		ref += ":" + strconv.Itoa(t.VerseLo)
	}
	return ref
}
