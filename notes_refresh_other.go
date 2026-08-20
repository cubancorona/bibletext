//go:build !darwin

package bibletext

// Windows, Linux and Android: there is no in-place note push here, so every
// note verb keeps the full reading-pane rebuild it has always taken.
//
// This is not an oversight on either platform, and the reasons differ:
//
//   - The STYLED pane (Windows/Linux) draws the note as a real Fyne band inside
//     the widget tree — a card, a tail, a byline row and two buttons, with the
//     band's height reserved by layoutChapter. Changing which note is drawn
//     changes the tree's geometry, so only a rebuild can move it. Nothing is
//     lost: the styled pane has no floating native view whose frame lags a
//     layout, which is the defect refreshNoteOnly exists to remove.
//
//   - ANDROID has a native sticker, but its TextView is handed a whole Spanned
//     rather than a mutable attributed string (the same asymmetry that makes it
//     ask the combined chapterRenderFingerprint where the Apple panes ask two
//     halves). There is no "one attribute over a known range" primitive to
//     mutate, so an in-place push would have to rebuild the Spanned anyway.
func refreshNoteInPlace(*AppState) bool { return false }
