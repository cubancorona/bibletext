//go:build audiosmoke && !windows

package bibletext

// The null-sink question is Windows-specific: it is oto's Windows driver that
// substitutes a silent context when no endpoint is found (see
// audio_backend_windows.go). On Linux the engine goes through ALSA, and a
// missing device surfaces as an error rather than as silence, so there is no
// equivalent fingerprint to read and the smoke test's own failures are already
// truthful.
//
// Reported as "not probed" rather than as "fine", so the caller distinguishes
// a backend it checked from one it never asked about.
func audioBackendModules() (loaded []string, probed bool) { return nil, false }
