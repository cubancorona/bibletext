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

// turnNotesOff is THE way the feature goes off, whoever asks — the Settings
// switch's nothing-saved short-circuit, its "Keep them" answer, the dev
// toggle. Beside flipping the preference it ends on the projection, at the
// moment notes go off: the off-branch of applyNoteForCurrentChapter puts out
// the mark the live note owns (ONLY the note's mark — ownership is recorded,
// mark.go, and a search result or a link span on the same chapter is not the
// notes feature's to take down; X4) AND clears the mirror the panes render
// from. Clearing the mark alone left the mirror standing, and the Apple
// sticker push gates on the mirror, not the feature (appleStickerPush): with
// a note open, Settings → off → close left the sticker on the page, expanded,
// verbs and all — tint gone — until the next navigation ran the off-branch.
// The same off-branch applies on every later re-derive, so a route that flips
// the preference some other way still cannot strand either past its next
// derive.
func turnNotesOff(state *AppState) {
	setNotesEnabled(false)
	applyNoteForCurrentChapter(state)
}

// turnNotesOn is the way back ON — the Settings switch's on-branch and the dev
// toggle. Beside flipping the preference it re-derives the projection at the
// moment of the switch: if any navigation happened while notes were off, the
// off-branch derive cleared the mirror, so without this a chapter's stored
// note stayed invisible on the Apple panes (the sticker push returns early on
// an empty mirror) and unwashed on the others (only the projection re-raises
// hlNote) until the next navigation. Same convention as every healthy verb:
// write the preference, end on the projection; the caller triggers the repaint
// it owns. (offerNoteLinkChoice's "turn notes back on" goes through
// applyShareTarget instead, whose own tail is this same projection.)
func turnNotesOn(state *AppState) {
	setNotesEnabled(true)
	applyNoteForCurrentChapter(state)
}

// deleteAllNotes throws away every stored note — received and sent, it is one
// store — after the reader explicitly confirmed.
//
// It writes the one-line header sentinel, NEVER the empty string: an empty
// value means "new reader", and a deliberate wipe must stay distinguishable
// from a value-level loss (notes_store.go). The ID counter is deliberately NOT
// reset — an ID, once issued, is never reused, however much is deleted.
//
// The quarantine goes too: "delete all" is the reader's explicit, confirmed
// destructive verb, and a control that promised everything gone while quietly
// keeping unreadable bytes would be lying. It stands down only when the store
// is unreadable as a whole — the same never-overwrite-what-you-cannot-read
// contract every writer honours (and the UI disables the control there anyway,
// because an unreadable store counts zero notes).
func deleteAllNotes(p prefStore) {
	if p == nil {
		return
	}
	if !readNoteStoreRaw(p).ok {
		return
	}
	p.SetString(prefNotesStore, notesWipedSentinel)
	// The pre-S5 blobs go with it: "all" must not leave un-migrated notes
	// behind to resurrect on the next read.
	p.SetString(prefLegacySharedNotes, "")
	p.SetString(prefLegacyMyNotes, "")
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
	count := storedNoteCount(appPrefs())

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
		// The store is empty; the LIVE mirror is not. Without this the banner
		// (and the iOS sticker) keep drawing the note the reader just confirmed
		// deleting — the render fingerprint has not changed, so nothing even
		// repaints — until they navigate to another chapter.
		clearLiveNote(state)
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

// promptDeleteAllNotes is the Settings → Shared notes "Delete all notes"
// question. It is the deliberate, always-available twin of the question the
// off-switch asks: a reader who wants the messages gone should not have to
// discover that turning the feature off is where deletion hides, nor should they
// have to turn a feature off to clear it.
//
// Deleting is destructive and irreversible — these are other people's messages
// and nothing re-fetches them — so the safe answer is the default and the
// destructive one is marked. onDone runs only if notes were actually deleted.
func promptDeleteAllNotes(state *AppState, onDone func()) {
	if state == nil || state.window == nil {
		return
	}
	cnv := pickerCanvas(state)
	if cnv == nil {
		return
	}
	count := storedNoteCount(appPrefs())
	if count == 0 {
		return // nothing to ask about
	}
	pal := state.pal()

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
			popup.Hide()
		}
		// Same rule as promptKeepOrDeleteNotes: this sits on top of the Settings
		// sheet, so only restore the reading overlay when nothing else owns the
		// canvas — otherwise the note sticker paints over the still-open sheet.
		if state.showReadingOverlay != nil && cnv.Overlays().Top() == nil {
			state.showReadingOverlay()
		}
	}

	noun := "notes"
	if count == 1 {
		noun = "note"
	}

	title := canvas.NewText("Delete all notes", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 18

	body := widget.NewLabel("This deletes all " + strconv.Itoa(count) + " saved " + noun +
		" for good. They are messages people sent you, and nothing can fetch them back — " +
		"the links themselves still work if you still have them.")
	body.Wrapping = fyne.TextWrapWord

	cancel := widget.NewButton("Cancel", closeIt)
	cancel.Importance = widget.HighImportance
	del := widget.NewButton("Delete them", func() {
		deleteAllNotes(appPrefs())
		// The store is empty; the LIVE mirror is not. Without this the banner
		// (and the iOS sticker) keep drawing the note the reader just confirmed
		// deleting — the render fingerprint has not changed, so nothing even
		// repaints — until they navigate to another chapter.
		clearLiveNote(state)
		closeIt()
		if onDone != nil {
			onDone()
		}
	})
	del.Importance = widget.DangerImportance

	form := container.NewVBox(
		title, body,
		widget.NewSeparator(),
		container.NewBorder(nil, nil, nil, container.NewHBox(del, cancel)),
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

// clearLiveNote drops the note AppState is currently showing, and the highlight
// that belongs to it. Deleting from the store is not enough: the panes render
// from this mirror, not from the store.
func clearLiveNote(state *AppState) {
	if state == nil {
		return
	}
	state.ActiveNote = ""
	state.NoteMinimized = false
	state.NoteVerseLo = 0
	// The identity goes with them. Left behind it would misdirect the next
	// Delete at a note the reader never had on screen.
	state.NoteID = 0
	// Only the mark the note owns — the same rule the verbs follow (X10): a
	// search result or a link span the reader was holding on this chapter is
	// not a note-deletion's to put out.
	state.clearMarkFromNote()
}
