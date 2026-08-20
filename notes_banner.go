package bibletext

// The note banner: how the chapter's notes are VISIBLE on the platforms whose
// reading pane cannot float the iOS sticker (the Windows/Linux styled pane,
// Android's overlay). One untagged Fyne widget above the reading pane serves

// everywhere at once, and the per-platform in-text sticker stays a possible
// refinement, not a prerequisite.
//
// SINCE S8 IT READS ONLY THE PLAN (notes_plan.go), and it draws the SET:
//
//   - the OPEN note (at most one — the cap) as the shared tailed bubble, the
//     byline outside it and the version abbreviation in the heading, with the
//     Hide and Delete verbs;
//   - every OTHER placed note as a CHIP — the sender's citation, the
//     translation label where one applies, the date, and a quiet marker when
//     the reader has stored-Minimized it. Tapping a chip focuses that note
//     (the cap keeps one open);
//   - then, behind a separator, the R4 UNPLACED group: notes filed under this
//     book with no home in the translation being read, each a chip with its
//     placementCopy sentence beneath. Nothing in the store is invisible.
//
// Suppression needs no code here: a foreign mark stands the notes down IN THE
// PLAN (zero Open), so the strip quietly shows chips only and comes back by
// itself — nothing is stored, nothing is restored. Feature off is the same
// gate the plan applies: the empty plan draws nothing.
//
// It follows every rule the bubble does: the note renders as TEXT (never
// markup), it is attributed to a person so it can never pass for the app's own
// voice, Hide keeps it (and drops the highlight with it), Delete is the
// destructive verb.

