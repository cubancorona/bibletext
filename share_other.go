//go:build !darwin && !android

package bibletext

// Platforms without a wired-up native share sheet (Linux/Windows desktop).
// darwin has the system share sheets; Android has ACTION_SEND via BtBridge
// (reading_android.go). These graceful no-ops keep the package building
// everywhere else.
func nativeShareText(string)  {}
func nativeShareImage(string) {}
