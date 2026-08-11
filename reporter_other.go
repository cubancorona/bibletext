//go:build !ios && !darwin

package bibletext

// reporterLayoutActive is false on Windows, Linux and Android. The styled
// desktop pane does its own width-gated reporter typesetting (see
// reading_styled_pane.go relayout) without consulting this; Android's HTML
// dialect keeps the phone layout.
func reporterLayoutActive() bool { return false }
