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
// rather than one hyphenated field.
type numberEntry struct {
	widget.Entry
}

func (e *numberEntry) Keyboard() mobile.KeyboardType { return mobile.NumberKeyboard }

func newNumberEntry() *numberEntry {
	e := &numberEntry{}
	e.ExtendBaseWidget(e)
	return e
}

// searchKeyEntry is a single-line Entry whose virtual keyboard shows a "return" key
// that SUBMITS (so OnSubmitted fires) instead of iOS's default single-line "Done" key,
// which merely dismisses the keyboard. Fyne's plain Entry asks for mobile.SingleLineKeyboard
// on non-multiline fields → UIReturnKeyDone → the native textFieldShouldReturn resigns the
// responder and never delivers '\n', so OnSubmitted can never fire from the keyboard.
// Requesting DefaultKeyboard restores the '\n' → KeyReturn → typedKeyReturn → OnSubmitted
// path, letting the reader run a search straight from the keyboard. (No effect on desktop,
// where the hardware Return already produces KeyReturn.)
type searchKeyEntry struct {
	widget.Entry
}

func (e *searchKeyEntry) Keyboard() mobile.KeyboardType { return mobile.DefaultKeyboard }

func newSearchEntry() *searchKeyEntry {
	e := &searchKeyEntry{}
	e.ExtendBaseWidget(e)
	return e
}

// gKeyboardInsetSetter is set by the open mobile verse picker to receive the iOS
// keyboard's exact on-screen overlap (in points) from the native keyboard observer
// (bibleTextKeyboardChanged in ai_menu_darwin.go), so it can lift the bottom verse row
// to sit exactly above the keyboard. nil when no picker is up (other keyboards ignored).
var gKeyboardInsetSetter func(float32)

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
		gKeyboardInsetSetter = nil // stop receiving keyboard-height updates
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
	var scrollChapterIntoView func()
	renderChapters = func() {
		grid, reselect, scrollSel := referenceChapterGrid(state, pal, selectedBook, highlightChapter(), func(ch int) {
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
		scrollChapterIntoView = scrollSel
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
	var scrollBookIntoView func() // scrolls the nav to the selected book/letter (keyboard)
	if withVerse {
		sortedBooks := alphabeticalBooks(state.Bible.Books) // groups "1/2/3 John" under J, etc.
		letters := bookLetters(sortedBooks)
		var bookScroll *container.Scroll
		var selectedBookBtn fyne.CanvasObject
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
				selectedBookBtn = nil
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
						selectedBookBtn = btn
					} else {
						btn.Importance = widget.LowImportance
					}
					rows.Add(btn)
				}
				bookScroll = container.NewVScroll(rows)
				bookPane.Objects = []fyne.CanvasObject{
					container.NewBorder(back, nil, nil, nil, bookScroll),
				}
			} else {
				head := canvas.NewText("Book", pal.TextMuted)
				head.TextSize = 12
				grid := container.NewGridWrap(fyne.NewSize(40, 34)) // dense → 3 cols, all letters
				selectedBookBtn = nil
				for _, r := range letters {
					letter := r
					btn := widget.NewButton(string(letter), func() {
						activeLetter = letter
						bookStage = 1
						renderBooks()
					})
					if letter == firstLetter(selectedBook) {
						btn.Importance = widget.HighImportance // guide the eye to the current book's letter
						selectedBookBtn = btn
					} else {
						btn.Importance = widget.LowImportance
					}
					grid.Add(btn)
				}
				bookScroll = container.NewVScroll(denseGrid(state, grid))
				bookPane.Objects = []fyne.CanvasObject{
					container.NewBorder(container.NewPadded(head), nil, nil, nil, bookScroll),
				}
			}
			bookPane.Refresh()
		}
		renderBooks()
		scrollBookIntoView = func() { scrollChildIntoView(bookScroll, selectedBookBtn) }
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

	var verseRow fyne.CanvasObject
	var kbInset *canvas.Rectangle // grows to lift the verse row above the soft keyboard
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
		row := container.NewPadded(container.NewBorder(nil, nil, nil, trailing, startFrame))
		// kbInset is a transparent spacer BELOW the verse row. While a verse field is
		// focused we grow it to the keyboard's height, lifting the verse row above the
		// soft keyboard — by shifting content INSIDE the card, never by resizing the
		// popup. The card stays full-screen, so a tap can never land "outside" it and the
		// picker can't self-dismiss (the bug that came from resizing on focus).
		kbInset = canvas.NewRectangle(color.Transparent)
		verseRow = container.NewVBox(row, kbInset)
	}

	// Verse row at the BOTTOM (above the inset spacer). Full-screen card; the popup is
	// never resized — see the popup section.
	body := container.NewBorder(header, verseRow, left, nil, container.NewPadded(chapterPane))
	card := surface(container.NewPadded(body), pal.SurfaceAlt, pal.Border, fyne.Size{})

	if withVerse && fyne.CurrentDevice().IsMobile() {
		// MOBILE verse picker: a non-modal popup anchored at the top, opened near
		// FULL-SCREEN so the whole alphabet grid, book list and chapter grid are visible.
		// The popup is NEVER resized — resizing moved the bottom verse field out from under
		// the in-flight tap and Fyne re-targeted the tap to the popup background (Hide),
		// self-dismissing the picker. Instead, focusing a verse field grows kbInset to lift
		// the verse row above the keyboard; the full-screen card means no tap is ever
		// "outside" it, so it can't self-dismiss.
		popup = widget.NewPopUp(card, cnv)
		w, h := pickerVerseSize(cnv)
		popup.Resize(fyne.NewSize(w, h))
		topY := float32(12)
		if pos, _ := cnv.InteractiveArea(); pos.Y > 0 {
			topY = pos.Y + 12
		}
		x := (cnv.Size().Width - w) / 2
		if x < 0 {
			x = 0
		}
		popup.ShowAtPosition(fyne.NewPos(x, topY))

		// Lift the verse row to sit EXACTLY above the soft keyboard, from the keyboard's
		// real on-screen overlap reported by the native observer (bibleTextKeyboardChanged
		// → gKeyboardInsetSetter; 0 when hidden). The card doesn't reach the screen bottom
		// (anchored below the safe area + double-padded by surface), so the verse row
		// already sits `belowCard + cardPad` above the screen bottom with no keyboard — lift
		// it by exactly the rest so its bottom lands on the keyboard top. Geometry only
		// shifts INSIDE the full-screen card (the popup is never resized), so the picker
		// can't self-dismiss. Guarded against a closed popup.
		if kbInset != nil {
			belowCard := cnv.Size().Height - topY - h // card bottom → screen bottom
			cardPad := 2 * theme.Padding()            // surface wraps the body in NewPadded twice
			gKeyboardInsetSetter = func(overlap float32) {
				if popup == nil || !popup.Visible() {
					return
				}
				inset := overlap - belowCard - cardPad
				if inset < 0 {
					inset = 0
				}
				kbInset.SetMinSize(fyne.NewSize(0, inset))
				body.Refresh()
				if inset > 0 {
					// The panes just shrank — keep the selected book + chapter on screen.
					// Deferred so the new (smaller) viewport + cell positions have settled.
					time.AfterFunc(60*time.Millisecond, func() {
						fyne.Do(func() {
							if popup == nil || !popup.Visible() {
								return
							}
							if scrollChapterIntoView != nil {
								scrollChapterIntoView()
							}
							if scrollBookIntoView != nil {
								scrollBookIntoView()
							}
						})
					})
				}
			}
		}
		// A non-modal popup also closes on a tap OUTSIDE the card (PopUp.Tapped → Hide),
		// which bypasses closePicker — so the reading overlay would stay suppressed and the
		// pane blank. Poll until the popup is gone by ANY route, then restore the overlay
		// + stop receiving keyboard updates (idempotent with closePicker).
		var watchDismiss func()
		watchDismiss = func() {
			if popup == nil || !popup.Visible() {
				gKeyboardInsetSetter = nil
				if state.showReadingOverlay != nil {
					state.showReadingOverlay()
				}
				return
			}
			time.AfterFunc(200*time.Millisecond, func() { fyne.Do(watchDismiss) })
		}
		time.AfterFunc(200*time.Millisecond, func() { fyne.Do(watchDismiss) })
	} else {
		// DESKTOP (and the non-verse chapter picker on any platform): a normal centered
		// modal. No soft keyboard, so none of the top-anchor / full-screen / tap-dismiss
		// machinery applies.
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

// caretTheme styles a single input field: it re-enables the blinking Entry caret (the
// base theme zeroes SizeNameInputBorder to hide the read-only reading Entry's caret —
// in Fyne 2.7.4 that size IS the caret width, entry.go moveCursor — so we restore 1px
// and make Fyne's own Entry border transparent so it isn't doubled with inputFrame's),
// and shrinks the field text. Fyne entries draw the placeholder + typed text at
// SizeNameText, which the app sets to 18pt for body text — too large for a hint — so
// fields use a smaller fieldTextSize.
type caretTheme struct{ fyne.Theme }

const fieldTextSize = 14 // input/placeholder text in search + text boxes (vs 18pt body)

func (c caretTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameText:
		return fieldTextSize
	}
	return c.Theme.Size(name)
}

