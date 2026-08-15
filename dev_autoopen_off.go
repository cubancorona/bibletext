//go:build !bibletextdev

package bibletext

// The dev sheet-opener, absent.
//
// THIS IS THE SHIPPING BUILD. Its twin lives in dev_autoopen_on.go behind
// `//go:build bibletextdev`, so a release binary does not merely ignore the
// environment variable — the code that reads it is not compiled in. Same
// reasoning as dev_links_off.go: a runtime flag would leave a way to open
// arbitrary sheets sitting inside the App Store build.
func devAutoOpenSheet(*AppState) {}

// devAutoSwitchVersion: see the dev twin. Empty in release builds.
func devAutoSwitchVersion(state *AppState) {}

// devNoteDebug: see the dev twin. Empty in release builds.
func devNoteDebug(state *AppState) string { return "" }
