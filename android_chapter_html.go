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
			// Runs first — in the BSB the words of Christ are a span inside the
			// verse, so the attribution and any other speaker beside it must stay
			// black (red_letter_runs.go). Every other edition yields one run,
			// which is the old behaviour exactly.
			runs := trimRuns(redLetterRuns(state.CurrentVersion, v, redLetter))
			if len(runs) > 1 {
				hl := isVerseHighlighted(state, v)
				for _, run := range runs {
					piece := strings.ReplaceAll(htmlEscape(run.Text), "\n", "<br>")
					switch {
					case hl && run.Red:
						fmt.Fprintf(&b, `<span style="background-color:%s"><font color="%s">%s</font></span>`, hlBG, red, piece)
					case hl:
						fmt.Fprintf(&b, `<span style="background-color:%s">%s</span>`, hlBG, piece)
					case run.Red:
						fmt.Fprintf(&b, `<font color="%s">%s</font>`, red, piece)
					default:
						b.WriteString(piece)
					}
				}
				continue
			}
			body := strings.ReplaceAll(htmlEscape(strings.TrimSpace(v.Text)), "\n", "<br>")
			wj := redLetter && isWordsOfChrist(v.BookName, v.Chapter, v.Verse)
			switch {
			case isVerseHighlighted(state, v) && wj:
				// Band AND colour, not one or the other — the Android twin of the
				// .hl/.wj pairing in reading.go. This arm did not exist, so the
				// highlight arm below swallowed every highlighted words-of-Christ
				// verse and it lost its red, exactly as the Apple pane did until
				// the owner caught it on John 11:25.
				//
				// The <font color> nests INSIDE the background span: Html.fromHtml
				// turns them into two independent spans over the same range, so the
				// inner colour keeps the outer band. Still NO <b> — see below.
				fmt.Fprintf(&b, `<span style="background-color:%s"><font color="%s">%s</font></span>`, hlBG, red, body)
			case isVerseHighlighted(state, v):
				// Colour and band only — NO <b>: bolding re-typesets the verse
				// (the bold serif sets ~17% wider), so the paragraph re-wrapped
				// and the text jumped when the highlight cleared. Matches iOS's
				// .hl and the desktop pane, neither of which changes weight.
				fmt.Fprintf(&b, `<span style="background-color:%s">%s</span>`, hlBG, body)
			case wj:
				fmt.Fprintf(&b, `<font color="%s">%s</font>`, red, body)
			default:
				b.WriteString(body)
			}
		}
		b.WriteString("</p>")
	}
	return b.String()
}
