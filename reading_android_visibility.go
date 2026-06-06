//go:build android

package bibletext

// notifyReadingOverlay is a no-op on Android; the Android build uses the Fyne
// RichText reading view (see reading_mobile.go), which is part of the regular
// Fyne widget tree and gets hidden by AppTabs automatically.
func notifyReadingOverlay(bool) {}
