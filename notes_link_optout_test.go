package bibletext

import (
	"runtime"
	"strings"
	"testing"
)

// The Settings sheet is shared code, and the sentence about turning link
// handling off names a place that only exists on one platform. It named iOS's
// on every platform — so an Android reader was sent to "iOS Settings", which is
// not a place, and a desktop reader to a phone.
//
// Pinned per platform rather than "does it mention iOS", because the failure
// was not a missing check; it was one string standing in for three answers.
func TestTheLinkOptOutSentenceNamesTheRightPlace(t *testing.T) {
	for _, c := range []struct {
		goos        string
		mustHave    string
		mustNotHave []string
	}{
		{"ios", "iOS Settings", []string{"Android", "Open by default"}},
		{"android", "Open by default", []string{"iOS"}},
	} {
		got := notesLinkOptOutSentence(c.goos)
		if !strings.Contains(got, c.mustHave) {
			t.Errorf("%s: sentence must name %q; got %q", c.goos, c.mustHave, got)
		}
		for _, bad := range c.mustNotHave {
			if strings.Contains(got, bad) {
				t.Errorf("%s: sentence names another platform's %q: %q", c.goos, bad, got)
			}
		}
	}

	// Desktop says nothing. There is no per-app link toggle on macOS, Windows or
	// Linux, and a plausible-sounding place a reader cannot find is worse than
	// silence — the surrounding sentence is already true without it.
	for _, goos := range []string{"darwin", "windows", "linux"} {
		if got := notesLinkOptOutSentence(goos); got != "" {
			t.Errorf("%s: desktop should add nothing, got %q", goos, got)
		}
	}

	// CONTROL: the function really does vary. Without this the test would pass
	// against a stub that returned "" for everything.
	if notesLinkOptOutSentence("ios") == notesLinkOptOutSentence("android") {
		t.Fatal("control: the sentence does not vary by platform at all")
	}

	// And the platform this build is FOR gets a sentence that fits it, so the
	// wiring to runtime.GOOS cannot be quietly dropped.
	switch runtime.GOOS {
	case "ios", "android":
		if notesLinkOptOutSentence(runtime.GOOS) == "" {
			t.Errorf("%s builds must carry the sentence", runtime.GOOS)
		}
	}
}
