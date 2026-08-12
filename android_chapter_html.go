package bibletext

// The Android reading overlay's chapter HTML builder. It lives untagged (NOT
// in the android-tagged reading_android.go) so host tests can pin the dialect
// alongside buildChapterHTML — the two must make identical poetry/join
// decisions or platforms diverge.

import (
	"fmt"
	"strings"
)

// buildChapterHTMLAndroid emits the Html.fromHtml-safe dialect of the chapter:
// no CSS classes (fromHtml ignores <style>), so verse numbers are
// <sup><small><font color><b>, red-letter is <font color>, and the search-jump
// highlight is an inline style= span (honored on API 24+).
func buildChapterHTMLAndroid(state *AppState, verses []Verse) string {
	pal := state.pal()
	vnum := nrgbaToHex(pal.VerseNumber)
	red := nrgbaToHex(pal.RedLetter)
	hlBG := nrgbaToHex(pal.Highlight)

	redLetter := redLetterEnabled()
	var b strings.Builder
	for _, para := range groupVersesIntoParagraphs(verses) {
		b.WriteString("<p>")
		for i, v := range para {
			if i > 0 {
				switch {
				case poeticJoin(para[i-1].Text, v.Text):
					b.WriteString("<br>")
				case isVerseHighlighted(state, para[i-1]) && isVerseHighlighted(state, v):
					// The joining space belongs to the band, or the highlight
					// comes out notched at every join falling mid-line. Same
					// fix, same reason, as the iOS dialect in reading.go.
					fmt.Fprintf(&b, `<span style="background-color:%s"> </span>`, hlBG)
				default:
					b.WriteString(" ")
				}
			}
			// The number joins the band too — leaving it out punches a pale
			// hole through the middle of the highlight (iOS parity).
			if isVerseHighlighted(state, v) {
				fmt.Fprintf(&b, `<span style="background-color:%s"><sup><small><font color="%s"><b>%d</b></font></small></sup>&nbsp;</span>`, hlBG, vnum, v.Verse)
			} else {
				fmt.Fprintf(&b, `<sup><small><font color="%s"><b>%d</b></font></small></sup>&nbsp;`, vnum, v.Verse)
			}
			// Authored poem lines become explicit <br> (Html.fromHtml collapses
			// a literal "\n" as whitespace; handleBr maps <br> to "\n", and the
			// TextView's INTER_WORD justification exempts hard-break lines, so
			// poem lines render ragged-right as in print).
			body := strings.ReplaceAll(htmlEscape(strings.TrimSpace(v.Text)), "\n", "<br>")
			switch {
			case isVerseHighlighted(state, v):
				// A search highlight wins visually over red-letter (as on iOS).
				// Colour and band only — NO <b>: bolding re-typesets the verse
				// (the bold serif sets ~17% wider), so the paragraph re-wrapped
				// and the text jumped when the highlight cleared. Matches iOS's
				// .hl and the desktop pane, neither of which changes weight.
				fmt.Fprintf(&b, `<span style="background-color:%s">%s</span>`, hlBG, body)
			case redLetter && isWordsOfChrist(v.BookName, v.Chapter, v.Verse):
				fmt.Fprintf(&b, `<font color="%s">%s</font>`, red, body)
			default:
				b.WriteString(body)
			}
		}
		b.WriteString("</p>")
	}
	return b.String()
}
