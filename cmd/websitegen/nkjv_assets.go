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
//     name with and the web subset leaves out. Declared with a unicode-range
//     AFTER reader.css's regular face, so for exactly those letters the browser
//     takes this face first and a page without one never downloads it.
//   - the reading face's italic, for the words the translators supplied and the
//     psalm titles, which would otherwise be a slanted regular.
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

// nkjvCSS fills the template with the two hashed faces and the small capitals'
// code points, read from the app's own table.
func nkjvCSS(smallCapsFile, italicFile string) string {
	runes := bibletext.WebSmallCapitalRunes()
	points := make([]string, 0, len(runes))
	for _, r := range runes {
		points = append(points, fmt.Sprintf("U+%04X", r))
	}
	return strings.NewReplacer(
		"__SCRIPTURE_SMALLCAPS__", smallCapsFile,
		"__SCRIPTURE_ITALIC__", italicFile,
		"__SMALLCAPS_RANGE__", strings.Join(points, ","),
	).Replace(nkjvCSSTemplate)
}

const nkjvCSSTemplate = `
/* Junicode (c) Peter S. Baker — SIL Open Font License 1.1, published beside
   these files as assets/junicode-OFL.txt. The small capitals the divine name
   is drawn with, and nothing else; the italic for supplied words and titles. */
@font-face{
  font-family:"Junicode"; font-style:normal; font-weight:400;
  font-display:swap; src:url(__SCRIPTURE_SMALLCAPS__) format("woff2");
  unicode-range:__SMALLCAPS_RANGE__;
}
@font-face{
  font-family:"Junicode"; font-style:italic; font-weight:400;
  font-display:swap; src:url(__SCRIPTURE_ITALIC__) format("woff2");
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
