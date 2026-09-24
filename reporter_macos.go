//go:build darwin && !ios

package bibletext

// reporterLayoutActive reports whether the reading pane is on the book page —
// the U.S. Reports set (reading_page.go). By WIDTH, as on every surface: a
// window wide enough for the column and its margins reads the book page, and a
// narrow one the phone page. The width is the one the NSTextView's scroll view
// reports (btMacReadingWidthChanged). The pane is handed its chapter and shows
// it before that report can land, so until it has reported, the window's width
// stands in (a desktop window has its size from the start): a narrow window
// opens on the phone page rather than importing the book page first. The
// window is wider than the pane by the rail beside it, so a window just over
// the switch still opens on the book page and corrects it once the pane
// reports; so does a rebuild that changes the pane's width and not the
// window's (full-screen reading dropping the rail). With no window — the host
// tests — the book page, as this pane always drew.
func reporterLayoutActive() bool { return currentReadingPage().Book() }

func init() { readingWindowWidth = widestWindowWidth }
