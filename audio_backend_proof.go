package bibletext

import "strings"

// WHAT COUNTS AS PROOF THAT A WINDOWS AUDIO BACKEND CARRIED THE SAMPLES.
//
// The decision lives here, with no build tag, so the test that holds it can
// run on every OS -- the probe that gathers the module list only exists on
// Windows (audio_backend_windows.go), but the reasoning about that list must
// be provable everywhere, or it is a claim rather than a check.
//
// It matters because the first version of the smoke gate got it wrong in a way
// that could never fire. It treated ANY of AUDIOSES.DLL, MMDevAPI.dll or
// winmm.dll being mapped as evidence, on the premise that the null sink loads
// none of them. oto's driver does not work that way (ebitengine/oto v3.4.0):
//
//   - newWASAPIContext CoCreateInstances the MMDeviceEnumerator -- which maps
//     MMDevAPI.dll -- BEFORE GetDefaultAudioEndPoint returns E_NOTFOUND
//     (driver_wasapi_windows.go:197, then :210-213), and never unmaps it.
//   - newWinMMContext calls winmm.Load() unconditionally BEFORE waveOutOpen
//     fails with no device (driver_winmm_windows.go:95, then :128-135).
//
// So on a headless runner the process that ends up with the silent nullContext
// has BOTH of those DLLs mapped, the "any of three" gate saw two names, logged
// a backend in use, and carried on -- the exact false green it was written to
// stop, now with an affirmative line in the log.
//
// AUDIOSES.DLL is different. It is the audio client's home, and oto only
// Activates an IAudioClient (driver_wasapi_windows.go:230) after a device has
// been found. A process that never got a device never maps it. That makes it
// the one module whose presence proves a WASAPI stream was set up, and the
// only positive evidence this decision accepts. MMDevAPI.dll and winmm.dll are
// reported as what was ATTEMPTED, which is the right diagnosis when the answer
// is no.
//
// A WinMM success leaves no DLL-only fingerprint (winmm.dll is mapped either
// way), and on the Windows floor this app ships to (10, build 19041) WASAPI is
// always present, so a machine where WinMM carried the samples and WASAPI did
// not is not a case this needs to prove. It is reported as unproven, not as
// success.

const audioProofModule = "AUDIOSES.DLL"

// audioBackendProof decides whether the loaded-module list proves that a real
// backend, not oto's silent nullContext, is carrying the samples. why is the
// diagnosis either way, written for the log of a headless runner.
func audioBackendProof(mods []string) (ok bool, why string) {
	has := func(name string) bool {
		for _, m := range mods {
			if strings.EqualFold(m, name) {
				return true
			}
		}
		return false
	}
	if has(audioProofModule) {
		return true, "WASAPI: " + audioProofModule + " is mapped, which oto loads only after an endpoint was found (" +
			strings.Join(mods, ", ") + ")"
	}
	if len(mods) == 0 {
		return false, "no audio backend module is mapped at all"
	}
	return false, "only " + strings.Join(mods, ", ") + " mapped: those are loaded on the ATTEMPT to find a " +
		"device and stay mapped when none is found, so this is oto's silent nullContext -- " +
		audioProofModule + " would be present if a WASAPI stream had been set up"
}
