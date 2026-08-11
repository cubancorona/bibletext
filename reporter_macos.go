//go:build darwin && !ios

package bibletext

// reporterLayoutActive on macOS: always. The desktop window is a book page
// (owner directive: desktop parity with the iPad), and the native measure
// centering handles a narrow window the same way a narrow iPad multitasking
// column is handled — the side margins floor out and the column just fills
// what is there. The 27.5em measure itself is applied by
// bibleTextMacSetReadingMeasure (reading_macos.go), the NSTextView twin of the
// iPad's bibleTextSetReadingMeasure.
func reporterLayoutActive() bool { return true }
