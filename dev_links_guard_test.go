//go:build !bibletextdev

package bibletext

// The guard that keeps the dev link-testing page out of the App Store.
//
// This file is itself untagged-for-release (`!bibletextdev`), so it runs in the
// ordinary `go test ./...` — the same run CI and every developer does — and
// fails the moment a release-shaped build can see the page.

import (
	"os"
	"strings"
	"testing"
)

func TestDevLinksAbsentFromReleaseBuilds(t *testing.T) {
	if devLinksEnabled {
		t.Fatal("devLinksEnabled is true without the bibletextdev tag — the dev page would ship")
	}
	if got := buildDevLinksTab(nil, nil); got != nil {
		t.Fatalf("buildDevLinksTab returned %T in a release build; want nil", got)
	}
}

// The release pipelines must never pass the tag. A well-meaning "make the build
// scripts consistent" edit is exactly how a dev-only page reaches production, so
// the scripts are asserted rather than trusted.
func TestReleaseScriptsNeverPassTheDevTag(t *testing.T) {
	for _, path := range []string{
		"scripts/release-ios.sh",
		"scripts/build-android.sh",
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if strings.Contains(string(b), "bibletextdev") {
			t.Errorf("%s mentions the bibletextdev tag — a release build must never compile the dev page", path)
		}
	}
}

// The dev-only scripts must still OFFER it, or the flag silently stops working
// and the page becomes untestable without anyone noticing.
func TestDevScriptsOfferTheDevFlag(t *testing.T) {
	for _, path := range []string{
		"scripts/run-ios-device.sh",
		"scripts/run-ios-sim.sh",
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		s := string(b)
		if !strings.Contains(s, "--dev") || !strings.Contains(s, "bibletextdev") {
			t.Errorf("%s no longer wires up --dev / bibletextdev", path)
		}
	}
}
