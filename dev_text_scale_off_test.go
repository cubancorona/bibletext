//go:build !bibletextdev

package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// A release build cannot move the reading size between the three named ones:
// nothing sets the override, and the Read tab has no slider above its pane.
func TestReleaseBuildsHaveNoTextSizeSlider(t *testing.T) {
	if readingTextScaleOverride != nil {
		t.Fatal("a release build sets the text-size override")
	}
	if devTextScaleStrip(&AppState{}, nil) != nil {
		t.Fatal("a release build puts a text-size slider on the Read tab")
	}
	if readingPaneWidthSeen != nil {
		t.Fatal("a release build listens for the pane's width reports")
	}
}

// The chapter's fingerprint folds the scale the page is set at, not the
// setting's name, so a size between the names re-renders the native panes.
func TestTheFingerprintFollowsTheTextScale(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	st := sampleState()
	before := chapterRenderFingerprint(st)
	readingTextScaleOverride = func() (float64, bool) { return 1.07, true }
	defer func() { readingTextScaleOverride = nil }()
	if chapterRenderFingerprint(st) == before {
		t.Error("a new text scale left the chapter's fingerprint unchanged, so a native pane would not re-render")
	}
}
