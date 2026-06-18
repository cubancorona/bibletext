package bibletext

import (
	"image/color"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// showGotoPicker is the "Go to" modal: a scrolling alphabetical book list (left), a
// calendar-style chapter grid (right), and an optional verse/range box + a Go button
// at the bottom. Tapping a book or chapter SELECTS it (highlights, no navigation);
// Go (or Return in the verse box) commits — to the selected book+chapter, honoring
// the verse box ("16" → highlight v16; "16-18"/"16:18" → highlight the range's first
// verse; empty → chapter top). Committing via Go (not a grid tap) is deliberate: once
// you type a verse the iOS keyboard covers the grid, so the Go button — which stays
// above the keyboard — is how you go. It's opened by the header "Go to" button and by
// tapping the book name / "Chapter N of M" line in the reading view.
//
// On iOS the reading pane is a native UITextView floating above the Fyne canvas, so
// (like every modal here) we hide it while the picker is up and restore on close.
func showGotoPicker(state *AppState) {
	cnv := pickerCanvas(state)
	if cnv == nil {
		return
	}
	pal := state.pal()

	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	var popup *widget.PopUp
	closePicker := func() {
		if popup != nil {
			popup.Hide()
		}
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	}

	// Books alphabetically, so a known book is quick to find by name.
	books := append([]string(nil), state.Bible.Books...)
	sort.Strings(books)
	selectedBook := state.CurrentBook
	selectedChapter := state.CurrentChapter

	// Verse / range box (bottom). caretTheme restores the blinking caret the base
	// theme zeroes out; inputFrame draws the visible border.
	verseEntry := widget.NewEntry()
	verseEntry.SetPlaceHolder("verse — e.g. 16 or 16-18")

	commit := func() {
		goToChapterWithVerse(state, selectedBook, selectedChapter, verseEntry.Text)
		closePicker()
	}
	verseEntry.OnSubmitted = func(string) { commit() }

	chapterPane := container.NewStack()
	var renderChapters func()
	renderChapters = func() {
		chapterPane.Objects = []fyne.CanvasObject{
			referenceChapterGrid(state, pal, selectedBook, selectedChapter, func(ch int) {
				selectedChapter = ch
				renderChapters() // re-highlight the newly selected chapter
			}),
		}
		chapterPane.Refresh()
	}
	renderChapters()

	list := widget.NewList(
		func() int { return len(books) },
		func() fyne.CanvasObject {
			lbl := widget.NewLabel("")
			lbl.Truncation = fyne.TextTruncateEllipsis
			return lbl
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			lbl := o.(*widget.Label)
			lbl.SetText(books[i])
			if books[i] == selectedBook {
				lbl.TextStyle = fyne.TextStyle{Bold: true}
			} else {
				lbl.TextStyle = fyne.TextStyle{}
			}
			lbl.Refresh()
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(books) {
			return
		}
		selectedBook = books[id]
		// Default the chapter: keep the reader's chapter if they re-picked the book
		// they're in, else the new book's first chapter.
		if selectedBook == state.CurrentBook {
			selectedChapter = state.CurrentChapter
		} else if nums := state.Bible.GetChapterNumbersForBook(selectedBook); len(nums) > 0 {
			selectedChapter = nums[0]
		} else {
			selectedChapter = 1
		}
		renderChapters()
		list.Refresh()
	}

	// Restore the blinking caret the base theme zeroes out (SizeNameInputBorder = 0),
	// scoped to just this field; inputFrame draws the visible border.
	var base fyne.Theme = theme.DefaultTheme()
	if state.theme != nil {
		base = state.theme
	}
	caret := container.NewThemeOverride(verseEntry, caretTheme{Theme: base})

	goBtn := widget.NewButton("Go", commit)
	goBtn.Importance = widget.HighImportance
	bottomRow := container.NewBorder(nil, nil, nil, goBtn, inputFrame(caret, pal.Border))

	title := canvas.NewText("Go to", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 18
	header := pickerHeader(title, closePicker)

	divider := canvas.NewRectangle(pal.Border)
	divider.SetMinSize(fyne.NewSize(1, 0))
	left := container.New(fixedWidthLayout{width: 152},
		container.NewBorder(nil, nil, nil, divider, list))
	body := container.NewBorder(
		header,                           // top
		container.NewPadded(bottomRow),   // bottom: verse/range box + Go
		left,                             // left: book list
		nil,                              // right
		container.NewPadded(chapterPane), // center: chapter grid
	)

	popup = widget.NewModalPopUp(surface(container.NewPadded(body), pal.Surface, pal.Border, fyne.Size{}), cnv)
	popup.Show()

	// Keep the modal short and anchored near the top so its bottom row (the verse box
	// + Go) stays clear of the iOS keyboard, which can cover the lower ~half of the
	// screen. PopUp.Move sets innerPos, which the renderer honors, so this sticks.
	cw, chH := cnv.Size().Width, cnv.Size().Height
	safePos, _ := cnv.InteractiveArea()
	popW, _ := pickerSplitSize(cnv)
	topY := safePos.Y + 8
	popH := chH*0.46 - topY // bottom edge lands ~46% down — above a tall soft keyboard
	if popH > 520 {
		popH = 520
	}
	if popH < 280 {
		popH = 280
	}
	popup.Resize(fyne.NewSize(popW, popH))
	popup.Move(fyne.NewPos((cw-popW)/2, topY))

	// Reveal + select the current book.
	for i, b := range books {
		if b == selectedBook {
			list.Select(i)
			list.ScrollTo(i)
			break
		}
	}
}

// goToChapterWithVerse navigates to book+chapter, honoring the optional verse box:
// empty → chapter top; "16" → highlight v16; a range → highlight its first verse. An
// out-of-range or unparseable verse silently falls back to the chapter top.
func goToChapterWithVerse(state *AppState, book string, chapter int, verseText string) {
	if v, ok := parseVerseBox(verseText); ok && state.Bible != nil {
		if match := state.Bible.GetVerse(book, chapter, v); match != nil {
			goToVerse(state, *match) // sets the highlight + scrolls to it
			return
		}
	}
	navigateToReference(state, book, chapter)
}

// parseVerseBox reads the optional verse box: "16" / " 16 " → 16; a range
// "16-18" / "16–18" / "16:18" → its first number (16); anything else → ok=false. The
// end of a range is intentionally ignored — we highlight where the passage begins.
func parseVerseBox(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if i := strings.IndexAny(s, "-–:"); i >= 0 { // hyphen, en-dash, or colon splits a range
		s = s[:i]
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

// caretTheme re-enables the blinking Entry caret for a single field. The base theme
// zeroes SizeNameInputBorder (theme.go) so the read-only reading Entry shows no caret
// — in Fyne 2.7.4 that size IS the caret width (entry.go moveCursor). We restore 1px
// here for the verse box and make Fyne's own (now 1px) Entry border transparent, so
// only this field gets a cursor and the border isn't doubled (inputFrame draws ours).
type caretTheme struct{ fyne.Theme }

func (c caretTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameInputBorder {
		return 1
	}
	return c.Theme.Size(name)
}

func (c caretTheme) Color(name fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameInputBorder {
		return color.Transparent
	}
	return c.Theme.Color(name, v)
}

// gotoButton is the small "Go to" button in the header center slot that opens
// showGotoPicker. HighImportance fills it with the accent so it reads clearly as a
// button against the (same-coloured) header surface — a Medium/Low button's fill
// matches the header and looks like flat text. It stays short, so the header height
// is unchanged.
func gotoButton(state *AppState) fyne.CanvasObject {
	btn := widget.NewButtonWithIcon("Go to", theme.NavigateNextIcon(), func() { showGotoPicker(state) })
	btn.Importance = widget.HighImportance
	return container.NewCenter(btn)
}
