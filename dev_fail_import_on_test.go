//go:build bibletextdev

package bibletext

import "testing"

// A development build asks the Apple panes to refuse every import when, and
// only when, BIBLETEXT_DEV_FAIL_IMPORT says so. It is the control for
// TestDevFailImportIsInertInReleaseBuilds.
func TestDevFailImportIsAskedForByTheEnvironment(t *testing.T) {
	for v, want := range map[string]bool{"1": true, "on": true, " ON ": true, "": false, "0": false, "off": false} {
		t.Setenv("BIBLETEXT_DEV_FAIL_IMPORT", v)
		if got := devFailImportsRequested(); got != want {
			t.Errorf("BIBLETEXT_DEV_FAIL_IMPORT=%q: asked=%v, want %v", v, got, want)
		}
	}
}
