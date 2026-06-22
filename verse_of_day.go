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
// gospel, who Jesus is, and the hope and peace found in him. Edit freely; any
// reference that doesn't resolve in the loaded translation is simply skipped.
var verseOfDayRefs = []dayVerseRef{
	{"John", 3, 16}, {"John", 14, 6}, {"John", 1, 1}, {"John", 1, 14},
	{"John", 8, 12}, {"John", 11, 25}, {"John", 10, 10}, {"John", 15, 5},
	{"Matthew", 11, 28}, {"Matthew", 6, 33}, {"Matthew", 28, 19}, {"Matthew", 5, 16},
	{"Mark", 12, 30}, {"Luke", 19, 10},
	{"Acts", 4, 12},
	{"Romans", 5, 8}, {"Romans", 8, 28}, {"Romans", 8, 38}, {"Romans", 10, 9},
	{"Romans", 6, 23}, {"Romans", 12, 2}, {"Romans", 15, 13},
	{"1 Corinthians", 13, 4}, {"2 Corinthians", 5, 17}, {"2 Corinthians", 5, 21},
	{"Galatians", 2, 20}, {"Galatians", 5, 22},
	{"Ephesians", 2, 8}, {"Philippians", 4, 13}, {"Philippians", 4, 6},
	{"Philippians", 4, 7}, {"Colossians", 3, 23},
	{"2 Timothy", 1, 7}, {"Hebrews", 11, 1}, {"Hebrews", 4, 16}, {"Hebrews", 12, 2},
	{"1 Peter", 5, 7}, {"1 John", 1, 9}, {"1 John", 4, 19}, {"1 John", 4, 9},
	{"Revelation", 3, 20},
	{"Psalms", 23, 1}, {"Psalms", 23, 4}, {"Psalms", 46, 1}, {"Psalms", 119, 105},
	{"Psalms", 27, 1}, {"Psalms", 34, 8}, {"Psalms", 100, 5},
	{"Proverbs", 3, 5}, {"Proverbs", 3, 6},
	{"Isaiah", 53, 5}, {"Isaiah", 53, 6}, {"Isaiah", 9, 6}, {"Isaiah", 40, 31},
	{"Isaiah", 41, 10}, {"Jeremiah", 29, 11}, {"Lamentations", 3, 22},
	{"Joshua", 1, 9}, {"Micah", 6, 8}, {"Zephaniah", 3, 17}, {"Nahum", 1, 7},
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

// verseOfTheDay picks today's verse: a stable choice for a given calendar day
// (day-of-year), rotating through the resolved list.
func verseOfTheDay(state *AppState) (Verse, bool) {
	valid := resolvedVerseOfDay(state)
	if len(valid) == 0 {
		return Verse{}, false
	}
	idx := (time.Now().YearDay() - 1) % len(valid)
	if idx < 0 {
		idx = 0
	}
	return valid[idx], true
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
