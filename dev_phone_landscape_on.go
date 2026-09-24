//go:build bibletextdev

package bibletext

import (
	"os"
	"strings"
)

// devPhoneLandscapeSeed is BIBLETEXT_DEV_PHONE_LANDSCAPE as the process was
// launched: "on" or "1" forces the presentation on, "off" or "0" forces it
// off. ("typo" forced the typography half on as well, when there was one; the
// page is the width's now, and "typo" is read as "on" so an old command line
// still means what it meant for the presentation.) Read once, so a scripted simulator run
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
