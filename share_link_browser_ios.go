//go:build ios

package bibletext

/*
#cgo LDFLAGS: -framework UIKit -framework Foundation
#include <stdlib.h>
void bibleTextOpenInBrowser(const char *url);
*/
import "C"

import "unsafe"

// openLinkInBrowser hands a shared link back to the system so it opens in
// Safari. Used when the link carries a note and notes are switched off: the app
// cannot stop the OS waking it, but it can decline the link and pass it on,
// where the note is still readable.
func openLinkInBrowser(rawURL string) {
	if rawURL == "" {
		return
	}
	c := C.CString(rawURL)
	defer C.free(unsafe.Pointer(c))
	C.bibleTextOpenInBrowser(c)
}
