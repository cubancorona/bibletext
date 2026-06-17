//go:build darwin

package bibletext

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

// bibleTextReadingScrolled is called from the native text overlays when the
// reader finishes a scroll. It persists the current reading position (book /
// chapter / scroll anchor) so the saved spot is always up to date — the iOS
// app-background lifecycle hook doesn't fire reliably, so we save on scroll-end
// rather than only at exit. It runs on the native UI thread; flushReadingState
// only reads the live scroll (already main-thread-safe) and writes a small
// preference blob, so no Fyne UI-goroutine hop is needed.
//
//export bibleTextReadingScrolled
func bibleTextReadingScrolled() {
	if state := activeAIState; state != nil {
		flushReadingState(state)
	}
}

// bibleTextAIMenuTapped is called from the HBReadingTextView subclasses when the
// user picks an AI study action. It runs on the native UI thread, so it copies the
// C strings right away and hops onto Fyne's UI goroutine before showing anything.
//
//export bibleTextAIMenuTapped
func bibleTextAIMenuTapped(cAction, cText *C.char) {
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

// bibleTextStudyMenuTapped is the sibling callback for the non-AI selection-menu
// actions (Share verse, and — as they land — Cross-references and Word study).
// Same threading contract as above: copy the C strings, then hop to Fyne's UI
// goroutine before touching any state.
//
//export bibleTextStudyMenuTapped
func bibleTextStudyMenuTapped(cAction, cText *C.char) {
	action := C.GoString(cAction)
	text := C.GoString(cText)
	state := activeAIState
	if state == nil {
		return
	}
	fyne.Do(func() {
		dispatchSelectionAction(state, action, text)
	})
}
