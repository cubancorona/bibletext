//go:build !bibletextdev || !darwin || ios

package bibletext

// The shipping half of debug_capture_macos.go: nothing. Kept as a real symbol
// so the entry point can call it unconditionally and no build tag leaks into
// cmd/desktop.
func InstallDebugCapture() {}
