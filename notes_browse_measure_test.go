package bibletext

// THE NOTES COLUMN'S MEASURE, and why it is a test rather than a constant with
// a comment.
//
// Everything in the notes browser was written for a phone, where "take the
// width you are given" is right because the width you are given is about 440pt.
// The iPad's two-pane layout hands the same view roughly 1,150pt, exposing two
// surfaces of the same cause:
//
//   - WITH notes: a one-line message became a long thin box with its words at
//     the far left, reading as an empty input field rather than as something a
//     person wrote.
//   - WITHOUT notes: the empty-state sentence spread into a single hairline
//     across the pane, so the eye went to the sidebar's book list instead and
//     the pane looked empty while the adjacent book list drew all attention.
//
// The cap fixes both. What these tests hold is the pair of properties that make
// it safe: it engages on a wide pane, and it is INERT on a phone — because the
// phone surface already reads correctly and a change there would be a
// regression dressed as a fix.

import (
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

func TestNotesColumnCapsAWidePaneAndCentresIt(t *testing.T) {
	child := canvas.NewRectangle(color.Transparent)
	l := notesMeasureLayout{}

	const paneW = 1150 // about what the iPad's regular layout gives the notes pane
	l.Layout([]fyne.CanvasObject{child}, fyne.NewSize(paneW, 800))

	if got := child.Size().Width; got != notesColumnMax {
		t.Errorf("on a %gpt pane the notes column is %gpt wide, want it capped at %gpt.\n"+
			"Uncapped, a one-line note is a thin box with its words stranded at the far "+
			"left — the defect this cap exists to remove.", float32(paneW), got, notesColumnMax)
	}
	if want := float32(paneW-notesColumnMax) / 2; child.Position().X != want {
		t.Errorf("the capped column sits at x=%g, want %g (centred). A capped column "+
			"pinned to one edge leaves the pane looking half-empty rather than composed.",
			child.Position().X, want)
	}
	if got := child.Size().Height; got != 800 {
		t.Errorf("the cap took height too (%g, want 800) — it is a MEASURE, and the "+
			"list must still fill the pane vertically or it cannot scroll.", got)
	}
}

// THE PHONE MUST NOT MOVE. This is the half that makes the change safe to ship:
// a phone pane is far narrower than the measure, so the layout must be a no-op
// there, not a 620pt cap that quietly re-centres a surface nobody complained
// about.
func TestNotesColumnLeavesAPhonePaneAlone(t *testing.T) {
	for _, paneW := range []float32{320, 390, 440, notesColumnMax} {
		child := canvas.NewRectangle(color.Transparent)
		notesMeasureLayout{}.Layout([]fyne.CanvasObject{child}, fyne.NewSize(paneW, 600))

		if child.Size().Width != paneW {
			t.Errorf("a %gpt pane was resized to %gpt — at or below the measure the "+
				"layout must be inert.", paneW, child.Size().Width)
		}
		if child.Position().X != 0 {
			t.Errorf("a %gpt pane was shifted to x=%g — at or below the measure "+
				"nothing should move.", paneW, child.Position().X)
		}
	}
}

// NEVER CLIP. If a child's own minimum is wider than the measure, capping to the
// measure would cut it off — which is a worse failure than a wide column, and
// the kind that only shows up at a text size or locale nobody tested.
func TestNotesColumnNeverCapsBelowWhatTheChildNeeds(t *testing.T) {
	child := canvas.NewRectangle(color.Transparent)
	child.SetMinSize(fyne.NewSize(notesColumnMax+180, 40))

	notesMeasureLayout{}.Layout([]fyne.CanvasObject{child}, fyne.NewSize(1150, 400))

	if got := child.Size().Width; got < notesColumnMax+180 {
		t.Errorf("a child needing %gpt was laid out at %gpt — the cap clipped it. "+
			"The measure is a preference; the child's minimum is a requirement.",
			notesColumnMax+180, got)
	}
}

// AND EVERY EXIT GOES THROUGH IT. buildNotesBrowseView returns from more than
// one place (the empty state is a different sentence from a filter that matched
// nothing), so every branch must apply the same wrapper. Otherwise bubbles and
// the empty state can retain different halves of the same sizing defect.
func TestEveryNotesBrowseReturnTakesTheMeasure(t *testing.T) {
	src := readNativeSource(t, "notes_browse.go")

	start := strings.Index(src, "func buildNotesBrowseView(")
	if start < 0 {
		t.Fatal("cannot find buildNotesBrowseView — this guard can no longer say anything")
	}
	// Cut at the next TOP-LEVEL func, not at a comment: the first attempt cut on
	// "// noteBrowseRow" and swept in a neighbouring builder's return, which is
	// the guard reporting a defect in itself rather than in the code.
	body := src[start:]
	if end := strings.Index(body[1:], "\nfunc "); end > 0 {
		body = body[:end+1]
	}

	// Only the function's OWN exits. A return nested three tabs deep belongs to
	// the list's CreateItem closure, which returns a row template rather than the
	// view — flagging it was the guard misreading its own scope.
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "return ") {
			continue
		}
		if depth := len(line) - len(strings.TrimLeft(line, "\t")); depth > 2 {
			continue
		}
		if strings.Contains(trimmed, "notesMeasureColumn(") {
			continue
		}
		t.Errorf("buildNotesBrowseView returns without the measure:\n    %s\n\n"+
			"Every exit must go through notesMeasureColumn, or that branch renders "+
			"edge-to-edge on iPad while the others are capped.", trimmed)
	}
}
