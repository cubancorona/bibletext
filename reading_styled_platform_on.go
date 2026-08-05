//go:build !ios && !darwin && !android

package bibletext

// Windows/Linux: the styled, selectable pane IS the desktop reading pane.
// Flipping this to false instantly reverts to the chapterText/Entry pane —
// the one-line fallback the milestone-4 swap promised.
const styledPaneEnabledOnPlatform = true
