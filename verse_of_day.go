package bibletext

// Verse of the day: a subtle header icon (a small sparkle, far right) opens a
// calm little card with one grace-filled, Christ-centred verse that rotates by
// the calendar day. It is intentionally NOT a feed or a page — just a quiet
// daily pointer back into the Word, with a "Read in context" jump.

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// dayVerseRef is a reference into the curated rotation. Book names are resolved
// through resolveBookName, so common forms ("Psalm"/"Psalms") and any future
// translation's naming still land on the right book.
type dayVerseRef struct {
	Book    string
	Chapter int
	Verse   int
}

// verseOfDayRefs is a hand-picked rotation that keeps the eyes on Christ — the
// gospel, who Jesus is, and the hope, peace, and promises found in him — drawn from
// across the whole canon (well over a year's worth, one per day). Grouped by book
// for easy editing; add, remove, or reorder freely. Any reference that doesn't
// resolve in the loaded translation is simply skipped, so the rotation stays valid
// across translations.
var verseOfDayRefs = []dayVerseRef{
	// Genesis
	{"Genesis", 1, 1}, {"Genesis", 1, 27}, {"Genesis", 15, 6}, {"Genesis", 28, 15},
	{"Genesis", 50, 20},
	// Exodus
	{"Exodus", 14, 14}, {"Exodus", 15, 2}, {"Exodus", 33, 14}, {"Exodus", 34, 6},
	// Numbers
	{"Numbers", 6, 24}, {"Numbers", 6, 25}, {"Numbers", 6, 26}, {"Numbers", 23, 19},
	// Deuteronomy
	{"Deuteronomy", 6, 5}, {"Deuteronomy", 7, 9}, {"Deuteronomy", 31, 6}, {"Deuteronomy", 31, 8},
	// Joshua
	{"Joshua", 1, 8}, {"Joshua", 1, 9}, {"Joshua", 24, 15},
	// Ruth
	{"Ruth", 1, 16},
	// 1 Samuel
	{"1 Samuel", 2, 2}, {"1 Samuel", 16, 7},
	// 2 Samuel
	{"2 Samuel", 22, 2}, {"2 Samuel", 22, 31},
	// 1 Chronicles
	{"1 Chronicles", 16, 11}, {"1 Chronicles", 16, 34}, {"1 Chronicles", 29, 11},
	// 2 Chronicles
	{"2 Chronicles", 7, 14}, {"2 Chronicles", 20, 15},
	// Nehemiah
	{"Nehemiah", 8, 10},
	// Esther
	{"Esther", 4, 14},
	// Job
	{"Job", 19, 25}, {"Job", 23, 10}, {"Job", 42, 2},
	// Psalms
	{"Psalms", 1, 1}, {"Psalms", 1, 2}, {"Psalms", 16, 8}, {"Psalms", 16, 11},
	{"Psalms", 18, 2}, {"Psalms", 19, 1}, {"Psalms", 19, 14}, {"Psalms", 23, 1},
	{"Psalms", 23, 4}, {"Psalms", 23, 6}, {"Psalms", 27, 1}, {"Psalms", 27, 4},
	{"Psalms", 27, 14}, {"Psalms", 28, 7}, {"Psalms", 30, 5}, {"Psalms", 31, 24},
	{"Psalms", 32, 8}, {"Psalms", 34, 8}, {"Psalms", 34, 18}, {"Psalms", 37, 4},
	{"Psalms", 37, 5}, {"Psalms", 42, 1}, {"Psalms", 42, 11}, {"Psalms", 46, 1},
	{"Psalms", 46, 10}, {"Psalms", 51, 10}, {"Psalms", 55, 22}, {"Psalms", 56, 3},
	{"Psalms", 62, 1}, {"Psalms", 63, 1}, {"Psalms", 73, 26}, {"Psalms", 84, 11},
	{"Psalms", 90, 12}, {"Psalms", 91, 1}, {"Psalms", 91, 2}, {"Psalms", 91, 11},
	{"Psalms", 94, 19}, {"Psalms", 100, 5}, {"Psalms", 103, 1}, {"Psalms", 103, 2},
	{"Psalms", 103, 8}, {"Psalms", 103, 12}, {"Psalms", 107, 1}, {"Psalms", 118, 24},
	{"Psalms", 119, 11}, {"Psalms", 119, 105}, {"Psalms", 121, 1}, {"Psalms", 121, 2},
	{"Psalms", 121, 7}, {"Psalms", 130, 5}, {"Psalms", 138, 8}, {"Psalms", 139, 14},
	{"Psalms", 139, 23}, {"Psalms", 143, 8}, {"Psalms", 145, 8}, {"Psalms", 145, 9},
	{"Psalms", 147, 3}, {"Psalms", 150, 6},
	// Proverbs
	{"Proverbs", 3, 5}, {"Proverbs", 3, 6}, {"Proverbs", 4, 23}, {"Proverbs", 11, 25},
	{"Proverbs", 16, 3}, {"Proverbs", 16, 9}, {"Proverbs", 18, 10}, {"Proverbs", 19, 21},
	{"Proverbs", 22, 6}, {"Proverbs", 29, 25}, {"Proverbs", 30, 5},
	// Ecclesiastes
	{"Ecclesiastes", 3, 1}, {"Ecclesiastes", 3, 11}, {"Ecclesiastes", 12, 13},
	// Song of Solomon
	{"Song of Solomon", 2, 4},
	// Isaiah
	{"Isaiah", 6, 8}, {"Isaiah", 9, 6}, {"Isaiah", 12, 2}, {"Isaiah", 25, 1},
	{"Isaiah", 26, 3}, {"Isaiah", 30, 21}, {"Isaiah", 40, 8}, {"Isaiah", 40, 28},
	{"Isaiah", 40, 29}, {"Isaiah", 40, 31}, {"Isaiah", 41, 10}, {"Isaiah", 41, 13},
	{"Isaiah", 43, 1}, {"Isaiah", 43, 2}, {"Isaiah", 43, 19}, {"Isaiah", 46, 4},
	{"Isaiah", 53, 4}, {"Isaiah", 53, 5}, {"Isaiah", 53, 6}, {"Isaiah", 54, 10},
	{"Isaiah", 55, 6}, {"Isaiah", 55, 8}, {"Isaiah", 55, 11}, {"Isaiah", 58, 11},
	{"Isaiah", 61, 1}, {"Isaiah", 64, 8},
	// Jeremiah
	{"Jeremiah", 1, 5}, {"Jeremiah", 17, 7}, {"Jeremiah", 29, 11}, {"Jeremiah", 29, 12},
	{"Jeremiah", 29, 13}, {"Jeremiah", 31, 3}, {"Jeremiah", 32, 17}, {"Jeremiah", 33, 3},
	// Lamentations
	{"Lamentations", 3, 22}, {"Lamentations", 3, 23}, {"Lamentations", 3, 25},
	// Ezekiel
	{"Ezekiel", 36, 26},
	// Daniel
	{"Daniel", 3, 17},
	// Micah
	{"Micah", 6, 8}, {"Micah", 7, 7},
	// Nahum
	{"Nahum", 1, 7},
	// Habakkuk
	{"Habakkuk", 3, 19},
	// Zephaniah
	{"Zephaniah", 3, 17},
	// Zechariah
	{"Zechariah", 4, 6},
	// Malachi
	{"Malachi", 3, 6},
	// Matthew
	{"Matthew", 4, 4}, {"Matthew", 5, 3}, {"Matthew", 5, 6}, {"Matthew", 5, 8},
	{"Matthew", 5, 14}, {"Matthew", 5, 16}, {"Matthew", 5, 44}, {"Matthew", 6, 21},
	{"Matthew", 6, 33}, {"Matthew", 6, 34}, {"Matthew", 7, 7}, {"Matthew", 7, 12},
	{"Matthew", 11, 28}, {"Matthew", 11, 29}, {"Matthew", 16, 24}, {"Matthew", 18, 20},
	{"Matthew", 19, 26}, {"Matthew", 22, 37}, {"Matthew", 22, 39}, {"Matthew", 28, 6},
	{"Matthew", 28, 19}, {"Matthew", 28, 20},
	// Mark
	{"Mark", 9, 23}, {"Mark", 10, 27}, {"Mark", 10, 45}, {"Mark", 11, 24},
	{"Mark", 12, 30}, {"Mark", 12, 31}, {"Mark", 16, 15},
	// Luke
	{"Luke", 1, 37}, {"Luke", 6, 31}, {"Luke", 6, 37}, {"Luke", 9, 23},
	{"Luke", 11, 9}, {"Luke", 12, 34}, {"Luke", 19, 10},
	// John
	{"John", 1, 1}, {"John", 1, 12}, {"John", 1, 14}, {"John", 1, 29},
	{"John", 3, 16}, {"John", 3, 17}, {"John", 4, 14}, {"John", 6, 35},
	{"John", 8, 12}, {"John", 8, 32}, {"John", 8, 36}, {"John", 10, 10},
	{"John", 10, 11}, {"John", 10, 28}, {"John", 11, 25}, {"John", 13, 34},
	{"John", 14, 1}, {"John", 14, 2}, {"John", 14, 3}, {"John", 14, 6},
	{"John", 14, 27}, {"John", 15, 5}, {"John", 15, 13}, {"John", 16, 33},
	{"John", 17, 3}, {"John", 20, 29},
	// Acts
	{"Acts", 1, 8}, {"Acts", 2, 38}, {"Acts", 4, 12}, {"Acts", 16, 31},
	{"Acts", 17, 28}, {"Acts", 20, 24},
	// Romans
	{"Romans", 1, 16}, {"Romans", 3, 23}, {"Romans", 5, 1}, {"Romans", 5, 8},
	{"Romans", 6, 23}, {"Romans", 8, 1}, {"Romans", 8, 11}, {"Romans", 8, 18},
	{"Romans", 8, 28}, {"Romans", 8, 31}, {"Romans", 8, 37}, {"Romans", 8, 38},
	{"Romans", 8, 39}, {"Romans", 10, 9}, {"Romans", 10, 13}, {"Romans", 12, 1},
	{"Romans", 12, 2}, {"Romans", 12, 12}, {"Romans", 12, 21}, {"Romans", 15, 13},
	// 1 Corinthians
	{"1 Corinthians", 1, 18}, {"1 Corinthians", 2, 9}, {"1 Corinthians", 6, 19}, {"1 Corinthians", 10, 13},
	{"1 Corinthians", 13, 4}, {"1 Corinthians", 13, 13}, {"1 Corinthians", 15, 57}, {"1 Corinthians", 15, 58},
	{"1 Corinthians", 16, 13}, {"1 Corinthians", 16, 14},
	// 2 Corinthians
	{"2 Corinthians", 1, 3}, {"2 Corinthians", 1, 4}, {"2 Corinthians", 4, 16}, {"2 Corinthians", 4, 17},
	{"2 Corinthians", 4, 18}, {"2 Corinthians", 5, 7}, {"2 Corinthians", 5, 17}, {"2 Corinthians", 5, 21},
	{"2 Corinthians", 9, 8}, {"2 Corinthians", 12, 9}, {"2 Corinthians", 3, 18},
	// Galatians
	{"Galatians", 2, 20}, {"Galatians", 5, 1}, {"Galatians", 5, 22}, {"Galatians", 5, 23},
	{"Galatians", 6, 9},
	// Ephesians
	{"Ephesians", 1, 7}, {"Ephesians", 2, 4}, {"Ephesians", 2, 8}, {"Ephesians", 2, 9},
	{"Ephesians", 2, 10}, {"Ephesians", 3, 20}, {"Ephesians", 4, 32}, {"Ephesians", 6, 10},
	{"Ephesians", 6, 11},
	// Philippians
	{"Philippians", 1, 6}, {"Philippians", 2, 3}, {"Philippians", 2, 5}, {"Philippians", 3, 14},
	{"Philippians", 4, 4}, {"Philippians", 4, 6}, {"Philippians", 4, 7}, {"Philippians", 4, 8},
	{"Philippians", 4, 13}, {"Philippians", 4, 19},
	// Colossians
	{"Colossians", 1, 16}, {"Colossians", 1, 17}, {"Colossians", 2, 6}, {"Colossians", 3, 1},
	{"Colossians", 3, 2}, {"Colossians", 3, 12}, {"Colossians", 3, 15}, {"Colossians", 3, 17},
	{"Colossians", 3, 23},
	// 1 Thessalonians
	{"1 Thessalonians", 5, 11}, {"1 Thessalonians", 5, 16}, {"1 Thessalonians", 5, 17}, {"1 Thessalonians", 5, 18},
	{"1 Thessalonians", 5, 24},
	// 2 Thessalonians
	{"2 Thessalonians", 3, 3},
	// 1 Timothy
	{"1 Timothy", 2, 5}, {"1 Timothy", 4, 12}, {"1 Timothy", 6, 6}, {"1 Timothy", 6, 12},
	// 2 Timothy
	{"2 Timothy", 1, 7}, {"2 Timothy", 2, 15}, {"2 Timothy", 3, 16}, {"2 Timothy", 4, 7},
	// Titus
	{"Titus", 2, 11}, {"Titus", 3, 5},
	// Hebrews
	{"Hebrews", 4, 12}, {"Hebrews", 4, 16}, {"Hebrews", 6, 19}, {"Hebrews", 10, 23},
	{"Hebrews", 10, 24}, {"Hebrews", 10, 25}, {"Hebrews", 11, 1}, {"Hebrews", 11, 6},
	{"Hebrews", 12, 1}, {"Hebrews", 12, 2}, {"Hebrews", 13, 5}, {"Hebrews", 13, 6},
	{"Hebrews", 13, 8},
	// James
	{"James", 1, 2}, {"James", 1, 3}, {"James", 1, 5}, {"James", 1, 12},
	{"James", 1, 17}, {"James", 1, 22}, {"James", 4, 7}, {"James", 4, 8},
	{"James", 5, 16},
	// 1 Peter
	{"1 Peter", 1, 3}, {"1 Peter", 2, 9}, {"1 Peter", 2, 24}, {"1 Peter", 3, 15},
	{"1 Peter", 4, 8}, {"1 Peter", 5, 6}, {"1 Peter", 5, 7}, {"1 Peter", 5, 8},
	{"1 Peter", 5, 10},
	// 2 Peter
	{"2 Peter", 1, 3}, {"2 Peter", 3, 9}, {"2 Peter", 3, 18},
	// 1 John
	{"1 John", 1, 7}, {"1 John", 1, 9}, {"1 John", 3, 1}, {"1 John", 3, 16},
	{"1 John", 4, 4}, {"1 John", 4, 7}, {"1 John", 4, 8}, {"1 John", 4, 9},
	{"1 John", 4, 10}, {"1 John", 4, 16}, {"1 John", 4, 18}, {"1 John", 4, 19},
	{"1 John", 5, 4}, {"1 John", 5, 5}, {"1 John", 5, 11}, {"1 John", 5, 14},
	// Jude
	{"Jude", 1, 24},
	// Revelation
	{"Revelation", 1, 8}, {"Revelation", 3, 20}, {"Revelation", 4, 11}, {"Revelation", 5, 12},
	{"Revelation", 7, 17}, {"Revelation", 21, 4}, {"Revelation", 21, 5}, {"Revelation", 22, 13},
}

