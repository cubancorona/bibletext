package bibletext

// THE READING FACE IS SHIPPED, NOT BORROWED.
//
// The app used to ask the operating system for a serif and take the first one
// it found: real Georgia on macOS and Windows, whatever the distribution
// installed on Linux, and the embedded Gelasio when neither answered. So the
// same chapter was set in a different type depending on where it was read, and
// nothing had chosen any of it — the fallbacks were simply what was left.
// Georgia could not be shipped to close the gap because it is a licensed
// system font.
//
// Spectral is shipped instead, under the SIL Open Font License, and every
// surface draws the same face. It was chosen by measurement rather than
// taste: summing advance widths over a line of John 3:16 puts it within 0.3%
// of Georgia's line, so a page wraps where a page of Georgia wrapped, and it
// is the only open face measured that carries real small-capital glyphs in
// both weights — which is what lets the divine name be set as the publisher
// sets it instead of uppercased into the stored text.
//
// Two of the four faces cost nothing: the share cards already embed the
// regular and the bold, and Go stores one copy of a file however many times it
// is embedded. Only the italics are new bytes, and the italic is not optional
// now that the translators' supplied words are captured and waiting to be
// drawn.

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed assets/fonts/share/Spectral-Italic.ttf
var readingFontItalic []byte

//go:embed assets/fonts/share/Spectral-BoldItalic.ttf
var readingFontBoldItalic []byte

// loadReadingFonts returns the embedded Spectral family the reading surfaces
// set scripture in. Never nil in a real build: the bytes are compiled in, so
// there is no file to be missing and no platform that can answer differently.
func loadReadingFonts() *bookFonts {
	res := func(name string, b []byte) fyne.Resource {
		if len(b) == 0 {
			return nil
		}
		return fyne.NewStaticResource(name, b)
	}
	reg := res("Spectral-Regular.ttf", shareFontSpectral)
	if reg == nil {
		return nil
	}
	return &bookFonts{
		regular:    reg,
		bold:       res("Spectral-Bold.ttf", shareFontSpectralBold),
		italic:     res("Spectral-Italic.ttf", readingFontItalic),
		boldItalic: res("Spectral-BoldItalic.ttf", readingFontBoldItalic),
	}
}
