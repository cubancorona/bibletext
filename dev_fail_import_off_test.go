//go:build !bibletextdev

package bibletext

import "testing"

// A release build never asks the Apple panes to refuse an import, whatever
// the environment says (dev_fail_import_off.go). The control, a development
// build answering the same environment, is
// TestDevFailImportIsAskedForByTheEnvironment.
func TestDevFailImportIsInertInReleaseBuilds(t *testing.T) {
	for _, v := range []string{"1", "on", " ON "} {
		t.Setenv("BIBLETEXT_DEV_FAIL_IMPORT", v)
		if devFailImportsRequested() {
			t.Errorf("a release build asks the panes to refuse every import for BIBLETEXT_DEV_FAIL_IMPORT=%q", v)
		}
	}
}
