package bibletext

import "fyne.io/fyne/v2"

// bookFonts holds one family and its cuts. Both families the app draws are
// EMBEDDED — Atkinson Hyperlegible for chrome (fonts_embed.go) and Spectral
// for scripture (reading_fonts_embed.go) — so this no longer reads anything
// from the operating system. It used to: the scripture face was whichever
// serif was found at a hard-coded path, which is why the same chapter was set
// in Georgia, DejaVu Serif or Gelasio depending on where it was read.
type bookFonts struct {
	regular    fyne.Resource
	bold       fyne.Resource
	italic     fyne.Resource
	boldItalic fyne.Resource
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
