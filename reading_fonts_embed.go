package bibletext

// THE READING FACES ARE SHIPPED, NOT BORROWED — AND THERE ARE TWO OF THEM.
//
// The app used to ask the operating system for a serif and take the first it
// found, so the same chapter was set in a different type depending on where it
// was read, and nothing had chosen any of it. Georgia could not be shipped to
// close the gap because it is a licensed system font.
//
// WHY TWO. The app draws three scripts: English, the polytonic Greek of the
// footnotes, and pointed Hebrew. Sixty-six faces were measured against what it
// actually needs and no single one passes. The faces that cover every script
// have three cuts, or put their small capitals in the regular alone, or reuse
// one upright regular-weight Greek in every cut — so Greek inside the
// translators' italicised words would render upright. The faces with four
// properly featured cuts have no Hebrew at all.
//
//	Junicode   Latin and polytonic Greek, four real cuts. SIL Open Font
//	           License, declaring no Reserved Font Name, so the subset in
//	           assets/fonts/reading keeps the family's own name.
//	Ezra SIL   pointed Hebrew as the BHS sets it, every mark in mark coverage.
//	           Shipped UNMODIFIED: "Ezra" and "SIL" are Reserved Font Names, and
//	           at 151 KB a subset is not worth the renaming obligation.
//
// WHAT JUNICODE WINS ON, and it is not a typographic argument. It carries the
// UNICODE SMALL CAPITALS — real codepoints, not an OpenType feature — in all
// four cuts. That lets the divine name be SET in small capitals on every
// surface with nothing but characters: no stylesheet, no sweep over an imported
// attributed string, no per-span paint setting, and no patched toolkit on the
// two platforms whose text stack exposes no OpenType control at all.
//
// Rebuild both with scripts/build-reading-fonts.sh, which checks that the
// subset kept the small capitals, the Greek Extended and the superior figures.
// The small capitals are scattered across three Unicode blocks and a subset
// that misses one loses letters from the divine name silently.

import (
	_ "embed"
	"sync"

	"fyne.io/fyne/v2"
)

//go:embed assets/fonts/reading/Junicode-Regular.ttf
var readingFontRegular []byte

//go:embed assets/fonts/reading/Junicode-Italic.ttf
var readingFontItalic []byte

//go:embed assets/fonts/reading/Junicode-Bold.ttf
var readingFontBold []byte

//go:embed assets/fonts/reading/Junicode-BoldItalic.ttf
var readingFontBoldItalic []byte

//go:embed assets/fonts/reading/EzraSIL-Regular.ttf
var readingFontHebrew []byte

// loadReadingFonts returns the family that sets Latin and Greek. Never nil in a
// real build: the bytes are compiled in, so there is no file to be missing and
// no platform that can answer differently.
func loadReadingFonts() *bookFonts {
	res := func(name string, b []byte) fyne.Resource {
		if len(b) == 0 {
			return nil
		}
		return fyne.NewStaticResource(name, b)
	}
	reg := res("Junicode-Regular.ttf", readingFontRegular)
	if reg == nil {
		return nil
	}
	return &bookFonts{
		regular:    reg,
		bold:       res("Junicode-Bold.ttf", readingFontBold),
		italic:     res("Junicode-Italic.ttf", readingFontItalic),
		boldItalic: res("Junicode-BoldItalic.ttf", readingFontBoldItalic),
	}
}

// hebrewFontOnce guards the Hebrew face for the same reason the Latin one is
// guarded: relayout runs continuously during a window drag-resize, and a fresh
// resource pointer each time would defeat the toolkit's own font cache, which
// is keyed on the resource.
var (
	hebrewFontOnce   sync.Once
	hebrewFontCached fyne.Resource
)

// hebrewReadingFont returns the face that draws pointed Hebrew.
//
// Without it a Hebrew run falls through to whatever the platform supplies,
// which differs on every platform and is exactly the borrowing this change
// exists to end. The Latin face has no Hebrew at all, so this is not a
// refinement: it is the difference between the app choosing the type its
// readers see and the operating system choosing it.
func hebrewReadingFont() fyne.Resource {
	hebrewFontOnce.Do(func() {
		if len(readingFontHebrew) > 0 {
			hebrewFontCached = fyne.NewStaticResource("EzraSIL-Regular.ttf", readingFontHebrew)
		}
	})
	return hebrewFontCached
}