// resolvedVerseOfDay returns the curated references that actually exist in the
// loaded translation, as real Verses (so we have their canonical book name).
func resolvedVerseOfDay(state *AppState) []Verse {
	if state == nil || state.Bible == nil {
		return nil
	}
	out := make([]Verse, 0, len(verseOfDayRefs))
	for _, r := range verseOfDayRefs {
		book, ok := resolveBookName(state.Bible.Books, r.Book)
		if !ok {
			continue
		}
		if v := state.Bible.GetVerse(book, r.Chapter, r.Verse); v != nil {
			out = append(out, *v)
		}
	}
	return out
}

// verseOfTheDay picks today's verse: a stable choice for a given calendar day that
// rotates through the resolved list. The index is a CONTINUOUS count of local days
// (localDayNumber), not day-of-year, so the rotation walks the entire list however
// long it is — a list longer than 366 still cycles fully, one verse per day — and
// the sequence never resets or skips at the New Year.
func verseOfTheDay(state *AppState) (Verse, bool) {
	valid := resolvedVerseOfDay(state)
	if len(valid) == 0 {
		return Verse{}, false
	}
	idx := int(localDayNumber() % int64(len(valid)))
	if idx < 0 {
		idx += len(valid)
	}
	return valid[idx], true
}

// localDayNumber is the count of whole days since the Unix epoch in LOCAL time, so
// today's verse rolls over at local midnight (as day-of-year did) but keeps counting
// unbroken across year boundaries instead of resetting to 1.
func localDayNumber() int64 {
	y, m, d := time.Now().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local).Unix() / 86400
}

