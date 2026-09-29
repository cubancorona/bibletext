package bibletext

// BRINGING BACK THE SHEET A LIGHT/DARK REBUILD TOOK.
//
// A real appearance change rebuilds the window, and the rebuild drains every
// overlay (rebuildWindow): a sheet cannot be re-lit in place, because its
// canvas objects were filled from state.pal() when it was built, so the only
// legible copy of it is a new one. This is the one seam that makes that new
// copy. A sheet, once it is showing, registers how to show itself again,
// showing the same thing; the appearance gate (appearance.go) takes the
// registration of the sheet on TOP of the canvas, rebuilds, and calls it.
//
// A registration is taken in two steps, because the rebuild sits between
// them. The CAPTURE runs at the take, before the rebuild, while the sheet is
// still whole on the canvas: whatever it needs that the gate is about to
// throw away is read then — the field that held the caret, which the gate
// drops before rebuilding so the soft keyboard goes down with the sheet
// instead of staying up over a field that is gone. It returns the REOPEN,
// which runs after the rebuild, in the new palette. Most sheets need nothing
// read early and register a plain reopen (registerSheetReopen); a sheet whose
// widgets still hold what it showed can read them in the reopen itself,
// because a drained sheet is hidden, not emptied.
//
// A registration belongs to its popup and lives exactly as long as the popup
// is showing. Every way a sheet closes ends in PopUp.Hide — its own close
// button, a tap outside a non-modal card, Escape (state.dismissSheet), a
// sheet whose action closes it and opens another, rebuildWindow's drain — and
// Hide takes the popup off the overlay stack and turns Visible off. Fyne gives
// no hook on Hide, so the registration is not cleared BY the close; it is
// dead from that moment, because nothing here ever answers for a popup that is
// not both visible and on the stack, and it is pruned — the closure, and the
// whole drained sheet it holds, let go — on the next register, the next take,
// and at every rebuild's drain. So a sheet the reader closed can never come
// back, and a take removes what it returns, so none comes back twice. The
// reopened sheet registers again as it opens, so the next change brings it
// back too.
//
// NESTED AND STACKED SHEETS: only the top one. A take answers for the sheet
// on top and nothing else; if that sheet registered nothing, nothing reopens
// and every sheet under it closes with it. A sheet opened over another — the
// model picker over Settings, the keep-or-delete question over Settings — is
// a choice being made inside the one below, and bringing back the bottom one
// without it would drop the choice half made.
//
// MENUS AND TOASTS ARE NOT SHEETS, and the take looks straight through them to
// the sheet beneath. A Fyne menu is on the overlay stack as Fyne's own
// container, not a *widget.PopUp: an entry's Cut/Copy/Paste menu (a
// right-click on desktop, a long-press on a phone — in the Go to verse fields,
// the Ask field, the note composer, Settings' key field), a Select's list, the
// selection menus. The desktop share confirmation is a *widget.PopUp that
// takes itself down after a second and a half (showShareNotice). None of them
// is something the reader opened to read, and any of them over a sheet made
// that sheet look unregistered: the drain took the sheet and nothing brought
// it back, so the Go to picker under a context menu went, with the reader's
// book, chapter and verse. So they are skipped — the drain closes them like
// everything else — and the first real sheet beneath them answers, under the
// rule above: its registration, or nothing.
//
// WHAT REOPENS, each registered beside its own popup, from what the sheet
// already holds or what state still says:
//
//   - the Go to picker, both flavours: the same book and chapter selected,
//     the alphabet navigator at the same letter, the verse and end fields'
//     text, and the caret back in the field that had it (goto.go);
//   - Verse of the day: the same verse, even when the day has turned since it
//     opened (verse_of_day.go);
//   - the translation picker, with its footer notice read again from state as
//     it now stands — the rebuild may itself be the one that paints a
//     download that landed while the picker was up — and without repeating
//     the manual retry that opening it performs (versions_ui.go);
//   - Settings, at its top (ai_settings.go — everything in it saves as it
//     changes, so nothing typed is lost), with a Test key's wait or verdict,
//     which is held on state rather than by the drained sheet
//     (key_test_progress.go);
//   - the audio source menu (audio_menu.go);
//   - cross-references for the same selection (crossref_panel.go);
//   - the Ask sheet with the question typed so far (ai_ask.go);
//   - the note composer with the note typed so far, on every platform — on
//     iOS the field is a native text view, and a drained composer's teardown
//     leaves it alone once a newer composer owns it (share_note_ui.go);
//   - an AI answer once it has landed: the reopen reads it back from aiCache,
//     so it costs no second request (ai_panel.go);
//   - the "Downloading…" modal, for as long as its download runs: the
//     download's dismissal closes whichever copy is showing, so a change no
//     longer leaves the reader free to open a sheet the landing's rebuild
//     would then close (versions_ui.go);
//   - the note-link offer, the link notices, and the translation-download
//     error (notes_offer.go, share_link_unavailable.go, versions_ui.go) — and
//     a notice that promises something still to come only while the promise
//     stands: the seed-parked link's notice does not come back once the
//     rebuild has opened the passage it was waiting for (share_link_open.go).
//
// WHAT CLOSES, deliberately, registering nothing:
//
//   - an AI answer still being fetched: the request belongs to the drained
//     panel, and that panel already reopens itself when its answer lands
//     (ai_panel.go); a second reopen here would stack two panels;
//   - the AI panel showing an error or asking for a key: an error is not an
//     answer aiCache holds, so reopening it would send the request again, on
//     the reader's key (ai_panel.go);
//   - the share-image preview: it carries rendered image files and the
//     Regenerate cycle the reader has been stepping through;
//   - the model picker and the notes keep-or-delete and delete-all questions:
//     each sits on top of Settings and answers into its controls (above);
//   - menus and the desktop share confirmation: not sheets (above), so they
//     close, and the sheet beneath them comes back;
//   - a notice whose promise the rebuild has kept (above).
//
// Any of these simply closes, which is still far better than an illegible
// sheet. UI goroutine only, like the overlay stack it reads.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// sheetReopen is one showing sheet's way back: capture runs at the take,
// before the rebuild, and returns the reopen that runs after it.
type sheetReopen struct {
	popup   *widget.PopUp
	capture func() func()
}

