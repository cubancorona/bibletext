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
// calendar-style chapter grid (right), and an optional verse/range box at the bottom.
// Tapping a chapter is the only commit — it navigates immediately, honoring the verse
// box if it has a value ("16" → highlight v16; "16-18" → highlight the range's first
// verse; empty → chapter top). The box is never auto-focused, so the common
// book+chapter case is two taps with no keyboard. It's also opened by the header
// "Go to" button and by tapping the book name / "Chapter N of M" line.
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
	selected := state.CurrentBook

	// Verse / range box (bottom). It is never auto-focused — the chapter grid is the
	// commit, so the keyboard only ever rises if the reader chooses to type a verse.
	verseEntry := widget.NewEntry()
	verseEntry.SetPlaceHolder("verse — e.g. 16 or 16-18")
	verseEntry.OnSubmitted = func(string) {
		// Return just lowers the keyboard so the (now fully visible) grid is tappable;
		// committing is always a chapter tap, which consults this box's value.
		cnv.Unfocus()
	}

	chapterPane := container.NewStack()
	renderChapters := func(book string) {
		chapterPane.Objects = []fyne.CanvasObject{referenceChapterGrid(state, pal, book, func(ch int) {
			goToChapterWithVerse(state, book, ch, verseEntry.Text)
			closePicker()
		})}
		chapterPane.Refresh()
	}
	renderChapters(selected)

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
			if books[i] == selected {
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
		selected = books[id]
		renderChapters(selected)
		list.Refresh()
	}

	// Restore the blinking caret the base theme zeroes out (SizeNameInputBorder = 0),
	// scoped to just this field; inputFrame draws the visible border.
	var base fyne.Theme = theme.DefaultTheme()
	if state.theme != nil {
		base = state.theme
	}
	caret := container.NewThemeOverride(verseEntry, caretTheme{Theme: base})
	verseBox := inputFrame(caret, pal.Border)

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
		container.NewPadded(verseBox),    // bottom: verse/range box
		left,                             // left: book list
		nil,                              // right
		container.NewPadded(chapterPane), // center: chapter grid
	)

	popup = widget.NewModalPopUp(surface(container.NewPadded(body), pal.Surface, pal.Border, fyne.Size{}), cnv)
	popup.Show()

	// Size + anchor near the top so the bottom verse box stays above the iOS soft
	// keyboard when focused. A *widget.PopUp re-centres on Refresh (which keyboard-open
	// triggers), so we re-pin on cursor changes.
	cw, chH := cnv.Size().Width, cnv.Size().Height
	safePos, _ := cnv.InteractiveArea()
	popW, _ := pickerSplitSize(cnv)
	popH := chH*0.52 - safePos.Y
	if popH > 560 {
		popH = 560
	}
	if popH < 300 {
		popH = 300
	}
	popup.Resize(fyne.NewSize(popW, popH))
	pinTop := func() { popup.Move(fyne.NewPos((cw-popW)/2, safePos.Y+8)) }
	pinTop()
	verseEntry.OnCursorChanged = pinTop

	// Reveal + select the current book.
	for i, b := range books {
		if b == selected {
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
// showGotoPicker. MediumImportance gives it a themed outline so it reads as a button
// (not flat text); it's still shorter than the title+subtitle column, so the header
// height is unchanged.
func gotoButton(state *AppState) fyne.CanvasObject {
	btn := widget.NewButtonWithIcon("Go to", theme.NavigateNextIcon(), func() { showGotoPicker(state) })
	btn.Importance = widget.MediumImportance
	return container.NewCenter(btn)
}
