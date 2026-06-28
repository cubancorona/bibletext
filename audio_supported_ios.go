//go:build ios

package bibletext

// audioSupported gates the reading-header play button. Only iOS has a wired-up
// native audio engine, so only iOS shows the control.
func audioSupported() bool { return true }
