package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

// Custom theme color names. ThemeColorName is just a string, so we can register
// our own and resolve them through bibleTheme.Color. RichText segments reference
// these names to stay in sync with light/dark mode automatically.
const (
	colorNameVerseNumber fyne.ThemeColorName = "holyVerseNumber"
	colorNameVerseText   fyne.ThemeColorName = "holyVerseText"
	colorNameHighlight   fyne.ThemeColorName = "holyHighlight"
	colorNameHighlightHi fyne.ThemeColorName = "holyHighlightText"
	colorNameMuted       fyne.ThemeColorName = "holyMuted"
)

// palette is the single source of truth for every colour in the UI. Routing all
// colours through here keeps the design consistent and makes dark mode a swap.
type palette struct {
	Background    color.NRGBA // window backdrop
	Surface       color.NRGBA // reading paper / cards
	SurfaceAlt    color.NRGBA // sidebar, header, chips
	Border        color.NRGBA
	Text          color.NRGBA
	TextMuted     color.NRGBA
	Accent        color.NRGBA // primary / interactive
	AccentText    color.NRGBA // text drawn on Accent
	Highlight     color.NRGBA // faint wash behind the highlighted verse
	HighlightText color.NRGBA // the highlighted verse's own text
	VerseNumber   color.NRGBA // superscript verse numbers
	Input         color.NRGBA
}

// Light: warm parchment ground so the crisp near-white "page" appears to glow.
var lightPalette = palette{
	Background:    color.NRGBA{R: 237, G: 233, B: 224, A: 255},
	Surface:       color.NRGBA{R: 253, G: 252, B: 248, A: 255},
	SurfaceAlt:    color.NRGBA{R: 244, G: 240, B: 232, A: 255},
	Border:        color.NRGBA{R: 224, G: 217, B: 205, A: 255},
	Text:          color.NRGBA{R: 37, G: 34, B: 29, A: 255},
	TextMuted:     color.NRGBA{R: 132, G: 124, B: 111, A: 255},
	Accent:        color.NRGBA{R: 146, G: 107, B: 51, A: 255},
	AccentText:    color.NRGBA{R: 253, G: 251, B: 246, A: 255},
	Highlight:     color.NRGBA{R: 249, G: 238, B: 206, A: 255},
	HighlightText: color.NRGBA{R: 116, G: 80, B: 28, A: 255},
	VerseNumber:   color.NRGBA{R: 162, G: 122, B: 64, A: 255},
	Input:         color.NRGBA{R: 252, G: 251, B: 247, A: 255},
}

// Dark: warm near-black with a soft gold accent — illuminated, not stark.
var darkPalette = palette{
	Background:    color.NRGBA{R: 25, G: 23, B: 21, A: 255},
	Surface:       color.NRGBA{R: 34, G: 31, B: 28, A: 255},
	SurfaceAlt:    color.NRGBA{R: 42, G: 38, B: 34, A: 255},
	Border:        color.NRGBA{R: 57, G: 52, B: 46, A: 255},
	Text:          color.NRGBA{R: 233, G: 227, B: 217, A: 255},
	TextMuted:     color.NRGBA{R: 157, G: 148, B: 135, A: 255},
	Accent:        color.NRGBA{R: 215, G: 179, B: 119, A: 255},
	AccentText:    color.NRGBA{R: 26, G: 22, B: 18, A: 255},
	Highlight:     color.NRGBA{R: 61, G: 51, B: 35, A: 255},
	HighlightText: color.NRGBA{R: 240, G: 214, B: 162, A: 255},
	VerseNumber:   color.NRGBA{R: 207, G: 171, B: 113, A: 255},
	Input:         color.NRGBA{R: 38, G: 35, B: 31, A: 255},
}

// bibleTheme is a Fyne theme whose colours come from the active palette. The dark
// flag is an explicit user choice, so we ignore the OS variant Fyne passes in.
type bibleTheme struct {
	dark  bool
	fonts *bookFonts // book-like serif; nil falls back to Fyne's bundled font
}

