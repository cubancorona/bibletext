package bibletext

// Share a verse from the selection menu. Two actions, both handed to the native
// OS share sheet (Messages, Mail, Notes, …):
//   - "Share with citation": plain text — the quoted selection plus a reference,
//     ready to drop into a message.
//   - "Share as image": a rendered card (see share_image.go).
//
// The dispatcher here is also where future selection-menu actions
// (cross-references, word study) are routed.

import (
	"fmt"
	"strings"
)

const (
	selActionShareCite  = "share-cite"
	selActionShareImage = "share-image"
	selActionCrossRef   = "crossref"
)

// dispatchSelectionAction routes a non-AI selection-menu action from the native
// callback (already on the Fyne UI goroutine).
func dispatchSelectionAction(state *AppState, action, text string) {
	if state == nil {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	switch action {
	case selActionShareCite:
		shareVerse(state, text, false)
	case selActionShareImage:
		shareVerse(state, text, true)
	case selActionCrossRef:
		showCrossRefs(state, text)
	}
}

// shareVerse formats the selection with a citation and hands it to the native
// share sheet, as text or as a rendered image.
func shareVerse(state *AppState, text string, asImage bool) {
	cite := citationForSelection(state, text)
	abbrev := state.currentVersion().Abbrev
	if asImage {
		if path, err := renderVerseImage(state, text, cite, abbrev); err == nil {
			nativeShareImage(path)
			return
		}
		// If image rendering fails for any reason, share the text instead.
	}
	nativeShareText(fmt.Sprintf("“%s”\n— %s (%s)", collapseSpaces(text), cite, abbrev))
}

// citationForSelection derives a "Book C:V" (or "…:V-W") reference for the
// selected text by matching it against the verses of the current chapter, so a
// shared selection carries an accurate citation. Falls back to "Book C" when the
// selection can't be pinned to specific verses (e.g. a partial phrase).
func citationForSelection(state *AppState, text string) string {
	book, ch := state.CurrentBook, state.CurrentChapter
	if state.Bible == nil {
		return fmt.Sprintf("%s %d", book, ch)
	}
	norm := collapseSpaces(text)
	lo, hi := 0, 0
	for _, v := range state.Bible.GetChapter(book, ch) {
		vt := collapseSpaces(v.Text)
		if len([]rune(vt)) < 8 {
			continue
		}
		probe := vt
		if r := []rune(vt); len(r) > 24 {
			probe = string(r[:24])
		}
		if strings.Contains(norm, probe) {
			if lo == 0 {
				lo = v.Verse
			}
			hi = v.Verse
		}
	}
	switch {
	case lo == 0:
		return fmt.Sprintf("%s %d", book, ch)
	case lo == hi:
		return fmt.Sprintf("%s %d:%d", book, ch, lo)
	default:
		return fmt.Sprintf("%s %d:%d-%d", book, ch, lo, hi)
	}
}
