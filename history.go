package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// buildHistoryBar renders a single slim strip of recently read chapters. It
// returns nil when there is nothing to show, so the reading area stays clean.
func buildHistoryBar(state *AppState) fyne.CanvasObject {
	targets := recentJumpTargets(state, 6)
	if len(targets) == 0 {
		return nil
	}
	pal := state.pal()

	label := canvas.NewText("Recent", pal.TextMuted)
	label.TextSize = 11
	label.TextStyle = fyne.TextStyle{Bold: true}

	// Consolidate by book: one book name followed by its chapter numbers, each
	// number individually clickable — e.g. "John 1 3 5   Genesis 1".
	chips := container.NewHBox()
	for _, g := range groupVisitsByBook(targets) {
		book := g.Book

		name := canvas.NewText(book, pal.Text)
		name.TextSize = 13
		name.TextStyle = fyne.TextStyle{Bold: true}

		group := container.NewHBox(container.NewCenter(name))
		for _, ch := range g.Chapters {
			chapter := ch
			num := widget.NewButton(fmt.Sprintf("%d", chapter), func() {
				navigateToVisit(state, ChapterVisit{Book: book, Chapter: chapter})
			})
			num.Importance = widget.LowImportance
			group.Add(num)
		}
		chips.Add(group)
		chips.Add(hgap(10))
	}

	clear := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		clearHistory(state)
		state.refreshReadingOnly()
	})
	clear.Importance = widget.LowImportance

	row := container.NewBorder(
		nil, nil,
		container.NewCenter(label),
		container.NewCenter(clear),
		container.NewHScroll(chips),
	)

	return surface(row, pal.SurfaceAlt, pal.Border, fyne.Size{})
}
