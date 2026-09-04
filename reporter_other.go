//go:build !ios && !darwin && !android

package bibletext

// reporterLayoutActive is false on Windows and Linux: the styled desktop pane
// does its own width-gated reporter typesetting (see reading_styled_pane.go
// relayout) without consulting this. Android answers for itself in
// reporter_android.go, where a phone in landscape reading mode takes the
// reporter page.
func reporterLayoutActive() bool { return false }
