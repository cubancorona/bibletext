//go:build !ios && !android

package bibletext

// Escape with a sheet up must close the sheet, not act on the page beneath it.
//
// Mutation this was written against: the overlay check at the top of the
// canvas key handler (installShortcuts) removed. widget.PopUp handles no keys
// and the card opens with nothing focused, so the press fell through and
// clearSearchState wiped whatever mark was live under the modal — the card
// stayed up and the reader found the wash gone on Close.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestEscapeClosesTheCardAndKeepsTheMarkBeneathIt(t *testing.T) {
	st, win := smallPhone(t)
	votdSynchronousRemeasure(t)
	installShortcuts(st)
	st.setHL(hlSearch, "John", 1, 1, 0)
	if !st.hasMark() {
		t.Fatal("fixture: the mark was not set")
	}

	showVerseOfDay(st)
	p := topPopup(t, win)
	if !p.Visible() {
		t.Fatal("fixture: the card did not open")
	}

	win.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyEscape})

	if p.Visible() || win.Canvas().Overlays().Top() != nil {
		t.Error("Escape left the card up")
	}
	if !st.hasMark() {
		t.Error("Escape cleared the mark beneath the card")
	}

	// The control: with no sheet up, the same key still clears the mark, so
	// the check above proves the guard and not a dead handler.
	win.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if st.hasMark() {
		t.Error("with nothing on top, Escape no longer clears the mark — the handler is dead, " +
			"and the assertion above proved nothing")
	}
	_ = test.NewApp
}
