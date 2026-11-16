//go:build !race

package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func TestCrossRefCompactPreviewUsesScopedSizing(t *testing.T) {
	th := appThemeForTest(t)
	withCrossRefIndex(t, "John.3.16\tRom.5.8\t10\n")
	previousRun, previousLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = previousRun, previousLoad })
	crossRefsRun = func(work func()) { work() }
	crossRefsLoad = func() error { return nil }
	bd := xrefBible(map[string]map[int]int{"John": {3: 36}, "Romans": {5: 21}})
	bd.Verses["Romans"][5][7].Text = "A synthetic preview with Lᴏʀᴅ and enough words to wrap across several short lines."
	w := test.NewWindow(widget.NewLabel("Reading"))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(375, 667))
	state := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3, CurrentVersion: "web", theme: th, window: w}
	showCrossRefs(state, "", selSpan{lo: 16, hi: 16})
	top := w.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("cross-reference panel did not open")
	}
	var cards []*tapCard
	var sources *widget.RichText
	walkTree(top, func(o fyne.CanvasObject) {
		if card, ok := o.(*tapCard); ok {
			cards = append(cards, card)
		}
		if text, ok := o.(*widget.RichText); ok && segmentText(text.Segments) == treasuryCredit {
			sources = text
		}
	})
	if len(cards) != 1 {
		t.Fatalf("got %d passage cards, want one", len(cards))
	}
	card := cards[0]
	var preview *widget.Label
	var reference *canvas.Text
	walkTree(card, func(o fyne.CanvasObject) {
		if label, ok := o.(*widget.Label); ok {
			preview = label
		}
		if text, ok := o.(*canvas.Text); ok && text.Text == "Romans 5:8" {
			reference = text
		}
	})
	if preview == nil || reference == nil || sources == nil {
		t.Fatal("preview, citation or source credit missing")
	}
	if preview.Theme().Size(theme.SizeNameText) != 17 || preview.Theme().Size(theme.SizeNameInnerPadding) != 0 {
		t.Fatal("preview must use 17-point text with no inner inset")
	}
	if reference.TextSize != 16 || preview.Wrapping != fyne.TextWrapWord {
		t.Fatal("citation size or preview wrapping changed")
	}
	if preview.Position().X != reference.Position().X || preview.Size().Width > card.Size().Width {
		t.Fatal("preview must align with the citation and fit within its card")
	}
	if card.Size().Height < 44 {
		t.Fatalf("passage tap height is %v, want at least 44", card.Size().Height)
	}
	closeButton := findTreeButton(top, "Close")
	if closeButton == nil || closeButton.Theme().Size(theme.SizeNameText) != th.Size(theme.SizeNameText) {
		t.Fatal("preview sizing leaked into the Close button")
	}
	if sources.Theme().Size(theme.SizeNameCaptionText) != 11 || sources.Theme().Size(theme.SizeNameInnerPadding) != 0 {
		t.Fatal("source credit must be an aligned 11-point caption")
	}
	// A short or empty row still has a usable tap target.
	tapped := false
	short := newTapCard(container.NewStack(), state.pal().SurfaceAlt, func() { tapped = true })
	if short.MinSize().Height != 44 {
		t.Fatalf("short row's tap height is %v, want 44", short.MinSize().Height)
	}
	test.Tap(short)
	if !tapped {
		t.Fatal("short row did not activate")
	}
	test.Tap(closeButton)
	if w.Canvas().Overlays().Top() != nil {
		t.Fatal("Close did not dismiss the panel")
	}
}
