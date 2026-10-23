package bibletext

// The desktop note composer on a short window: its card is sized from its
// content and centred, so the excerpt's rows are what put its top inside the
// header (share_note_ui.go). These lay the real window out at the heights the
// window allows, as sheet_header_clearance_test.go does for the other sheets.

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// THE NOTE COMPOSER GIVES UP EXCERPT ROWS TO CLEAR THE HEADER. Its desktop
// card is sized from its content and centred, so on a short window the
// three-row excerpt put its top inside the header, partway down the Go to
// chip at 390pt. Where the card is taller than the room below the header the
// excerpt gives up rows down to one, and takes them back when the window
// grows. Below about 400pt even one row leaves the card too tall for the gap
// every sheet keeps below the header, and there the card keeps its rows and
// covers the header (sheet_header_cover_test.go). Mutations guarded: the card
// sized from the full excerpt whatever the window (inside the header at
// 1280x440); a card that cannot clear the header left where one row puts it.
func TestTheNoteComposerGivesUpExcerptRowsToClearTheHeader(t *testing.T) {
	const long = "For God so loved the world that he gave his one and only Son, that whoever believes in him " +
		"should not perish but have eternal life, for God did not send his Son into the world to judge the world, " +
		"but that the world should be saved through him, and whoever believes in him is not judged."
	open := func(t *testing.T, s *AppState) *widget.PopUp {
		t.Helper()
		return pickerPopup(t, s, func(s *AppState) { promptShareNote(s, long, selSpan{}) })
	}
	rows := func(t *testing.T, p *widget.PopUp) int {
		t.Helper()
		_, r := noteSheetExcerpt(t, p)
		return len(r.drawn())
	}

	st, _ := desktopWindow(t, fyne.NewSize(1280, 800))
	popup := open(t, st)
	if n := rows(t, popup); n != noteExcerptMaxLines {
		t.Fatalf("control: the excerpt takes %d rows at 1280x800, want %d", n, noteExcerptMaxLines)
	}
	popup.Hide()

	for _, win := range []fyne.Size{{Width: 1280, Height: 440}, {Width: 1280, Height: 400}} {
		t.Run(fmt.Sprintf("%.0fx%.0f", win.Width, win.Height), func(t *testing.T) {
			st, w := desktopWindow(t, win)
			popup := open(t, st)
			defer popup.Hide()
			wantClearOfHeader(t, st, w, popup)
			if n := rows(t, popup); n >= noteExcerptMaxLines {
				t.Errorf("the excerpt keeps %d rows on a %.0fpt window", n, win.Height)
			}
		})
	}
	for _, win := range []fyne.Size{{Width: 507, Height: 390}, {Width: 507, Height: 386}} {
		t.Run(fmt.Sprintf("%.0fx%.0f", win.Width, win.Height), func(t *testing.T) {
			st, w := desktopWindow(t, win)
			popup := open(t, st)
			defer popup.Hide()
			if !wantClearOrCovering(t, st, w, popup) {
				t.Errorf("one row clears the header on a %.0fpt window; the card should have kept more", win.Height)
			}
			if n := rows(t, popup); n != noteExcerptMaxLines {
				t.Errorf("covering the header on a %.0fpt window, the excerpt takes %d rows, want %d", win.Height, n, noteExcerptMaxLines)
			}
		})
	}

	// Sized again with the window: the rows go as it shrinks and come back
	// as it grows, and on the least window the card covers the header with
	// them.
	st, w := desktopWindow(t, fyne.NewSize(1280, 800))
	popup = open(t, st)
	defer popup.Hide()
	w.Resize(fyne.NewSize(1280, 440))
	wantClearOfHeader(t, st, w, popup)
	if n := rows(t, popup); n >= noteExcerptMaxLines {
		t.Errorf("shrunk to 1280x440, the excerpt keeps %d rows", n)
	}
	w.Resize(fyne.NewSize(1280, 800))
	if n := rows(t, popup); n != noteExcerptMaxLines {
		t.Errorf("grown back to 1280x800, the excerpt takes %d rows, want %d", n, noteExcerptMaxLines)
	}
	w.Resize(fyne.NewSize(1280, 386))
	if !wantClearOrCovering(t, st, w, popup) {
		t.Error("shrunk to 1280x386, the card clears the header; it cannot there")
	}
	if n := rows(t, popup); n != noteExcerptMaxLines {
		t.Errorf("shrunk to 1280x386, the excerpt takes %d rows, want %d", n, noteExcerptMaxLines)
	}
}
