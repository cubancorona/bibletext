//go:build ios || !darwin

package bibletext

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// readingPaneTheme styles ONLY the scripture pane: the reader's chosen text
// size (Settings → Text size), which previously changed nothing on the Fyne
// pane. rewrap measures at the same size (chapterText.textSize), so the wrap
// exactly matches what renders.
//
// Deliberately SIZE-ONLY — no Font override. On stock fyne 2.7.4 a per-widget
// font override is unsound: canvas.Text.MinSize() measures with the DEFAULT-
// scope app font while the GL painter draws with the override face, so row
// textures are sized for one font and drawn with another (right-edge glyph
// clipping), rows added by a later rewrap never receive the override scope
// (mixed typefaces down the page), and selection hit-testing measures the
// wrong face. The book-serif parity for Win/Linux therefore waits on fyne
// upstream (canvas.Text.MinSize consulting the object's override scope) or an
// extension of the project's fyne-patch mechanism to desktop builds.
type readingPaneTheme struct {
	fyne.Theme
	size float32
}

func (t readingPaneTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameText {
		return t.size
	}
	return t.Theme.Size(name)
}

// readingScrollArea (Fyne) is the scrollable chapter text used on every desktop
// platform except macOS, plus the compiled-but-unused fallback on the mobile
// builds. It wraps the whole chapter in one chapterText (a read-only,
// drag-selectable widget.Entry) inside a centred, width-capped column.
//
// macOS uses a native NSTextView overlay instead — see reading_macos.go — to
// get the system selection menu (Copy / Look Up / Translate / Share). This file
// and reading_macos.go are mutually exclusive by build tag.
func readingScrollArea(state *AppState, verses []Verse, pal palette) fyne.CanvasObject {
	// Windows/Linux dispatch the styled, selectable pane (reading_styled_*.go);
	// the constant is FALSE on iOS (where this file is dead code) and on the
	// Android bridge-absent fallback, so those paths remain byte-identical.
	if useStyledPane() {
		return styledReadingScrollArea(state, verses, pal)
	}
	col := &readingColumn{maxWidth: 760}
	var child fyne.CanvasObject
	var chapter *chapterText
	if len(verses) == 0 {
		msg := widget.NewLabel("No verses are available for this chapter yet.")
		msg.Wrapping = fyne.TextWrapWord
		child = msg
	} else {
		// One widget for the whole chapter, so selection and copy span the
		// entire passage, not just a single paragraph. Wrapped in the reading
		// theme (the text-size setting); rewrap measures at the same size so
		// wrapping stays exact.
		chapter = newChapterText(state, verses)
		col.chapter = chapter
		var base fyne.Theme = theme.DefaultTheme()
		if state.theme != nil {
			base = state.theme
		}
		child = container.NewThemeOverride(chapter,
			readingPaneTheme{Theme: base, size: chapter.textSize})
	}

	paneObjects := []fyne.CanvasObject{child}
	if chapter != nil && state.HasHighlightedVerse {
		band := canvas.NewRectangle(highlightBandColor(state.pal()))
		band.CornerRadius = 6
		band.Hide() // shown by readingColumn.Layout once the geometry is real
		col.band = band
		paneObjects = append(paneObjects, band) // after child → drawn over it
	}
	scroll := container.NewVScroll(container.New(col, paneObjects...))
	col.scroll = scroll
	if chapter != nil {
		chapter.parentScroll = scroll
	}

	// Register the pane for within-chapter scroll capture/restore (position
	// persistence + history-tap restore). Real on Linux/Windows, a no-op on the
	// native-overlay platforms, where this pane is only the inert fallback.
	wireFyneReadingScroll(state, scroll, chapter)

	return surface(container.NewPadded(scroll), pal.Background, pal.Border, fyne.Size{})
}

// highlightBandColor is the translucent wash readingColumn draws OVER the
// highlighted verse. It renders on top of the text (the Entry's background is
// opaque, so behind is invisible), so it must stay light enough to read
// through: the lapis Highlight hue on the light page, the pale HighlightText
// hue at low alpha on the dark one (the dark Highlight wash would vanish
// against the dark paper).
func highlightBandColor(pal palette) color.NRGBA {
	if int(pal.Surface.R)+int(pal.Surface.G)+int(pal.Surface.B) < 384 { // dark paper
		c := pal.HighlightText
		c.A = 46
		return c
	}
	c := pal.Highlight
	c.A = 110
	return c
}

// setReadingOverlayVisible is a no-op where there's no native text overlay.
func setReadingOverlayVisible(bool) {}
