package bibletext

import (
	"image/color"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/mobile"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// numberEntry is an Entry that requests the iOS number pad. iOS number pads have no
// hyphen, so the Goto picker takes a verse range as TWO number fields (start + end)
// rather than one hyphenated field. onFocus fires on focus gain/loss so the picker can
// shrink to clear the keyboard only while a field is being typed into.
type numberEntry struct {
	widget.Entry
	onFocus func(focused bool)
}

func (e *numberEntry) Keyboard() mobile.KeyboardType { return mobile.NumberKeyboard }

func (e *numberEntry) FocusGained() {
	e.Entry.FocusGained()
	if e.onFocus != nil {
		e.onFocus(true)
	}
}

func (e *numberEntry) FocusLost() {
	e.Entry.FocusLost()
	if e.onFocus != nil {
		e.onFocus(false)
	}
}

func newNumberEntry() *numberEntry {
	e := &numberEntry{}
	e.ExtendBaseWidget(e)
	return e
}

// gotoPickerModal is the shared book + chapter picker. The RIGHT pane is always the
// chapter grid for the selected book; the LEFT pane and bottom differ by flavour:
//
//   - withVerse=true  (the header "Go to" button → showGotoPicker): the LEFT pane is a
//     two-stage alphabet navigator (letter grid → that letter's books → back row), and a
//     verse-range row (start + end number fields) + Go button sit at the bottom. Tapping
//     a book or chapter only SELECTS it; Go commits, honoring the range. Committing via
//     Go — not a grid tap — is deliberate: once the iOS keyboard is up it covers the
//     grid, so Go stays reachable above it.
//   - withVerse=false (the book-name / "Chapter N of M" tap → showChapterPicker): the
//     LEFT pane is a plain scrolling book LIST in Bible order, there's no verse row, and
//     tapping a CHAPTER navigates immediately. (Tapping a BOOK still only selects it.)
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

	var startEntry, endEntry *numberEntry
	commit := func() {
		start, end := "", ""
		if startEntry != nil {
			start = strings.TrimSpace(startEntry.Text)
		}
		if endEntry != nil {
			end = strings.TrimSpace(endEntry.Text)
		}
		verseText := start // "" → chapter top; "16" → v16; "16"+"18" → "16-18"
		if start != "" && end != "" {
			verseText = start + "-" + end
		}
		goToChapterWithVerse(state, selectedBook, selectedChapter, verseText)
		closePicker()
	}

	chapterPane := container.NewStack()
	var renderChapters func()
	var chapterReselect func(int) // re-highlights a chapter IN PLACE (no grid rebuild)
	renderChapters = func() {
		grid, reselect := referenceChapterGrid(state, pal, selectedBook, highlightChapter(), func(ch int) {
			selectedChapter = ch
			if withVerse {
				// Re-highlight in place — rebuilding the grid here reflowed it (the
				// re-created theme override momentarily changed the cell metrics); Go
				// commits the selection.
				if chapterReselect != nil {
					chapterReselect(ch)
				}
			} else {
				navigateToReference(state, selectedBook, ch) // usual picker: go now
				closePicker()
			}
		})
		chapterReselect = reselect
		chapterPane.Objects = []fyne.CanvasObject{grid}
		chapterPane.Refresh()
	}
	renderChapters()

	// Tapping a book SELECTS it and refreshes the chapter grid — it does not navigate
	// or change the book-navigator stage. Shared by both left-pane flavours.
	selectBookInPicker := func(book string) {
		selectedBook = book
		if selectedBook == state.CurrentBook {
			selectedChapter = state.CurrentChapter
		} else if nums := state.Bible.GetChapterNumbersForBook(selectedBook); len(nums) > 0 {
			selectedChapter = nums[0]
		} else {
			selectedChapter = 1
		}
		renderChapters()
	}

	// Left pane: the verse "Go to" picker uses the alphabet-grid navigator (letters →
	// books-for-letter → back). The reading-page chapter picker (withVerse=false) keeps
	// the familiar scrolling book LIST in Bible order.
	var leftPane fyne.CanvasObject
	var bookList *widget.List
	if withVerse {
		sortedBooks := alphabeticalBooks(state.Bible.Books) // groups "1/2/3 John" under J, etc.
		letters := bookLetters(sortedBooks)
		// A two-stage navigator swapped IN PLACE (bookPane.Objects + Refresh, the same
		// idiom as renderChapters) — no popup rebuild, so the non-modal anchor and the
		// keyboard never churn. Stage 0 = alphabet grid; stage 1 = the tapped letter's
		// books with a back row to the alphabet.
		bookPane := container.NewStack()
		bookStage := 0
		activeLetter := firstLetter(state.CurrentBook)
		var renderBooks func()
		renderBooks = func() {
			if bookStage == 1 {
				// "‹ J" back row: a chevron + the active letter, pinned above the books.
				back := widget.NewButtonWithIcon(string(activeLetter), theme.NavigateBackIcon(), func() {
					bookStage = 0
					renderBooks()
				})
				back.Importance = widget.LowImportance
				back.Alignment = widget.ButtonAlignLeading
				rows := container.NewVBox()
				for _, b := range sortedBooks { // alphabetical, so the bucket is ordered
					if firstLetter(b) != activeLetter {
						continue
					}
					book := b
					// renderBooks re-highlights the tapped book (HighImportance); the
					// stage is unchanged, so the list stays put and the chapter grid updates.
					btn := widget.NewButton(book, func() { selectBookInPicker(book); renderBooks() })
					btn.Alignment = widget.ButtonAlignLeading
					if book == selectedBook {
						btn.Importance = widget.HighImportance
					} else {
						btn.Importance = widget.LowImportance
					}
					rows.Add(btn)
				}
				bookPane.Objects = []fyne.CanvasObject{
					container.NewBorder(back, nil, nil, nil, container.NewVScroll(rows)),
				}
			} else {
				head := canvas.NewText("Book", pal.TextMuted)
				head.TextSize = 12
				grid := container.NewGridWrap(fyne.NewSize(40, 34)) // dense → 3 cols, all letters
				for _, r := range letters {
					letter := r
					btn := widget.NewButton(string(letter), func() {
						activeLetter = letter
						bookStage = 1
						renderBooks()
					})
					if letter == firstLetter(selectedBook) {
						btn.Importance = widget.HighImportance // guide the eye to the current book's letter
					} else {
						btn.Importance = widget.LowImportance
					}
					grid.Add(btn)
				}
				bookPane.Objects = []fyne.CanvasObject{
					container.NewBorder(container.NewPadded(head), nil, nil, nil,
						container.NewVScroll(denseGrid(state, grid))),
				}
			}
			bookPane.Refresh()
		}
		renderBooks()
		leftPane = bookPane
	} else {
		books := state.Bible.Books // canonical Bible order
		bookList = widget.NewList(
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
		bookList.OnSelected = func(id widget.ListItemID) {
			if id < 0 || id >= len(books) {
				return
			}
			selectBookInPicker(books[id])
			bookList.Refresh()
		}
		leftPane = bookList
	}

	title := canvas.NewText("Go to", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 18
	header := pickerHeader(title, closePicker)

	divider := canvas.NewRectangle(pal.Border)
	divider.SetMinSize(fyne.NewSize(1, 0))
	left := container.New(fixedWidthLayout{width: 152},
		container.NewBorder(nil, nil, nil, divider, leftPane))

	var bottom fyne.CanvasObject
	if withVerse {
		// A verse RANGE as two number fields — "verse [16] to [18]" — so there's no
		// hyphen to type (the iOS number pad has none) and no decoded glyph to puzzle
		// over. Each field needs its OWN withCaret wrapper: the base theme zeroes the
		// caret size globally, so a shared wrapper would leave one field caretless. The
		// number pad has no return key, so Go is the only commit path.
		startEntry = newNumberEntry()
		startEntry.SetPlaceHolder("verse")
		startEntry.OnSubmitted = func(string) { commit() }
		endEntry = newNumberEntry()
		endEntry.SetPlaceHolder("end")
		endEntry.OnSubmitted = func(string) { commit() }
		toLabel := canvas.NewText("to", pal.TextMuted)
		toLabel.TextSize = 14
		goBtn := widget.NewButton("Go", commit)
		goBtn.Importance = widget.HighImportance
		startFrame := inputFrame(withCaret(state, startEntry), pal.Border)
		endCell := container.New(fixedWidthLayout{width: 64}, inputFrame(withCaret(state, endEntry), pal.Border))
		// startFrame flexes (Border center); "to" + end field + Go stay compact on the
		// right, keeping the row's MinSize bounded so the no-clamp non-modal card fits.
		trailing := container.NewHBox(container.NewCenter(toLabel), endCell, goBtn)
		bottom = container.NewPadded(container.NewBorder(nil, nil, nil, trailing, startFrame))
	}

	body := container.NewBorder(header, bottom, left, nil, container.NewPadded(chapterPane))
	card := surface(container.NewPadded(body), pal.Surface, pal.Border, fyne.Size{})

	if withVerse {
		// Top-anchored, non-modal so the bottom verse box stays ABOVE the iOS
		// keyboard. A modal popup always centers and ignores Move (so the verse box
		// would land under the keyboard, which the canvas doesn't shrink for); a
		// non-modal popup honors Move and dismisses on a tap outside the card.
		popup = widget.NewPopUp(card, cnv)
		// Anchor the top just below the safe-area inset; the card grows downward from
		// there (the renderer clamps X to the canvas anyway).
		topY := float32(12)
		if pos, _ := cnv.InteractiveArea(); pos.Y > 0 {
			topY = pos.Y + 12
		}
		// resizePicker swaps between near-full-screen (keyboard down → the whole grids
		// and lists are visible) and the upper ~55% (keyboard up → the bottom verse box
		// clears the number pad), staying top-anchored + centered. A non-modal popup
		// honors Move/Resize, so this is a cheap re-layout, not a rebuild.
		resizePicker := func(keyboard bool) {
			w, h := pickerVerseSize(cnv, keyboard)
			popup.Resize(fyne.NewSize(w, h))
			x := (cnv.Size().Width - w) / 2
			if x < 0 {
				x = 0
			}
			popup.Move(fyne.NewPos(x, topY))
		}
		// Shrink only while a verse field is focused (the keyboard is up). ALWAYS run the
		// resize off the event handler (AfterFunc + fyne.Do): resizing the popup
		// synchronously inside FocusGained re-enters Fyne's layout during its own tap/focus
		// processing AND moves the field out from under the in-flight tap, which tore the
		// picker down ("crashes away" when tapping the verse box). The brief delay also lets
		// moving start↔end settle so we only grow back when neither field holds focus.
		onVerseFocus := func(bool) {
			time.AfterFunc(60*time.Millisecond, func() {
				fyne.Do(func() {
					f := cnv.Focused()
					resizePicker(f == startEntry || f == endEntry)
				})
			})
		}
		startEntry.onFocus = onVerseFocus
		endEntry.onFocus = onVerseFocus

		w, h := pickerVerseSize(cnv, false) // open near full-screen
		popup.Resize(fyne.NewSize(w, h))
		x := (cnv.Size().Width - w) / 2
		if x < 0 {
			x = 0
		}
		popup.ShowAtPosition(fyne.NewPos(x, topY))
		// A non-modal popup also closes on a tap OUTSIDE the card (Fyne's PopUp.Tapped →
		// Hide), which bypasses closePicker — so without this the reading overlay would
		// stay suppressed (hideReadingOverlay latched it down) and the reading pane would
		// go blank until another modal cycled through closePicker. Poll until the popup is
		// gone by ANY route, then restore the overlay (idempotent with closePicker).
		var watchDismiss func()
		watchDismiss = func() {
			if popup == nil || !popup.Visible() {
				if state.showReadingOverlay != nil {
					state.showReadingOverlay()
				}
				return
			}
			time.AfterFunc(200*time.Millisecond, func() { fyne.Do(watchDismiss) })
		}
		time.AfterFunc(200*time.Millisecond, func() { fyne.Do(watchDismiss) })
	} else {
		popup = widget.NewModalPopUp(card, cnv)
		popup.Show()
		w, h := pickerSplitSize(cnv)
		popup.Resize(fyne.NewSize(w, h))
	}

	// List flavour: reveal + highlight the current book once the popup is laid out.
	if bookList != nil {
		for i, b := range state.Bible.Books {
			if b == selectedBook {
				bookList.Select(i)
				bookList.ScrollTo(i)
				break
			}
		}
	}
}

// showGotoPicker is the header "Go to" button's picker: the alphabet-grid book
// navigator + a verse-range row. showChapterPicker is the book-name / chapter-line tap
// while reading: a plain scrolling book LIST in Bible order, no verse row, chapter-tap
// navigates immediately.
func showGotoPicker(state *AppState)    { gotoPickerModal(state, true) }
func showChapterPicker(state *AppState) { gotoPickerModal(state, false) }

// bookBase returns a book's sort name and leading ordinal, treating a leading number as
// a book ordinal rather than a sort character: "1 John" -> ("john", 1), "Genesis" ->
// ("genesis", 0). Shared by alphabeticalBooks (ordering) and firstLetter (grouping) so
// the two can never diverge.
func bookBase(b string) (string, int) {
	if i := strings.IndexByte(b, ' '); i > 0 {
		if n, err := strconv.Atoi(b[:i]); err == nil {
			return strings.ToLower(b[i+1:]), n // "1 John" -> ("john", 1)
		}
	}
	return strings.ToLower(b), 0
}

// firstLetter is the uppercase letter a book is grouped under in the alphabet grid,
// after stripping a leading "N ": "1 John" -> 'J', "Genesis" -> 'G'.
func firstLetter(b string) rune {
	name, _ := bookBase(b)
	for _, r := range name {
		return unicode.ToUpper(r)
	}
	return ' '
}

// bookLetters returns the distinct first letters of the given (already alphabetical)
// books in first-seen (A→Z) order — only letters that actually have books, so the
// alphabet grid has no dead keys.
func bookLetters(sorted []string) []rune {
	seen := map[rune]bool{}
	var out []rune
	for _, b := range sorted {
		if r := firstLetter(b); !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

// alphabeticalBooks returns the books sorted by name, treating a leading number as a
// book ordinal rather than a sort character: "1 John"/"2 John"/"3 John" group under
// "John" (after the gospel of John), "1 Corinthians" under "Corinthians", etc.
func alphabeticalBooks(books []string) []string {
	out := append([]string(nil), books...)
	sort.Slice(out, func(i, j int) bool {
		ni, oi := bookBase(out[i])
		nj, oj := bookBase(out[j])
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

// withCaret wraps an Entry so it shows a normal blinking iOS caret. The base theme
// zeroes SizeNameInputBorder globally (to hide the read-only reading Entry's caret),
// which also hides the caret in every other field; this scopes a 1px caret back to
// just the wrapped entry. Pair with inputFrame for the visible border.
func withCaret(state *AppState, e fyne.CanvasObject) fyne.CanvasObject {
	var base fyne.Theme = theme.DefaultTheme()
	if state.theme != nil {
		base = state.theme
	}
	return container.NewThemeOverride(e, caretTheme{Theme: base})
}

// denseGridTheme tightens the inter-cell gap (SizeNamePadding) and button inner
// padding for the picker's letter + chapter grids, so more letters/chapters fit
// without scrolling. The base theme's padding is a roomy 7pt, which (with GridWrap's
// per-cell spacing) wastes horizontal room and forces too few columns; 3pt packs the
// grid tightly while the cells themselves stay finger-sized.
type denseGridTheme struct{ fyne.Theme }

func (t denseGridTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 3
	case theme.SizeNameInnerPadding:
		return 4
	}
	return t.Theme.Size(name)
}

// denseGrid wraps a picker grid so its cells pack tightly (see denseGridTheme).
func denseGrid(state *AppState, obj fyne.CanvasObject) fyne.CanvasObject {
	var base fyne.Theme = theme.DefaultTheme()
	if state.theme != nil {
		base = state.theme
	}
	return container.NewThemeOverride(obj, denseGridTheme{Theme: base})
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
