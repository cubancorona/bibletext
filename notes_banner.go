package bibletext

// The note banner: how a stored note is VISIBLE on the platforms whose reading
// pane cannot float the iOS sticker (macOS's NSTextView, the Windows/Linux
// styled pane, Android's overlay). One untagged Fyne widget above the reading
// pane serves all four — the owner's parity rule made concrete: the feature
// lands everywhere at once, and the per-platform in-text sticker stays a
// possible refinement, not a prerequisite.
//
// It follows every rule the bubble does: the note renders as TEXT (never
// markup), it is attributed to a person so it can never pass for the app's own
// voice, Hide keeps it (and drops the highlight with it), Delete is the
// destructive verb, and a hidden note leaves a small chip so it can be found
// again. All through the same store helpers the iOS sticker uses, so the two
// surfaces cannot drift.

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// nativeNoteSticker is the platform answer behind a seam, the same arrangement
// reporterLayout uses: the HOST for these tests is darwin, where the answer is
// true, so a compile-time constant would make every banner test unrunnable on
// the only machine that runs them. The banner still ships to Windows, Linux and
// Android, so it still has to be testable.
var nativeNoteSticker = func() bool { return nativeNoteStickerOnPlatform }

// buildNoteBanner returns the banner for the current chapter's note, the chip
// for a minimized one, or nil when there is nothing to show. The caller slots
// it above the reading pane.
func buildNoteBanner(state *AppState) fyne.CanvasObject {
	if state == nil || !notesFeatureOn(state) || state.ActiveNote == "" {
		return nil
	}
	// Where the pane draws the note in the text itself, the banner would be the
	// SAME note a second time, in a worse place. macOS earned its way out of
	// here by growing the bubble (owner: the banner "doesn't move with the text,
	// doesn't have a tail, and the hidden note placeholder is frozen at the top"
	// — all three are what living outside the text costs).
	if nativeNoteSticker() {
		return nil
	}
	pal := state.pal()

	if state.NoteMinimized {
		// The collapsed chip: small, quiet, unmistakably a thing to press —
		// the note is still there and the reader must be able to find it again.
		show := widget.NewButtonWithIcon("Show note", iconNoteBubble, func() {
			restoreCurrentNote(state)
			state.refreshReadingOnly()
		})
		show.Importance = widget.LowImportance
		return container.NewHBox(show)
	}

	who := canvas.NewText("Note from Friend", pal.TextMuted)
	who.TextSize = 11

	ref := canvas.NewText(state.CurrentBook+" "+strconv.Itoa(state.CurrentChapter), pal.Accent)
	ref.TextStyle = fyne.TextStyle{Bold: true}
	ref.TextSize = 12

	body := widget.NewLabel(state.ActiveNote)
	body.Wrapping = fyne.TextWrapWord

	hide := widget.NewButtonWithIcon("", theme.VisibilityOffIcon(), func() {
		hideCurrentNote(state)
		state.refreshReadingOnly()
	})
	hide.Importance = widget.LowImportance
	del := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		dropCurrentNote(state)
		state.refreshReadingOnly()
	})
	del.Importance = widget.LowImportance

	inner := container.NewBorder(
		nil, nil, nil,
		container.NewVBox(container.NewHBox(hide, del)),
		container.NewVBox(container.NewHBox(who, ref), body),
	)
	return surface(container.NewPadded(inner), pal.SurfaceAlt, pal.Border, fyne.Size{})
}
