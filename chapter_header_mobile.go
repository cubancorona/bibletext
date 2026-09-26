package bibletext

// The compact mobile chapter toolbar, shared by the iOS and Android native
// reading views (each platform's buildReadingViewMobile calls it). Split out
// of reading_ios.go when Android grew its own native overlay. Both platforms
// have full native audio engines (AVFoundation / BtAudio.java), so the audio
// control — gated on chapterAudioAvailable() — is effectively always present
// here (TTS covers chapters with no recording).
//
// The file carries no build tag although only the two native panes call it:
// nothing in it is platform-specific, and building it everywhere lets the host
// test suite lay out and tap the real phone header (chapter_header_cover_test.go)
// rather than a copy of it that could drift.

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
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
	// The copy icon, the chapter arrows and the full-screen button each sit in a
	// slot that keeps its size while the open narration card hides it (coverSlot,
	// audio_button.go; the audio control's comment below says why it hides them).
	titleRow := container.NewHBox(ref, hgap(6), coverSlot(copyBtn))

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
	// hit box (a nested spacer-VBox left it unresponsive to taps on iOS). The
	// arrows' slots lay each arrow out over exactly the box the HBox gives the
	// slot, so an arrow's hit box is the one it had in the HBox itself.
	chapterRow := container.NewHBox(chapterLine, hgap(8), coverSlot(prev), coverSlot(next))

	// Full-screen is the lone control on the right. A widget.Button, extended
	// only so that the open card can tell where it draws its icon and hide it
	// (coverButton, audio_button.go).
	fullScreenBtn := newCoverButton(theme.ViewFullScreenIcon(), func() {
		state.IsFullScreen = true
		rebuildWindow(state)
	})
	fullScreenBtn.Importance = widget.LowImportance

	// Tighter-than-default gap between the two rows so the book heading and the
	// chapter/nav line read as one compact block, not two airy lines.
	left := container.New(layout.NewCustomPaddedVBoxLayout(2), titleRow, chapterRow)

	// The audio control sits in the Border CENTRE — the gap between the chapter block
	// (left) and the full-screen button (right), vertically centred on that gap.
	// Collapsed it's a speaker. Open, it's the two-row transport card, and on a
	// phone the gap is narrower than the card, so the card spills out of it: it
	// lies over the next-chapter arrow and the full-screen button, and with a
	// long heading or on the narrowest phones the copy icon and the heading's
	// end as well. That is accepted, because the open card is a pop-up: its ✕
	// collapses it back to the speaker while narration carries on, and what it
	// covered is back.
	//
	// Covering has to mean covering, though. container.NewBorder puts the centre
	// FIRST in its Objects, so the left and right parts were drawn after the card
	// and on top of it: the → arrow and the full-screen glyph showed over the
	// card's skip button and corner, and took taps there. So while the card is
	// open the row draws the audio control LAST, above both, and the card
	// swallows taps across its whole rectangle (tapShield, audio_button.go): a
	// tap on it reaches the card's own control or nothing, never a control it
	// covers.
	//
	// Nor may a control show in part beside the card. The card's edge seldom
	// falls between two controls, and drawn above them it left the → arrow's
	// tail showing to its left and a bracket of the full-screen glyph to its
	// right (a 402-wide iPhone in Matthew 27). So while the card is open each of
	// the copy icon, the two arrows and the full-screen button whose glyph the
	// card touches is hidden altogether — not drawn, not tappable anywhere on
	// its box, not reached by Tab — and it is back when the card closes
	// (cardCover, audio_button.go). What the card touches is worked out again
	// after every layout of the row, a rotation's included (coverRowLayout),
	// and whenever the card opens or closes (stack, below). The heading and the
	// chapter line name the chapter and are only covered. Each hideable control
	// sits in a slot that keeps its size, hidden or shown, so hiding one moves
	// nothing.
	//
	// And the ✕ has to be on the screen. Centred on the gap, the card ran past
	// the header's right edge wherever a long heading or a narrow phone put the
	// gap far enough right (in every book on a 320-wide canvas, and in Song of
	// Solomon up to a 430-wide one), and took its ✕ partly or wholly off the
	// screen: in 68 of the 73 books at 320, 12 at 360, and Song of Solomon up
	// to 402. A card that covers the full-screen button and cannot be closed
	// traps the reader, so the open card stops at the header's right edge
	// (phoneAudioCellLayout); wherever it already fits it stays where
	// NewCenter put it.
	//
	// While it is the speaker, the control is drawn FIRST, beneath its
	// neighbours, exactly as NewBorder drew it and where NewBorder put it. The
	// speaker is centred on the same gap, and where that gap is narrow its tap
	// box reaches under the copy icon and the full-screen button (on a 320pt
	// canvas it lies over 31pt of that button's 36 in Song of Solomon), where
	// those controls have always won the tap. So the row holds two places for
	// the control, one drawn before the chapter block and the full-screen
	// button and one after them, and moves the control from the one to the
	// other as the card opens and closes (stack, below). The row's own parts
	// never change, nor does their order. A painting canvas keeps a record of
	// each object it draws, matched to a container's objects in order, and
	// reordering the row's parts made every record in the row anew, so the
	// next frame laid the whole row out again; moving the control between two
	// places, each of which asks for the control's size whether it holds it or
	// not (audioPlaceLayout), makes new records only for the control, and that
	// frame lays out only the control and the place it has moved to. The row
	// keeps one BorderLayout (coverRowLayout runs it, and then works out what
	// the card hides), which lays both places out over the gap and places the
	// chapter block and the full-screen button by identity, so every part is
	// where NewBorder put it at every width, open or closed, save the open card
	// that phoneAudioCellLayout moves in from the edge. Opening or closing the
	// card is a repaint of the row and at most a move of the card, never a
	// relayout of the header.
	// (The translators'-footnotes toggle is deliberately NOT in this header: the
	// feature's one control is the Settings card, by design — and any control in
	// the right column, beside the full-screen button or stacked under it, would
	// lie under the open card on a phone; see footnote_section.go.)
	right := container.NewVBox(layout.NewSpacer(), coverSlot(fullScreenBtn), layout.NewSpacer())
	cover := &cardCover{controls: []coverable{copyBtn, prev, next, fullScreenBtn}}
	fullScreenBtn.onFocusChange = cover.update
	rowLayout := coverRowLayout{inner: layout.NewBorderLayout(nil, nil, left, right), cover: cover}
	var row *fyne.Container
	if !chapterAudioAvailable(state) {
		row = container.New(rowLayout, left, right)
		cover.row = row
	} else {
		cell := &phoneAudioCellLayout{right: right}
		var audio, beneath, above *fyne.Container
		stack := func(open bool) {
			if audio == nil {
				return // the control's first render, before the row holds it
			}
			from, to := above, beneath
			if open {
				from, to = beneath, above
			}
			if len(to.Objects) == 1 && cell.open == open {
				return // an audio state change that neither opens nor closes the card
			}
			cell.open = open
			cell.Layout(audio.Objects, audio.Size())
			from.Objects, to.Objects = nil, []fyne.CanvasObject{audio}
			// After the move, so what the card hides is worked out from where it
			// now lies.
			cover.setOpen(open)
			canvas.Refresh(row)
		}
		audio = container.New(cell, audioControl(state, boxH, stack))
		place := audioPlaceLayout{audio: audio}
		beneath, above = container.New(place, audio), container.New(place)
		row = container.New(rowLayout, beneath, left, right, above)
		cover.row = row
		cover.card = audio.Objects[0]
		stack(audioPanelOpen)
	}

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

