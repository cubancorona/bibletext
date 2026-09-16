//go:build !windows && !linux && !bibletextdev

package bibletext

// Release builds for macOS (and the mobile platforms, which have their own
// delegates): no listener, no record, no forwarding. The Mac App Store
// sandbox forbids network.server, and LaunchServices single-instances the
// app already. See single_instance_on.go for the desktops that need it.

func forwardToRunningInstance(string) bool { return false }

func claimSingleInstance(*AppState, string) (func(), bool) { return func() {}, false }

func singleInstanceListening() bool { return false }
