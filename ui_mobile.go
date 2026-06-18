//go:build ios || android

package bibletext

import (
	"strings"

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
	registerAIState(state)
	if state.theme == nil {
		state.theme = &bibleTheme{fonts: loadBookFonts()}
	}
	applyTheme(app, state)
	pal := state.pal()

	// Startup: the Bible loads on a background goroutine, so until it's ready we
	// render only the loading/error screen and keep the native UITextView overlay
	// detached (there's no chapter to show yet, and pinning it over a tree with no
	// reading view is exactly the black-rectangle hazard).
	switch state.loadPhase {
	case loadPending:
		notifyReadingOverlay(false)
		return buildLoadingView(state)
	case loadFailed:
		notifyReadingOverlay(false)
		return buildLoadErrorView(state)
	}

	// Distraction-free reading mode: the entire window becomes the reading
	// pane plus a small exit affordance — no top header, no bottom tabs.
	// On iOS the native UITextView overlay therefore fills nearly the whole
	// screen.
	if state.IsFullScreen {
		base := canvas.NewRectangle(pal.Background)
		return container.NewStack(base, buildReadingViewMobile(state))
	}

	// gotoReadTab is used by Books/Search to jump back to the reading pane after
	// the user picks a book or a search result. We rebuild the window on every
	// tab change (reliable repaint — Fyne's in-place host-swap doesn't always
	// repaint a UITextView-overlaid tree) so this just sets the tab + rebuilds.
	gotoReadTab := func() {
		state.CurrentTab = 0
		rebuildWindow(state)
	}
	state.surfaceReading = gotoReadTab
	// "Back to results" returns to the real Search tab (restoring its query, results
	// and scroll position) rather than showing results inline in the reading pane.
	state.surfaceSearch = func() {
		state.CurrentTab = 2
		rebuildWindow(state)
	}

	// On mobile we don't have a sidebar to re-highlight; syncSidebar is a no-op.
	state.syncSidebar = func() {}

	// Build only the active tab's content — the others are constructed on
	// demand when the user switches (rebuildWindow re-runs CreateMainUI).
	var content fyne.CanvasObject
	switch state.CurrentTab {
	case 1:
		content = buildMobileBooksTab(state, gotoReadTab)
		notifyReadingOverlay(overlayShouldShow(state))
	case 2:
		content = buildMobileSearchTab(state, gotoReadTab)
		notifyReadingOverlay(overlayShouldShow(state))
	default: // 0 = Read
		readingHost := container.NewStack(rebuildMobileReadingPane(state))
		state.showReading = func() {
			readingHost.Objects = []fyne.CanvasObject{rebuildMobileReadingPane(state)}
			readingHost.Refresh()
			// rebuildMobileReadingPane swaps between the reading view and the
			// search-results list; the native overlay must only show over the
			// former, or it paints on top of the results.
			notifyReadingOverlay(overlayShouldShow(state))
		}
		// The Goto field is now a popup opened from the header's centered button
		// (showGotoPopup), so the reading view reserves no inline row.
		content = readingHost
		// When a search is active the Read tab shows the results list (Fyne), so
		// the native overlay has to stay hidden to avoid overlapping it.
		notifyReadingOverlay(overlayShouldShow(state))
	}

	tabBar := buildMobileTabBar(state)
	body := container.NewBorder(buildHeader(state), tabBar, nil, nil, content)

	base := canvas.NewRectangle(pal.Background)
	return container.NewStack(base, body)
}

// overlayShouldShow is the single source of truth for native reading-overlay
// visibility on mobile: the iOS UITextView must be visible exactly when the
// reading view is the content actually on screen — the Read tab with no active
// search, or distraction-free full-screen reading. Every place that toggles the
// overlay derives the answer from here, and afterRebuild re-asserts it as the
// last word after each window rebuild, so a stray async show/hide during the
// rebuild can't leave the overlay floating over the Books/Search tabs as a
// blank (black) rectangle.
func overlayShouldShow(state *AppState) bool {
	if state.IsFullScreen {
		return true
	}
	return state.CurrentTab == 0 && !state.IsSearching
}

// rebuildMobileReadingPane returns the search-results view when a search is
// active, otherwise the native reading view.
func rebuildMobileReadingPane(state *AppState) fyne.CanvasObject {
	if state.IsSearching {
		return buildSearchResultsView(state)
	}
	return buildReadingViewMobile(state)
}

