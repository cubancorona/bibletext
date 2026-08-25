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

// devAutoReadAlong: see the dev twin. Empty in release builds.
func devAutoReadAlong(state *AppState) {}

// devAutoTintBench: see the dev twin. Empty in release builds.
func devAutoTintBench(state *AppState) {}

// devAutoNotesS8: see the dev twin. Empty in release builds.
func devAutoNotesS8(state *AppState) {}

// devTraceNotePlacement is diagnostic output in development builds only.
func devTraceNotePlacement(state *AppState, event string) {}

// devAppID preserves the shipping application identity.
func devAppID(id string) string { return id }

// devNoteDebug: see the dev twin. Empty in release builds.
func devNoteDebug(state *AppState) string { return "" }
