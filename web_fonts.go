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

// WebUIFontRegular is the subsetted Atkinson Hyperlegible regular face (WOFF2).
func WebUIFontRegular() []byte { return webUIFontRegular }

// WebUIFontBold is the subsetted Atkinson Hyperlegible bold face (WOFF2).
func WebUIFontBold() []byte { return webUIFontBold }

// WebUIFontLicense is the SIL Open Font License text that must be published
// alongside the faces above.
func WebUIFontLicense() []byte { return webUIFontLicense }
