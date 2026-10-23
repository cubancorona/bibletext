//go:build !bibletextdev

package bibletext

// Forcing the Apple panes' fallback page, absent.
//
// This is the shipping build. Only dev_fail_import_on.go, behind
// `//go:build bibletextdev`, reads BIBLETEXT_DEV_FAIL_IMPORT; here the answer
// is false whatever the environment says, so a release build never asks the
// panes to refuse an import, and the switch they keep for it stays off.
// TestDevFailImportIsInertInReleaseBuilds holds that.

func devFailImportsRequested() bool { return false }
