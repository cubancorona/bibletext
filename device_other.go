//go:build !ios && !android

package bibletext

// deviceIsTablet is the desktop/wasm fallback: desktop uses its own dedicated
// shell (ui_desktop.go), so mobile tablet detection never runs
// there. iOS answers via the UIKit idiom (device_ios.go); Android via the
// sw600dp-style dimension heuristic (device_android.go).
func deviceIsTablet() bool { return false }

// Off Android there is no phone-specific landscape-rail policy.
func phoneLandscapeNavRail() bool { return false }

// No phone, no landscape reading mode (phone_landscape.go).
func phoneLandscapeReadingSupported() bool { return false }

var phoneLandscapeTypographySupported = func() bool { return false }

func rotationRestoreNeeded() bool { return false }

// layoutMayChange gates installing the layoutWatcher; moot off mobile.
func layoutMayChange() bool { return false }
