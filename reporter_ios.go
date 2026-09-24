//go:build ios

package bibletext

// reporterLayoutActive reports whether the reading pane is on the book page —
// the U.S. Reports set (reading_page.go). By WIDTH, as on every surface: an
// iPad or a landscape iPhone reads the book page because it is wide enough,
// and a narrow iPad window reads the phone page because it is not. Until the
// pane has reported its width the window's stands in for it.
func reporterLayoutActive() bool { return currentReadingPage().Book() }

func init() { readingWindowWidth = widestWindowWidth }