// iconVerseOfDay is a small filled four-point "sparkle" — a quiet light, not a
// loud badge. Themed so it tracks the foreground colour in light/dark mode.
var iconVerseOfDay = theme.NewThemedResource(fyne.NewStaticResource("votd.svg", []byte(
	`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#000000" d="M12 2c.4 4.6 2.4 6.6 7 7-4.6.4-6.6 2.4-7 7-.4-4.6-2.4-6.6-7-7 4.6-.4 6.6-2.4 7-7z"/></svg>`)))

// verseOfDayButton builds the subtle header affordance.
func verseOfDayButton(state *AppState) *widget.Button {
	b := widget.NewButtonWithIcon("", iconVerseOfDay, func() { showVerseOfDay(state) })
	b.Importance = widget.LowImportance
	return b
}

// goToVerse navigates to a verse and highlights it in context. Unlike opening a
// search result, it does not leave a "back to results" trail — this is a direct
// jump (verse of the day, a cross-reference) into the reading view.
func goToVerse(state *AppState, v Verse) {
	goToVerseRange(state, v.BookName, v.Chapter, v.Verse, v.Verse)
}

// goToVerseRange navigates to book+chapter and highlights verses [start, end]
// (end == start for a single verse), scrolling to the first highlighted verse. The
// native overlays wash every .hl verse and scroll to the first; the Fyne reading
// widget scrolls to the start verse.
func goToVerseRange(state *AppState, book string, chapter, start, end int) {
	if end < start {
		end = start
	}
	selectBook(state, book, false)
	state.CurrentChapter = chapter
	addRecentChapter(state, book, chapter)
	state.HighlightedBook = book
	state.HighlightedChapter = chapter
	state.HighlightedVerse = start
	state.HighlightedVerseEnd = end
	state.HasHighlightedVerse = true
	state.IsSearching = false
	state.CanReturnToSearchResults = false
	state.refresh()
	if state.surfaceReading != nil {
		state.surfaceReading()
	}
}

