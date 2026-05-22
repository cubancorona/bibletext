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

	chips := container.NewHBox()
	for _, t := range targets {
		v := t
		chip := widget.NewButton(fmt.Sprintf("%s %d", v.Book, v.Chapter), func() {
			navigateToVisit(state, v)
		})
		chip.Importance = widget.LowImportance
		chips.Add(chip)
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
