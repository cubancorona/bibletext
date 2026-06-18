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

// buildGotoBar is a compact "jump to a reference" field for the top of the reading
// view. Type a citation — "John 3:16", "Ps 23", "1 cor 13" — and press return to go
// there; while you type the book name, matching books appear as tappable chips so
// you don't have to spell the whole thing. It reuses the shared reference parser
// (aliases + prefixes), so it understands the same forms the search box does.
func buildGotoBar(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	entry := widget.NewEntry()
	entry.PlaceHolder = "Go to a verse — e.g. John 3:16"

	hint := canvas.NewText("", pal.TextMuted)
	hint.TextSize = 12

	// Chips live in a horizontal scroll so a book with many matches ("1/2/3 John")
	// never pushes the field off-screen; the row collapses to nothing when empty.
	suggestions := container.NewHBox()

	var refreshSuggestions func(string)

	jump := func() {
		if goToReference(state, entry.Text) {
			entry.SetText("")
			refreshSuggestions("")
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
		// A single exact match needs no chip — just press return.
		if len(matches) == 1 && strings.EqualFold(matches[0], portion) {
			suggestions.Refresh()
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
				if state.window != nil {
					state.window.Canvas().Focus(entry)
				}
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

	goBtn := widget.NewButtonWithIcon("", theme.NavigateNextIcon(), jump)
	goBtn.Importance = widget.LowImportance

	field := container.NewBorder(nil, nil, nil, goBtn, inputFrame(entry, pal.Border))

	return container.NewVBox(
		container.NewPadded(field),
		container.NewHScroll(suggestions),
		hint,
	)
}
