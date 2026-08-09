//go:build android

package bibletext

// The Java→Go callback for a tapped shared link on Android, reached through the
// JNI thunk in share_link_jni_android.c. Same empty-preamble //export split the
// audio and reading bridges use.

import "C"

// btOpenedLink receives the URL of an App Link the user tapped. It runs on the
// Android UI thread, which is NOT the Fyne UI goroutine — deliverShareLink
// marshals through fyne.Do and ignores anything that isn't a reader link.
//
//export btOpenedLink
func btOpenedLink(cURL *C.char) {
	deliverShareLink(C.GoString(cURL))
}
