package bibletext

// A PHONE SHEET IS SIZED TO THE AREA THE KEYBOARD IS NOT IN (sheetArea,
// sheet_fit.go). Fyne's iOS driver counts a raised keyboard as the canvas's
// bottom inset, and a light/dark reopen sizes the sheet it brings back while
// that inset is still deep: the reopened sheet ended at the keyboard's top
// and stayed that short once the keyboard went down, the page showing
// beneath it, and a tap there closed a non-modal sheet with what had been
// typed in it.

import (
	"fmt"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// phoneInsets gives the test driver's canvas an iPhone's insets — 47pt of
// status bar, 34pt of home indicator — and a 300pt keyboard while up is set,
// counted as the bottom inset the way the iOS driver counts it. It also
// forgets any keyboard-free foot an earlier test left behind.
func phoneInsets(t *testing.T) (up *bool) {
	t.Helper()
	const safeTop, foot, keyboardH = 47, 34, 300
	keyboardUp := false
	prevArea, prevFoot := canvasArea, keyboardFreeFoot
	keyboardFreeFoot.known, keyboardFreeFoot.foot = false, 0
	canvasArea = func(c fyne.Canvas) (fyne.Position, fyne.Size) {
		h := c.Size().Height - safeTop - foot
		if keyboardUp {
			h = c.Size().Height - safeTop - keyboardH
		}
		return fyne.NewPos(0, safeTop), fyne.NewSize(c.Size().Width, h)
	}
	t.Cleanup(func() { canvasArea, keyboardFreeFoot = prevArea, prevFoot })
	return &keyboardUp
}

// keyboardReported sets what the keyboard observers last reported.
func keyboardReported(t *testing.T, shown bool) {
	t.Helper()
	softKeyboardShown = shown
}

func TestSheetAreaGivesTheKeyboardBack(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	w := test.NewWindow(widget.NewLabel(""))
	defer w.Close()
	w.Resize(fyne.NewSize(390, 844))
	cnv := w.Canvas()
	up := phoneInsets(t)
	softKeyboard(t, false)

	area := func() (float32, float32) {
		pos, sz := sheetArea(cnv)
		return pos.Y, pos.Y + sz.Height
	}

	// The keyboard down: the area as the driver reports it, its foot remembered.
	if top, bottom := area(); top != 47 || bottom != 810 {
		t.Fatalf("keyboard down: the area spans %v..%v, want 47..810", top, bottom)
	}
	// Up, in the inset and in the report: the foot the keyboard-free read left.
	*up, softKeyboardShown = true, true
	if top, bottom := area(); top != 47 || bottom != 810 {
		t.Errorf("keyboard up: the area spans %v..%v, want 47..810, the keyboard given back", top, bottom)
	}
	// In the inset before the report has said so: a keyboard-deep inset is
	// never taken for a safe inset.
	softKeyboardShown = false
	if top, bottom := area(); top != 47 || bottom != 810 {
		t.Errorf("keyboard in the inset, not yet reported: the area spans %v..%v, want 47..810", top, bottom)
	}
	// Reported up with no keyboard in the inset (Android, or the report a
	// frame ahead of the driver): nothing to give back.
	*up, softKeyboardShown = false, true
	if top, bottom := area(); top != 47 || bottom != 810 {
		t.Errorf("keyboard reported but not in the inset: the area spans %v..%v, want 47..810", top, bottom)
	}
	// Down again: the foot is read afresh, so a rotation's new inset is taken.
	softKeyboardShown = false
	if _, bottom := area(); bottom != 810 {
		t.Errorf("keyboard down again: the area ends at %v, want 810", bottom)
	}

	// Before any keyboard-free foot is known, a read with the keyboard up
	// gives the area as reported: nothing is stretched under a navigation bar
	// on a guess.
	keyboardFreeFoot.known = false
	*up, softKeyboardShown = true, true
	if _, bottom := area(); bottom != 844-300 {
		t.Errorf("no foot known yet: the area ends at %v, want the reported %v", bottom, 844-300)
	}
}

// phoneSheet is one phone sheet with something typed into it, opened as the
// reader opens it, that the light/dark reopen brings back.
type phoneSheet struct {
	name   string
	native bool // the note composer with iOS's native field
	open   func(*testing.T, *AppState)
}

func phoneSheets(t *testing.T) []phoneSheet {
	t.Helper()
	compose := func(_ *testing.T, s *AppState) {
		promptShareNote(s, "For God so loved the world", selSpan{})
	}
	return []phoneSheet{
		{"the note composer", false, compose},
		{"the note composer, native field", true, compose},
		{"Settings", false, func(_ *testing.T, s *AppState) { showAISettings(s) }},
		{"Ask", false, func(_ *testing.T, s *AppState) { promptAskQuestion(s, "For God so loved the world") }},
		{"Verse of the day", false, func(_ *testing.T, s *AppState) { showVerseOfDayCard(s, longDayPassage()) }},
		{"Go to", false, func(_ *testing.T, s *AppState) { showGotoPicker(s) }},
	}
}

// A SHEET REOPENED WITH THE KEYBOARD UP ENDS WHERE AN ORDINARY OPEN'S ENDS.
// Each sheet is opened with the keyboard down, its box read, the keyboard
// raised (in the inset and in the report), the window flipped, the keyboard
// dropped and every held watchdog run: the reopened sheet spans what the
// first did. Mutation guarded: sheetArea returning the interactive area as
// reported (the reopened sheet ends 266pt short, at the keyboard's top).
func TestPhoneSheetsReopenedUnderTheKeyboardKeepTheirFoot(t *testing.T) {
	for _, sh := range phoneSheets(t) {
		t.Run(sh.name, func(t *testing.T) {
			h := phoneSheetHarness(t)
			held := holdSheetTimers(t)
			noteHeld := holdNoteSheetTimers(t)
			if sh.native {
				nativeNoteField(t, h.state.window)
			}
			up := phoneInsets(t)
			softKeyboard(t, false)
			cnv := h.state.window.Canvas()

			sh.open(t, h.state)
			p := h.top()
			if p == nil {
				t.Fatal("control: the sheet did not open")
			}
			top, bottom := sheetBox(t, p)
			if bottom > 844-34+0.5 {
				t.Fatalf("control: the sheet opened with the keyboard down ends at %.1f, under the home indicator", bottom)
			}
			if bottom < 600 {
				t.Fatalf("control: the sheet ends at %.1f; a keyboard 300pt tall could not shorten it", bottom)
			}
			runHeld(held)
			runHeld(noteHeld)

			// The keyboard comes up: the driver deepens the inset, the
			// observer reports it, and the canvas lays out for it.
			*up = true
			keyboardReported(t, true)
			p.Refresh()
			runHeld(held)
			runHeld(noteHeld)

			h.flip()
			again := h.top()
			if again == nil || again == p || !again.Visible() || h.overlays() != 1 {
				t.Fatalf("the sheet must come back, rebuilt, alone (overlays %d)", h.overlays())
			}

			// The keyboard goes down, and the watchdogs run their passes.
			*up = false
			keyboardReported(t, false)
			again.Refresh()
			for i := 0; i < 3; i++ {
				runHeld(held)
				runHeld(noteHeld)
			}
			if !again.Visible() || cnv.Overlays().Top() != again {
				t.Fatalf("the reopened sheet went away under the watchdogs; overlays %d", h.overlays())
			}
			nt, nb := sheetBox(t, again)
			if math.Abs(float64(nt-top)) > 0.5 || math.Abs(float64(nb-bottom)) > 0.5 {
				t.Errorf("reopened with the keyboard up, the sheet spans %.1f..%.1f once it is down; opened with it down it spans %.1f..%.1f",
					nt, nb, top, bottom)
			}
		})
	}
}

var _ = fmt.Sprint
