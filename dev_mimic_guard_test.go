//go:build !bibletextdev

package bibletext

// The guard that keeps the platform-mimic mode out of the App Store — the
// dev_links_guard_test.go arrangement: this file is untagged-for-release
// (`!bibletextdev`), so it runs in the ordinary `go test ./...` — the same run
// CI and every developer does — and fails the moment a release-shaped build
// can see the switch. (TestReleaseScriptsNeverPassTheDevTag already pins that
// the release pipelines never pass the tag.)

import "testing"

func TestMimicAbsentFromReleaseBuilds(t *testing.T) {
	// The environment variable set exactly as a mimic launch would set it: in a
	// release-shaped build the code that reads it is not compiled in, so
	// nothing may move.
	t.Setenv("BIBLETEXT_MIMIC", "windows")
	devApplyMimic()

	if got := devMimicTarget(); got != "" {
		t.Fatalf("devMimicTarget() = %q in a release build — the mimic switch would ship", got)
	}
	if got := devMimicLabel(); got != "" {
		t.Fatalf("devMimicLabel() = %q in a release build — the badge (and the mode behind it) would ship", got)
	}
	// Every seam the mimic flips must still answer with its platform constant.
	// Compared against the constants rather than literals so this passes on any
	// host GOOS the suite runs on.
	if useStyledPane() != styledPaneEnabledOnPlatform {
		t.Error("useStyledPane moved off its platform constant in a release build")
	}
	if sheetConsumeClosure() != sheetConsumeClosureOnPlatform {
		t.Error("sheetConsumeClosure moved off its platform constant in a release build")
	}
	// nativeNoteSticker is composed, not a bare constant: since the styled pane
	// grew its own in-text sticker the seam asks "does the pane draw the note
	// itself?" (notes_banner.go), so the release answer is the constant OR the
	// styled pane's own constant. Compared against that composition for the
	// same reason the others are compared against constants — it must hold on
	// every host GOOS the suite runs on, and on Windows/Linux the two halves
	// disagree.
	if nativeNoteSticker() != (nativeNoteStickerOnPlatform || styledPaneEnabledOnPlatform) {
		t.Error("nativeNoteSticker moved off its platform answer in a release build")
	}
	if ttsSupported() != ttsSupportedOnPlatform {
		t.Error("ttsSupported moved off its platform constant in a release build")
	}
}
