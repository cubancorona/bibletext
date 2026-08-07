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

// dismissKeyboard drops focus from whatever field owns the soft keyboard, which makes
// Fyne's mobile driver hide it (canvas OnUnfocus → hideVirtualKeyboard). Used after a
// search/ask is submitted from the keyboard so the results get the full pane instead of
// sitting behind the keyboard.
func dismissKeyboard(state *AppState) {
	if state != nil && state.window != nil {
		state.window.Canvas().Unfocus()
	}
}

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
		state.theme = &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
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
	// screen. (Layout-agnostic: a tablet in full-screen reading looks the same
	// as a phone, so there is no sidebar to add here.)
	if state.IsFullScreen {
		// The state hooks MUST be rewired to THIS tree (review finding): with
		// narration playing, a chapter's natural end calls state.refresh() from
		// advanceAndContinue — if showReading still pointed at the previous
		// build's detached host, the fresh nativeReadingHost would be built into
		// that dead tree, steal the currentHost singleton, and leave the visible
		// overlay unframeable (pushFrame bails on currentHost != h) with a stale
		// corner label. Desktop installs its hooks before its full-screen branch
		// for the same reason.
		readingHost := container.NewStack(buildReadingViewMobile(state))
		state.showReading = func() {
			readingHost.Objects = []fyne.CanvasObject{buildReadingViewMobile(state)}
			readingHost.Refresh()
			notifyReadingOverlay(overlayShouldShow(state))
		}
		state.syncSidebar = func() {}
		base := canvas.NewRectangle(pal.Background)
		return container.NewStack(base, readingHost)
	}

	// Pick the layout from the live canvas width: a wide-enough tablet gets the
	// regular sidebar+split layout (ui_regular.go); everything else (phones, and
	// a tablet squeezed into a narrow multitasking column) gets the compact
	// bottom-tab layout below.
	var root fyne.CanvasObject
	if state.layoutClass() == layoutRegular {
		root = buildRegularWidthUI(state)
	} else {
		root = buildCompactUI(state)
	}

	// Wrap the root wherever the layout could change at runtime: iPads (static
	// idiom), and ALL of Android — there the tablet test reads live window
	// dimensions, which are 0×0 before the first layout, so the watcher must be
	// armed to catch the real size (it stays inert on phones: the class never
	// changes). iPhone paths remain unwrapped and byte-for-byte unchanged.
	if layoutMayChange() {
		return newLayoutWatcher(state, root)
	}
	return root
}

// buildCompactUI is the phone (and narrow-tablet) layout: the app header on top,
// a full-screen tab body (Read / Books / Search), and the compact bottom tab bar.
func buildCompactUI(state *AppState) fyne.CanvasObject {
	pal := state.pal()

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

	// In the compact layout there is no sidebar to re-highlight; syncSidebar is a
	// no-op. (The regular layout's buildSidebar wires a real one.)
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

	header := buildHeader(state)
	tabBar := buildMobileTabBar(state)
	body := container.NewBorder(header, tabBar, nil, nil, content)

	base := canvas.NewRectangle(pal.Background)
	return container.NewStack(base, body)
}