// registerSheetReopen records how the sheet showing in popup comes back after
// a rebuild. Call it once the popup is SHOWING — a popup not yet on the stack
// counts as closed. Registering the same popup again replaces its closure.
func registerSheetReopen(state *AppState, popup *widget.PopUp, reopen func()) {
	if reopen == nil {
		return
	}
	registerSheetReopenCapture(state, popup, func() func() { return reopen })
}

// registerSheetReopenCapture is registerSheetReopen for a sheet that must read
// something before the rebuild: capture runs at the take, while the sheet is
// still whole, and returns the reopen (nil for none).
func registerSheetReopenCapture(state *AppState, popup *widget.PopUp, capture func() func()) {
	if state == nil || popup == nil || capture == nil {
		return
	}
	pruneSheetReopens(state)
	for i := range state.sheetReopens {
		if state.sheetReopens[i].popup == popup {
			state.sheetReopens[i].capture = capture
			return
		}
	}
	state.sheetReopens = append(state.sheetReopens, sheetReopen{popup: popup, capture: capture})
}

// forgetSheetReopen withdraws popup's registration while it is still showing:
// for a sheet that has moved into a state reopening would be wrong for (an AI
// answer going back into flight).
func forgetSheetReopen(state *AppState, popup *widget.PopUp) {
	if state == nil || popup == nil {
		return
	}
	kept := state.sheetReopens[:0]
	for _, r := range state.sheetReopens {
		if r.popup != popup {
			kept = append(kept, r)
		}
	}
	clearSheetReopenTail(state.sheetReopens, len(kept))
	state.sheetReopens = kept
}

// sheetShowing reports whether popup is on the window's overlay stack and
// visible — the one test of "still open" every close path fails.
func sheetShowing(state *AppState, popup *widget.PopUp) bool {
	if state == nil || state.window == nil || popup == nil || !popup.Visible() {
		return false
	}
	for _, o := range state.window.Canvas().Overlays().List() {
		if o == popup {
			return true
		}
	}
	return false
}

// pruneSheetReopens drops every registration whose sheet has closed.
func pruneSheetReopens(state *AppState) {
	if state == nil || len(state.sheetReopens) == 0 {
		return
	}
	kept := state.sheetReopens[:0]
	for _, r := range state.sheetReopens {
		if sheetShowing(state, r.popup) {
			kept = append(kept, r)
		}
	}
	clearSheetReopenTail(state.sheetReopens, len(kept))
	state.sheetReopens = kept
}

// clearSheetReopenTail zeroes what an in-place filter left past its new length,
// so the backing array holds no closure — and no drained sheet — alive.
func clearSheetReopenTail(all []sheetReopen, n int) {
	for i := n; i < len(all); i++ {
		all[i] = sheetReopen{}
	}
}

// selfDismissingOverlay reports whether an overlay is a notice that takes
// itself down — chrome, not a sheet (MENUS AND TOASTS, above). The desktop
// share confirmation fills it in (share_fallback.go); on the phones the OS's
// own share sheet confirms, and no overlay is one.
var selfDismissingOverlay = func(fyne.CanvasObject) bool { return false }

// topSheet is the sheet on top of the canvas: the highest *widget.PopUp on the
// overlay stack that is not a self-dismissing notice, looking past any of
// Fyne's menus above it. nil when no sheet is showing.
func topSheet(state *AppState) *widget.PopUp {
	if state == nil || state.window == nil {
		return nil
	}
	list := state.window.Canvas().Overlays().List()
	for i := len(list) - 1; i >= 0; i-- {
		if p, ok := list[i].(*widget.PopUp); ok && !selfDismissingOverlay(p) {
			return p
		}
	}
	return nil
}

// takeTopSheetReopen removes the registration of the sheet on top of the
// canvas, runs its capture, and returns the reopen — nil when that sheet
// registered none. Called before the rebuild whose drain closes that sheet.
func takeTopSheetReopen(state *AppState) func() {
	pruneSheetReopens(state)
	top := topSheet(state)
	if top == nil {
		return nil
	}
	for _, r := range state.sheetReopens {
		if r.popup == top {
			forgetSheetReopen(state, top)
			return r.capture()
		}
	}
	return nil
}
