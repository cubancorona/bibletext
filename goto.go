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
		books = alphabeticalBooks(books)
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
	card := surface(container.NewPadded(body), pal.Surface, pal.Border, fyne.Size{})

	if withVerse {
		// The verse box sits at the BOTTOM of the card, but a modal popup always
		// centers and can't be moved (see the fyne-modal-keyboard note), so a centered
		// card's bottom would be under the iOS keyboard. Anchor the card near the TOP
		// via a full-canvas layout, so its bottom row clears the keyboard. The modal
		// underlay still dims the whole screen.
		boxW, _ := pickerSplitSize(cnv)
		safePos, _ := cnv.InteractiveArea()
		topY := safePos.Y + 8
		boxH := cnv.Size().Height*0.52 - topY
		if boxH > 520 {
			boxH = 520
		}
		if boxH < 300 {
			boxH = 300
		}
		content := container.New(topBoxLayout{box: fyne.NewSize(boxW, boxH), topY: topY}, card)
		popup = widget.NewModalPopUp(content, cnv)
		popup.Show()
		popup.Resize(cnv.Size()) // fill the canvas so the layout gets the full size
	} else {
		popup = widget.NewModalPopUp(card, cnv)
		popup.Show()
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

// alphabeticalBooks returns the books sorted by name, treating a leading number as a
// book ordinal rather than a sort character: "1 John"/"2 John"/"3 John" group under
// "John" (after the gospel of John), "1 Corinthians" under "Corinthians", etc.
func alphabeticalBooks(books []string) []string {
	out := append([]string(nil), books...)
	base := func(b string) (string, int) {
		if i := strings.IndexByte(b, ' '); i > 0 {
			if n, err := strconv.Atoi(b[:i]); err == nil {
				return strings.ToLower(b[i+1:]), n // "1 John" -> ("john", 1)
			}
		}
		return strings.ToLower(b), 0
	}
	sort.Slice(out, func(i, j int) bool {
		ni, oi := base(out[i])
		nj, oj := base(out[j])
		if ni != nj {
			return ni < nj
		}
		return oi < oj
	})
	return out
}

// goToChapterWithVerse navigates to book+chapter, honoring the optional verse box:
// empty → chapter top; "16" → highlight v16; "16-18" → highlight verses 16 through 18.
// An out-of-range start or unparseable input silently falls back to the chapter top.
func goToChapterWithVerse(state *AppState, book string, chapter int, verseText string) {
	if start, end, ok := parseVerseBox(verseText); ok && state.Bible != nil {
		if match := state.Bible.GetVerse(book, chapter, start); match != nil {
			goToVerseRange(state, book, chapter, start, end) // wash the whole range, scroll to start
			return
		}
	}
	navigateToReference(state, book, chapter)
}

// parseVerseBox reads the optional verse box. "16" / " 16 " → (16, 16); a range
// "16-18" / "16–18" / "16:18" → (16, 18). The first number is the start; a missing,
// reversed, or unparseable end collapses to a single verse (end == start). Returns
// ok=false for empty or non-numeric input.
func parseVerseBox(s string) (start, end int, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false
	}
	a, b := s, ""
	for _, sep := range []string{"-", "–", ":"} { // hyphen, en-dash, or colon splits a range
		if i := strings.Index(s, sep); i >= 0 {
			a, b = s[:i], s[i+len(sep):]
			break
		}
	}
	start, err := strconv.Atoi(strings.TrimSpace(a))
	if err != nil || start < 1 {
		return 0, 0, false
	}
	end = start
	if b = strings.TrimSpace(b); b != "" {
		if e, err := strconv.Atoi(b); err == nil && e >= start {
			end = e
		}
	}
	return start, end, true
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

// topBoxLayout positions its single child as a fixed-size box, horizontally centered
// and anchored at a given Y, within the full parent (the canvas-sized modal content).
// It lets the Goto picker's card sit near the top — so its bottom verse box clears the
// iOS keyboard — even though a modal PopUp itself can't be moved (it always centers).
type topBoxLayout struct {
	box  fyne.Size
	topY float32
}

func (l topBoxLayout) MinSize(_ []fyne.CanvasObject) fyne.Size { return fyne.Size{} }

func (l topBoxLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) == 0 {
		return
	}
	w := l.box.Width
	if w > size.Width {
		w = size.Width
	}
	h := l.box.Height
	if l.topY+h > size.Height {
		h = size.Height - l.topY
	}
	objs[0].Resize(fyne.NewSize(w, h))
	objs[0].Move(fyne.NewPos((size.Width-w)/2, l.topY))
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
