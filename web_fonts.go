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

//go:embed assets/fonts/reading/web/Junicode-BoldSmallCaps.woff2
var webScriptureFontBoldSmallCaps []byte

//go:embed assets/fonts/reading/web/Junicode-ItalicSmallCaps.woff2
var webScriptureFontItalicSmallCaps []byte

//go:embed assets/fonts/reading/web/BibleTextHebrew.woff2
var webHebrewFont []byte

//go:embed assets/fonts/reading/web/Junicode-Greek.woff2
var webScriptureFontGreek []byte

// webGreekRunes are the Greek code points the site's pages draw, and exactly
// what the web Greek supplement (WebScriptureFontGreek) carries: the Greek of
// the WEB's and WEB Catholic's notes, basic and polytonic, measured over every
// page on 2 October 2026. The app sets it in Junicode, which carries the whole
// script in every cut; the web regular subset carries none of it, so a note
// drew basic Greek from one system face and polytonic from another, inside one
// word. A page that comes to draw any other Greek needs it added here and the
// supplement rebuilt (scripts/build-web-nkjv-fonts.sh reads this list).
var webGreekRunes = []rune{
	'\u039C', '\u03AC', '\u03AD', '\u03AF', '\u03B1', '\u03B2', '\u03B3', '\u03B4',
	'\u03B5', '\u03B7', '\u03B8', '\u03B9', '\u03BA', '\u03BB', '\u03BC', '\u03BD',
	'\u03BF', '\u03C0', '\u03C1', '\u03C2', '\u03C3', '\u03C4', '\u03C5', '\u03C7',
	'\u03C9',
	// Greek Extended: the breathings, accents and iota subscripts.
	'\u1F04', '\u1F10', '\u1F30', '\u1F7A', '\u1FB3', '\u1FC6', '\u1FE5', '\u1FE6',
}

//go:embed assets/fonts/reading/EzraSIL-Licenses.txt
var webHebrewFontLicense []byte

// webHebrewRunes are the Hebrew code points the site's pages draw, and exactly
// what the web Hebrew face (WebHebrewFont) carries. Measured over every page of
// all four editions on 2 October 2026: the twenty-two letters of Psalm 119's
// stanza headings in the NKJV, and the letters, vowels and cantillation marks
// of the WEB's and WEB Catholic's notes on the divine name. A page that comes
// to draw any other Hebrew needs it added here and the face rebuilt
// (scripts/build-web-nkjv-fonts.sh reads this list).
var webHebrewRunes = []rune{
	// Marks: etnahta, tevir; hataf segol, hiriq, tsere, holam, dagesh.
	'\u0591', '\u059B', '\u05B1', '\u05B4', '\u05B5', '\u05B9', '\u05BC',
	// Letters: alef to tav without the final forms, and final mem.
	'\u05D0', '\u05D1', '\u05D2', '\u05D3', '\u05D4', '\u05D5', '\u05D6', '\u05D7',
	'\u05D8', '\u05D9', '\u05DB', '\u05DC', '\u05DD', '\u05DE', '\u05E0', '\u05E1',
	'\u05E2', '\u05E4', '\u05E6', '\u05E7', '\u05E8', '\u05E9', '\u05EA',
}

// WebScriptureFontRegular is the subsetted reading face (WOFF2). Built from the
// SAME file the app embeds, so the site and the app can never drift to
// different releases of it. Narrower than the app's subset: none of the
// public-domain editions marks a divine name, so it carries no small capitals
// (the NKJV's pages add them from WebScriptureFontSmallCaps), and the Greek and
// Hebrew a page's notes draw come from supplements declared beside it
// (WebScriptureFontGreek, WebHebrewFont), so a page without them downloads
// neither.
func WebScriptureFontRegular() []byte { return webScriptureFontRegular }

// WebScriptureFontSmallCaps is the reading face's Unicode small capitals and
// nothing else (WOFF2, scripts/build-web-nkjv-fonts.sh): the letters the app
// draws the NKJV's divine name with (smallCapitals). Only the site's NKJV pages
// load it, through a unicode-range declared after the regular face, so a page
// that draws no small capital never downloads it.
func WebScriptureFontSmallCaps() []byte { return webScriptureFontSmallCaps }

// WebScriptureFontItalic is the reading face's italic over the web subset's
// ranges (WOFF2, scripts/build-web-nkjv-fonts.sh), for the psalm titles of
// every edition and the words the NKJV's translators supplied. The app sets
// both in this true italic; the site used to give the public-domain titles a
// slanted regular. A page with no italic never downloads it.
func WebScriptureFontItalic() []byte { return webScriptureFontItalic }

// WebScriptureFontBoldSmallCaps is the bold cut's Unicode small capitals and
// nothing else (WOFF2, scripts/build-web-nkjv-fonts.sh), for the divine name in
// the NKJV's section headings, which are set bold. The bold web subset carries
// none, and a bold heading would otherwise draw them from a system face.
func WebScriptureFontBoldSmallCaps() []byte { return webScriptureFontBoldSmallCaps }

// WebScriptureFontItalicSmallCaps is the italic cut's Unicode small capitals
// and nothing else (WOFF2), for the divine name inside the words the NKJV's
// translators supplied and in its psalm titles, both set in italic. A browser
// matches an italic run against italic faces only, so the upright supplement
// is never consulted there.
func WebScriptureFontItalicSmallCaps() []byte { return webScriptureFontItalicSmallCaps }

// WebHebrewFont is the Hebrew face of the site's pages (WOFF2), for Psalm 119's
// stanza letters in the NKJV and the WEB's notes on the divine name: the app's
// Ezra SIL (assets/fonts/reading/EzraSIL-Regular.ttf, the face every app pane
// draws Hebrew in) subsetted to WebHebrewRunes. "Ezra" and "SIL" are Reserved
// Font Names, so the subset is renamed "BibleText Hebrew"; its copyright and
// licence strings are kept, and WebHebrewFontLicense is published beside it.
func WebHebrewFont() []byte { return webHebrewFont }

// WebHebrewFontLicense is the Hebrew face's licence (the SIL Open Font License
// and the MIT licence of its layout tables), published beside the face.
func WebHebrewFontLicense() []byte { return webHebrewFontLicense }

// WebHebrewRunes is what WebHebrewFont carries, sorted: the unicode-range the
// stylesheet declares it with.
func WebHebrewRunes() []rune {
	return append([]rune(nil), webHebrewRunes...)
}

// WebScriptureFontGreek is the reading face's regular cut over WebGreekRunes
// and nothing else (WOFF2, scripts/build-web-nkjv-fonts.sh): the Greek of the
// notes, in the face the app sets it in.
func WebScriptureFontGreek() []byte { return webScriptureFontGreek }

// WebGreekRunes is what WebScriptureFontGreek carries, sorted: the
// unicode-range the stylesheet declares it with.
func WebGreekRunes() []rune {
	return append([]rune(nil), webGreekRunes...)
}

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