// buildMobileTabBar renders the compact bottom tab strip. Selecting a tab sets
// state.CurrentTab and rebuilds the window. Each tab is a tabCell (icon + tiny
// label); the active one is accent-coloured.
func buildMobileTabBar(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	items := []struct {
		label string
		icon  fyne.Resource
	}{
		{"Read", theme.DocumentIcon()},
		{"Books", theme.MenuIcon()},
		{"Search", theme.SearchIcon()},
	}

	cells := make([]fyne.CanvasObject, len(items))
	for i, it := range items {
		i, it := i, it
		cell := newTabCell(state, it.icon, it.label, i == state.CurrentTab, func() {
			if state.CurrentTab == i {
				return
			}
			state.CurrentTab = i
			rebuildWindow(state)
		})
		cells[i] = cell
	}

	rule := canvas.NewLine(pal.Border)
	rule.StrokeWidth = 1
	bg := canvas.NewRectangle(pal.SurfaceAlt)
	row := container.NewGridWithColumns(len(items), cells...)
	return container.NewStack(bg, container.NewVBox(rule, container.NewPadded(row)))
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

// buildMobileSearchTab is the full-screen search experience. A "Find / Ask AI"
// toggle switches the single field between keyword search (live results as you
// type; an exact reference like "John 3:16" jumps on Submit) and natural-language
// AI search ("what did God say to Jonah?"), which returns relevant passages.
// Tapping any hit jumps to that verse in context and switches to the Read tab.
func buildMobileSearchTab(state *AppState, switchToRead func()) fyne.CanvasObject {
	pal := state.pal()

	resultsHost := container.NewStack()

	// --- Keyword search. ---
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search…")
	searchEntry.SetText(state.SearchQuery)

	// Reroute showReading so live, as-you-type keyword search repaints the results
	// panel here. We deliberately do NOT chain to the Read tab's showReading (which
	// drives the native overlay); the Read tab rebuilds fresh from state on switch.
	// In AI mode the results are rendered by the Ask handler, not live search.
	state.showReading = func() {
		notifyReadingOverlay(overlayShouldShow(state))
		if !state.aiSearchMode {
			resultsHost.Objects = []fyne.CanvasObject{buildSearchResultsView(state)}
			resultsHost.Refresh()
		}
	}

	onSearchChanged, stopSearchDebounce := newSearchDebouncer(state)
	searchEntry.OnChanged = onSearchChanged
	searchEntry.OnSubmitted = func(s string) {
		stopSearchDebounce() // Enter searches now; cancel the pending debounced run
		wasSearching := state.IsSearching
		executeSearch(state, s)
		if wasSearching && !state.IsSearching {
			switchToRead() // an exact ref jumped to a verse — show it
		}
	}

	state.focusSearch = func() {
		if state.window != nil {
			state.window.Canvas().Focus(searchEntry)
		}
	}
	state.setSearchText = func(s string) { searchEntry.SetText(s) }

	// --- Ask-AI search. ---
	aiEntry := widget.NewEntry()
	aiEntry.SetPlaceHolder("Ask for passages — e.g. what did God say to Jonah?")
	aiEntry.SetText(state.aiSearchQuery) // restore the last question on tab return

	var aiBar *widget.ProgressBarInfinite
	stopAIBar := func() {
		if aiBar != nil {
			aiBar.Stop()
			aiBar = nil
		}
	}

	var runAsk func(string)
	runAsk = func(q string) {
		q = strings.TrimSpace(q)
		if q == "" {
			return
		}
		state.searchScrollY = 0 // new results start at the top
		if !hasAIKey(state) {
			resultsHost.Objects = []fyne.CanvasObject{aiNoKeyView(state)}
			resultsHost.Refresh()
			return
		}
		bar := widget.NewProgressBarInfinite()
		aiBar = bar
		msg := canvas.NewText("Searching with AI…", pal.TextMuted)
		msg.Alignment = fyne.TextAlignCenter
		resultsHost.Objects = []fyne.CanvasObject{container.NewCenter(container.NewVBox(
			msg, spacer(8),
			container.NewGridWrap(fyne.NewSize(220, bar.MinSize().Height), bar),
		))}
		resultsHost.Refresh()

		startAISearch(state, q, func(verses []Verse, err error) {
			stopAIBar()
			switch {
			case err != nil && isNoKeyError(err):
				resultsHost.Objects = []fyne.CanvasObject{aiNoKeyView(state)}
			case err != nil:
				resultsHost.Objects = []fyne.CanvasObject{
					aiSearchMessageView(friendlyAIError(err), "Try again", func() { runAsk(q) }),
				}
			default:
				// Persist in state so the results survive a tab switch and power
				// "back to results".
				state.aiSearchActive = true
				state.aiSearchQuery = q
				state.aiSearchResults = verses
				resultsHost.Objects = []fyne.CanvasObject{aiResultsView(state, q, verses)}
			}
			resultsHost.Refresh()
		})
	}
	aiEntry.OnSubmitted = runAsk
	askBtn := widget.NewButtonWithIcon("", theme.SearchIcon(), func() { runAsk(aiEntry.Text) })
	askBtn.Importance = widget.LowImportance

	// --- Mode toggle + field swap (no window rebuild, so the keyboard survives). ---
	fieldHost := container.NewStack()
	var applyMode func()

	// X buttons clear the field and its results.
	clearKwBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		searchEntry.SetText("")
		stopSearchDebounce()
		executeSearch(state, "") // clears results immediately
		applyMode()
	})
	clearKwBtn.Importance = widget.LowImportance
	clearAskBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		aiEntry.SetText("")
		stopAIBar()
		state.aiSearchResults = nil
		state.aiSearchQuery = ""
		applyMode()
	})
	clearAskBtn.Importance = widget.LowImportance

	applyMode = func() {
		if state.aiSearchMode {
			fieldHost.Objects = []fyne.CanvasObject{
				container.NewBorder(nil, nil, nil, container.NewHBox(clearAskBtn, askBtn), inputFrame(withCaret(state, aiEntry), pal.Border)),
			}
			switch {
			case !hasAIKey(state):
				resultsHost.Objects = []fyne.CanvasObject{aiNoKeyView(state)}
			case len(state.aiSearchResults) > 0:
				// Restore the last AI results on tab return.
				resultsHost.Objects = []fyne.CanvasObject{aiResultsView(state, state.aiSearchQuery, state.aiSearchResults)}
			default:
				resultsHost.Objects = []fyne.CanvasObject{aiSearchPromptView(state)}
			}
		} else {
			stopAIBar()
			fieldHost.Objects = []fyne.CanvasObject{
				container.NewBorder(nil, nil, nil, clearKwBtn, inputFrame(withCaret(state, searchEntry), pal.Border)),
			}
			resultsHost.Objects = []fyne.CanvasObject{buildSearchResultsView(state)}
		}
		fieldHost.Refresh()
		resultsHost.Refresh()
	}

	toggle := buildSearchModeToggle(state, func(ai bool) {
		state.aiSearchMode = ai
		state.aiSearchActive = ai // switch the results context with the mode
		applyMode()
	})

	header := container.NewVBox(toggle, fieldHost)
	applyMode() // initialise to the persisted mode
	return container.NewBorder(container.NewPadded(header), nil, nil, nil, resultsHost)
}

