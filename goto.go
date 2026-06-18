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

// gotoPickerModal is the shared book + chapter picker. Two flavours:
//
//   - withVerse=true  (the header "Go to" button → showGotoPicker): books are
//     ALPHABETICAL and a verse/range box + Go button sit at the bottom. Tapping a
//     book or chapter only SELECTS it; Go (or Return in the box) commits, honoring
//     the verse box. Committing via Go — not a grid tap — is deliberate: once you
//     type a verse the iOS keyboard covers the grid, so Go stays reachable above it.
//   - withVerse=false (the book-name / "Chapter N of M" tap → showChapterPicker):
//     the usual quick picker — books in BIBLE order, no verse box, and tapping a
//     chapter navigates immediately.
//
// On iOS the reading pane is a native UITextView floating above the Fyne canvas, so
// (like every modal here) we hide it while the picker is up and restore on close.
func gotoPickerModal(state *AppState, withVerse bool) {
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

	// Header "Go to" lists books alphabetically (find by name); the usual picker keeps
	// canonical Bible order.
	books := state.Bible.Books
	if withVerse {
		books = append([]string(nil), books...)
		sort.Strings(books)
	}
	selectedBook := state.CurrentBook
	selectedChapter := state.CurrentChapter

	// Which chapter the grid fills: the live selection (verse flavour), else just the
	// reader's current chapter when they're browsing their own book.
	highlightChapter := func() int {
		if withVerse {
			return selectedChapter
		}
		if selectedBook == state.CurrentBook {
			return state.CurrentChapter
		}
		return -1
	}

	var verseEntry *widget.Entry
	commit := func() {
		verseText := ""
		if verseEntry != nil {
			verseText = verseEntry.Text
		}
		goToChapterWithVerse(state, selectedBook, selectedChapter, verseText)
		closePicker()
	}

	chapterPane := container.NewStack()
	var renderChapters func()
	renderChapters = func() {
		chapterPane.Objects = []fyne.CanvasObject{
			referenceChapterGrid(state, pal, selectedBook, highlightChapter(), func(ch int) {
				selectedChapter = ch
				if withVerse {
					renderChapters() // select + re-highlight; Go commits
				} else {
					navigateToReference(state, selectedBook, ch) // usual picker: go now
					closePicker()
				}
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

	title := canvas.NewText("Go to", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 18
	header := pickerHeader(title, closePicker)

	divider := canvas.NewRectangle(pal.Border)
	divider.SetMinSize(fyne.NewSize(1, 0))
	left := container.New(fixedWidthLayout{width: 152},
		container.NewBorder(nil, nil, nil, divider, list))

	var bottom fyne.CanvasObject
	if withVerse {
		verseEntry = widget.NewEntry()
		verseEntry.SetPlaceHolder("verse — e.g. 16 or 16-18")
		verseEntry.OnSubmitted = func(string) { commit() }
		// Restore the blinking caret the base theme zeroes out, scoped to this field.
		var base fyne.Theme = theme.DefaultTheme()
		if state.theme != nil {
			base = state.theme
		}
		caret := container.NewThemeOverride(verseEntry, caretTheme{Theme: base})
		goBtn := widget.NewButton("Go", commit)
		goBtn.Importance = widget.HighImportance
		bottom = container.NewPadded(container.NewBorder(nil, nil, nil, goBtn, inputFrame(caret, pal.Border)))
	}

	body := container.NewBorder(header, bottom, left, nil, container.NewPadded(chapterPane))
	popup = widget.NewModalPopUp(surface(container.NewPadded(body), pal.Surface, pal.Border, fyne.Size{}), cnv)
	popup.Show()

	if withVerse {
		// Keep the modal short and anchored near the top so its bottom row (verse box +
		// Go) clears the iOS keyboard. PopUp.Move sets innerPos, which the renderer
		// honors, so the anchor sticks even as the keyboard opens.
		cw, chH := cnv.Size().Width, cnv.Size().Height
		safePos, _ := cnv.InteractiveArea()
		popW, _ := pickerSplitSize(cnv)
		topY := safePos.Y + 8
		popH := chH*0.46 - topY
		if popH > 520 {
			popH = 520
		}
		if popH < 280 {
			popH = 280
		}
		popup.Resize(fyne.NewSize(popW, popH))
		popup.Move(fyne.NewPos((cw-popW)/2, topY))
	} else {
		w, h := pickerSplitSize(cnv) // roomy + centered; no keyboard in this flavour
		popup.Resize(fyne.NewSize(w, h))
	}

	for i, b := range books {
		if b == selectedBook {
			list.Select(i)
			list.ScrollTo(i)
			break
		}
	}
}

// showGotoPicker is the header "Go to" button's picker: alphabetical books + a verse
// box. showChapterPicker is the usual book-name / chapter-line tap: Bible order, no
// verse box, chapter-tap navigates immediately.
func showGotoPicker(state *AppState)    { gotoPickerModal(state, true) }
func showChapterPicker(state *AppState) { gotoPickerModal(state, false) }

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

// smallChipTheme shrinks a button's text + padding so it reads as a small, quiet
// chip rather than a full-size button.
type smallChipTheme struct{ fyne.Theme }

func (t smallChipTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 12
	case theme.SizeNameInnerPadding:
		return 5
	}
	return t.Theme.Size(name)
}

// gotoButton is the small, quiet "Go to" chip in the header center slot that opens
// showGotoPicker. A low-importance button (no loud fill) with shrunk text inside a
// thin rounded outline reads as a small, elegant, barely-there button — not a flat
// label (which a plain low-importance button looks like) and not an intrusive
// accent-filled block. It stays short, so the header height is unchanged.
func gotoButton(state *AppState) fyne.CanvasObject {
	pal := state.pal()
	btn := widget.NewButton("Go to", func() { showGotoPicker(state) })
	btn.Importance = widget.LowImportance
	var base fyne.Theme = theme.DefaultTheme()
	if state.theme != nil {
		base = state.theme
	}
	chip := container.NewThemeOverride(btn, smallChipTheme{Theme: base})
	return container.NewCenter(inputFrame(chip, pal.Border))
}
