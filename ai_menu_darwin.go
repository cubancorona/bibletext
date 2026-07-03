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
// rather than only at exit. It runs on the native (main) UI thread, so it uses
// the async flush: it reads the live scroll there (TextKit is main-thread-only)
// but writes the preference blob on a goroutine, so a finger-lift never blocks
// the main thread with a JSON encode + write (which made scrolling feel laggy).
//
//export bibleTextReadingScrolled
func bibleTextReadingScrolled() {
	if state := activeAIState; state != nil {
		// A genuine user scroll (this callback only fires on drag/decelerate end,
		// never on our own programmatic restore) means any pending restore target is
		// now obsolete — drop it so a later same-chapter re-push won't yank the
		// reader back. The native side already disarmed its copy in scrollViewDidScroll.
		state.restore = nil
		flushReadingStateAsync(state)
	}
}

// bibleTextKeyboardChanged reports the iOS soft keyboard's on-screen overlap (its height
// in points, 0 when hidden) from a keyboard-frame observer, so the Goto verse picker can
// lift its bottom row to sit EXACTLY above the keyboard rather than estimating. Runs on
// the native main thread; it hops to Fyne's goroutine and forwards to whatever inset
// setter the open picker registered (nil when no picker is up, so other keyboards — e.g.
// the search field — are ignored).
//
//export bibleTextKeyboardChanged
func bibleTextKeyboardChanged(height C.double) {
	h := float32(height)
	fyne.Do(func() {
		if gKeyboardInsetSetter != nil {
			gKeyboardInsetSetter(h)
		}
	})
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

// bibleTextHighlightCleared fires when the reader single-taps a highlighted verse
// and picks "Clear highlight" from the inline native menu. It drops the highlight
// and re-renders so the .hl background wash disappears. Runs on the native UI
// thread, so it hops to Fyne's UI goroutine before touching state. refreshReadingOnly
// (not refresh): the sidebar doesn't reflect highlight state, so this is the
// narrowest correct refresh. Idempotent — a double-tap's second call is a no-op.
//
//export bibleTextHighlightCleared
func bibleTextHighlightCleared() {
	state := activeAIState
	if state == nil {
		return
	}
	fyne.Do(func() {
		if !state.HasHighlightedVerse {
			return
		}
		clearHighlightedVerse(state)
		state.refreshReadingOnly()
	})
}
