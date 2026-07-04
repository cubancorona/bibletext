//go:build android

package bibletext

// Reading-position persistence hooks (Android). captureReadingAnchor and
// armReadingRestore live in reading_android.go — they call the bridge's C
// helpers, which are static to that file's cgo preamble. Only the inert
// initial-touch hooks live here.
//
// The initial-touch "you left off here" marker is an iOS-only feature (needs
// touch capture the Fyne Android driver doesn't surface; the feature is also
// globally off — touchResumeEnabled=false). Inert here, as on desktop.

func captureLastTouch() (verse int, delta float64, ok bool) { return 0, 0, false }

func armReadingMarker(verse int, r, g, b float64) {}
