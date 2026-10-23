//go:build bibletextdev

package bibletext

// Forcing the Apple panes' fallback page, in development builds only; see
// dev_fail_import_off.go.
//
// The iPhone, iPad and Mac show the page Go builds in
// reading_apple_fallback.go only when the system's HTML importer fails three
// times for one chapter, which cannot be arranged from outside the app. With
// BIBLETEXT_DEV_FAIL_IMPORT=1 in the environment of a development build, the
// panes refuse every import (bibleTextIOSFailImports,
// bibleTextMacFailImports), so every chapter opens on the fallback page and a
// simulator or a scratch Mac build can be checked on it:
//
//	SIMCTL_CHILD_BIBLETEXT_DEV_FAIL_IMPORT=1 xcrun simctl launch <udid> <bundle id>

import (
	"os"
	"strings"
)

// devFailImportsRequested reports whether this run asked for every import to
// fail.
func devFailImportsRequested() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BIBLETEXT_DEV_FAIL_IMPORT"))) {
	case "1", "on":
		return true
	}
	return false
}
