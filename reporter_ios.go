//go:build ios

package bibletext

// reporterLayoutActive reports whether the reading pane should use the
// U.S. Reports-style page layout (see the reporter* constants in reading.go):
// iPads only — phones keep the airier compact styling, and Android tablets
// wait until their native overlay grows the matching inset support.
func reporterLayoutActive() bool { return deviceIsTablet() }
