//go:build ios

package bibletext

// reporterLayoutActive reports whether the reading pane should use the
// U.S. Reports-style page layout (see the reporter* constants in reading.go):
// iPads, and an iPhone in landscape when both halves of the gated landscape
// reading mode are on (phone_landscape.go). Phones in portrait keep the airier
// compact styling, and Android tablets wait until their native overlay grows
// the matching inset support.
func reporterLayoutActive() bool {
	return deviceIsTablet() || (phoneLandscapeTypographyEnabled() && phoneLandscapeReading())
}
