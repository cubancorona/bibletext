// Command desktop is the macOS / Windows / Linux entry point for the Holy Bible
// reader. It opens a sized window with the desktop layout (HSplit + sidebar +
// keyboard shortcuts).
//
//	go build -o holy-bible ./cmd/desktop && ./holy-bible
package main

import "holybible"

func main() {
	holybible.Run()
}
