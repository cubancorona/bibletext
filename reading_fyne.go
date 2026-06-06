//go:build ios || !darwin

package bibletext

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// readingScrollArea (Fyne) is the scrollable chapter text used on every desktop
// platform except macOS, plus the compiled-but-unused fallback on the mobile
// builds. It wraps the whole chapter in one chapterText (a read-only,
// drag-selectable widget.Entry) inside a centred, width-capped column.
//
// macOS uses a native NSTextView overlay instead — see reading_macos.go — to
// get the system selection menu (Copy / Look Up / Translate / Share). This file
// and reading_macos.go are mutually exclusive by build tag.
func readingScrollArea(state *AppState, verses []Verse, pal palette) fyne.CanvasObject {
	col := &readingColumn{maxWidth: 760}
	var child fyne.CanvasObject
	var chapter *chapterText
	if len(verses) == 0 {
		msg := widget.NewLabel("No verses are available for this chapter yet.")
		msg.Wrapping = fyne.TextWrapWord
		child = msg
	} else {
		// One widget for the whole chapter, so selection and copy span the
		// entire passage, not just a single paragraph.
		chapter = newChapterText(state, verses)
		col.chapter = chapter
		child = chapter
	}

	scroll := container.NewVScroll(container.New(col, child))
	col.scroll = scroll
	if chapter != nil {
		chapter.parentScroll = scroll
	}

	return surface(container.NewPadded(scroll), pal.Surface, pal.Border, fyne.Size{})
}

// setReadingOverlayVisible is a no-op where there's no native text overlay.
func setReadingOverlayVisible(bool) {}
