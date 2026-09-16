//go:build darwin && !bibletextdev

package bibletext

import (
	"os"
	"path/filepath"
	"testing"
)

// A darwin release build must never listen: the Mac App Store sandbox forbids
// network.server, and the entitlements file says nothing listens. This is the
// build the Mac Store ships (the release scripts never pass the dev tag).
func TestSingleInstanceAbsentFromDarwinReleaseBuilds(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if forwardToRunningInstance("https://bibletext.co.uk/web/john/3/#v16") {
		t.Fatal("a darwin release build tried to forward to another instance")
	}
	stop, forwarded := claimSingleInstance(NewLoadingState(), "")
	defer stop()
	if forwarded || singleInstanceListening() {
		t.Fatal("a darwin release build is listening or forwarding")
	}
	matches, _ := filepath.Glob(filepath.Join(home, "Library", "Caches", "bibletext", "single-instance.*"))
	if len(matches) != 0 {
		t.Fatalf("a record was written: %v", matches)
	}
	if _, err := os.Stat(filepath.Join(home, "Library", "Caches", "bibletext")); err == nil {
		t.Error("the single-instance directory was created")
	}
}
