package main

// The NKJV pages' own stylesheet — a THIRD hashed asset, loaded only by the
// pages under /nkjv/ and only while the NKJV's text is published
// (nkjv_text.go), after reader.css and, on a canon-gap page, notice.css.
//
// It is separate for the reason notice.css is (notice_assets.go): reader.css is
// content-hashed into every page of the public-domain editions, so a rule added
// there rewrites all of them, and with the switch off the site must not move by
// one byte. Everything the NKJV pages need beyond the reader is here:
//
//   - the reading face's Unicode small capitals, which the app draws the divine
//     name with and the web subsets leave out — in the regular cut for a verse,
//     the bold for a section heading ("The LORD Is My Shepherd") and the italic
//     for the words the translators supplied and the psalm titles. A browser
//     matches a run against faces of its own weight and style and no other, so
//     each cut needs its own supplement. Each is declared with a unicode-range
//     AFTER the face it supplements, so for exactly those letters the browser
//     takes it first and a page without one never downloads it.
//   - the reading face's italic, for the supplied words and the psalm titles,
//     which would otherwise be a slanted regular.
//   - the app's Hebrew face, for the letters of Psalm 119's stanza headings,
//     which the app draws in Ezra SIL and the web subset does not carry. It
//     joins the "Junicode" family through a unicode-range of its own code
//     points, declared at the regular weight AND the bold — a heading is bold,
//     and a face must match a run's weight exactly to join its composite —
//     which also keeps the browser from thickening it: on the Apple panes the
//     bold run cascades to the one Hebrew cut as it is, at the same size.
//   - the footer's copyright notice and retrieval line, in the footer's small,
//     muted type, which follows the light and dark palettes through --muted.
//   - the footer in print. reader.css hides it on paper; a printed page of the
//     NKJV must still carry its notice, so here only the app link and the
//     platform row are hidden.

import (
	"fmt"
	"strings"

	bibletext "github.com/cubancorona/bibletext"
)

// nkjvCSSName is the content-hashed path for this build, set in writeSite
// before any page is rendered and "" when no licensed text is loaded.
var nkjvCSSName string

// nkjvFonts names the hashed face files nkjv.css loads. A struct for the reason
// webFonts is one: five filenames in a row are five chances to swap two.
type nkjvFonts struct {
	smallCaps       string
	boldSmallCaps   string
	italicSmallCaps string
	italic          string
	hebrew          string
}

// unicodeRange writes code points as a CSS unicode-range.
func unicodeRange(runes []rune) string {
	points := make([]string, 0, len(runes))
	for _, r := range runes {
		points = append(points, fmt.Sprintf("U+%04X", r))
	}
	return strings.Join(points, ",")
}

// nkjvCSS fills the template with the hashed faces and their code points, read
// from the app's own tables.
func nkjvCSS(f nkjvFonts) string {
	return strings.NewReplacer(
		"__SCRIPTURE_SMALLCAPS__", f.smallCaps,
		"__SCRIPTURE_BOLD_SMALLCAPS__", f.boldSmallCaps,
		"__SCRIPTURE_ITALIC_SMALLCAPS__", f.italicSmallCaps,
		"__SCRIPTURE_ITALIC__", f.italic,
		"__SCRIPTURE_HEBREW__", f.hebrew,
		"__SMALLCAPS_RANGE__", unicodeRange(bibletext.WebSmallCapitalRunes()),
		"__HEBREW_RANGE__", unicodeRange(bibletext.WebHebrewRunes()),
	).Replace(nkjvCSSTemplate)
}

const nkjvCSSTemplate = `
/* Junicode (c) Peter S. Baker — SIL Open Font License 1.1, published beside
   these files as assets/junicode-OFL.txt. The small capitals the divine name
   is drawn with, and nothing else, in the regular, bold and italic cuts; the
   italic for supplied words and titles. */
@font-face{
  font-family:"Junicode"; font-style:normal; font-weight:400;
  font-display:swap; src:url(__SCRIPTURE_SMALLCAPS__) format("woff2");
  unicode-range:__SMALLCAPS_RANGE__;
}
@font-face{
  font-family:"Junicode"; font-style:normal; font-weight:700;
  font-display:swap; src:url(__SCRIPTURE_BOLD_SMALLCAPS__) format("woff2");
  unicode-range:__SMALLCAPS_RANGE__;
}
@font-face{
  font-family:"Junicode"; font-style:italic; font-weight:400;
  font-display:swap; src:url(__SCRIPTURE_ITALIC__) format("woff2");
}
@font-face{
  font-family:"Junicode"; font-style:italic; font-weight:400;
  font-display:swap; src:url(__SCRIPTURE_ITALIC_SMALLCAPS__) format("woff2");
  unicode-range:__SMALLCAPS_RANGE__;
}
/* BibleText Hebrew: Ezra SIL (c) SIL International, subsetted and renamed,
   because "Ezra" and "SIL" are Reserved Font Names — SIL Open Font License
   1.1, published beside these files as assets/hebrew-OFL.txt. The face the
   app draws Hebrew in, at the regular weight and the bold, unthickened. */
@font-face{
  font-family:"Junicode"; font-style:normal; font-weight:400;
  font-display:swap; src:url(__SCRIPTURE_HEBREW__) format("woff2");
  unicode-range:__HEBREW_RANGE__;
}
@font-face{
  font-family:"Junicode"; font-style:normal; font-weight:700;
  font-display:swap; src:url(__SCRIPTURE_HEBREW__) format("woff2");
  unicode-range:__HEBREW_RANGE__;
}
/* The rights holder's notice and when this text was retrieved, under the app
   link and the platform row at the very foot of the page: small and muted, in
   the footer's own colour. The link is underlined, because in a sentence it
   would otherwise be invisible. */
.foot .lic,.foot .retrieved{max-width:34rem; margin:.7rem auto 0; color:var(--muted);
  font-size:.72rem; line-height:1.5}
.foot .lic + .retrieved{margin-top:.2rem}
.foot .retrieved a{font-size:inherit; text-decoration:underline}
@media print{.foot{display:block} .foot #getapp,.foot .plats{display:none}}
`
