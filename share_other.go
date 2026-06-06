//go:build !darwin

package bibletext

// Platforms without a wired-up native share sheet (Linux/Windows desktop,
// Android). The share actions are reachable only from the native selection menu
// (darwin today), so these are graceful no-ops that keep the package building
// everywhere. A Fyne-side share affordance can replace them later.
func nativeShareText(string)  {}
func nativeShareImage(string) {}
