package bibletext

// THE PHONE-LANDSCAPE GATES: preferences on by default in every build, a
// switch turns the presentation off and takes the typography half with it,
// the typography half is ANDed with the pane's reporter support, and a
// decision that answers only for a sized landscape canvas on a phone.

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

// The gates are preferences, on by default, that a switch can turn off; the
// typography half is read as AND with the presentation and with the pane's
// support for the reporter page (none on the host).
func TestPhoneLandscapeGatesDefaultOnAndSwitchOff(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	if !phoneLandscapeReadingEnabled() {
		t.Fatal("the presentation gate is off by default")
	}
	if phoneLandscapeTypographyEnabled() != (phoneLandscapeReadingEnabled() && phoneLandscapeTypographySupported()) {
		t.Fatal("the typography gate does not follow the presentation and the pane's support")
	}
	setPhoneLandscapeReadingEnabled(false)
	if phoneLandscapeReadingEnabled() || phoneLandscapeTypographyEnabled() {
		t.Fatal("turning the presentation off left a half on")
	}
	setPhoneLandscapeReadingEnabled(true)
	if !phoneLandscapeReadingEnabled() {
		t.Fatal("the stored presentation gate does not read back")
	}
	// The host is not a phone: the live decision is off whatever the gates say.
	if phoneLandscapeReadingActive() {
		t.Fatal("phoneLandscapeReadingActive() answered true on the host")
	}
}

// The typography default is on where a pane supports the reporter page: the
// host answers no support, so the seam stands in for iOS here.
func TestPhoneLandscapeTypographyDefaultsOnWhereSupported(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	orig := phoneLandscapeTypographySupported
	phoneLandscapeTypographySupported = func() bool { return true }
	defer func() { phoneLandscapeTypographySupported = orig }()
	if !phoneLandscapeTypographyEnabled() {
		t.Fatal("with the pane's support the typography half is off by default")
	}
	setPhoneLandscapeTypographyEnabled(false)
	if phoneLandscapeTypographyEnabled() {
		t.Fatal("the typography preference does not turn the half off")
	}
}

// The rotation anchor is captured only where the pane re-imports under a new
// grammar, never over a restore already held or an arrival in flight.
func TestRotationAnchorWanted(t *testing.T) {
	held := &restoreAnchor{Book: "John", Chapter: 3}
	for _, tc := range []struct {
		name    string
		needed  bool
		restore *restoreAnchor
		force   bool
		want    bool
	}{
		{"iOS, nothing pending", true, nil, false, true},
		{"Android: the bridge re-places", false, nil, false, false},
		{"a restore already captured", true, held, false, false},
		{"an arrival in flight", true, nil, true, false},
	} {
		if got := rotationAnchorWanted(tc.needed, tc.restore, tc.force); got != tc.want {
			t.Errorf("%s: wanted = %v, want %v", tc.name, got, tc.want)
		}
	}
}
