package bibletext

// The paragraph body of the Fyne fallback reading pane (reading_mobile.go): the
// RichText segments behind one selectableParagraph.
//
// It lives here UNTAGGED rather than in reading_mobile.go, which is
// //go:build android — the same split, for the same reason, as
// android_chapter_html.go out of reading_android.go: nothing behind the android
// tag can be exercised by a host test, and the rules below (which words are red,
// what a highlight is allowed to do to them) are exactly the rules that were
// wrong here.

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// mobileParagraphSegments renders one paragraph — its verses flowing inline
// behind superscript numbers — as RichText segments.
//
// Red letters: this pane drew NONE, on any edition, while every other pane drew
// them and Settings still offered the switch, so a reader who fell back to it
// (no BtBridge) just silently lost the feature. The split now comes from the
// shared run splitter, so the BSB reddens only Christ's words while every other
// edition yields one run — the whole-verse red the other panes always drew.
func mobileParagraphSegments(state *AppState, verses []Verse) []widget.RichTextSegment {
	redLetter := redLetterEnabled()
	verseNumStyle := widget.RichTextStyle{
		Inline:    true,
		ColorName: colorNameVerseNumber,
		SizeName:  theme.SizeNameCaptionText,
		TextStyle: fyne.TextStyle{Bold: true},
	}
	segs := make([]widget.RichTextSegment, 0, len(verses)*4)
	for i, v := range verses {
		if i > 0 {
			// A poetic verse boundary is a line boundary (RichText splits an
			// inline segment's text on literal "\n" into separate rows, so a
			// "\n" separator renders as exactly one hard break).
			sep := " "
			if poeticJoin(verses[i-1].Text, v.Text) {
				sep = "\n"
			}
			segs = append(segs, &widget.TextSegment{
				Text:  sep,
				Style: widget.RichTextStyle{Inline: true, ColorName: colorNameVerseText},
			})
		}
		segs = append(segs, &widget.TextSegment{
			Text:  superscriptNumber(v.Verse) + " ",
			Style: verseNumStyle,
		})
		hl := isVerseHighlighted(state, v)
		runs := trimRuns(redLetterRuns(state.CurrentVersion, v, redLetter))
		// ONE run means no span data — every edition but the BSB and the NKJV,
		// and any verse whose text no longer matches the offsets. Take the
		// original single-segment path there, keeping strings.TrimSpace rather
		// than trimRuns: they are not the same trim, and swapping them silently
		// changed what happens to a trailing \r or U+00A0 and to a verse that is
		// nothing but whitespace. The sibling panes guard the same way.
		if len(runs) < 2 {
			red := len(runs) == 1 && runs[0].Red
			segs = append(segs, &widget.TextSegment{
				Text: strings.TrimSpace(v.Text),
				Style: widget.RichTextStyle{
					Inline:    true,
					ColorName: mobileRunColorName(red, hl),
				},
			})
			continue
		}
		// One segment per run. Authored poem lines stay inside the run text —
		// RichText still renders each as its own row — and trimRuns trims the
		// verse as a whole, so the space between "…do you have?”" and "Jesus
		// asked." survives the split.
		for _, run := range runs {
			segs = append(segs, &widget.TextSegment{
				Text: run.Text,
				Style: widget.RichTextStyle{
					Inline:    true,
					ColorName: mobileRunColorName(run.Red, hl),
				},
			})
		}
	}
	return segs
}

// mobileRunColorName picks a run's colour. It is also where the two defects this
// pane carried are answered.
//
// Red beats the highlight. A highlighted verse used to be recoloured WHOLE to
// colorNameHighlightHi, which swallowed the words of Christ inside it — the same
// bug the native panes had until the owner caught it on John 11:25, where the
// rule became: the band marks the highlight, the text keeps its own colour
// (reading_styled_pane.go, runColor).
//
// This pane has no band to fall back on, and I first wrote it as if it had: a
// RichText segment cannot paint a background, and a paragraph here is ONE
// wrapped RichText with no per-verse geometry to lay a rect over — obtaining
// that geometry is the entire reason the styled desktop pane exists. So dropping
// the recolour outright would leave a highlighted verse with no mark at all.
// The recolour therefore stays for everything that is not red: the highlight
// keeps looking exactly like it did, and only the red is rescued from it.
//
// Nothing here sets Bold. The old code bolded a highlighted verse, which is the
// re-typesetting every other pane refuses to do: the bold serif sets ~17% wider,
// so the paragraph re-wrapped and the text jumped the moment the highlight
// cleared.
func mobileRunColorName(red, highlighted bool) fyne.ThemeColorName {
	switch {
	case red:
		return colorNameRedLetter
	case highlighted:
		return colorNameHighlightHi
	default:
		return colorNameVerseText
	}
}