// showVerseOfDay presents the calm one-verse card.
func showVerseOfDay(state *AppState) {
	if state == nil || state.window == nil {
		return
	}
	cnv := state.window.Canvas()
	if cnv == nil {
		return
	}
	v, ok := verseOfTheDay(state)
	if !ok {
		return
	}
	pal := state.pal()

	// The native reading overlay (macOS/iOS) floats above the canvas; drop it
	// while the card is up, restore on close — same dance as the AI panel.
	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	restore := func() {
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	}

	kicker := canvas.NewText("Verse of the day", pal.Accent)
	kicker.TextStyle = fyne.TextStyle{Bold: true}
	kicker.TextSize = 12

	body := widget.NewRichTextWithText(strings.TrimSpace(strings.ReplaceAll(v.Text, "\n", " ")))
	body.Wrapping = fyne.TextWrapWord

	ref := canvas.NewText(
		fmt.Sprintf("%s %d:%d · %s", v.BookName, v.Chapter, v.Verse, state.currentVersion().Abbrev),
		pal.TextMuted)
	ref.TextStyle = fyne.TextStyle{Italic: true}
	ref.TextSize = subheadingTextSize

	// Width: comfortable for one verse, capped, with margins on a phone.
	w := cnv.Size().Width - 72
	if w > 420 {
		w = 420
	}
	if w < 260 {
		w = 260
	}
	// Pre-wrap the verse at the inner width so its height is known.
	body.Resize(fyne.NewSize(w-48, body.MinSize().Height))
	bodyH := body.MinSize().Height

	var popup *widget.PopUp
	closeAnd := func(after func()) func() {
		return func() {
			if popup != nil {
				popup.Hide()
			}
			restore()
			if after != nil {
				after()
			}
		}
	}
	readBtn := widget.NewButton("Read in context", closeAnd(func() { goToVerse(state, v) }))
	readBtn.Importance = widget.HighImportance
	closeBtn := widget.NewButton("Close", closeAnd(nil))

	content := container.NewVBox(
		kicker,
		body,
		ref,
		widget.NewSeparator(),
		container.NewHBox(layout.NewSpacer(), closeBtn, readBtn),
	)
	card := surface(container.NewPadded(content), pal.SurfaceAlt, pal.Border, fyne.Size{})
	popup = widget.NewModalPopUp(card, cnv)
	popup.Show()
	popup.Resize(fyne.NewSize(w, bodyH+150))

	// Re-measure once the real layout has landed so the card fits the verse snugly.
	time.AfterFunc(40*time.Millisecond, func() {
		fyne.Do(func() {
			if popup != nil {
				popup.Resize(fyne.NewSize(w, body.MinSize().Height+150))
			}
		})
	})
}
