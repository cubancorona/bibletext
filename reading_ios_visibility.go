//go:build ios

package bibletext

// notifyReadingOverlay is the build-tag shim ui_mobile.go uses to keep the
// native UITextView overlay in sync with the bottom-tab selection — visible
// only when the Read tab is in front.
func notifyReadingOverlay(visible bool) {
	if visible {
		showNativeReadingOverlay()
	} else {
		hideNativeReadingOverlay()
	}
}
