package bibletext

import (
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/mobile"
)

// TestVCenterLayoutCentersChildAtNaturalHeight locks in the fix for top-biased field
// text: vCenterLayout must hold its child at the child's natural MinSize height (never
// stretch it to fill) and center it vertically in the space the row hands it. When a row
// stretches a 14pt field taller than its natural height, Fyne's top-aligned text would
// otherwise drift to the top; centering the whole field keeps the text visually centered.
func TestVCenterLayoutCentersChildAtNaturalHeight(t *testing.T) {
	child := canvas.NewRectangle(color.Black)
	child.SetMinSize(fyne.NewSize(100, 30)) // "natural" field height

	var l vCenterLayout
	if got := l.MinSize([]fyne.CanvasObject{child}); got != fyne.NewSize(100, 30) {
		t.Fatalf("MinSize = %v, want 100x30 (passes the child's natural size through)", got)
	}

	// The row is much taller than the field's natural height (the bug condition).
	l.Layout([]fyne.CanvasObject{child}, fyne.NewSize(200, 80))

	if got := child.Size(); got != fyne.NewSize(200, 30) {
		t.Errorf("child size = %v, want 200x30 (full width, NOT stretched past natural height)", got)
	}
	// (80-30)/2 = 25 → vertically centered.
	if got := child.Position(); got != fyne.NewPos(0, 25) {
		t.Errorf("child pos = %v, want (0,25) — vertically centered in the stretched row", got)
	}
}

// TestSearchEntryRequestsSubmittingKeyboard locks in the fix that lets the iOS keyboard's
// return key run a search. A plain single-line Entry asks for SingleLineKeyboard, which iOS
// maps to a "Done" key that resigns the responder WITHOUT sending a newline — so OnSubmitted
// never fires. searchKeyEntry must request DefaultKeyboard so the return key delivers '\n'
// → KeyReturn → OnSubmitted. (numberEntry must keep its number pad.)
func TestSearchEntryRequestsSubmittingKeyboard(t *testing.T) {
	if kb := newSearchEntry().Keyboard(); kb != mobile.DefaultKeyboard {
		t.Errorf("searchKeyEntry.Keyboard() = %v, want DefaultKeyboard (return key submits)", kb)
	}
	if kb := newNumberEntry().Keyboard(); kb != mobile.NumberKeyboard {
		t.Errorf("numberEntry.Keyboard() = %v, want NumberKeyboard", kb)
	}
}

// collectText gathers the .Text of every canvas.Text under o (depth-first).
func collectText(o fyne.CanvasObject) []string {
	switch v := o.(type) {
	case *canvas.Text:
		return []string{v.Text}
	case *fyne.Container:
		var out []string
		for _, c := range v.Objects {
			out = append(out, collectText(c)...)
		}
		return out
	}
	return nil
}

// TestAskPromptHintUsesStraightQuotes locks in the Jonah-example formatting fix: the hint
// must quote the example with straight ASCII quotes, not curly quotes (the custom serif
// font has no curly-quote glyphs, so they rendered as missing-glyph boxes).
func TestAskPromptHintUsesStraightQuotes(t *testing.T) {
	texts := collectText(aiSearchPromptView(sampleState()))

	var hint string
	for _, s := range texts {
		if strings.Contains(s, "Jonah") {
			hint = s
			break
		}
	}
	if hint == "" {
		t.Fatal("Ask prompt view has no Jonah example hint")
	}
	if strings.ContainsAny(hint, "“”") {
		t.Errorf("Jonah hint %q still uses curly quotes (no glyphs in the serif font)", hint)
	}
	if !strings.Contains(hint, `"what did God say to Jonah?"`) {
		t.Errorf("Jonah hint %q is missing the straight-quoted example", hint)
	}
}
