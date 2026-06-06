package bibletext

import (
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
)

// bookFonts holds a book-like serif family (and its variants) used for a warmer,
// more page-like reading experience. It is loaded from the OS at startup; when no
// serif is found (e.g. non-macOS) the theme falls back to Fyne's bundled font.
type bookFonts struct {
	regular    fyne.Resource
	bold       fyne.Resource
	italic     fyne.Resource
	boldItalic fyne.Resource
}

// serifFontCandidates lists full variant sets to try, best first. The regular
// face must load for the set to be used; missing variants fall back to regular.
var serifFontCandidates = [][4]string{
	{ // Georgia — a screen-optimised book serif (macOS)
		"/System/Library/Fonts/Supplemental/Georgia.ttf",
		"/System/Library/Fonts/Supplemental/Georgia Bold.ttf",
		"/System/Library/Fonts/Supplemental/Georgia Italic.ttf",
		"/System/Library/Fonts/Supplemental/Georgia Bold Italic.ttf",
	},
	{ // Georgia (Windows)
		`C:\Windows\Fonts\georgia.ttf`,
		`C:\Windows\Fonts\georgiab.ttf`,
		`C:\Windows\Fonts\georgiai.ttf`,
		`C:\Windows\Fonts\georgiaz.ttf`,
	},
	{ // common Linux serif
		"/usr/share/fonts/truetype/dejavu/DejaVuSerif.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSerif-Bold.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSerif-Italic.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSerif-BoldItalic.ttf",
	},
}

// loadBookFonts returns a serif family if one can be loaded, else nil.
func loadBookFonts() *bookFonts {
	for _, set := range serifFontCandidates {
		regular := loadFontFile(set[0])
		if regular == nil {
			continue
		}
		return &bookFonts{
			regular:    regular,
			bold:       loadFontFile(set[1]),
			italic:     loadFontFile(set[2]),
			boldItalic: loadFontFile(set[3]),
		}
	}
	return nil
}

func loadFontFile(path string) fyne.Resource {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return fyne.NewStaticResource(filepath.Base(path), data)
}

// face returns the variant for a text style, falling back to regular.
func (f *bookFonts) face(style fyne.TextStyle) fyne.Resource {
	switch {
	case style.Bold && style.Italic && f.boldItalic != nil:
		return f.boldItalic
	case style.Bold && f.bold != nil:
		return f.bold
	case style.Italic && f.italic != nil:
		return f.italic
	}
	return f.regular
}
