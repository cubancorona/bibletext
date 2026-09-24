//go:build darwin && !ios

package bibletext

// reporterLayoutActive reports whether the reading pane is on the book page —
// the U.S. Reports set (reading_page.go). By WIDTH, as on every surface: a
// window wide enough for the column and its margins reads the book page, and a
// narrow one the phone page. The width is the one the NSTextView's scroll view
// reports (btMacReadingWidthChanged); until it has reported one — and in the
// host tests, which have no pane — the book page, as this pane always drew.
func reporterLayoutActive() bool { return currentReadingPage().Book() }