import (
	"time"

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
// SINCE 19 AUG THE SEAM ASKS THE REAL QUESTION rather than naming platforms:
// "does the reading pane draw the note itself?". The styled Windows/Linux pane
// now does (reading_styled_note.go), so it answers yes through useStyledPane()
// — and if styledPaneEnabledOnPlatform is ever flipped back to the legacy
// chapterText Entry pane (the documented one-line revert), the banner RETURNS
// automatically, because that pane has no sticker. A bare `true` on the
// !darwin && !android constant could not do that.
var nativeNoteSticker = func() bool { return nativeNoteStickerOnPlatform || useStyledPane() }

// buildNoteBanner returns the banner for the current chapter's notes, or nil
// when there is nothing to show. The caller slots it above the reading pane.
func buildNoteBanner(state *AppState) fyne.CanvasObject {
	if state == nil || !notesFeatureOn(state) {
		return nil
	}
	plan := buildChapterPlan(state, appPrefs(), state.Bible)
	// A payload that could not be rendered is TOLD, in the note's place
	// (docs/NOTE_WIRE_FORMAT.md rule 5): quiet non-chrome styling, attributed
	// to nobody, no call to action, no link. This branch runs BEFORE the
	// native-sticker suppression below on purpose — the in-text sticker only
	// ever draws a decoded note, so there is nothing for a notice to duplicate,
	// and without this the macOS reader would never be told at all.
	if plan.Notice != "" {
		return buildNoteNotice(state)
	}
	// Where the pane draws the notes in the text itself, the banner would be
	// the same set a second time, in a worse place. The Apple panes carry the
	// open note as the native sticker and the rest of the set in the sticker's
	// own WHO line (appleStickerPush) — richness differs, truth does not.
	pal := state.pal()
	if nativeNoteSticker() {
		// THE ONE SURFACE THE STICKER CANNOT CARRY. The R4 group's sentence
		// ("Not in this translation. Read it in <T>") does not fit a one-band
		// sticker without roughly doubling its height, so on the styled pane
		// those rows KEEP the banner — never lose a surface the reader had.
		// The Apple and Android panes are byte-identical to before: they fold
		// the unplaced COUNT into the sticker's who line and have never drawn
		// the sentences here.
		if useStyledPane() && len(plan.Unplaced) > 0 {
			return noteUnplacedStrip(plan, pal)
		}
		return nil
	}
	rows := container.NewVBox()

	open, hasOpen := plan.openNote()
	if hasOpen {
		rows.Add(noteOpenBubble(state, open, pal))
	} else if state.ActiveNote != "" && state.NoteID == 0 {
		// A session-only note the store refused (it arrived while the store
		// was unreadable): the mirror is the only place it exists, so failing
		// open toward showing means drawing it from there. The plan cannot
		// carry what the store never held.
		rows.Add(noteOpenBubble(state, drawnNote{Note: StoredNote{
			VersionID: state.currentVersion().ID,
			Book:      state.CurrentBook,
			Chapter:   state.CurrentChapter,
			VerseLo:   state.NoteVerseLo,
			Text:      state.ActiveNote,
			Minimized: state.NoteMinimized,
		}}, pal))
		hasOpen = true
	}

	// The chips row: every placed note that is not the open bubble. When
	// nothing is open — all minimized, an explicit close, or a foreign mark
	// standing the notes down — the whole set is chips, and stays quiet.
	chips := container.NewVBox()
	for _, d := range plan.Notes {
		if d.Open {
			continue
		}
		chips.Add(noteBannerChip(state, d, pal))
	}
	rows.Add(chips)

	// The R4 group, behind a rule: notes on this book that have no home in
	// the translation being read. Chips with their reason beneath — never a
	// silent skip.
	if len(plan.Unplaced) > 0 {
		rows.Add(widget.NewSeparator())
		for _, d := range plan.Unplaced {
			rows.Add(noteUnplacedRow(d, pal))
		}
	}

	if !hasOpen && len(chips.Objects) == 0 && len(plan.Unplaced) == 0 {
		return nil
	}
	return rows
}

// noteChipLabel is a chip's text: the sender's own citation, the translation
// label where one applies (drawnNote.Label — placedNative carries none, R2's
// rule), the date, and the quiet stored-Minimized marker.
func noteChipLabel(d drawnNote) string {
	label := noteReference(d.Note)
	if d.Label != "" {
		label += " (" + d.Label + ")"
	}
	if when := noteDateLabel(d.Note.Received, time.Now()); when != "" {
		label += " · " + when
	}
	if d.Note.Minimized {
		label += " · hidden"
	}
	return label
}

// noteOpenBubble draws the open note: a heading carrying the sender's own
// citation (and the translation abbreviation when the note is from another
// one), the verbs, then the shared tailed bubble with the byline OUTSIDE it —
// exactly the shape the notes browser draws, from the same builder, so the two
// surfaces cannot drift.
func noteOpenBubble(state *AppState, d drawnNote, pal palette) fyne.CanvasObject {
	ref := canvas.NewText(noteReference(d.Note), pal.Accent)
	ref.TextStyle = fyne.TextStyle{Bold: true}
	ref.TextSize = 12
	heading := fyne.CanvasObject(ref)
	if d.Label != "" {
		v := canvas.NewText("("+d.Label+")", pal.TextMuted)
		v.TextSize = 11
		heading = container.NewHBox(ref, container.NewCenter(v))
	}

	hide := widget.NewButtonWithIcon("", theme.VisibilityOffIcon(), func() {
		hideCurrentNote(state)
		state.refreshReadingOnly()
	})
	hide.Importance = widget.LowImportance
	del := widget.NewButtonWithIcon("", noteTrashIcon(pal.TextMuted), func() {
		dropCurrentNote(state)
		state.refreshReadingOnly()
	})
	del.Importance = widget.LowImportance

	head := container.NewBorder(nil, nil, container.NewHBox(heading), container.NewHBox(hide, del))
	return container.NewVBox(head, noteBubbleWithByline(d.Note.Text, noteByline(d.Note), pal))
}

// noteBannerChip is one closed note: quiet, small, unmistakably a thing to
// press. Tapping it focuses the note — the reader's session choice, which the
// cap answers by opening this one and closing the other — restoring it first
// if the reader had stored-Minimized it (the chip IS the Show verb there).
func noteBannerChip(state *AppState, d drawnNote, pal palette) fyne.CanvasObject {
	id := d.Note.ID
	minimized := d.Note.Minimized
	chip := widget.NewButtonWithIcon(noteChipLabel(d), iconNoteBubble, func() {
		if minimized {
			// The reader pressed the note's chip: that is Show, the one
			// durable restore — by the note's own identity, handed here by the
			// plan, never rebuilt.
			setNoteMinimizedByID(appPrefs(), id, false)
		}
		// Tapping a chip is the reader choosing this note as the page's
		// reason, so a foreign mark stands aside (the identity table: "taps a
		// note chip instead → that is the new choice"). The note's own mark is
		// re-raised by the re-projection below.
		if state.mark.live() && !state.mark.fromNote() {
			state.clearMark()
		}
		state.focusNote(id)
		applyNoteForCurrentChapter(state)
		state.refreshReadingOnly()
	})
	chip.Importance = widget.LowImportance
	return container.NewHBox(chip)
}

// noteUnplacedStrip is the R4 group ALONE — what survives of the banner on a
// platform whose pane draws the sticker in the text. It is deliberately the
// same rows the full banner builds, from the same builder, so the two cannot
// drift; only the bubble and the chips are gone, replaced by the sticker.
func noteUnplacedStrip(plan chapterPlan, pal palette) fyne.CanvasObject {
	rows := container.NewVBox()
	for _, d := range plan.Unplaced {
		rows.Add(noteUnplacedRow(d, pal))
	}
	return rows
}

// noteUnplacedRow is one R4 note: the chip line, then the placementCopy
// sentence saying why nothing is tinted for it. Inert on purpose — there is
// nothing on this page to focus; the note is reachable in the notes browser,
// which can switch to the translation that has it.
func noteUnplacedRow(d drawnNote, pal palette) fyne.CanvasObject {
	line := canvas.NewText(noteChipLabel(d), pal.TextMuted)
	line.TextSize = 12
	line.TextStyle = fyne.TextStyle{Bold: true}
	why := widget.NewLabel(d.sentence())
	why.Wrapping = fyne.TextWrapWord
	why.Importance = widget.LowImportance
	return container.NewVBox(line, why)
}

// buildNoteNotice renders the could-not-read-the-note sentence where the note
// bubble would have been. Deliberately BARE: no byline (it is from nobody), no
// reference row, no buttons, no link — a reader in a message thread must never
// see anything here that reads as "tap this" or "install that"
// (docs/NOTE_WIRE_FORMAT.md rule 5, and the phishing argument behind it). The
// text is the app's, so LowImportance keeps it quieter than a person's words.
func buildNoteNotice(state *AppState) fyne.CanvasObject {
	pal := state.pal()
	body := widget.NewLabel(state.NoteNotice)
	body.Wrapping = fyne.TextWrapWord
	body.Importance = widget.LowImportance
	return surface(container.NewPadded(body), pal.SurfaceAlt, pal.Border, fyne.Size{})
}
