//go:build android

package bibletext

// reporterLayoutActive reports whether the reading pane is on the book page —
// the U.S. Reports set (reading_page.go). By WIDTH, as on every surface: a
// phone in landscape and a tablet read the book page because they are wide
// enough, and a phone in portrait or a narrow split-screen window reads the
// phone page because it is not. The width is the overlay's, in dp, reported by
// the bridge (BtBridge's content layout listener, btaReadingWidthChanged);
// until it has reported, the window's stands in for it.
//
// The page reaches the pane by three routes and all three ask THIS question:
// the paragraph grammar is markup (android_chapter_html.go), the measure is
// pushed to the bridge and centred there (reading_android.go,
// BtBridge.applyReadingPadding), and the pitch is pushed with the style.
func reporterLayoutActive() bool { return currentReadingPage().Book() }

func init() { readingWindowWidth = widestWindowWidth }