func (c caretTheme) Color(name fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameInputBorder {
		return color.Transparent
	}
	return c.Theme.Color(name, v)
}

// vCenterLayout holds its single child at the child's natural MinSize height and
// vertically centers it in whatever height the row hands it. Fyne's entryRenderer
// top-aligns the text inside the field: at the field's natural height the inner padding
// balances and the text looks centered, but when a row stretches the field taller (e.g. a
// 14pt search box sitting beside 18pt buttons) all the extra height falls BELOW the
// top-aligned text, so it reads as top-biased. Pinning the field to its natural height and
// centering the whole field restores visual centering without patching Fyne internals.
type vCenterLayout struct{}

func (vCenterLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	if len(objs) == 0 {
		return fyne.Size{}
	}
	return objs[0].MinSize()
}

func (vCenterLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) == 0 {
		return
	}
	o := objs[0]
	h := o.MinSize().Height
	if h > size.Height {
		h = size.Height
	}
	o.Resize(fyne.NewSize(size.Width, h))
	o.Move(fyne.NewPos(0, (size.Height-h)/2))
}

// withCaret wraps an Entry so it shows a normal blinking iOS caret AND vertically centers
// its (smaller) text within the row. The base theme zeroes SizeNameInputBorder globally (to
// hide the read-only reading Entry's caret), which also hides the caret in every other
// field; this scopes a 1px caret back to just the wrapped entry. The vCenterLayout keeps the
// field at its natural height so its 14pt text doesn't top-bias when the row stretches it.
// Pair with inputFrame for the visible border.
func withCaret(state *AppState, e fyne.CanvasObject) fyne.CanvasObject {
	var base fyne.Theme = theme.DefaultTheme()
	if state.theme != nil {
		base = state.theme
	}
	centered := container.New(vCenterLayout{}, e)
	return container.NewThemeOverride(centered, caretTheme{Theme: base})
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
