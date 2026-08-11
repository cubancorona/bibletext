package bibletext

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// The shared-notes switch.
//
// Notes are the one feature that puts SOMEBODY ELSE'S words on the reader's
// page. That is worth a way out — and a way out that does not quietly destroy
// what people have already been sent, which is why turning it off asks whether
// to keep or delete rather than deciding for them.
//
// WHAT THE SWITCH CANNOT DO. It cannot stop shared links reaching the app.
// Whether iOS or Android hands a bibletext.co.uk link to BibleText is settled by
// the entitlement, the manifest and the published association files — all fixed
// when the build is made, none of them reachable from a running app. So the
// honest scope of "off" is: the app still opens the passage a link names, and
// simply has nothing to do with the note attached to it. Nothing is shown,
// nothing is stored, and the verb for writing one disappears.
//
// A reader who wants links to stop opening in the app at all does that where it
// actually lives: iOS Settings → BibleText, or Android's "Open by default".

const prefNotesEnabled = "notes.enabled"

// notesEnabled reports whether shared notes are on. It defaults to ON: a link
// carrying a note is useless without it, and a reader who has never opened the
// setting should get the feature the link assumes.
func notesEnabled() bool {
	p := appPrefs()
	if p == nil {
		return true // no app (unit tests): behave as the default
	}
	return p.StringWithFallback(prefNotesEnabled, "on") != "off"
}

func setNotesEnabled(on bool) {
	p := appPrefs()
	if p == nil {
		return
	}
	if on {
		p.SetString(prefNotesEnabled, "on")
		return
	}
	p.SetString(prefNotesEnabled, "off")
}

// deleteAllNotes throws away every stored note. Only ever called from the
// explicit "delete them" answer when the switch is turned off.
func deleteAllNotes(p prefStore) {
	if p == nil {
		return
	}
	p.SetString(prefSharedNotes, "")
}

// notesFeatureOn is the single gate the rest of the app asks. It exists so no
// caller has to remember both halves: a note is only live if the reader wants
// notes at all.
func notesFeatureOn(state *AppState) bool { return notesEnabled() }

// promptKeepOrDeleteNotes is the question the switch asks on its way off: the
// notes already received are somebody else's messages, and a switch has no
// business binning a stack of them silently. Keeping is the default and the
// non-destructive answer; deleting is marked as the destructive one.
//
// onOff runs for either answer — the feature goes off regardless. onCancel puts
// the switch back, so backing out of the question leaves nothing changed.
func promptKeepOrDeleteNotes(state *AppState, onOff func(), onCancel func()) {
	if state == nil || state.window == nil {
		onOff()
		return
	}
	cnv := pickerCanvas(state)
	if cnv == nil {
		onOff()
		return
	}
	pal := state.pal()
	count := len(readNotes(appPrefs()))

	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	var popup *widget.PopUp
	closed := false
	closeIt := func() {
		if closed {
			return
		}
		closed = true
		if popup != nil {
			popup.Hide() // removes it from the overlay stack synchronously
		}
		// This prompt sits ON TOP of the Settings sheet, which hid the reading
		// overlay when IT opened and restores it when IT closes. Restoring here
		// unconditionally painted the native note sticker straight over the
		// still-open sheet (caught in the simulator). Restore only when nothing
		// else owns the canvas — the same rule the sheet watchdogs follow.
		if state.showReadingOverlay != nil && cnv.Overlays().Top() == nil {
			state.showReadingOverlay()
		}
	}

	title := canvas.NewText("Turn off shared notes", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 18

	noun := "notes"
	if count == 1 {
		noun = "note"
	}
	body := widget.NewLabel("You have " + strconv.Itoa(count) + " saved " + noun +
		". Keep them in case you turn notes back on, or delete them for good?")
	body.Wrapping = fyne.TextWrapWord

	keep := widget.NewButton("Keep them", func() {
		closeIt()
		onOff()
	})
	keep.Importance = widget.HighImportance
	del := widget.NewButton("Delete them", func() {
		deleteAllNotes(appPrefs())
		closeIt()
		onOff()
	})
	del.Importance = widget.DangerImportance
	cancel := widget.NewButton("Cancel", func() {
		closeIt()
		if onCancel != nil {
			onCancel()
		}
	})

	form := container.NewVBox(
		title, body,
		widget.NewSeparator(),
		container.NewBorder(nil, nil, cancel, container.NewHBox(del, keep)),
	)
	card := surface(container.NewPadded(form), pal.SurfaceAlt, pal.Border, fyne.Size{})
	popup = widget.NewModalPopUp(card, cnv)
	popup.Show()
	w := float32(420)
	if cw := cnv.Size().Width - 40; cw > 260 && w > cw {
		w = cw
	}
	popup.Resize(fyne.NewSize(w, card.MinSize().Height))
}
