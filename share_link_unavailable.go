package bibletext

// "This link is in a translation you don't have."
//
// A shared link names its translation in the path (/nkjv/john/3/), and the app
// switches to it — unless it is one this reader has not unlocked. Before this,
// the app just carried on in whatever translation they were already reading:
// no switch, no message, no trace. The rule is plain: a reader whose copy is
// not licensed for the link's translation gets a message — and the reason it
// matters is not politeness. Verse numbering is not interchangeable (Romans
// 14/16 genuinely renumber between translations; see docs/TEXTUAL-DATA.md), so a
// reader silently landed in the wrong translation can be reading DIFFERENT text
// from the one the sender pointed at, with nothing on screen to suggest it.
//
// WHAT IT DOES NOT DO:
//
//   - It does not withhold the passage. The verse is not the licensed part;
//     refusing to open scripture because of a licence would be a worse answer
//     than any wording. applyShareTarget opens it first and this card goes up
//     over it, so the reader sees the passage and the explanation together.
//   - It does not offer the browser. The website's /nkjv/ route is a no-text
//     signpost, not the requested translation, so it cannot solve this card's
//     problem. The reader already has a public edition open underneath.
//   - It does not promise the translation is obtainable. It may be one key away
//     (the NKJV unlocks with an API.Bible key in Settings) or it may not be, and
//     this card cannot tell. It names the translation and stops.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// showLinkVersionUnavailable tells the reader that the link they tapped was
// shared in a translation this copy of BibleText cannot open, and that the
// passage in front of them is a different one. name is the full display name of
// the translation the link asked for ("New King James Version").
//
// With no canvas to draw on — a cold start whose window is not built yet — it
// says nothing rather than queueing: the link has already opened the passage,
// and a card that appears minutes later attached to nothing is worse than
// silence. (That path is rare by construction: applyShareTarget only runs once
// loadPhase is ready, and HandleShareLink parks everything earlier.)
func showLinkVersionUnavailable(state *AppState, name string) {
	if state == nil || name == "" {
		return
	}
	// Says the two things a reader can act on: which translation they are
	// actually looking at, and that the numbering may not line up. No apology,
	// no licensing lecture — neither helps them read the verse.
	showLinkNotice(state, "Shared in "+name, "Showing "+state.currentVersion().Name,
		name+" isn't available in your copy of BibleText, so this passage "+
			"is open in the translation you were already reading. Verse numbering can differ slightly "+
			"between translations.")
}

// showLinkNotice is the card itself: a heading, an accent line naming what the
// card is about, and a paragraph, over the passage. It was extracted from
// showLinkVersionUnavailable when a SECOND thing a link needs to say arrived —
// "this passage hasn't downloaded yet, it will open when it does"
// (applyShareTarget's seed park). Both are the same sentence to the reader ("the
// app has your link and here is what is happening to it"), so they are the same
// card, and only the wording differs; the hide/restore dance, the sizing rule
// and the OK button are exactly what they were.
func showLinkNotice(state *AppState, heading, subheading, para string) {
	if state == nil || heading == "" {
		return
	}
	cnv := pickerCanvas(state)
	if cnv == nil {
		return
	}
	pal := state.pal()

	// The native reading overlay floats ABOVE the Fyne canvas on iOS and
	// Android, so it would paint straight over this card and its button — the
	// same hide/restore dance every other modal here does.
	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}

	title := canvas.NewText(heading, pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 18

	here := canvas.NewText(subheading, pal.Accent)
	here.TextStyle = fyne.TextStyle{Bold: true}
	here.TextSize = subheadingTextSize

	body := widget.NewLabel(para)
	body.Wrapping = fyne.TextWrapWord

	var popup *widget.PopUp
	ok := widget.NewButton("OK", func() {
		if popup != nil {
			popup.Hide()
		}
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	})
	ok.Importance = widget.HighImportance

	form := container.NewVBox(
		title, here,
		widget.NewSeparator(),
		body,
		container.NewCenter(ok),
	)

	// Same sizing rule as the note offer: 440pt where there is room, and never
	// wider than the canvas less its margins, so the text wraps on a phone
	// instead of running off the edge.
	w := float32(440)
	if cw := cnv.Size().Width - 40; cw > 260 && w > cw {
		w = cw
	}
	card := surface(container.NewPadded(form), pal.SurfaceAlt, pal.Border, fyne.Size{})
	popup = widget.NewModalPopUp(card, cnv)
	popup.Show()
	popup.Resize(fyne.NewSize(w, card.MinSize().Height))
}
