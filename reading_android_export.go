//go:build android

package bibletext

// The native→Go callbacks for the Android reading overlay — the Android twin
// of ai_menu_darwin.go. A file containing //export directives may only have C
// *declarations* in its cgo preamble; the JNI thunk *definitions* that call
// these live in reading_jni_android.c (suffix-scoped to GOOS=android), and the
// helpers that call INTO Java live in reading_android.go.

import "C"

import "fyne.io/fyne/v2"

// btaSelectionAction is called (via the JNI thunk, on the Android UI thread)
// when the reader picks one of BibleText's items from the text-selection
// toolbar. Same action strings as iOS; the strings are copied before this
// call returns, and dispatch happens on the Fyne goroutine into the shared,
// untagged AI/share/cross-reference code.
//
//export btaSelectionAction
func btaSelectionAction(cAction, cText *C.char) {
	action := C.GoString(cAction)
	text := C.GoString(cText)
	state := activeAIState
	if state == nil || text == "" {
		return
	}
	fyne.Do(func() {
		switch action {
		case "ask", "explain", "context", "translation":
			dispatchAIAction(state, action, text)
		default:
			dispatchSelectionAction(state, action, text)
		}
	})
}

// btaScrolled fires ~200ms after the reader's scroll goes idle — the Android
// stand-in for the iOS scroll-end delegate. A genuine reader scroll obsoletes
// any pending restore target, and the fresh position is persisted.
//
//export btaScrolled
func btaScrolled(frac C.float) {
	if state := activeAIState; state != nil {
		fyne.Do(func() {
			state.restore = nil
			flushReadingStateAsync(state)
		})
	}
}
