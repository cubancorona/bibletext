//go:build !ios

package bibletext

// reporterLayoutActive is false off-iOS: the desktop/Android panes don't have
// the native inset plumbing that centers the reporter measure yet.
func reporterLayoutActive() bool { return false }
