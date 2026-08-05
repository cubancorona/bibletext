//go:build !darwin && !race

package bibletext

// legacyPaneForTest builds the LEGACY chapterText pane directly for the
// reading_layout_test.go assertions. On these platforms readingScrollArea
// dispatches to the styled pane (styledPaneEnabledOnPlatform), so going through
// buildReadingView would find no chapterText in the tree — but the legacy pane
// is still shipping code (the Android fallback and the desktop burn-in
// fallback), so its layout tests target it directly. Styled wiring residue is
// cleared first so the scroll-anchor delegation stays on the legacy path.

import "fyne.io/fyne/v2"

func legacyPaneForTest(state *AppState) fyne.CanvasObject {
	resetStyledWiring()
	verses := state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter)
	return chapterTextScrollArea(state, verses, state.pal())
}
