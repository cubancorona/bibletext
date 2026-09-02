//go:build ios || android

package bibletext

// The compact mobile chapter toolbar, shared by the iOS and Android native
// reading views (each platform's buildReadingViewMobile calls it). Split out
// of reading_ios.go when Android grew its own native overlay. Both platforms
// have full native audio engines (AVFoundation / BtAudio.java), so the audio
// control — gated on chapterAudioAvailable() — is effectively always present
// here (TTS covers chapters with no recording).

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// chapterHeaderMobile is a compact, low-chrome chapter toolbar tuned for the
// mobile reading view. The book heading carries the current chapter number
// ("John 1") with a small inline copy icon; the muted chapter line below it
// (tappable to open the picker) carries the prev/next chapter arrows, so all
// the chapter navigation clusters next to the book + chapter text. Full-screen
// is the lone control on the right.
//
//	┌─────────────────────────────────────────────────────┐
//	│ John 1 ⧉                                       ⤢    │
//	│ Chapter 1 of 21 ▾   ←  →                            │
//	└─────────────────────────────────────────────────────┘
func chapterHeaderMobile(state *AppState, chapterNumbers []int) fyne.CanvasObject {
	pal := state.pal()
	total := len(chapterNumbers)

	// "John 10 ⌄" — one cohesive tap target (text + a clear dropdown chevron) that
	// opens the combined reference picker (book list + chapter grid). A roomy box
	// height makes it a comfortable touch target.
	// One even box height for BOTH rows, so the title row and the chapter/nav row
	// share the same vertical rhythm and the toolbar stays compact. A slightly
	// smaller heading (vs the 26px page heading) keeps it closer in scale to the
	// chapter line below, so that line no longer floats in an over-tall box.
	// boxH 36 (was 30): taller boxes = taller tap targets for every control in both
	// header rows, and it raises the ceiling the expanded audio card must fit under
	// (2×boxH+2 = 74 > the card's 72 — see buildAudioCard's row comment).
	const boxH = 36
	const headSize = 22
	ref := newReferenceButton(fmt.Sprintf("%s %d", state.CurrentBook, state.CurrentChapter), pal.Text, headSize, boxH, func() {
		showChapterPicker(state)
	})

	// Small copy icon tucked after the heading. The tap flashes a checkmark —
	// copying has no visible result of its own, so the button itself confirms.
	var copyBtn *iconTapButton
	copyBtn = newIconTapButton(state, theme.ContentCopyIcon(), 18, boxH, func() {
		copyChapter(state)
		copyBtn.flashIcon(theme.ConfirmIcon(), 1200*time.Millisecond)
	})
	titleRow := container.NewHBox(ref, hgap(6), copyBtn)

	// Quiet chapter context below the heading — also a picker target, so the
	// whole "Chapter N of M" line opens the picker too.
	chapText := fmt.Sprintf("Chapter %d of %d", state.CurrentChapter, total)
	if total <= 1 {
		chapText = fmt.Sprintf("Chapter %d", state.CurrentChapter)
	}
	chapterLine := newTapTextStyled(chapText, pal.TextMuted, subheadingTextSize, boxH, false, func() {
		showChapterPicker(state)
	})

	idx := indexOf(chapterNumbers, state.CurrentChapter)

	// Prev/next as compact icon buttons sitting next to the chapter line, so
	// they're close to the book + chapter text rather than floating far right.
	prev := newIconTapButton(state, theme.NavigateBackIcon(), 22, boxH, func() {
		if moveChapter(state, -1) {
			state.refresh()
		}
	})
	prev.disabled = idx <= 0

	next := newIconTapButton(state, theme.NavigateNextIcon(), 22, boxH, func() {
		if moveChapter(state, 1) {
			state.refresh()
		}
	})
	next.disabled = idx < 0 || idx >= total-1

	// Controls sit directly in the HBox so the picker anchor keeps a first-class
	// hit box (a nested spacer-VBox left it unresponsive to taps on iOS).
	chapterRow := container.NewHBox(chapterLine, hgap(8), prev, next)

	// Full-screen is the lone control on the right.
	fullScreenBtn := widget.NewButtonWithIcon("", theme.ViewFullScreenIcon(), func() {
		state.IsFullScreen = true
		rebuildWindow(state)
	})
	fullScreenBtn.Importance = widget.LowImportance

	// Tighter-than-default gap between the two rows so the book heading and the
	// chapter/nav line read as one compact block, not two airy lines.
	left := container.New(layout.NewCustomPaddedVBoxLayout(2), titleRow, chapterRow)

	// The audio control sits in the Border CENTRE — the gap between the chapter block
	// (left) and the full-screen button (right), vertically centred. Collapsed it's a
	// speaker; expanded it's a compact two-row card that fits the gap. (The
	// translators'-footnotes toggle is deliberately NOT in this header: the
	// feature's one control is the Settings card, by design — and a wider
	// right column overlaps the expanded audio card's reserved footprint on
	// 375pt phones; see footnote_section.go.)
	right := container.NewVBox(layout.NewSpacer(), fullScreenBtn, layout.NewSpacer())
	var centre fyne.CanvasObject
	if chapterAudioAvailable(state) {
		centre = container.NewCenter(audioControl(state, boxH))
	}
	row := container.NewBorder(nil, nil, left, right, centre)

	// TEMPORARY, dev builds only: what the note state actually is, on screen, so
	// a switch that loses the note can be diagnosed from a screenshot instead of
	// guessed at. Its OWN row under the toolbar — appended to the chapter line it
	// widened that line, shoved the nav arrows into the audio control and
	// jumbled the header; here it costs one short line of reading height and
	// every control keeps its release-build position. Empty in release builds
	// (dev_autoopen_off.go).
	if d := devNoteDebug(state); d != "" {
		debugLine := newTapTextStyled(d, pal.TextMuted, subheadingTextSize-2, 20, false, nil)
		return container.New(layout.NewCustomPaddedVBoxLayout(0), row, debugLine)
	}

	// No divider under the header — the flat reading surface separates the chapter
	// toolbar from the verses with whitespace (the text view's top inset) instead
	// of a hard rule.
	return row
}
