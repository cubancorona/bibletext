//go:build !android

package bibletext

// foregroundOverlayRecovery is Android-only (reading_android.go): Android can
// RECREATE the activity while the app is backgrounded (swipe-away with the
// process kept alive by the audio service, rotation, memory pressure), leaving
// the native reading overlay blank until re-driven. The Apple platforms keep
// their native text views in the app's own (never-recreated) window, so the
// foreground hook has nothing to recover there; desktop has no backgrounding.
func foregroundOverlayRecovery(state *AppState) {}