// buildSearchModeToggle is a two-segment control switching the Search tab between
// keyword ("Find") and natural-language ("Ask AI") search; the active half is filled.
func buildSearchModeToggle(state *AppState, onSelect func(ai bool)) fyne.CanvasObject {
	var find, ask *widget.Button
	apply := func(ai bool) {
		find.Importance = widget.MediumImportance
		ask.Importance = widget.MediumImportance
		if ai {
			ask.Importance = widget.HighImportance
		} else {
			find.Importance = widget.HighImportance
		}
		find.Refresh()
		ask.Refresh()
	}
	find = widget.NewButton("Find", func() { apply(false); onSelect(false) })
	ask = widget.NewButton("Ask AI", func() { apply(true); onSelect(true) })
	apply(state.aiSearchMode)
	return container.NewGridWithColumns(2, find, ask)
}

// ----------------------------------------------------------------------------
// Custom bottom tab bar
// ----------------------------------------------------------------------------

// tabCell is one tappable icon+label slot inside the compact bottom bar. The
// bar itself is assembled in buildMobileTabBar; selecting a cell sets
// state.CurrentTab and rebuilds the window (reliable repaint).
type tabCell struct {
	widget.BaseWidget
	state    *AppState
	icon     fyne.Resource
	label    string
	active   bool
	onTapped func()

	iconImg *canvas.Image
	text    *canvas.Text
}

func newTabCell(state *AppState, icon fyne.Resource, label string, active bool, onTapped func()) *tabCell {
	c := &tabCell{state: state, icon: icon, label: label, active: active, onTapped: onTapped}
	c.ExtendBaseWidget(c)
	return c
}

func (c *tabCell) Tapped(*fyne.PointEvent) {
	if c.onTapped != nil {
		c.onTapped()
	}
}

func (c *tabCell) CreateRenderer() fyne.WidgetRenderer {
	pal := c.state.pal()
	tint := pal.TextMuted
	if c.active {
		tint = pal.Accent
	}

	// Tint the SVG icon to the same colour as the label by binding it to a
	// theme colour name (Primary for active, Foreground for inactive — both
	// are already correct in bibleTheme).
	c.iconImg = canvas.NewImageFromResource(c.themedIcon())
	c.iconImg.FillMode = canvas.ImageFillContain
	c.iconImg.SetMinSize(fyne.NewSize(20, 20))

	c.text = canvas.NewText(c.label, tint)
	c.text.TextSize = 10
	c.text.Alignment = fyne.TextAlignCenter
	c.text.TextStyle = fyne.TextStyle{Bold: c.active}

	col := container.NewVBox(
		container.NewCenter(c.iconImg),
		spacer(2),
		container.NewCenter(c.text),
	)
	return widget.NewSimpleRenderer(col)
}

// themedIcon returns the cell's icon as a colour-bound theme resource so it
// re-tints automatically with the active palette.
func (c *tabCell) themedIcon() fyne.Resource {
	if c.active {
		return theme.NewColoredResource(c.icon, theme.ColorNamePrimary)
	}
	// Inactive: muted foreground — we use the existing "muted" theme colour
	// name from theme.go (colorNameMuted), which bibleTheme resolves to
	// pal.TextMuted.
	return theme.NewColoredResource(c.icon, colorNameMuted)
}

// Compile-time interface checks: tab cells must be Tappable for the bottom
// bar to dispatch taps to them.
var _ fyne.Tappable = (*tabCell)(nil)
