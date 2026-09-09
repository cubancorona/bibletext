//go:build !android && !ios

package bibletext

// foregroundOverlayRecovery has a real implementation on the two platforms whose
// native reading overlay can be emptied behind the app's back: Android, where the
// ACTIVITY is recreated (reading_android.go), and iOS, where a long background can
// leave the UITextView holding nothing (overlay_recovery_ios.go).
//
// macOS keeps its NSTextView in a window the system never recreates, and the
// desktop builds do not background at all, so there is nothing to recover here.
func foregroundOverlayRecovery(state *AppState) {}
