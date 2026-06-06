// Command desktop is the macOS / Windows / Linux entry point for the BibleText
// reader. It opens a sized window with the desktop layout (HSplit + sidebar +
// keyboard shortcuts).
//
//	go build -o bibletext ./cmd/desktop && ./bibletext
package main

import "bibletext"

func main() {
	bibletext.Run()
}
