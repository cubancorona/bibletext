package bibletext

// THE PHONE-LANDSCAPE GATES: off by default, unreadable in a release build,
// and a decision that answers only for a sized landscape canvas on a phone.

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestPhoneLandscapeReadingWanted(t *testing.T) {
	for _, tc := range []struct {
		name           string
		phone, enabled bool
		w, h           float32
		want           bool
	}{
		{"iPhone landscape, gate on", true, true, 874, 402, true},
		{"iPhone portrait, gate on", true, true, 402, 874, false},
		{"iPhone landscape, gate off", true, false, 874, 402, false},
		{"iPad landscape, gate on", false, true, 1366, 1024, false},
		{"unsized canvas, gate on", true, true, 0, 0, false},
		{"square canvas counts as landscape", true, true, 400, 400, true},
	} {
		if got := phoneLandscapeReadingWanted(tc.phone, tc.enabled, tc.w, tc.h); got != tc.want {
			t.Errorf("%s: wanted = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The gates are preferences only a dev build can read. In the release shape
// (the plain `go test ./...` run) a stored key must change nothing; in the dev
// shape the setters round-trip and the typography half is read as AND.
func TestPhoneLandscapeGatesFollowTheBuild(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	if phoneLandscapeReadingEnabled() || phoneLandscapeTypographyEnabled() {
		t.Fatal("the phone-landscape gates are on by default")
	}
	setPhoneLandscapeTypographyEnabled(true)
	if phoneLandscapeTypographyEnabled() {
		t.Fatal("the typography half reads true without the presentation half")
	}
	setPhoneLandscapeReadingEnabled(true)
	if devPhoneLandscapeAvailable {
		if !phoneLandscapeReadingEnabled() || !phoneLandscapeTypographyEnabled() {
			t.Fatal("dev build: the stored gates do not read back")
		}
		setPhoneLandscapeReadingEnabled(false)
		if phoneLandscapeReadingEnabled() || phoneLandscapeTypographyEnabled() {
			t.Fatal("dev build: turning the presentation off left a half on")
		}
	} else if phoneLandscapeReadingEnabled() || phoneLandscapeTypographyEnabled() {
		t.Fatal("release build: a stored preference switched the phone-landscape mode on")
	}
	// The host is not a phone: the live decision is off whatever the gates say.
	if phoneLandscapeReadingActive() {
		t.Fatal("phoneLandscapeReadingActive() answered true on the host")
	}
}
