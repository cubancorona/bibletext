//go:build android

package bibletext

// The native→Go callbacks for the Android reading overlay — the Android twin
// of ai_menu_darwin.go. A file containing //export directives may only have C
// *declarations* in its cgo preamble; the JNI thunk *definitions* that call
// these live in reading_jni_android.c (suffix-scoped to GOOS=android), and the
// helpers that call INTO Java live in reading_android.go.

import "C"

import (
	"fyne.io/fyne/v2"
)

// btaSelectionAction is called (via the JNI thunk, on the Android UI thread)
// when the reader picks one of BibleText's items from the text-selection
// toolbar. Same action strings as iOS; the strings are copied before this
// call returns, and dispatch happens on the Fyne goroutine into the shared,
// untagged AI/share/cross-reference code. lo/hi is the selection's verse span,
// resolved in Java against the Spanned's verse index (BtBridge.verseAtOffset)
// — same contract as the Apple exports: position decides which verses are
// cited, 0,0 = unresolved.
//
//export btaSelectionAction
func btaSelectionAction(cAction, cText *C.char, lo, hi C.int) {
	action := C.GoString(cAction)
	text := C.GoString(cText)
	span := selSpanFromNative(int(lo), int(hi))
	state := activeAIState
	if state == nil || text == "" {
		return
	}
	fyne.Do(func() {
		switch action {
		case "ask", "explain", "context", "translation":
			dispatchAIAction(state, action, text, span)
		default:
			dispatchSelectionAction(state, action, text, span)
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

// btaReadAlongUserScrolled fires when the reader scrolls BY HAND while read-along
// is live (BtBridge distinguishes it from our own follow-scroll). It suspends the
// follow (the highlight keeps tracking the voice) until the "Follow narration"
// pill is tapped — the Android twin of bibleTextReadAlongUserScrolled.
//
//export btaReadAlongUserScrolled
func btaReadAlongUserScrolled() {
	gAudio.onReadAlongUserScroll()
}

// btaReadAlongFollowTapped fires when the reader taps the floating "Follow
// narration" pill — re-attach the view to the narration. The Android twin of
// bibleTextReadAlongFollowTapped.
//
//export btaReadAlongFollowTapped
func btaReadAlongFollowTapped() {
	gAudio.resumeReadAlongFollow()
}

// btaKeyboardChanged is the Android twin of iOS's bibleTextKeyboardChanged:
// the soft keyboard's live on-screen overlap, observed on the activity window by
// BtBridge.installKeyboardWatcher. It feeds the goto picker's verse-row lift
// (gKeyboardInsetSetter) and nothing else — the canvas is never resized, so the
// tablet-layout classification never sees the IME.
//
// The overlap arrives in PIXELS and is converted here with the live canvas
// scale, the same px<->unit factor pushChapterHTML uses for the text size. It
// must NOT arrive in Android dp: a dp is dpi/160 while a Fyne unit is a POINT
// (fyne's pixelsPerPt = dpi/72), so dp read as units overstates the lift ~2.2x
// — which the goto card's own clamp silently absorbed in portrait and could not
// absorb in landscape, leaving the verse row under the keyboard it exists to
// clear (emulator-caught).
//
//export btaKeyboardChanged
func btaKeyboardChanged(overlapPx C.float) {
	px := float32(overlapPx)
	fyne.Do(func() {
		if gKeyboardInsetSetter == nil {
			return
		}
		scale := float32(1)
		if st := activeAIState; st != nil && st.window != nil {
			if c := st.window.Canvas(); c != nil && c.Scale() > 0 {
				scale = c.Scale()
			}
		}
		gKeyboardInsetSetter(px / scale)
	})
}
