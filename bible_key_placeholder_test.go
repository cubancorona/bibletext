package bibletext

// THE EMPTY KEY BOX DESCRIBES THE STATE IT IS EMPTY IN.
//
// The bundled API.Bible key's characters are deliberately withheld, so the
// field sits empty while a key is working. Reading "Paste your API.Bible key"
// there says the opposite of the truth, and the correction was below the box

//
// Two things are pinned: the wording per state, and that it FITS. The fuller
// sentence that names both the state and the action was tried once and ran off
// the end of the field, so the width is not a detail to be re-litigated by eye.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestBibleKeyPlaceholderFollowsTheState(t *testing.T) {
	if got, want := bibleKeyPlaceholder(true), "Included with BibleText"; got != want {
		t.Errorf("with the bundled key in force the box says %q, want %q — an empty "+
			"box must not claim there is no key when one is working", got, want)
	}
	if got, want := bibleKeyPlaceholder(false), "Paste your API.Bible key"; got != want {
		t.Errorf("with no bundled key the box says %q, want %q", got, want)
	}
}

func TestBibleKeyPlaceholderFitsTheNarrowestField(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(&bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()})

	// The budget recorded in bibleKeySection: a 320pt phone leaves the box about
	// 199pt of usable width, and the app draws body text at 18 — theme.TextSize()
	// is the stock 14 and flatters every string by 29%.
	const narrowest = float32(199)
	for _, usingBundled := range []bool{true, false} {
		s := bibleKeyPlaceholder(usingBundled)
		if w := fyne.MeasureText(s, 18, fyne.TextStyle{}).Width; w > narrowest {
			t.Errorf("placeholder %q measures %.0fpt at 18, over the %.0fpt a 320pt "+
				"phone gives the field — it will run off the end", s, w, narrowest)
		}
	}
}
