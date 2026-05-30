package holybible

import (
	"fmt"
	"os"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

// LoadAndPrepareState loads the Bible (from cache, or the API on first run) and
// returns a fully-initialised AppState ready to hand to CreateMainUI. A first-run
// download is announced on stdout; any failure is fatal and exits the process.
//
// This is exported so each cmd/* entry point can do the same load before opening
// its platform-specific window (a desktop window vs. a Fyne mobile window).
func LoadAndPrepareState() *AppState {
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
		Annotations:    NewAnnotationStore(),
	}
	if chapters := bibleData.GetChapterNumbersForBook(state.CurrentBook); len(chapters) > 0 {
		state.CurrentChapter = chapters[0]
	}
	addRecentChapter(state, state.CurrentBook, state.CurrentChapter)
	return state
}

// Run is the desktop entry: loads the data, opens a sized window, and starts the
// event loop. Mobile entries (Fyne iOS) use the same data path but configure the
// window differently — see cmd/mobile/main.go.
func Run() {
	myApp := app.NewWithID("holy-bible")
	state := LoadAndPrepareState()

	window := myApp.NewWindow("Holy Bible — World English Bible")
	window.Resize(fyne.NewSize(1280, 860))
	window.SetContent(CreateMainUI(myApp, state, window))
	ObserveSystemThemeChanges(myApp, state)
	window.ShowAndRun()
}

// systemThemeOnce guarantees we install the system-appearance listener exactly
// once per process — both cmd/desktop (via Run) and cmd/mobile call
// ObserveSystemThemeChanges, and we don't want stacked subscribers.
var systemThemeOnce sync.Once

// ObserveSystemThemeChanges subscribes to Fyne's settings-change channel so a
// system light/dark switch rebuilds the window content. Fyne re-runs Color()
// automatically when the variant changes, but anything generated outside the
// theme callback (like the HTML the iOS UITextView consumes) is stale until we
// rebuild the tree.
//
// CreateMainUI calls app.Settings().SetTheme() on every build, which ALSO
// fires this listener — so we must guard against a rebuild loop by only acting
// when the actual light/dark variant has changed since the last rebuild.
func ObserveSystemThemeChanges(myApp fyne.App, state *AppState) {
	systemThemeOnce.Do(func() {
		ch := make(chan fyne.Settings, 1)
		myApp.Settings().AddChangeListener(ch)
		lastVariant := myApp.Settings().ThemeVariant()
		go func() {
			for range ch {
				v := myApp.Settings().ThemeVariant()
				if v == lastVariant {
					continue // theme object changed but not the variant — ignore
				}
				lastVariant = v
				fyne.Do(func() {
					if state.window != nil && state.app != nil {
						state.window.SetContent(CreateMainUI(state.app, state, state.window))
					}
				})
			}
		}()
	})
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
