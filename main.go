package main

import (
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

// main loads the Bible (from cache, or the API on first run) and opens the
// reading window. Detailed progress is printed only on a first-run download.
func main() {
	myApp := app.NewWithID("holy-bible")

	cachePath := defaultCachePath()
	bibleData, source, err := loadBibleData(FetchBibleFromAPI, cachePath, currentUTCTime)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Holy Bible failed to load:", err)
		os.Exit(1)
	}
	if source == "api" {
		fmt.Printf("Downloaded the World English Bible (%d books) and saved a local cache.\n", len(bibleData.Books))
	}

	state := &AppState{
		Bible:          bibleData,
		CurrentBook:    defaultStartBook(bibleData),
		CurrentChapter: 1,
	}
	if chapters := bibleData.GetChapterNumbersForBook(state.CurrentBook); len(chapters) > 0 {
		state.CurrentChapter = chapters[0]
	}
	addRecentChapter(state, state.CurrentBook, state.CurrentChapter)

	window := myApp.NewWindow("Holy Bible — World English Bible")
	window.Resize(fyne.NewSize(1280, 860))
	window.SetContent(CreateMainUI(myApp, state, window))
	window.ShowAndRun()
}

// defaultStartBook opens on John when available, else the first loaded book.
func defaultStartBook(bd *BibleData) string {
	if bd.GetChaptersForBook("John") > 0 {
		return "John"
	}
	if len(bd.Books) > 0 {
		return bd.Books[0]
	}
	return "John"
}

func currentUTCTime() time.Time {
	return time.Now().UTC()
}
