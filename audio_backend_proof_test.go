package bibletext

import (
	"strings"
	"testing"
)

// The gate must FAIL on the shape a headless runner actually produces. That
// shape is not "no modules" -- it is the two modules oto maps on the way to
// discovering there is no device. A gate that only fails on an empty list can
// never fire where it matters, which is how the first version shipped.
func TestAudioProofRejectsTheNullSinksOwnFingerprint(t *testing.T) {
	cases := []struct {
		name string
		mods []string
		want bool
	}{
		{"headless runner: both attempt DLLs mapped, no client", []string{"MMDevAPI.dll", "winmm.dll"}, false},
		{"WinMM attempt alone", []string{"winmm.dll"}, false},
		{"MMDevAPI attempt alone", []string{"MMDevAPI.dll"}, false},
		{"nothing mapped", nil, false},
		{"a real WASAPI stream", []string{"AUDIOSES.DLL", "MMDevAPI.dll"}, true},
		{"a real WASAPI stream, odd casing", []string{"audioses.dll", "MMDevAPI.dll", "winmm.dll"}, true},
	}
	for _, c := range cases {
		ok, why := audioBackendProof(c.mods)
		if ok != c.want {
			t.Errorf("%s: proof=%v, want %v (%s)", c.name, ok, c.want, why)
		}
		if why == "" {
			t.Errorf("%s: no diagnosis returned; the log is the only evidence a runner leaves", c.name)
		}
	}
}

// The diagnosis for the null sink must say what WAS mapped and why that is
// not enough, because that line is what a person reads on a red run.
func TestAudioProofDiagnosisNamesTheAttemptedModules(t *testing.T) {
	_, why := audioBackendProof([]string{"MMDevAPI.dll", "winmm.dll"})
	for _, want := range []string{"MMDevAPI.dll", "winmm.dll", "AUDIOSES.DLL", "nullContext"} {
		if !strings.Contains(why, want) {
			t.Errorf("diagnosis does not mention %q: %s", want, why)
		}
	}
}
