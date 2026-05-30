//go:build darwin

package holybible

// This file holds the single native→Go callback for the AI selection-menu items.
// It lives on its own because a file containing an //export directive may only
// have C *declarations* in its cgo preamble — and reading_ios.go / reading_macos.go
// preambles are full of C *definitions*. So the export goes here (empty preamble),
// and those files declare it `extern` to call it.
//
// `darwin` covers both macOS (GOOS=darwin) and iOS (GOOS=ios implies darwin), so
// one file serves both native overlays.

import "C"

import "fyne.io/fyne/v2"

// holyBibleAIMenuTapped is called from the HBReadingTextView subclasses when the
// user picks an AI study action. It runs on the native UI thread, so it copies the
// C strings right away and hops onto Fyne's UI goroutine before showing anything.
//
//export holyBibleAIMenuTapped
func holyBibleAIMenuTapped(cAction, cText *C.char) {
	action := C.GoString(cAction)
	text := C.GoString(cText)
	state := activeAIState
	if state == nil {
		return
	}
	fyne.Do(func() {
		dispatchAIAction(state, action, text)
	})
}
