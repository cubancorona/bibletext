//go:build darwin

package bibletext

// The native→Go callback for a tapped shared link.
//
// It lives alone, with an EMPTY cgo preamble, because a file containing an
// //export directive may only carry C *declarations* — and share_link_ios.go's
// preamble is full of C definitions (the delegate category). Same split as
// ai_menu_darwin.go and audio_export_apple.go; that file declares this function
// `extern` and calls it.
//
// Built for `darwin` rather than `ios` so the file compiles on macOS too, where
// it is simply never called (macOS has no Universal Links entitlement here).

import "C"

// bibleTextOpenedLink receives a URL the OS handed to the app because it
// matched our claimed domain and paths. It runs on the native main thread;
// deliverShareLink marshals onto the Fyne UI goroutine and drops anything that
// isn't one of our reader links.
//
// Returns 1 when the URL is one of our reader links and the app has taken it,
// 0 when it is not — the native side reports that straight back to the OS so an
// unclaimed link falls through to the browser instead of vanishing. C int
// rather than bool because cgo's C.int is the portable thing to hand ObjC.
//
//export bibleTextOpenedLink
func bibleTextOpenedLink(cURL *C.char) C.int {
	if deliverShareLink(C.GoString(cURL)) {
		return 1
	}
	return 0
}
