//go:build bibletextdev

package bibletext

import (
	"os"
	"strings"
)

// The phone-landscape gates exist in the dev build (phone_landscape.go).
const devPhoneLandscapeAvailable = true

// devPhoneLandscapeSeed is BIBLETEXT_DEV_PHONE_LANDSCAPE as the process was
// launched: "1" turns the presentation on, "typo" both halves. Read once, so a
// scripted simulator run (scripts/run-ios-sim.sh forwards it) starts in the
// mode without touching the preferences; the dev tab's switches still work on
// top of it.
var devPhoneLandscapeSeed = strings.ToLower(strings.TrimSpace(os.Getenv("BIBLETEXT_DEV_PHONE_LANDSCAPE")))

func devPhoneLandscapeSeedOn() bool {
	return devPhoneLandscapeSeed == "1" || devPhoneLandscapeSeed == "typo"
}

func devPhoneLandscapeSeedTypography() bool { return devPhoneLandscapeSeed == "typo" }
