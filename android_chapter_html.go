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
// <sup><small><font color><b>, red-letter is <font color>, and a verse's wash is
// an inline style= span (honored on API 24+).
//
// The dialect differences all live in androidTintHTML (tint.go) now. What is
// left here is the paragraph and join structure — which must stay identical to
// buildChapterHTML's, because a poem that breaks in different places on two
// platforms is the divergence this file was split out to prevent.
func buildChapterHTMLAndroid(state *AppState, verses []Verse) string {
	pal := state.pal()
	redLetter := redLetterEnabled()
	// ONE tint answer for the whole chapter (tint.go), asked per verse below,
	// plus this dialect's markup for each tint. The markup table is built HERE,
	// per render, because it carries palette colours rather than class names —
	// so the theme is baked in once instead of at every verse.
	tints := chapterTint(state)
	markup := androidTintHTML(pal, nrgbaToHex(pal.VerseNumber), nrgbaToHex(pal.RedLetter))

	var b strings.Builder
	for _, para := range groupVersesIntoParagraphs(verses) {
		b.WriteString("<p>")
		for i, v := range para {
			mk := markupFor(markup[:], tints.of(v))
			if i > 0 {
				switch {
				case poeticJoin(para[i-1].Text, v.Text):
					b.WriteString("<br>")
				case tints.joins(para[i-1], v):
					// The joining space belongs to the band, or the highlight
					// comes out notched at every join falling mid-line. Same
					// fix, same reason, as the iOS dialect in reading.go — and
					// the same SAME-TINT rule: a join between two different
					// washes belongs to neither.
					b.WriteString(mk.JoinSpace)
				default:
					b.WriteString(" ")
				}
			}
			// The number joins the band too — leaving it out punches a pale
			// hole through the middle of the highlight (iOS parity). Whether it
			// is wrapped is mk.Number's business.
			fmt.Fprintf(&b, mk.Number, v.Verse)
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
				for _, run := range runs {
					piece := strings.ReplaceAll(htmlEscape(run.Text), "\n", "<br>")
					writeTintedHTML(&b, mk, run.Red, piece)
				}
				continue
			}
			body := strings.ReplaceAll(htmlEscape(strings.TrimSpace(v.Text)), "\n", "<br>")
			// From the runs — see the note in reading.go.
			writeTintedHTML(&b, mk, len(runs) == 1 && runs[0].Red, body)
		}
		b.WriteString("</p>")
	}
	return b.String()
}
