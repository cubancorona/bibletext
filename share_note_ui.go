package bibletext

// Shared notes in the app: the sheet that writes one, and the card that shows
// one that arrived.
//
// Untagged on purpose — no build tag, no cgo — so the whole flow unit-tests on
// the host and the same code serves iPhone, iPad, macOS, Android, Windows and
// Linux. That matters more here than usual: the reading pane is a native
// overlay on three of those platforms, and a per-platform bubble would have
// been four implementations of the same conversation.
//
// Both surfaces observe the overlay invariant ([redacted-retired-private-reference] → Native text
// overlays): the native reading view floats ABOVE the Fyne canvas, so anything
// Fyne draws must hide it on open and restore it on close, or it renders behind
// the scripture and looks like nothing happened.

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// promptShareNote collects an optional note, then shares the link carrying it.
// It is a SECOND verb beside "Share as link", never a step in front of it: the
// plain share stays one tap, because most shares carry no note and a modal in
// everyone's way to serve the minority is the wrong trade.
func promptShareNote(state *AppState, selectedText string) {
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

	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	var popup *widget.PopUp
	closed := false
	closeSheet := func() {
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

	title := canvas.NewText("Add a note", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 20

	ref := canvas.NewText(shareNoteReference(state, selectedText), pal.Accent)
	ref.TextStyle = fyne.TextStyle{Bold: true}
	ref.TextSize = subheadingTextSize

	entry := newSearchEntry()
	entry.SetPlaceHolder("Say something about this passage…")

	// A live count, because the cap is real: the note rides inside the link, and
	// a link a messenger truncates is a link that opens nothing.
	left := canvas.NewText("", pal.TextMuted)
	left.TextSize = 11
	updateLeft := func() {
		n := utf8.RuneCountInString(strings.TrimSpace(entry.Text))
		switch {
		case n == 0:
			left.Text = "Optional. The note travels inside the link — it is never uploaded."
			left.Color = pal.TextMuted
		case n > NoteMaxRunes:
			left.Text = "Too long by " + strconv.Itoa(n-NoteMaxRunes) + " — it will be shortened"
			left.Color = pal.RedLetter
		default:
			left.Text = strconv.Itoa(NoteMaxRunes-n) + " characters left"
			left.Color = pal.TextMuted
		}
		left.Refresh()
	}
	entry.OnChanged = func(string) { updateLeft() }
	updateLeft()

	send := func() {
		note := strings.TrimSpace(entry.Text)
		closeSheet()
		shareVerseLinkWithNote(state, selectedText, note)
	}
	entry.OnSubmitted = func(string) { send() }

	sendBtn := widget.NewButton("Share", send)
	sendBtn.Importance = widget.HighImportance
	cancelBtn := widget.NewButton("Cancel", closeSheet)
	actions := container.NewBorder(nil, nil, nil, container.NewHBox(cancelBtn, sendBtn))

	form := container.NewVBox(
		title, ref,
		widget.NewSeparator(),
		inputFrame(withCaret(state, entry), pal.Border),
		left,
		actions,
	)

	focusEntry := func() {
		if state.window != nil {
			state.window.Canvas().Focus(entry)
		}
	}

	if !mobile {
		card := surface(container.NewPadded(form), pal.SurfaceAlt, pal.Border, fyne.Size{})
		popup = widget.NewModalPopUp(card, cnv)
		popup.Show()
		w := float32(460)
		if cw := cnv.Size().Width - 80; cw > 280 && w > cw {
			w = cw
		}
		popup.Resize(fyne.NewSize(w, card.MinSize().Height))
		focusEntry()
		return
	}

	// Mobile: the same full-canvas, top-anchored, non-modal sheet promptAskQuestion
	// uses, and for the same reason — a centered modal puts the field under the
	// soft keyboard, and a full-canvas card means no tap lands "outside" it and
	// leaves the reading overlay latched hidden.
	body := container.NewVBox(form, layout.NewSpacer())
	card := surface(container.NewPadded(body), pal.SurfaceAlt, pal.Border, fyne.Size{})
	popup = widget.NewPopUp(card, cnv)
	cw, ch := cnv.Size().Width, cnv.Size().Height
	topY := float32(0)
	if pos, sz := cnv.InteractiveArea(); sz.Height > 0 {
		topY, ch = pos.Y, sz.Height
	}
	popup.Resize(fyne.NewSize(cw, ch))
	popup.ShowAtPosition(fyne.NewPos(0, topY))
	focusEntry()
}

// showSharedNote presents a note that arrived on a link. It is deliberately a
// card the reader dismisses rather than something drawn into the scripture:
// the note is somebody's message about the passage, and it must never be
// mistaken for the passage. The attribution line is load-bearing — a message
// that could pass for the app's own voice would be a phishing surface.
func showSharedNote(state *AppState, note string) {
	if state == nil || state.window == nil {
		return
	}
	note = strings.TrimSpace(note)
	if note == "" {
		return
	}
	cnv := pickerCanvas(state)
	if cnv == nil {
		return
	}
	pal := state.pal()

	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	var popup *widget.PopUp
	closed := false
	closeCard := func() {
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

	who := canvas.NewText("Note from Friend", pal.TextMuted)
	who.TextSize = 11

	ref := canvas.NewText(state.CurrentBook+" "+strconv.Itoa(state.CurrentChapter), pal.Accent)
	ref.TextStyle = fyne.TextStyle{Bold: true}
	ref.TextSize = subheadingTextSize

	// A plain wrapped Label: the note is TEXT. Nothing here interprets markup,
	// and nothing in it becomes tappable.
	body := widget.NewLabel(note)
	body.Wrapping = fyne.TextWrapWord

	okBtn := widget.NewButton("Read the passage", closeCard)
	okBtn.Importance = widget.HighImportance

	form := container.NewVBox(
		who, ref,
		widget.NewSeparator(),
		body,
		container.NewBorder(nil, nil, nil, container.NewHBox(okBtn)),
	)

	card := surface(container.NewPadded(form), pal.SurfaceAlt, pal.Border, fyne.Size{})
	popup = widget.NewModalPopUp(card, cnv)
	popup.Show()
	w := float32(460)
	if cw := cnv.Size().Width - 40; cw > 260 && w > cw {
		w = cw
	}
	popup.Resize(fyne.NewSize(w, card.MinSize().Height))
}

// shareNoteReference is the passage label on the compose sheet — the same
// citation the share itself will carry, so the writer can see what they are
// attaching the note to.
func shareNoteReference(state *AppState, selection string) string {
	if _, cite := prepareShareQuote(state, selection); cite != "" {
		return cite
	}
	return state.CurrentBook + " " + strconv.Itoa(state.CurrentChapter)
}
