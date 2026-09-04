package bibletext

// THE COMPACT LAYOUT — SHARED BY EVERY PLATFORM.
//
// This was ui_mobile.go until 21 Aug 2026, tagged `ios || android`, and the tag
// was the only thing mobile about it. The tab bar, the books grid, the search
// tab and the composition are plain Fyne over plain AppState; the only genuinely
// per-platform pieces are the reading pane itself (compactReadingPane), the
// native overlay notifier, and dropping the soft keyboard — three small seams,
// declared per platform, rather than a whole second layout.
//
// The reason, and the one to weigh before splitting it again: compatibility
// and uniformity across platforms come first wherever they sensibly can, so
// that the same work does not have to be reworked for each platform in turn.

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// buildCompactUI is the shared layout on every platform: the app header on top,
// a full-screen tab body (Read / Books / Search), and the destinations drawn as
// a bottom bar or a leading rail (compactNavRail decides which).
func buildCompactUI(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	// gotoReadTab is used by Books/Search to jump back to the reading pane after
	// the user picks a book or a search result. We rebuild the window on every
	// tab change (reliable repaint — Fyne's in-place host-swap doesn't always
	// repaint a UITextView-overlaid tree) so this just sets the tab + rebuilds.
	gotoReadTab := func() {
		// ALREADY THERE IS DONE. HandleShareLink surfaces the reading tab
		// itself (state.surfaceReading), so a tap handler that opens a link and
		// then calls this ran TWO rebuilds back to back. The second one
		// re-derived a just-placed arrival into a position carry, and the carry
		// then replayed through the new pane's still-reflowing geometry —
		// landing the reader roughly where they started instead of on the note
		// they tapped (the styled scroll trace has the whole story).
		if state.CurrentTab == 0 {
			return
		}
		state.CurrentTab = 0
		leaveSearchForRead(state, 0)
		rebuildWindow(state)
	}
	state.surfaceReading = gotoReadTab
	// "Back to results" returns to the real Search tab (restoring its query, results
	// and scroll position) rather than showing results inline in the reading pane.
	state.surfaceSearch = func() {
		state.CurrentTab = 2
		rebuildWindow(state)
	}

	// The shared layout has no chapter sidebar to re-highlight, so syncSidebar is
	// a no-op. (The retained former regular layout wires a real one.)
	state.syncSidebar = func() {}

	// Build only the active tab's content — the others are constructed on
	// demand when the user switches (rebuildWindow re-runs CreateMainUI).
	//
	// showReading is NEUTRALIZED first and reassigned only by the Read case
	// below. Left standing, it points at the PREVIOUS build's detached host,
	// and a refresh from another tab then rebuilds the reading pane into that
	// dead tree — the mobile layout documents the same hazard. The sharpest
	// consequence here: a share-link arrival while the Links tab is front ran
	// wireStyledReadingScroll against a zero-sized, off-screen pane, which
	// CONSUMED forceReposition — so the real pane, built a moment later by
	// switchToRead, never learned an arrival had happened and left the reader
	// wherever they were. With no reading view on screen there is nothing to
	// show; the state is simply newer than the view, and the Read tab's own
	// rebuild renders it.
	state.showReading = nil
	var content fyne.CanvasObject
	switch state.CurrentTab {
	case 1:
		content = buildMobileBooksTab(state, gotoReadTab)
		notifyReadingOverlay(overlayShouldShow(state))
	case 2:
		content = buildMobileSearchTab(state, gotoReadTab)
		notifyReadingOverlay(overlayShouldShow(state))
	case 3:
		// Dev builds only; in a release build devLinksEnabled is false and this
		// tab does not exist to be selected.
		if devLinksEnabled {
			content = buildDevLinksTab(state, gotoReadTab)
			notifyReadingOverlay(false)
			break
		}
		state.CurrentTab = 0
		content = container.NewStack(compactReadingPane(state))
	default: // 0 = Read
		readingHost := container.NewStack(compactReadingPane(state))
		state.showReading = func() {
			readingHost.Objects = []fyne.CanvasObject{compactReadingPane(state)}
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

	// THE NAVIGATION'S PLACE. The same destinations on one of two edges,
	// decided by compactNavRail: the bottom bar on phones and on tablets in
	// portrait; the leading rail on tablets and Android phones in landscape and
	// on desktop windows (BIBLETEXT_DESKTOP_TABS overrides, ui_compact_desktop.go).
	// Only the edge they sit on differs.
	var body fyne.CanvasObject
	if compactNavRail(state) {
		body = container.NewBorder(header, nil, buildTabRail(state), nil, content)
	} else {
		body = container.NewBorder(header, buildMobileTabBar(state), nil, nil, content)
	}

	base := canvas.NewRectangle(pal.Background)
	return container.NewStack(base, body)
}

// buildMobileTabBar renders the compact bottom tab strip. Selecting a tab sets
// state.CurrentTab and rebuilds the window. Each tab is a tabCell (icon + tiny
// label); the active one is accent-coloured.
// tabDestination is one entry in the app's navigation, independent of whether it
// is drawn as a bar across the bottom or a rail down the side.
type tabDestination struct {
	label string
	icon  fyne.Resource
}

// tabDestinations is the navigation itself — the single list both presentations
// read. Adding a destination here adds it everywhere.
func tabDestinations() []tabDestination {
	items := []tabDestination{
		{"Read", theme.DocumentIcon()},
		{"Books", theme.MenuIcon()},
		{"Search", theme.SearchIcon()},
	}
	// Development builds only (-tags bibletextdev). devLinksEnabled is a compile-
	// time constant, so in a release build this branch and everything it reaches
	// are eliminated — the page cannot ship. See dev_links_off.go.
	if devLinksEnabled {
		items = append(items, tabDestination{"Links", theme.MailSendIcon()})
	}
	return items
}

// tabCellsFor builds the tappable cells for the destinations. Shared by the bar
// and the rail so the two can never drift in what they do — only in how they
// are arranged.
func tabCellsFor(state *AppState, items []tabDestination) []fyne.CanvasObject {
	cells := make([]fyne.CanvasObject, len(items))
	for i, it := range items {
		i, it := i, it
		cells[i] = newTabCell(state, it.icon, it.label, i == state.CurrentTab, func() {
			if state.CurrentTab == i {
				return
			}
			// A tab switch rebuilds the window, which discards the Search tab's
			// live widgets. Abandon any in-flight Find first — the rebuilt tab
			// cannot reach the old request, so without this it billed on for
			// the rest of aiRequestBudget with no control able to stop it.
			abandonAISearch(state)
			state.CurrentTab = i
			leaveSearchForRead(state, i)
			rebuildWindow(state)
		})
	}
	return cells
}

func buildMobileTabBar(state *AppState) fyne.CanvasObject {
	pal := state.pal()
	items := tabDestinations()
	cells := tabCellsFor(state, items)

	row := container.NewGridWithColumns(len(items), cells...)

	// THE PHONE KEEPS THE EDGE-TO-EDGE BAR; THE TABLET FLOATS.
	//
	// One layout, two presentations, and the split is a convention rather than a
	// preference: a bar pinned across the bottom is what every iPhone app has,
	// and a floating bar is what iPadOS itself introduced for the larger screen.
	// The pill was tried on both and kept only where it belongs.
	//
	// Note what does NOT differ: the tabs, their order, their behaviour, and the
	// tightened icon-to-label gap are one piece of code for both. This is the
	// bar's dress, not a second navigation model — which is the whole point of
	// the layout unification this sits inside.
	switch tabBarStyleFor(state) {
	case tabBarEdgeSpread:
		// The literal phone treatment: chrome edge to edge, tabs spread evenly
		// across the whole width by the same GridWithColumns the phone uses.
		rule := canvas.NewLine(pal.Border)
		rule.StrokeWidth = 1
		bg := canvas.NewRectangle(pal.SurfaceAlt)
		return container.NewStack(bg, edgeBarBody(rule, row))

	case tabBarEdgeCentred:
		// Chrome edge to edge, tabs CENTRED at their natural width — which is
		// what UIKit's own UITabBar does on iPad. A phone spreads three tabs
		// across 393pt; the same grid across 1032pt puts them 340pt apart, so
		// the eye reads three separate controls rather than one bar. Centring
		// keeps the full-width chrome the phone has while holding the tabs at
		// the spacing they were designed at.
		rule := canvas.NewLine(pal.Border)
		rule.StrokeWidth = 1
		bg := canvas.NewRectangle(pal.SurfaceAlt)
		centred := container.New(tabBarCentreLayout{want: tabBarGroupWidth(len(items))}, row)
		return container.NewStack(bg, edgeBarBody(rule, centred))
	}

	// The reason the tablet's bar only LOOKS like it floats is worth stating,
	// because the obvious implementation is broken on the platform it was asked
	// for.
	//
	// The scripture on iOS is a native UITextView floating ABOVE the whole Fyne
	// canvas, with its frame pinned to the reading host's rect. A tab bar that
	// genuinely overlapped the content would be painted over by that view — the
	// bar would simply disappear behind the text on the one tab it matters most
	// on. So the bar still occupies the Border's bottom slot and still reserves
	// its height; what changed is that the slot is the page ground and the bar
	// is a rounded, inset pill drawn inside it.
	//
	// That is also the cheaper answer for every other platform: no overlay
	// arithmetic, no per-platform inset, nothing for the native panes to know
	// about. The look is the same everywhere and the geometry is unchanged.
	pill := canvas.NewRectangle(pal.SurfaceAlt)
	pill.CornerRadius = tabBarPillRadius
	pill.StrokeColor = pal.Border
	pill.StrokeWidth = 1

	// CENTRED AND ONLY AS WIDE AS IT NEEDS TO BE. A pill that spans the whole
	// width is just a bar with rounded ends — the thing that makes iPadOS's own
	// floating bar read as floating is that ground shows on both sides of it. On
	// a phone the cap is wider than the screen, so it stays edge-to-edge exactly
	// as before; on a tablet it becomes a floating control.
	bar := container.New(
		layout.NewCustomPaddedLayout(0, tabBarInsetY, tabBarInsetX, tabBarInsetX),
		container.New(tabBarCentreLayout{want: tabBarPillWidth(len(items))},
			container.NewStack(
				pill,
				container.New(layout.NewCustomPaddedLayout(
					tabBarPillPadY, tabBarPillPadY, tabBarPillPadX, tabBarPillPadX), row),
			),
		),
	)
	return bar
}

// edgeBarBody stacks the hairline over the tabs with EQUAL air above and below
// them.
//
// The obvious spelling — NewVBox(rule, NewPadded(tabs)) — is not symmetric, and
// the asymmetry is invisible in the code: a VBox spaces its children by theme
// padding, so the tabs got that 7pt PLUS the 7 from NewPadded above, and only
// the 7 from NewPadded below. Two to one, on every phone and every tablet,
// and plainly visible on device as unequal vertical margins in the nav bar.
//
// This is the SAME trap the tab cell's icon-to-label gap hit — a VBox's padding
// is between its children, not something you can reason about from the call
// site — which is why the fix here is the same: take the inter-child padding to
// zero and state both margins explicitly.
func edgeBarBody(rule, tabs fyne.CanvasObject) fyne.CanvasObject {
	padded := container.New(
		layout.NewCustomPaddedLayout(tabBarEdgePadY, tabBarEdgePadY, 0, 0), tabs)
	return container.New(layout.NewCustomPaddedVBoxLayout(0), rule, padded)
}

// tabBarCentreLayout caps the pill at tabBarMaxWidth and centres it. Same idea
// as readableColumn (readable_column.go) and deliberately not the same call: a
// control's comfortable width is not a line-length measure, and tying them
// together would mean one could not be tuned without moving the other.
type tabBarCentreLayout struct{ want float32 }

// tabBarPillWidth is the bar's natural width: a comfortable slot per tab plus
// the pill's own padding.
//
// Computed from the tab COUNT rather than read from the row's MinSize, which
// was the first attempt and is circular — the row's minimum depends on the
// width it is given, so asking it produced a bar one tab wide with the rest
// stacked underneath. An explicit number per tab is also what makes the bar
// correct for the dev build's fourth tab without a second constant.
func tabBarPillWidth(tabs int) float32 {
	return tabBarGroupWidth(tabs) + 2*tabBarPillPadX
}

func (l tabBarCentreLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	for _, o := range objs {
		w := l.want
		if w > s.Width {
			w = s.Width // a narrow phone keeps the edge-to-edge bar
		}
		o.Resize(fyne.NewSize(w, s.Height))
		o.Move(fyne.NewPos((s.Width-w)/2, 0))
	}
}

func (l tabBarCentreLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var m fyne.Size
	for _, o := range objs {
		m = m.Max(o.MinSize())
	}
	return fyne.NewSize(l.want, m.Height)
}

// The floating bar's metrics, named so the shape is adjustable in one place.
//
// ONE INNER PADDING, BOTH AXES. The first cut used 6 across and 4 down while
// giving each tab a 104pt slot for a ~34pt label — so the pill carried about
// 40pt of dead space either side of every icon and 4pt above and below it
// — wide horizontal margins with hardly any vertical margin at all. The
// numbers below are derived rather than picked: a tab's content is a 20pt
// icon, a 2pt gap and a 10pt label — about 35pt tall — so a single 12pt pad
// on every side gives a ~59pt pill with the icon and the label evenly inset,
// and the radius is half of that, which is what makes it read as a
// floating capsule rather than a rounded bar.
const (
	tabBarInsetX float32 = 14 // gap from the screen edges to the pill
	tabBarInsetY float32 = 10 // gap from the bottom safe area to the pill

	// The pill's inner margins, horizontal ≈ 3 × vertical. A capsule
	// wants generous end caps and a tight cap above and below — equal padding on
	// all four sides, which is what this was for one iteration, makes it read as
	// a rounded rectangle that happens to have curved ends rather than as one
	// continuous control.
	tabBarPillPadY float32 = 10
	tabBarPillPadX float32 = 30

	// A tab's slot: wide enough for the longest label with real air around it,
	// narrow enough that three of them read as one control.
	tabBarCellMin float32 = 72

	// The tablet's slot is wider. 72pt is a phone's share of a 393pt screen; the
	// same number centred on an iPad reads as three controls huddled together in
	// the middle of a lot of nothing. The group has to look deliberate at that
	// scale, not left over.
	tabBarCellTablet float32 = 104

	// The gap between a tab's icon and its label — the whole gap, not a nudge on
	// top of theme padding. Applies on the phone too, which is where it was
	// first noticed. Tuned twice: Fyne's VBox padding was too loose, 2pt then
	// read as cramped, and 4 is where the pair sits as one control without the
	// label crowding the glyph.
	tabBarIconLabelGap float32 = 4

	// The tab cell's own drawing sizes. Named because the rail derives its
	// thickness from them (tab_rail.go): a literal here and a literal there
	// would drift the moment either moved.
	// The air above and below the tabs in the edge-to-edge bar, on BOTH sides.
	// Larger than the theme's 7 because the bar wanted to be roomier and the
	// old asymmetric spelling gave 14 above; this keeps that generosity
	// while making the two match.
	tabBarEdgePadY float32 = 12

	tabCellIconSize  float32 = 20
	tabCellLabelSize float32 = 10

	// Half the pill's height (~35pt of content + 2 × 10pt pad ≈ 55), so the ends
	// are semicircular. Deliberately a touch under half: Fyne clamps a radius
	// larger than the box, and a hair under reads identically.
	tabBarPillRadius float32 = 27
)

// buildMobileBooksTab is a touch-sized, scrollable book list with a filter on
// top. Tapping a book selects it (resetting to its first chapter) and switches
// to the Read tab.
// buildMobileBooksTab is the canon as a GRID, grouped by testament.
//
// WHY IT IS NOT A LIST ANY MORE: the list was awkward, and was rethought from
// a design, elegance and usability perspective. It was 66 rows of 44pt: about
// 2,900pt of scrolling to reach Revelation, each row spending the pane's whole
// width on one short word, with nothing to tell you where you were except the
// accent on the current book. That is a phone compromise, and it was inherited
// onto the iPad unexamined when the tablet started using this view.
//
// Three things change, and each is a usability answer rather than a decoration:
//
//  1. A GRID, so width buys columns instead of white space. The whole Old
//     Testament is a glance rather than a scroll, and a target is a short
//     move away instead of a long one.
//  2. TESTAMENT HEADINGS, so the list has landmarks. Derived from where Matthew
//     falls in the LOADED translation's own book order rather than a hardcoded
//     count, so the 73-book Catholic canon groups correctly without a second
//     code path — and any future canon degrades to one unlabelled run rather
//     than to a wrong label.
//  3. The filter hides the headings, because a filtered set is a RESULT, not
//     the canon — headings over three matches would be furniture pretending to
//     be structure.
//
// It uses the app's own denseGridWrapLayout (reading.go), the same layout the
// Go-to picker's letter grid and the chapter grid use, so this reads as the
// app's existing vocabulary rather than a new one.
func buildMobileBooksTab(state *AppState, switchToRead func()) fyne.CanvasObject {
	pal := state.pal()

	bookFilter := widget.NewEntry()
	bookFilter.SetPlaceHolder("Filter books")
	bookFilter.SetText(state.BookFilterQuery)

	body := container.NewVBox()
	rebuild := func() {
		body.Objects = body.Objects[:0]
		filtered := filterBooks(state.Bible.Books, state.BookFilterQuery)
		for _, sec := range bookSections(state.Bible.Books, filtered, state.BookFilterQuery != "") {
			if sec.title != "" {
				body.Add(spacer(6))
				body.Add(sectionLabel(sec.title, pal))
				body.Add(spacer(4))
			}
			grid := container.New(&denseGridWrapLayout{cell: fyne.NewSize(bookCellW, bookCellH), centre: true})
			for _, name := range sec.books {
				name := name
				btn := widget.NewButton(name, func() {
					selectBook(state, name, true)
					state.refresh()
					switchToRead()
				})
				if name == state.CurrentBook {
					btn.Importance = widget.HighImportance // where you are, at a glance
				} else {
					btn.Importance = widget.LowImportance
				}
				grid.Add(btn)
			}
			body.Add(grid)
		}
		body.Refresh()
	}
	rebuild()

	bookFilter.OnChanged = func(s string) {
		state.BookFilterQuery = s
		rebuild()
	}

	headerItems := make([]fyne.CanvasObject, 0, 4)
	if b := incompleteBibleBanner(state); b != nil {
		headerItems = append(headerItems, b, spacer(8))
	}
	headerItems = append(headerItems, inputFrame(withCaret(state, bookFilter), pal.Border))
	header := container.NewVBox(headerItems...)

	scroll := container.NewVScroll(container.NewPadded(body))
	// A GRID WANTS MORE WIDTH THAN A LIST, and that is not a contradiction of
	// the measure the other surfaces take (readable_column.go): a list row is
	// one item and reads badly when stretched, while a grid turns extra width
	// into columns, which is the whole point. The cap here is generous enough
	// for five columns and stops the canon sprawling across a desktop window.
	return boundedColumn(bookGridMaxWidth,
		container.NewBorder(container.NewPadded(header), nil, nil, nil, scroll))
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
	// Same for notes: a mode left over from before the reader switched them off
	// would render a browser whose toggle is no longer on screen.
	if !notesFeatureOn(state) {
		setNotesMode(state, false)
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
	// window rebuilds (edit the query, resubmit, progress flashes,
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
		// Write the SAME state the desktop path does. The phone painted only
		// into its captured resultsHost, so a rebuild (tab switch, rotation,
		// theme flip) stranded a live search: no spinner, no Cancel, and an
		// error that reached no state at all vanished silently.
		state.aiSearchLoading = true
		state.aiSearchErr = nil
		state.aiSearchCancelled = false
		bar := widget.NewProgressBarInfinite()
		aiBar = bar
		msg := canvas.NewText("Searching with AI…", pal.TextMuted)
		msg.Alignment = fyne.TextAlignCenter
		// caption() is the app's muted, WRAPPING caption style — a canvas.Text
		// would neither wrap nor bound the column's width, which is what pushed
		// the progress bar off-centre (the VBox grew to the hint's full width
		// while the fixed-width bar stayed left-aligned inside it).
		hint := container.NewGridWrap(fyne.NewSize(260, captionHeightFor(2)),
			centeredCaption("Capable models can take a minute or more."))
		// Declared before the call so the hook can close over it; the real cancel
		// func replaces it the moment startAISearch returns. Published to
		// state.cancelAISearch so EVERY teardown route (a bottom-tab switch that
		// rebuilds this tab, the ✕, the mode toggle, Settings → Assistant →
		// None) can abandon the request through abandonAISearch — otherwise the
		// only handle lives in this closure and a rebuild orphans the request
		// for the rest of the multi-minute budget.
		cancelSearch := func() {}
		installAISearchCancel(state, func() {
			askSession.Invalidate() // a late completion must not repaint this pane
			cancelSearch()          // abandon the request itself, not just its callback
			stopAIBar()
		})
		var fasterRow fyne.CanvasObject = spacer(0)
		if pid, fm, label, ok := fasterModelOffer(state); ok {
			fasterRow = container.NewVBox(spacer(6), fasterModelControl(label, func() {
				abandonAISearch(state)
				applyFasterModel(state, pid, fm)
				runAsk(q) // re-ask the same question on the quick model
			}))
		}
		cancelBtn := widget.NewButton("Cancel", func() {
			abandonAISearch(state)
			state.aiSearchCancelled = true
			resultsHost.Objects = []fyne.CanvasObject{aiSearchPromptView(state)}
			resultsHost.Refresh()
			aiDisclaimer.Show() // the prompt state always shows it (see applyMode)
		})
		resultsHost.Objects = []fyne.CanvasObject{container.NewCenter(container.NewVBox(
			container.NewCenter(msg), spacer(10),
			container.NewCenter(container.NewGridWrap(fyne.NewSize(240, bar.MinSize().Height), bar)),
			spacer(10), container.NewCenter(hint),
			// inputFrame: the theme's SurfaceAlt button fill is near-invisible
			// on this ground, so give Cancel the app's standard visible outline.
			spacer(4), container.NewCenter(inputFrame(cancelBtn, state.pal().Border)),
			fasterRow,
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
			state.aiSearchLoading = false
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
				state.aiSearchErr = err // so a rebuild re-renders the failure
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

	// --- Notes (the messages people have shared with you). ---
	// Its own field, and its own query: switching Search → Notes with a scripture
	// term still in the box would answer "no notes match" to a search the reader
	// never made of their notes. Filtering is live and undebounced — it is a
	// substring scan over short strings already in memory.
	notesEntry := newSearchEntry()
	notesEntry.SetPlaceHolder("Search your notes…")
	notesEntry.SetText(state.NotesQuery)
	repaintNotes := func() {
		resultsHost.Objects = []fyne.CanvasObject{buildNotesBrowseView(state)}
		resultsHost.Refresh()
	}
	notesEntry.OnChanged = func(q string) {
		state.NotesQuery = q
		repaintNotes()
	}
	notesEntry.OnSubmitted = func(string) { dismissKeyboard(state) }
	clearNotesBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		notesEntry.SetText("") // fires OnChanged → repaint
	})
	clearNotesBtn.Importance = widget.LowImportance

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
		if searchModeOf(state) == modeNotes {
			aiDisclaimer.Hide()
			stopAIBar()
			fieldHost.Objects = []fyne.CanvasObject{
				container.NewBorder(nil, nil, nil, clearNotesBtn, inputFrame(withCaret(state, notesEntry), pal.Border)),
			}
			resultsHost.Objects = []fyne.CanvasObject{buildNotesBrowseView(state)}
			fieldHost.Refresh()
			resultsHost.Refresh()
			return
		}
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

	// Shared by both controls, so switching mode means the same thing however the
	// reader got there.
	applyModeSwitch := func(mode searchMode) {
		ai := mode == modeFind
		inFlight := state.cancelAISearch != nil
		abandonAISearch(state) // cancel the REQUEST (invalidates the session too)
		stopAIBar()
		state.aiSearchCancelled = inFlight // abandoning is not a zero-result answer
		state.aiSearchMode = ai
		state.aiSearchActive = ai // switch the results context with the mode
		setNotesMode(state, mode == modeNotes)
		applyMode()
	}
	// One row owning Search / Find / the notes bubble, so the active fill can move
	// between all three. No rebuild on a mode change: applyModeSwitch swaps the
	// field and results in place, and the row repaints its own buttons — a rebuild
	// here would drop keyboard focus mid-search.
	modeRow := buildSearchModeControls(state, applyModeSwitch)
	var header *fyne.Container
	if aiOn || notesFeatureOn(state) {
		header = container.NewVBox(modeRow, fieldHost, aiDisclaimer)
	} else {
		header = container.NewVBox(fieldHost, aiDisclaimer) // nothing to switch between
	}
	applyMode() // initialise to the persisted mode (also shows/hides the AI disclaimer)
	// Same measure as the books and notes lists: a hit's reference and its verse
	// must not end up a tablet's width apart.
	return readableColumn(container.NewBorder(container.NewPadded(header), nil, nil, nil, resultsHost))
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
	c.iconImg.SetMinSize(fyne.NewSize(tabCellIconSize, tabCellIconSize))

	c.text = canvas.NewText(c.label, tint)
	c.text.TextSize = tabCellLabelSize
	c.text.Alignment = fyne.TextAlignCenter
	c.text.TextStyle = fyne.TextStyle{Bold: c.active}

	// THE ICON AND ITS LABEL ARE ONE THING, so they sit together.
	//
	// This was a NewVBox with a spacer(2) between them, which reads as a 2pt gap
	// and is not: VBox lays its children out with theme padding BETWEEN them, so
	// the spacer was padded above and below and the real gap was about 10pt —
	// enough to make the label look like a caption under the icon rather than
	// part of the same control, on both form factors. A custom padded
	// VBox takes the theme padding out of it, so the number below is the gap.
	col := container.New(
		layout.NewCustomPaddedVBoxLayout(tabBarIconLabelGap),
		container.NewCenter(c.iconImg),
		container.NewCenter(c.text),
	)

	// CENTRED IN THE CELL, not resting on its ceiling.
	//
	// A VBox lays its children out from the TOP at their minimum height and
	// leaves any slack at the bottom, so the icon-and-label pair sat against the
	// top of a cell that is taller than they are — which reads as the bar having
	// more air below the labels than above the icons, no matter what the bar's
	// own padding does — plainly visible on device.
	//
	// NewCenter is the whole fix: it gives the column its minimum size and puts
	// it in the middle of whatever the cell turns out to be.
	return widget.NewSimpleRenderer(container.NewCenter(col))
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

// --- TAB BAR DRESS ---------------------------------------------------------
//
// One bar, two presentations, chosen by WIDTH rather than by device. That is
// deliberate and it is what makes this rule cover a phone, an iPad and a desktop
// window without any of them naming the others: a narrow screen has no spare
// width, so the tabs spread to fill it exactly as every phone's bar does; a wide
// one has width to spare, so the tabs hold their designed spacing and centre in
// it. A desktop window dragged narrow becomes a phone bar on the way past, which
// is the correct behaviour and not a special case anyone had to write.
//
// The floating pill was the tablet's dress until the two were compared on real
// screenshots and the grounded bar won; the pill stays implemented because the
// choice was between two finished things, and reversing it should be a
// constant rather than a rebuild.
type tabBarStyle int

const (
	tabBarPill        tabBarStyle = iota // floating, inset, rounded (iPadOS-style)
	tabBarEdgeSpread                     // full-width chrome, tabs spread across it
	tabBarEdgeCentred                    // full-width chrome, tabs centred
)

// tabBarSpreadMaxWidth is the width at or below which the tabs spread to fill
// the bar. Above it they centre at their designed slot width.
//
// 560 sits above every phone in portrait (the widest is 440) and below every
// tablet the regular layout ever claimed (700), so no real device lands near the
// boundary — which matters, because crossing it mid-session is a visible change
// of dress, and the only way to do that is to resize a desktop window on purpose.
const tabBarSpreadMaxWidth float32 = 560

// tabBarGroupWidth is the width of the tabs THEMSELVES — no chrome. The pill
// adds its own side padding on top of this; a full-width bar has none to add,
// so centring on the pill's width would push the tabs 60pt further apart than
// the design calls for.
func tabBarGroupWidth(tabs int) float32 {
	return float32(tabs) * tabBarCellTablet
}

func tabBarStyleFor(state *AppState) tabBarStyle {
	w := state.canvasWidth()
	if w <= 0 {
		// Before the first layout pass there is no width to judge. Trust the
		// idiom so a tablet's first real frame is already correct rather than
		// spreading once and snapping.
		if deviceIsTablet() {
			return tabBarEdgeCentred
		}
		return tabBarEdgeSpread
	}
	if w <= tabBarSpreadMaxWidth {
		return tabBarEdgeSpread
	}
	return tabBarEdgeCentred
}
