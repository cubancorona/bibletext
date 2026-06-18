package bibletext

import (
	"regexp"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// chapterTailPattern strips a trailing chapter/verse off a citation, leaving just
// the book-name portion to autocomplete. It matches the first whitespace-then-digit
// run through the end, so a leading book number survives: "John 3:16" -> "John",
// "1 John 3" -> "1 John", "1 John" -> "1 John".
var chapterTailPattern = regexp.MustCompile(`\s+\d.*$`)

// maxGotoSuggestions caps the book chips so the row stays tidy on a phone.
const maxGotoSuggestions = 4

// showGotoPopup opens a small modal for jumping straight to a reference: type a
// citation — "John 3:16", "Ps 23", "1 cor 13" — and press Go. While you type the
// book name, matching books appear as tappable chips. It is opened from the centered
// "Go to" button in the header, so the reading layout reserves no inline row.
//
// On iOS the reading pane is a native UITextView floating above the Fyne canvas, so
// (like showReferencePicker) we hide it while the modal is up and restore on close.
func showGotoPopup(state *AppState) {
	cnv := pickerCanvas(state)
	if cnv == nil {
		return
	}
	pal := state.pal()

	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	var popup *widget.PopUp
	closePopup := func() {
		if popup != nil {
			popup.Hide()
		}
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	}

	entry := widget.NewEntry()
	entry.PlaceHolder = "e.g. John 3:16, Ps 23, 1 Cor 13"

	hint := canvas.NewText("", pal.TextMuted)
	hint.TextSize = 12

	suggestions := container.NewHBox()

	var refreshSuggestions func(string)
	jump := func() {
		if goToReference(state, entry.Text) {
			closePopup()
			return
		}
		if strings.TrimSpace(entry.Text) != "" {
			hint.Text = "Hmm — that doesn’t look like a reference yet."
			hint.Refresh()
		}
	}
	refreshSuggestions = func(text string) {
		if hint.Text != "" {
			hint.Text = ""
			hint.Refresh()
		}
		suggestions.RemoveAll()

		portion := strings.TrimSpace(chapterTailPattern.ReplaceAllString(text, ""))
		// Stop suggesting once a chapter number is being typed (the book is settled):
		// the stripped portion then differs from the full text.
		if portion == "" || portion != strings.TrimSpace(text) || state.Bible == nil {
			suggestions.Refresh()
			return
		}
		matches := filterBooks(state.Bible.Books, portion)
		if len(matches) == 1 && strings.EqualFold(matches[0], portion) {
			suggestions.Refresh() // a single exact match needs no chip
			return
		}
		for i, b := range matches {
			if i >= maxGotoSuggestions {
				break
			}
			book := b // capture
			chip := widget.NewButton(book, func() {
				entry.SetText(book + " ")
				entry.CursorColumn = len([]rune(entry.Text))
				cnv.Focus(entry)
				entry.Refresh()
				refreshSuggestions(entry.Text)
			})
			chip.Importance = widget.LowImportance
			suggestions.Add(chip)
		}
		suggestions.Refresh()
	}
	entry.OnChanged = refreshSuggestions
	entry.OnSubmitted = func(string) { jump() }

	goBtn := widget.NewButtonWithIcon("Go", theme.NavigateNextIcon(), jump)
	goBtn.Importance = widget.HighImportance

	field := container.NewBorder(nil, nil, nil, goBtn, inputFrame(entry, pal.Border))

	title := canvas.NewText("Go to a verse", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 18
	header := pickerHeader(title, closePopup)

	body := container.NewVBox(
		header,
		container.NewPadded(field),
		container.NewHScroll(suggestions),
		container.NewPadded(hint),
	)

	popup = widget.NewModalPopUp(surface(container.NewPadded(body), pal.Surface, pal.Border, fyne.Size{}), cnv)
	popup.Show()
	w := minF(cnv.Size().Width-40, 460)
	popup.Resize(fyne.NewSize(w, popup.MinSize().Height))

	// Focus the field so the soft keyboard appears immediately on a phone.
	cnv.Focus(entry)
}

// gotoButton is the single centered control in the header that opens showGotoPopup.
// It is shorter than the title+subtitle column on its left, so dropping it into the
// header's center slot doesn't change the bar's height.
func gotoButton(state *AppState) fyne.CanvasObject {
	btn := widget.NewButtonWithIcon("Go to", theme.NavigateNextIcon(), func() { showGotoPopup(state) })
	btn.Importance = widget.LowImportance
	return container.NewCenter(btn)
}