func (t *bibleTheme) palette() palette {
	if t.dark {
		return darkPalette
	}
	return lightPalette
}

func (t *bibleTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	p := t.palette()
	switch name {
	case theme.ColorNameBackground:
		return p.Background
	case theme.ColorNameHeaderBackground:
		return p.SurfaceAlt
	case theme.ColorNameForeground:
		return p.Text
	case theme.ColorNamePrimary:
		return p.Accent
	case theme.ColorNameButton:
		return p.SurfaceAlt
	case theme.ColorNameInputBackground:
		return p.Input
	case theme.ColorNameInputBorder:
		return p.Border
	case theme.ColorNamePlaceHolder, colorNameMuted:
		return p.TextMuted
	case theme.ColorNameSeparator:
		return p.Border
	case theme.ColorNameScrollBar:
		return withAlpha(p.TextMuted, 120)
	case theme.ColorNameHover:
		return withAlpha(p.Accent, 28)
	case theme.ColorNamePressed:
		return withAlpha(p.Accent, 48)
	case theme.ColorNameSelection:
		return withAlpha(p.Accent, 40)
	case theme.ColorNameShadow:
		if t.dark {
			return color.NRGBA{A: 90}
		}
		return color.NRGBA{A: 24}
	case colorNameVerseNumber:
		return p.VerseNumber
	case colorNameVerseText:
		return p.Text
	case colorNameHighlight:
		return p.Highlight
	case colorNameHighlightHi:
		return p.HighlightText
	}

	// Fall back to Fyne's defaults, but force the variant to match our mode so
	// any colour we don't override still looks right in dark mode.
	v := theme.VariantLight
	if t.dark {
		v = theme.VariantDark
	}
	return theme.DefaultTheme().Color(name, v)
}

func (t *bibleTheme) Font(style fyne.TextStyle) fyne.Resource {
	// Keep monospace/symbol text on the default faces; everything else uses the
	// book-like serif when available for a warmer, more page-like feel.
	if t.fonts != nil && !style.Monospace && !style.Symbol {
		return t.fonts.face(style)
	}
	return theme.DefaultTheme().Font(style)
}

func (t *bibleTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *bibleTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText: // body text (reading + UI); Entry forces one size
		return 16
	case theme.SizeNameInputBorder:
		// The read-only reading text is an Entry; its blinking caret is drawn at
		// this width. Zero removes the caret. Entry outlines are supplied by our
		// own bordered surfaces instead (see the search/filter fields).
		return 0
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameHeadingText:
		return 23
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNamePadding:
		return 7
	case theme.SizeNameInnerPadding:
		return 7
	case theme.SizeNameLineSpacing:
		return 6
	}
	return theme.DefaultTheme().Size(name)
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

// surface draws content on a bordered, padded card in the given colours. It is
// the building block for the sidebar, reading paper, history bar and popups.
func surface(content fyne.CanvasObject, bg, border color.Color, minSize fyne.Size) fyne.CanvasObject {
	frame := canvas.NewRectangle(bg)
	frame.StrokeColor = border
	frame.StrokeWidth = 1
	frame.CornerRadius = 8
	if minSize.Width > 0 || minSize.Height > 0 {
		frame.SetMinSize(minSize)
	}
	return container.NewStack(frame, container.NewPadded(content))
}

// inputFrame draws a thin rounded outline around an input field without adding
// padding. We zero the theme's input-border size (to hide the read-only reading
// caret), so fields get their outline here instead. The rectangle is
// non-interactive, so clicks still reach the entry beneath it.
func inputFrame(content fyne.CanvasObject, border color.Color) fyne.CanvasObject {
	outline := canvas.NewRectangle(color.Transparent)
	outline.StrokeColor = border
	outline.StrokeWidth = 1
	outline.CornerRadius = 6
	return container.NewStack(content, outline)
}
