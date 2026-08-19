//go:build !bibletextdev

package bibletext

// The platform-mimic switch, absent.
//
// THIS IS THE SHIPPING BUILD. Everything the mimic mode is made of lives in
// dev_mimic_on.go behind `//go:build bibletextdev`, so a release binary does
// not merely ignore BIBLETEXT_MIMIC — the code that reads it is not compiled
// in. Same reasoning as dev_links_off.go / dev_autoopen_off.go: a runtime flag
// would leave a way to run the App Store build down untested cross-platform
// paths. The release pipelines never pass the tag
// (TestReleaseScriptsNeverPassTheDevTag), and dev_mimic_guard_test.go proves a
// release-shaped build cannot see the switch at all.
//
// devMimicTarget returns a constant "", so the mimic branches in untagged code
// (reading_macos.go's share verbs, the header badge) are dead code here.

func devApplyMimic() {}

func devMimicTarget() string { return "" }

func devMimicLabel() string { return "" }