// overlayShouldShow is the single source of truth for native reading-overlay
// visibility on mobile: the iOS UITextView must be visible exactly when the
// reading view is the content actually on screen. Every place that toggles the
// overlay derives the answer from here, and afterRebuild re-asserts it as the
// last word after each window rebuild, so a stray async show/hide during the
// rebuild can't leave the overlay floating over the wrong content as a blank
// (black) rectangle.
//
//   - Full-screen (distraction-free) reading: always show.
//   - Regular (tablet) layout: the reading pane is always beside the sidebar, so
//     show whenever a search's results aren't occupying it.
//   - Compact layout: only the Read tab hosts the reading pane, and only when no
//     search is active.
func overlayShouldShow(state *AppState) bool {
	if state.IsFullScreen {
		return true
	}
	if state.layoutClass() == layoutRegular {
		return !state.IsSearching
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

	headerItems := make([]fyne.CanvasObject, 0, 4)
	if b := incompleteBibleBanner(state); b != nil {
		headerItems = append(headerItems, b, spacer(8))
	}
	headerItems = append(headerItems,
		sectionLabel("BOOKS", pal),
		inputFrame(withCaret(state, bookFilter), pal.Border),
	)
	header := container.NewVBox(headerItems...)
	return container.NewBorder(container.NewPadded(header), nil, nil, nil, list)
}

// buildMobileSearchTab is the full-screen search experience. A "Search / Find"
// toggle switches the single field between keyword search (live results as you
// type; an exact reference like "John 3:16" jumps on Submit) and AI passage
// search ("the fruit of the Spirit"), which returns relevant passages. Tapping
// any hit jumps to that verse in context and switches to the Read tab. (The
// narrative-answer "Ask" lives on the reading selection menu, not here.)
func buildMobileSearchTab(state *AppState, switchToRead func()) fyne.CanvasObject {
	pal := state.pal()

	// Settings → Assistant "None" hides AI Find. Force keyword mode so a leftover
	// aiSearchMode from before the switch can't strand the tab in a hidden Find
	// mode; the Search/Find toggle is omitted from the header below.
	aiOn := aiFeaturesEnabled(state)
	if !aiOn {
		state.aiSearchMode = false
		state.aiSearchActive = false
	}

	resultsHost := container.NewStack()

	// --- Keyword search. ---
	searchEntry := newSearchEntry() // keyboard "return" submits (see searchKeyEntry)
	searchEntry.SetPlaceHolder("Search…")
	searchEntry.SetText(state.SearchQuery)

	// Reroute showReading so live, as-you-type keyword search repaints the results
	// panel here. We deliberately do NOT chain to the Read tab's showReading (which
	// drives the native overlay); the Read tab rebuilds fresh from state on switch.
	// In AI mode the results are rendered by the Find handler, not live search.
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
		dismissKeyboard(state) // keyboard is done; drop it so results get the full pane
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

	// --- AI Find (passage search). ---
	aiEntry := newSearchEntry() // keyboard "return" submits (see searchKeyEntry)
	aiEntry.SetPlaceHolder("In your own words…")
	aiEntry.SetText(state.aiSearchQuery) // restore the last question on tab return

	// A disclaimer beneath the Ask field, shown only BEFORE results (the prompt state).
	// It collapses once results appear so they get most of the pane — the results view
	// carries its own compact note.
	disc := widget.NewRichText(&widget.TextSegment{
		Text: "These results are generated by AI and may be inaccurate or incomplete. " +
			"Always read each passage in its full context and confirm it in Scripture.",
		Style: widget.RichTextStyle{
			Alignment: fyne.TextAlignCenter,
			ColorName: colorNameMuted,
			SizeName:  theme.SizeNameCaptionText,
		},
	})
	disc.Wrapping = fyne.TextWrapWord
	aiDisclaimer := container.NewPadded(disc)

	var aiBar *widget.ProgressBarInfinite
	stopAIBar := func() {
		if aiBar != nil {
			aiBar.Stop()
			aiBar = nil
		}
	}

	// The supersession guard lives on AppState (state.askSession) so it survives
	// window rebuilds (field-reported: edit the query, resubmit, progress flashes,
	// then the OLD results reappear — and the rebuild variant of the same race).
	askSession := &state.askSession

	var runAsk func(string)
	runAsk = func(q string) {
		// Defense in depth (mirrors dispatchAIAction): with the assistant on
		// "None" no caller should reach this, but never start an AI search then.
		if !aiFeaturesEnabled(state) {
			return
		}
		q = strings.TrimSpace(q)
		if q == "" {
			return
		}
		gen := askSession.Start()
		dismissKeyboard(state)  // question submitted; drop the keyboard so results are visible
		aiDisclaimer.Hide()     // leaving the prompt state → collapse the disclaimer
		state.searchScrollY = 0 // new results start at the top
		if !hasAIKey(state) {
			resultsHost.Objects = []fyne.CanvasObject{aiNoKeyView(state)}
			resultsHost.Refresh()
			return
		}
		// The submitted query is the live context NOW: persist it and drop the
		// previous results immediately, so anything that re-renders this pane
		// mid-flight (a tab return, a mode re-apply, a window rebuild) shows the
		// in-progress state — never the previous query's results.
		state.aiSearchActive = true
		state.aiSearchQuery = q
		state.aiSearchResults = nil
		bar := widget.NewProgressBarInfinite()
		aiBar = bar
		msg := canvas.NewText("Searching with AI…", pal.TextMuted)
		msg.Alignment = fyne.TextAlignCenter
		hint := canvas.NewText("A high-capability model can take a minute or more.", pal.TextMuted)
		hint.Alignment = fyne.TextAlignCenter
		hint.TextSize = 12
		// Declared before the call so the hook can close over it; the real cancel
		// func replaces it the moment startAISearch returns. Published to
		// state.cancelAISearch so EVERY teardown route (a bottom-tab switch that
		// rebuilds this tab, the ✕, the mode toggle, Settings → Assistant →
		// None) can abandon the request through abandonAISearch — otherwise the
		// only handle lives in this closure and a rebuild orphans the request
		// for the rest of the three-minute budget.
		cancelSearch := func() {}
		state.cancelAISearch = func() {
			askSession.Invalidate() // a late completion must not repaint this pane
			cancelSearch()          // abandon the request itself, not just its callback
			stopAIBar()
		}
		cancelBtn := widget.NewButton("Cancel", func() {
			abandonAISearch(state)
			state.aiSearchCancelled = true
			resultsHost.Objects = []fyne.CanvasObject{aiSearchPromptView(state)}
			resultsHost.Refresh()
			aiDisclaimer.Show() // the prompt state always shows it (see applyMode)
		})
		resultsHost.Objects = []fyne.CanvasObject{container.NewCenter(container.NewVBox(
			msg, spacer(8),
			container.NewGridWrap(fyne.NewSize(220, bar.MinSize().Height), bar),
			spacer(6), container.NewCenter(hint),
			spacer(10), container.NewCenter(cancelBtn),
		))}
		resultsHost.Refresh()

		cancelSearch = startAISearch(state, q, func(verses []Verse, err error) {
			if !askSession.Current(gen) {
				return // superseded: a newer ask/clear/toggle owns the pane now
			}
			// Stop the progress bar BEFORE the context check below: a completion
			// dropped because the assistant flipped to "None" mid-flight must
			// still halt the spinner, or an orphaned ProgressBarInfinite keeps
			// animating (and repainting the canvas) until something rebuilds the
			// tab. (After the session check, though — a superseded completion
			// must never stop a NEWER ask's bar.)
			stopAIBar()
			state.cancelAISearch = nil // this request is done; nothing to abandon
			if !state.aiSearchActive {
				// The AI results context was torn down mid-flight (the assistant
				// flipped to "None" — clearAISearchContext): drop the result
				// instead of painting it into a pane that no longer owns it.
				return
			}
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
		abandonAISearch(state) // cancel the REQUEST (invalidates the session too)
		aiEntry.SetText("")
		stopAIBar()
		state.aiSearchResults = nil
		state.aiSearchQuery = ""
		state.aiSearchCancelled = false
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
				aiDisclaimer.Hide()
				resultsHost.Objects = []fyne.CanvasObject{aiNoKeyView(state)}
			case len(state.aiSearchResults) > 0:
				// Results present → collapse the disclaimer so results get the pane.
				aiDisclaimer.Hide()
				resultsHost.Objects = []fyne.CanvasObject{aiResultsView(state, state.aiSearchQuery, state.aiSearchResults)}
			default:
				aiDisclaimer.Show() // before results
				resultsHost.Objects = []fyne.CanvasObject{aiSearchPromptView(state)}
			}
		} else {
			aiDisclaimer.Hide()
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
		abandonAISearch(state) // cancel the REQUEST (invalidates the session too)
		stopAIBar()
		state.aiSearchMode = ai
		state.aiSearchActive = ai // switch the results context with the mode
		applyMode()
	})

	var header *fyne.Container
	if aiOn {
		header = container.NewVBox(toggle, fieldHost, aiDisclaimer)
	} else {
		header = container.NewVBox(fieldHost, aiDisclaimer) // no Search/Find toggle
	}
	applyMode() // initialise to the persisted mode (also shows/hides the AI disclaimer)
	return container.NewBorder(container.NewPadded(header), nil, nil, nil, resultsHost)
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
