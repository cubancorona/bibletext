//go:build !ios

package bibletext

// audioSupported is false off iOS — no native audio engine, so the reading
// header omits the play button entirely rather than show a dead control.
func audioSupported() bool { return false }
