//go:build bibletextdev

package bibletext

import (
	"os"
	"strings"
)

// THE WINDOWS AND LINUX PANE, JUSTIFIED OR RAGGED — for setting the two side by
// side on one build (reading_page.go, readingJustifyProse). DEVELOPMENT BUILDS
// ONLY: the release build has no file that sets the override, so a reader gets
// what the spec says.
//
// BIBLETEXT_DEV_JUSTIFY=on|off decides from launch; the Links tab's "Justify
// the Windows and Linux pane" box changes it while the app runs.

// devJustify is the forced answer: "on", "off", or "" for the spec.
var devJustify = devJustifyFrom(os.Getenv("BIBLETEXT_DEV_JUSTIFY"))

func devJustifyFrom(v string) string {
	switch v = strings.ToLower(strings.TrimSpace(v)); v {
	case "on", "off":
		return v
	}
	return ""
}

func init() {
	readingJustifyOverride = func() (bool, bool) {
		switch devJustify {
		case "on":
			return true, true
		case "off":
			return false, true
		}
		return false, false
	}
}