// phoneAudioCellLayout places the phone header's audio control in the gap the
// row's BorderLayout leaves it. The control is a cell reserved at the open
// card's size (audioControl), and it is centred on the gap exactly as
// container.NewCenter centres it, open or closed, save for one case: an open
// card that would run past the header's right edge is moved left until its
// right edge is the header's, so its ✕ and skip-forward stay on the screen.
// The speaker is never moved, and the header's height never changes, so
// opening the card is still a repaint of the row and a move of the card, not
// a relayout of the header or of the text beneath it.
//
// The header's right edge is worked out the way BorderLayout places the right
// part: at its MinSize width, one theme pad beyond the gap. The layout cannot
// ask the row where it put that part, because BorderLayout resizes the gap
// (which is what runs this) before it moves it.
type phoneAudioCellLayout struct {
	right fyne.CanvasObject // the row's right part: the full-screen button's column
	open  bool
}

func (l *phoneAudioCellLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	layout.NewCenterLayout().Layout(objects, size)
	if !l.open {
		return
	}
	edge := size.Width
	if l.right.Visible() {
		edge += theme.Padding() + l.right.MinSize().Width
	}
	for _, o := range objects {
		if over := o.Position().X + o.Size().Width - edge; over > 0 {
			o.Move(o.Position().SubtractXY(over, 0))
		}
	}
}

func (l *phoneAudioCellLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return layout.NewCenterLayout().MinSize(objects)
}

// audioPlaceLayout lays out one of the phone header row's two places for the
// audio control, which the row's BorderLayout lays over the gap: the control,
// while the place holds it, over the whole place. It asks for the control's
// MinSize whether the place holds the control or not, so moving the control
// from one place to the other changes no MinSize in the row, and a painting
// canvas finds nothing there to lay out again but the control and its new
// place.
type audioPlaceLayout struct {
	keepSizeLayout
	audio fyne.CanvasObject
}

func (l audioPlaceLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return l.audio.MinSize()
}
