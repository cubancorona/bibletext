//go:build darwin && !ios

package bibletext

// The macOS pane's width, reported by the NSTextView's scroll view whenever it
// changes (btMacApplyFrame, reading_macos.go). It lives on its own because a
// file with an //export may hold only C declarations in its preamble, and
// reading_macos.go's is full of definitions (see ai_menu_darwin.go).

import "C"

import "fyne.io/fyne/v2"

//export btMacReadingWidthChanged
func btMacReadingWidthChanged(w C.double) {
	width := float64(w)
	fyne.Do(func() {
		noteReadingPaneWidth(width, func() {
			if h := macCurrentHost; h != nil && h.state != nil {
				h.state.refreshReadingOnly()
			}
		})
	})
}
