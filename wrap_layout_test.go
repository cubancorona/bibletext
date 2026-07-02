//go:build !race

// Skipped under -race for the same reason as ui_render_test.go: Fyne's test app
// clears its font cache on a background goroutine, which races text measurement.

package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// TestWrappedParagraphReservesWrappedHeight locks in the fix for the empty states
// where an action button overlapped the body text (AI-Find "Set up AI", load-error
// "Retry", AI-search "Try again"). A word-wrapping widget.Label reports only its
// single-line MinSize until it has been laid out at a width, so the old
// container.NewGridWrap(fyne.NewSize(w, lbl.MinSize().Height), lbl) reserved ONE line
// and the following sibling overlapped the wrapped text. wrappedParagraph must reserve
// the real multi-line height (and pin the requested width).
func TestWrappedParagraphReservesWrappedHeight(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	const long = "Describe what you're looking for and AI finds the passages. " +
		"It uses your own AI provider key, stored only on this device."

	oneLine := widget.NewLabel("X").MinSize().Height // a genuine single line

	lbl := widget.NewLabel(long)
	lbl.Wrapping = fyne.TextWrapWord
	para := wrappedParagraph(lbl, 300)

	if h := para.MinSize().Height; h <= oneLine*1.5 {
		t.Fatalf("wrappedParagraph reserved %.0fpt (~%.1f lines of %.0fpt); the text wraps to "+
			"several lines at 300px, so it must reserve more — the pre-fix bug reserved a single "+
			"line and the button overlapped the wrapped text", h, h/oneLine, oneLine)
	}
	if w := para.MinSize().Width; w != 300 {
		t.Errorf("wrappedParagraph width = %.0f, want 300 (fixed content width)", w)
	}
}
