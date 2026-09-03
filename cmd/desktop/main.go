// Command desktop is the macOS / Windows / Linux entry point for the BibleText
// reader. It opens a sized window with the shared Read / Books / Search layout,
// a leading navigation rail, and desktop keyboard shortcuts. The former HSplit
// and sidebar remain available only through the explicit diagnostic override.
//
//	go build -o bibletext ./cmd/desktop && ./bibletext
package main

import "github.com/cubancorona/bibletext"

func main() {
	bibletext.Run()
}
