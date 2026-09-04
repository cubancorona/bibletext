//go:build !bibletextdev

package bibletext

// The phone-landscape gates, absent.
//
// THIS IS THE SHIPPING BUILD. phoneLandscapeReadingEnabled (phone_landscape.go)
// returns false before it reads anything, so a preference key a dev install
// left behind cannot switch this build on, and the launch seed is not compiled
// in. Same reasoning as dev_mimic_off.go and dev_links_off.go; the release
// pipelines never pass the tag (TestReleaseScriptsNeverPassTheDevTag).

const devPhoneLandscapeAvailable = false

func devPhoneLandscapeSeedOn() bool { return false }

func devPhoneLandscapeSeedTypography() bool { return false }
