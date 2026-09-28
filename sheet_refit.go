package bibletext

// REFITTING THE OPEN SHEETS WHEN A DESKTOP WINDOW CHANGES SIZE.
//
// A desktop sheet takes its size from the window as it opens (sheet_fit.go),
// and the toolkit never sizes it again: on a window resize the driver only
// re-centres each open popup at the size it already has. A sheet sized for a
// taller window therefore moved up by half the height the window lost, and
// its top edge landed partway down the header's Go to chip, the very arc
// headerClearance keeps sheets clear of, when the reader restored a maximised
// window, dragged an edge up, or shrank the window the app opens at. A sheet
// sized for a shorter window stayed short in a taller one.
//
// So a sheet whose size comes from the window registers how to size itself
// again, beside its popup, and the window's root runs every registration each
// time it is laid out at a new size (windowRoot). The driver lays the root out
// straight after re-centring the popups, on a real window and under the test
// driver alike, so the refit has the last word. A refit sizes the sheet as
// opening it at the new size would, from what the sheet holds now; nothing
// typed, chosen or scrolled in it is touched.
//
// A registration lives exactly as long as its popup is showing, as a reopen's
// does (sheet_reopen.go): it is dropped at the next register, the next refit,
// and every rebuild's drain.
//
// DESKTOP ONLY. On a phone or tablet the content also changes height as the
// soft keyboard comes and goes (layoutWatcher, ui_regular.go), and a sheet
// resized under the reader's typing is what the Go to picker's rule against
// resizing exists to prevent. There a rotation that moves the navigation
// rebuilds the window, which closes the sheets; the Go to picker and the note
// composer refit themselves on the canvas changes that remain (goto.go,
// share_note_ui.go); the others keep the size they opened at, which a modal
// popup clamps to the canvas. Recorded in docs/BACKLOG.md.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// sheetRefit is one showing sheet's way of sizing itself to the window again.
type sheetRefit struct {
	popup *widget.PopUp
	fit   func()
}

// registerSheetRefit records how the sheet showing in popup sizes itself to
// the window, to be run whenever the window changes size. Call it once the
// popup is showing. Registering the same popup again replaces its fit. A
// no-op on a phone or tablet (above).
func registerSheetRefit(state *AppState, popup *widget.PopUp, fit func()) {
	if state == nil || popup == nil || fit == nil || fyne.CurrentDevice().IsMobile() {
		return
	}
	pruneSheetRefits(state)
	for i := range state.sheetRefits {
		if state.sheetRefits[i].popup == popup {
			state.sheetRefits[i].fit = fit
			return
		}
	}
	state.sheetRefits = append(state.sheetRefits, sheetRefit{popup: popup, fit: fit})
}

// refitSheets sizes every showing sheet to the window as it now is.
func refitSheets(state *AppState) {
	pruneSheetRefits(state)
	if state == nil || len(state.sheetRefits) == 0 {
		return
	}
	// A copy: a fit may open or close a sheet, which prunes the list.
	fits := make([]func(), 0, len(state.sheetRefits))
	for _, r := range state.sheetRefits {
		fits = append(fits, r.fit)
	}
	for _, fit := range fits {
		fit()
	}
}

// pruneSheetRefits drops every registration whose sheet has closed, and
// zeroes what the in-place filter left past the new length, so nothing holds
// a closed sheet alive.
func pruneSheetRefits(state *AppState) {
	if state == nil || len(state.sheetRefits) == 0 {
		return
	}
	kept := state.sheetRefits[:0]
	for _, r := range state.sheetRefits {
		if sheetShowing(state, r.popup) {
			kept = append(kept, r)
		}
	}
	for i := len(kept); i < len(state.sheetRefits); i++ {
		state.sheetRefits[i] = sheetRefit{}
	}
	state.sheetRefits = kept
}

// windowRoot is the root of the window's tree: its objects stacked as
// container.NewStack stacks them, laid out by a layout that refits the open
// sheets whenever the window gives the root a new size.
func windowRoot(state *AppState, objects ...fyne.CanvasObject) *fyne.Container {
	return container.New(&sheetRefitLayout{state: state}, objects...)
}

// sheetRefitLayout is a stack layout that also calls refitSheets when the size
// it lays out at changes. The root is laid out whenever the container is
// refreshed as well, at the same size, so the size is what is compared.
type sheetRefitLayout struct {
	state *AppState
	last  fyne.Size
}

func (l *sheetRefitLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return layout.NewStackLayout().MinSize(objects)
}

func (l *sheetRefitLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	// The tree first: a refit reads where the header now ends.
	layout.NewStackLayout().Layout(objects, size)
	if size == l.last {
		return
	}
	l.last = size
	refitSheets(l.state)
}
