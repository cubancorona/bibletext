//go:build !bibletextdev

package bibletext

import "fyne.io/fyne/v2"

// The text-size slider, absent.
//
// THIS IS THE SHIPPING BUILD. Nothing here sets readingTextScaleOverride, so a
// reader gets the three named text sizes and nothing between them, and the
// Read tab has no strip above its pane (dev_text_scale_on.go). The release
// pipelines never pass the tag (TestReleaseScriptsNeverPassTheDevTag).

func devTextScaleStrip(*AppState, fyne.CanvasObject) fyne.CanvasObject { return nil }
