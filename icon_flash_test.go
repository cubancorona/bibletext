package bibletext

// The copy-chapter button's pressed feedback: copying has no visible result of
// its own, so the button flashes a checkmark. Pinned here at the widget level —
// tap swaps the glyph, restore brings the original back, and a newer flash
// supersedes an older one's restore.

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func TestIconTapButtonFlash(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	var b *iconTapButton
	b = newIconTapButton(nil, theme.ContentCopyIcon(), 17, 34, func() {
		b.flashIcon(theme.ConfirmIcon(), time.Hour) // restore far off; tested directly below
	})
	test.WidgetRenderer(b) // realize the glyph image, as attaching to a canvas would

	if b.img == nil {
		t.Fatal("renderer did not capture the glyph image")
	}
	base := b.img.Resource.Name()

	b.Tapped(nil)
	if flashed := b.img.Resource.Name(); flashed == base {
		t.Fatalf("tap must swap the glyph; still %q", flashed)
	}

	// Restore path (what the timer's fyne.Do runs).
	b.setIcon(b.baseIcon())
	if got := b.img.Resource.Name(); got != base {
		t.Fatalf("restore must bring back the original glyph; got %q, want %q", got, base)
	}

	// A rapid second tap bumps the generation so the FIRST flash's restore
	// (now stale) must not undo the newer flash early.
	b.Tapped(nil)
	gen1 := b.flashGen
	b.Tapped(nil)
	if b.flashGen <= gen1 {
		t.Fatal("a newer flash must supersede the older one's restore")
	}
}
