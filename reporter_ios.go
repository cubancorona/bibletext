//go:build ios

package bibletext

// reporterLayoutActive reports whether the reading pane is on the book page —
// the U.S. Reports set (reading_page.go). By WIDTH, as on every surface: an
// iPad or a landscape iPhone reads the book page because it is wide enough,
// and a narrow iPad window reads the phone page because it is not. Until the
// pane has reported its width the window's stands in for it, and until the
// window has a size — the canvas has none before its first paint — the
// device's resting page does: an iPhone's is the phone page, an iPad's the
// book page (readingUnsizedPage).
func reporterLayoutActive() bool { return currentReadingPage().Book() }

func init() {
	readingWindowWidth = widestWindowWidth
	readingUnsizedPage = func() readingPageKind {
		if deviceIsTablet() {
			return readingPageBook
		}
		return readingPagePhone
	}
}
