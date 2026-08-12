//go:build !bibletextdev

package bibletext

import "fyne.io/fyne/v2"

// The dev link-testing page, absent.
//
// THIS IS THE SHIPPING BUILD. Everything the page is made of lives in
// dev_links_on.go behind `//go:build bibletextdev`, so a release build does not
// merely hide it — the code is not compiled in at all. release-ios.sh and
// build-android.sh never pass the tag, which is what makes shipping it by
// accident impossible rather than merely unlikely. A runtime flag would have
// left the scenarios sitting inside the App Store binary.
//
// Build one with: scripts/run-ios-device.sh --dev  (or run-ios-sim.sh --dev)

const devLinksEnabled = false

func buildDevLinksTab(*AppState, func()) fyne.CanvasObject { return nil }
