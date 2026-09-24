//go:build bibletextdev

package bibletext

import (
	"os"
	"strings"
)

// THE READING PAGE, FORCED — for looking at either page on a pane of any width
// (reading_page.go, readingPageOverride). DEVELOPMENT BUILDS ONLY: the release
// build has no file that sets the override, so a reader always gets the page
// the width chooses.
//
// BIBLETEXT_DEV_READING_PAGE=book|phone forces a page from launch (a scripted
// simulator run forwards it: scripts/run-ios-sim.sh); the Links tab's "Reading
// page" choice changes it while the app runs, and "By width" hands the choice
// back to the rule.

// devReadingPage is the forced page: "book", "phone", or "" for the rule.
var devReadingPage = devReadingPageFrom(os.Getenv("BIBLETEXT_DEV_READING_PAGE"))

func devReadingPageFrom(v string) string {
	switch v = strings.ToLower(strings.TrimSpace(v)); v {
	case "book", "phone":
		return v
	}
	return ""
}

func init() {
	readingPageOverride = func() (readingPageKind, bool) {
		switch devReadingPage {
		case "book":
			return readingPageBook, true
		case "phone":
			return readingPagePhone, true
		}
		return 0, false
	}
}
