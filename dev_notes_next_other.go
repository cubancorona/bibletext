//go:build bibletextdev && !darwin

package bibletext

import "fyne.io/fyne/v2"

// devNoteNextTap drives the styled/native count control's shared Go verb.
func devNoteNextTap(state *AppState) {
	fyne.Do(func() {
		advanceNoteFocus(state)
		devTraceNotePlacement(state, "cycle")
		state.refreshReadingOnly()
	})
}
