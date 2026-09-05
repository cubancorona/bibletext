package bibletext

// The one row the presented reader keeps. Both native panes drew it inline and
// identically (reading_ios.go, reading_android.go); it lives here so the two
// cannot drift, and so the host can test what the row offers in each of the
// two ways the presented mode is entered.

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// fullScreenExitRow is the thin row above the reading pane in the presented
// mode: a quiet "Book Chapter" marker on the left so the reader keeps their
// place, and on the right whatever that entry way owes them
// (fullScreenExitControls). Muted so it never competes with the verse text.
func fullScreenExitRow(state *AppState) fyne.CanvasObject {
	ref := canvas.NewText(fmt.Sprintf("%s %d", state.CurrentBook, state.CurrentChapter), state.pal().TextMuted)
	ref.TextSize = 16
	refBox := container.NewVBox(layout.NewSpacer(), ref, layout.NewSpacer())
	return container.NewBorder(nil, nil, refBox, fullScreenExitControls(state), nil)
}

// fullScreenExitControls is the right-hand end of that row, and the two ways
// into the presented mode want opposite things there.
//
// The reader's OWN full-screen is a choice, and the restore button is how the
// choice is unmade. The phone-landscape presentation is not a choice but an
// orientation: the button would write IsFullScreen=false and rebuild straight
// back into the same tree, so rotation is the way out and the button is not
// offered rather than offered and inert.
//
// What landscape does owe the reader is the one control that reading itself
// needs. Every other control — the picker, Go-to, search, narration, the
// results trail — is a rotation away, which is the mode's bargain; but running
// out of chapter is the one thing that happens while READING, and rotating to
// portrait, tapping an arrow and rotating back is a poor way to turn a page.
func fullScreenExitControls(state *AppState) fyne.CanvasObject {
	if phoneLandscapeReading() {
		return chapterArrowPair(state)
	}
	btn := widget.NewButtonWithIcon("", theme.ViewRestoreIcon(), func() {
		state.IsFullScreen = false
		rebuildWindow(state)
	})
	btn.Importance = widget.LowImportance
	return btn
}

// chapterArrowPair is the previous/next pair, disabled at the ends of the book
// exactly as the portrait chapter header disables them (chapter_header_mobile.go)
// — the same widgets, the same moveChapter, so a page turned in landscape is
// the same event as one turned in portrait. Returns nil before a Bible is
// loaded, which Border reads as "no right-hand object".
func chapterArrowPair(state *AppState) fyne.CanvasObject {
	if state == nil || state.Bible == nil {
		return nil
	}
	numbers := state.Bible.GetChapterNumbersForBook(state.CurrentBook)
	idx := indexOf(numbers, state.CurrentChapter)
	// The row is exactly as tall as its tallest object, and the label alone
	// makes it 22pt: a 16pt canvas.Text. Any taller box pushes the chapter text
	// down by the difference, in the one orientation whose whole point is
	// height — measured on the iPhone 16 Pro simulator, a 28pt box cost 8pt, so
	// the portrait header's 36 (chapter_header_mobile.go) would cost 16. At the
	// label's own height the arrows cost the reader nothing; the tap box is the
	// row, 38pt wide, which is landscape's compromise.
	const boxH = 22
	prev := newIconTapButton(state, theme.NavigateBackIcon(), 20, boxH, func() {
		if moveChapter(state, -1) {
			state.refresh()
		}
	})
	prev.disabled = idx <= 0
	next := newIconTapButton(state, theme.NavigateNextIcon(), 20, boxH, func() {
		if moveChapter(state, 1) {
			state.refresh()
		}
	})
	next.disabled = idx < 0 || idx >= len(numbers)-1
	return container.NewHBox(prev, next)
}
