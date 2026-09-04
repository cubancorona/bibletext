//go:build bibletextdev

package bibletext

import (
	"os"
	"strings"
)

// devPhoneLandscapeSeed is BIBLETEXT_DEV_PHONE_LANDSCAPE as the process was
// launched: "on" or "1" forces the presentation on and leaves the typography
// half to its preference (on by default), "typo" forces both halves on, "off"
// or "0" forces the mode off. Read once, so a scripted simulator run
// (scripts/run-ios-sim.sh forwards it) starts in a known state without
// touching the preferences. A seed wins over the stored preferences —
// phoneLandscapeReadingEnabled reads it first — so the dev tab's switches
// change nothing while one is set; clear the variable to hand control back.
var devPhoneLandscapeSeed = strings.ToLower(strings.TrimSpace(os.Getenv("BIBLETEXT_DEV_PHONE_LANDSCAPE")))

func devPhoneLandscapeSeedOn() bool {
	return devPhoneLandscapeSeed == "on" || devPhoneLandscapeSeed == "1" || devPhoneLandscapeSeed == "typo"
}

func devPhoneLandscapeSeedOff() bool {
	return devPhoneLandscapeSeed == "off" || devPhoneLandscapeSeed == "0"
}

func devPhoneLandscapeSeedTypography() bool { return devPhoneLandscapeSeed == "typo" }
