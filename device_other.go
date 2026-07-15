//go:build !ios

package bibletext

// deviceIsTablet is the non-iOS fallback. Desktop uses its own dedicated layout
// (ui_desktop.go), and Android tablets are not yet opted into the regular-width
// layout — both keep their existing behaviour, so this reports false everywhere
// off iOS. (An Android tablet idiom check can be added here later without
// touching the shared layout logic in layout.go.)
func deviceIsTablet() bool { return false }
