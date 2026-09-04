//go:build !bibletextdev

package bibletext

// The phone-landscape launch seed, absent.
//
// THIS IS THE SHIPPING BUILD. The mode itself ships (phone_landscape.go reads
// its preferences in every build); only the launch seed the simulator script
// uses is dev-only, so a release binary has no environment switch for it.
// Same reasoning as dev_mimic_off.go; the release pipelines never pass the tag
// (TestReleaseScriptsNeverPassTheDevTag).

func devPhoneLandscapeSeedOn() bool { return false }

func devPhoneLandscapeSeedOff() bool { return false }

func devPhoneLandscapeSeedTypography() bool { return false }
