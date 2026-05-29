//go:build ios || android

package holybible

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// CreateMainUI (mobile) lays the app out as three full-screen tabs across the
// bottom: Read, Books, Search. Phones don't have room for the desktop's HSplit,
// and iOS users don't expect a persistent sidebar — tapping a book or a search
// hit selects it and switches to the Read tab automatically.
//
// Like the desktop layout, navigation swaps the Read tab's content rather than
// rebuilding the chrome, so the search field never loses focus mid-keystroke.
func CreateMainUI(app fyne.App, state *AppState, window fyne.Window) fyne.CanvasObject {
	state.app = app
	state.window = window
	if state.theme == nil {
		state.theme = &bibleTheme{fonts: loadBookFonts()}
	}
	app.Settings().SetTheme(state.theme)
	pal := state.pal()

	// Mobile uses a different reading pane (RichText, not the desktop Entry-based
	// chapterText) — see reading_mobile.go for why.
	mobileReading := func() fyne.CanvasObject {
		if state.IsSearching {
			return buildSearchResultsView(state)
		}
		return buildReadingViewMobile(state)
	}
	readingHost := container.NewStack(mobileReading())
	state.showReading = func() {
		readingHost.Objects = []fyne.CanvasObject{mobileReading()}
		readingHost.Refresh()
	}

	// Tabs need to exist before the helpers that switch between them.
	var tabs *container.AppTabs
	gotoReadTab := func() {
		if tabs != nil {
			tabs.SelectIndex(0)
		}
	}
	// surfaceReading is called by state.openSearchResult so tapping a result
	// brings the reading pane forward instead of leaving the user on Search.
	state.surfaceReading = gotoReadTab

	booksTab := buildMobileBooksTab(state, gotoReadTab)
	searchTab := buildMobileSearchTab(state, gotoReadTab)

	// On mobile we don't have a sidebar to re-highlight; syncSidebar is a no-op.
	state.syncSidebar = func() {}

	tabs = container.NewAppTabs(
		container.NewTabItemWithIcon("Read", theme.DocumentIcon(), readingHost),
		container.NewTabItemWithIcon("Books", theme.MenuIcon(), booksTab),
		container.NewTabItemWithIcon("Search", theme.SearchIcon(), searchTab),
	)
	tabs.SetTabLocation(container.TabLocationBottom)
	// On iOS the reading text is rendered by a UITextView overlay; we have to
	// hide it when Books or Search is on screen, or it'd float on top of those
	// tabs. notifyReadingOverlay is a build-tag shim — no-op on Android.
	tabs.OnSelected = func(ti *container.TabItem) {
		if ti.Text == "Read" {
			notifyReadingOverlay(true)
		} else {
			notifyReadingOverlay(false)
		}
	}

	body := container.NewBorder(buildHeader(state), nil, nil, nil, tabs)

	base := canvas.NewRectangle(pal.Background)
	return container.NewStack(base, body)
}

// buildMobileBooksTab is a touch-sized, scrollable book list with a filter on
// top. Tapping a book selects it (resetting to its first chapter) and switches
// to the Read tab.
func buildMobileBooksTab(state *AppState, switchToRead func()) fyne.CanvasObject {
	pal := state.pal()

	filtered := filterBooks(state.Bible.Books, state.BookFilterQuery)

	bookFilter := widget.NewEntry()
	bookFilter.SetPlaceHolder("Filter books")
	bookFilter.SetText(state.BookFilterQuery)

	const mobileBookRowHeight = 44 // ≥ Apple's 44pt touch target

	list := widget.NewList(
		func() int { return len(filtered) },
		func() fyne.CanvasObject {
			label := canvas.NewText("", pal.Text)
			label.TextSize = 16
			return container.NewPadded(label)
		},
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			if i < 0 || i >= len(filtered) {
				return
			}
			label := obj.(*fyne.Container).Objects[0].(*canvas.Text)
			book := filtered[i]
			label.Text = book
			if book == state.CurrentBook {
				label.Color = pal.Accent
				label.TextStyle = fyne.TextStyle{Bold: true}
			} else {
				label.Color = pal.Text
				label.TextStyle = fyne.TextStyle{}
			}
			label.Refresh()
		},
	)
	for i := 0; i < len(filtered); i++ {
		list.SetItemHeight(widget.ListItemID(i), mobileBookRowHeight)
	}
	list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(filtered) {
			return
		}
		selectBook(state, filtered[id], true)
		state.refresh()
		switchToRead()
	}

	bookFilter.OnChanged = func(s string) {
		state.BookFilterQuery = s
		filtered = filterBooks(state.Bible.Books, s)
		for i := 0; i < len(filtered); i++ {
			list.SetItemHeight(widget.ListItemID(i), mobileBookRowHeight)
		}
		list.UnselectAll()
		list.Refresh()
	}

	header := container.NewVBox(
		sectionLabel("BOOKS", pal),
		inputFrame(bookFilter, pal.Border),
	)
	return container.NewBorder(container.NewPadded(header), nil, nil, nil, list)
}

// buildMobileSearchTab is the full-screen search experience. Live results
// populate as the user types; tapping a hit jumps to that verse in context and
// switches to the Read tab. An exact reference (e.g. "John 3:16") on Submit
// also jumps directly.
func buildMobileSearchTab(state *AppState, switchToRead func()) fyne.CanvasObject {
	pal := state.pal()

	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search…")
	searchEntry.SetText(state.SearchQuery)

	resultsHost := container.NewStack(buildSearchResultsView(state))

	// Reroute showReading so a tap on a search result repaints the results panel
	// here (if still on the search tab) and also primes the Read tab with the
	// chosen verse. The desktop version pipes this through the persistent
	// readingHost; on mobile both views need to refresh.
	previousShow := state.showReading
	state.showReading = func() {
		if previousShow != nil {
			previousShow()
		}
		resultsHost.Objects = []fyne.CanvasObject{buildSearchResultsView(state)}
		resultsHost.Refresh()
	}

	searchEntry.OnChanged = func(s string) {
		searchResultsOnly(state, s)
	}
	searchEntry.OnSubmitted = func(s string) {
		wasSearching := state.IsSearching
		executeSearch(state, s)
		// executeSearch jumps to a verse only when an exact ref was matched;
		// in that case IsSearching becomes false. Switch to Read so the jump
		// is visible.
		if wasSearching && !state.IsSearching {
			switchToRead()
		}
	}

	state.focusSearch = func() {
		if state.window != nil {
			state.window.Canvas().Focus(searchEntry)
		}
	}
	state.setSearchText = func(s string) {
		searchEntry.SetText(s)
	}

	header := container.NewVBox(
		sectionLabel("SEARCH", pal),
		inputFrame(searchEntry, pal.Border),
		caption("Keyword, or a reference like John 3:16."),
	)
	return container.NewBorder(container.NewPadded(header), nil, nil, nil, resultsHost)
}
