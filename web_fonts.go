package bibletext

// The web reader's copy of the app's CHROME typeface.
//
// The app draws its UI in Atkinson Hyperlegible (fonts_embed.go, theme.Font)
// and its scripture in Georgia — two faces, deliberately distinct. The web
// reader gets Georgia for free (it is a system font on the phones and desktops
// that open a shared link), but Atkinson is not installed anywhere, so matching
// the app's chrome means shipping it.
//
// These are WOFF2 subsets — Latin plus the punctuation the chrome actually uses
// — built from the same TTFs the app embeds, so the two can never drift to
// different releases of the face. ~15 KB each, and the stylesheet loads them
// with font-display:swap, so the page still paints immediately in the fallback
// sans on a phone with a poor connection. Only Regular and Bold: the chrome has
// no italics.
//
// Licence: SIL Open Font License 1.1. WebUIFontLicense is published beside the
// fonts because the OFL requires the licence to travel with them.

import _ "embed"

//go:embed assets/fonts/atkinson/web/AtkinsonHyperlegible-Regular.woff2
var webUIFontRegular []byte

//go:embed assets/fonts/atkinson/web/AtkinsonHyperlegible-Bold.woff2
var webUIFontBold []byte

//go:embed assets/fonts/atkinson/OFL.txt
var webUIFontLicense []byte

//go:embed assets/fonts/reading/web/Junicode-Regular.woff2
var webScriptureFontRegular []byte

//go:embed assets/fonts/reading/web/Junicode-Bold.woff2
var webScriptureFontBold []byte

//go:embed assets/fonts/reading/Junicode-OFL.txt
var webScriptureFontLicense []byte

//go:embed assets/fonts/reading/web/Junicode-SmallCaps.woff2
var webScriptureFontSmallCaps []byte

//go:embed assets/fonts/reading/web/Junicode-Italic.woff2
var webScriptureFontItalic []byte

// WebScriptureFontRegular is the subsetted reading face (WOFF2). Built from the
// SAME file the app embeds, so the site and the app can never drift to
// different releases of it. Narrower than the app's subset: none of the
// public-domain editions marks a divine name, so it carries no small capitals
// (the NKJV's pages add them from WebScriptureFontSmallCaps), and its note
// chrome is set in the UI face, so its scripture face never draws Greek or
// Hebrew.
func WebScriptureFontRegular() []byte { return webScriptureFontRegular }

// WebScriptureFontSmallCaps is the reading face's Unicode small capitals and
// nothing else (WOFF2, scripts/build-web-nkjv-fonts.sh): the letters the app
// draws the NKJV's divine name with (smallCapitals). Only the site's NKJV pages
// load it, through a unicode-range declared after the regular face, so a page
// that draws no small capital never downloads it.
func WebScriptureFontSmallCaps() []byte { return webScriptureFontSmallCaps }

// WebScriptureFontItalic is the reading face's italic over the web subset's
// ranges (WOFF2, scripts/build-web-nkjv-fonts.sh), for the words the NKJV's
// translators supplied and its psalm titles. Only the site's NKJV pages load
// it; the public-domain pages keep the synthesised slant they have always had.
func WebScriptureFontItalic() []byte { return webScriptureFontItalic }

// WebScriptureFontBold is the subsetted reading face, bold. Required, and not
// obviously so: the only bold inside the reading column is the verse number,
// and with a webfont a weight of 600 resolves to the 700 face — so shipping
// regular alone would leave every verse number synthesised.
func WebScriptureFontBold() []byte { return webScriptureFontBold }

// WebScriptureFontLicense is the reading face's licence. The OFL requires it to
// travel with the font, which is why the site publishes it beside the file.
func WebScriptureFontLicense() []byte { return webScriptureFontLicense }

// WebUIFontRegular is the subsetted Atkinson Hyperlegible regular face (WOFF2).
func WebUIFontRegular() []byte { return webUIFontRegular }

// WebUIFontBold is the subsetted Atkinson Hyperlegible bold face (WOFF2).
func WebUIFontBold() []byte { return webUIFontBold }

// WebUIFontLicense is the SIL Open Font License text that must be published
// alongside the faces above.
func WebUIFontLicense() []byte { return webUIFontLicense }
