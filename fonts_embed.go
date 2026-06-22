package bibletext

// Embedded UI font. Atkinson Hyperlegible (by the Braille Institute, OFL) is the
// app's UI / chrome typeface — settings, headers, search, pickers, buttons. It is
// a legibility-first humanist sans, deliberately distinct from the serif used for
// scripture (the reading text is a native overlay on iOS/macOS, and the bundled
// serif on the Fyne-fallback platforms / share images — see font.go, theme.Font).
//
// It is EMBEDDED in the binary (not read from the OS) so it ships identically on
// every platform — notably iOS, whose sandbox can't read /System/Library/Fonts.
// Licence: assets/fonts/atkinson/OFL.txt (SIL Open Font License 1.1).

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed assets/fonts/atkinson/AtkinsonHyperlegible-Regular.ttf
var atkinsonRegular []byte

//go:embed assets/fonts/atkinson/AtkinsonHyperlegible-Bold.ttf
var atkinsonBold []byte

//go:embed assets/fonts/atkinson/AtkinsonHyperlegible-Italic.ttf
var atkinsonItalic []byte

//go:embed assets/fonts/atkinson/AtkinsonHyperlegible-BoldItalic.ttf
var atkinsonBoldItalic []byte

// loadUIFonts returns the embedded Atkinson Hyperlegible family for UI chrome, or
// nil if the regular face is somehow missing (then theme.Font falls back to the
// serif, then Fyne's default). Set theme.uiFonts to the result; setting it to nil
// is the one-line switch back to the previous chrome font.
func loadUIFonts() *bookFonts {
	res := func(name string, b []byte) fyne.Resource {
		if len(b) == 0 {
			return nil
		}
		return fyne.NewStaticResource(name, b)
	}
	reg := res("AtkinsonHyperlegible-Regular.ttf", atkinsonRegular)
	if reg == nil {
		return nil
	}
	return &bookFonts{
		regular:    reg,
		bold:       res("AtkinsonHyperlegible-Bold.ttf", atkinsonBold),
		italic:     res("AtkinsonHyperlegible-Italic.ttf", atkinsonItalic),
		boldItalic: res("AtkinsonHyperlegible-BoldItalic.ttf", atkinsonBoldItalic),
	}
}
