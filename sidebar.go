package bibletext

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const bookRowHeight = 28

// buildSidebar creates the navigation panel once. Its widgets are persistent for
// the lifetime of the window: filtering and external navigation only refresh the
// book list, never rebuild the entry widgets, so typing never loses focus.
func buildSidebar(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	// Settings → Assistant "None" hides AI Find. Force keyword mode so a leftover
	// aiSearchMode from before the switch can't strand the reader in a hidden Find
	// mode; the Search/Find toggle is omitted from the header below.
	aiOn := aiFeaturesEnabled(state)
	if !aiOn {
		state.aiSearchMode = false
		state.aiSearchActive = false
	}

	// --- Search ---
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search…")
	searchEntry.SetText(state.SearchQuery)

	// Live search lists matches as you type, debounced so a fast typist doesn't
	// queue a whole-corpus scan + results rebuild on every keystroke; pressing
	// Enter cancels the pending run and additionally jumps to an exact verse ref.
	onSearchChanged, stopSearchDebounce := newSearchDebouncer(state)
	searchEntry.OnChanged = onSearchChanged
	searchEntry.OnSubmitted = func(s string) {
		stopSearchDebounce()
		executeSearch(state, s)
	}

	clearSearch := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		searchEntry.SetText("")
	})
	clearSearch.Importance = widget.LowImportance
	searchRow := container.NewBorder(nil, nil, nil, clearSearch, inputFrame(withCaret(state, searchEntry), pal.Border))

	state.setSearchText = func(s string) {
		searchEntry.SetText(s)
	}

	// --- AI Find (natural-language passage search). Results replace the reading
	// pane (the same path as keyword search: IsSearching → buildReadingPane →
	// buildSearchResultsView), with the in-progress / no-key / error states driven from
	// state so the shared results view can render them. ---
	aiEntry := widget.NewEntry()
	aiEntry.SetPlaceHolder("In your own words…")
	aiEntry.SetText(state.aiSearchQuery)

	// The supersession guard lives on AppState (state.askSession) so it survives
	// window rebuilds — a local here would let a pre-rebuild completion clobber a
	// post-rebuild query (see the observed in practice history in ai_search.go).
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
		state.searchScrollY = 0
		state.aiSearchActive = true
		state.aiSearchQuery = q
		state.aiSearchErr = nil
		state.IsSearching = true // results replace the reading pane
		if !hasAIKey(state) {
			state.aiSearchResults = nil
			state.aiSearchLoading = false
			state.refresh() // → aiNoKeyView
			return
		}
		state.aiSearchResults = nil
		state.aiSearchLoading = true
		state.aiSearchCancelled = false
		// Installed BEFORE the refresh below, because that refresh SYNCHRONOUSLY
		// builds aiSearchingView — which only renders Cancel when this hook is
		// set. Assigning after would leave the first Find of a session with no
		// Cancel at all. The closure reads the cancelSearch VARIABLE (filled in
		// once startAISearch returns), so it always reaches the request that is
		// actually in flight rather than a captured stale one. UI goroutine
		// only, so no synchronisation. (Mirrors the mobile twin.)
		cancelSearch := func() {}
		installAISearchCancel(state, func() {
			askSession.Invalidate()
			cancelSearch()
		})
		state.refresh() // → aiSearchingView (with Cancel)
		cancelSearch = startAISearch(state, q, func(verses []Verse, err error) {
			if !askSession.Current(gen) {
				return // superseded: a newer ask/clear owns the results now
			}
			if !state.aiSearchActive {
				// The AI results context was torn down mid-flight (the assistant
				// flipped to "None" — clearAISearchContext — or the toggle went
				// back to keyword): drop the result instead of repopulating state.
				return
			}
			state.aiSearchLoading = false
			switch {
			case err != nil && isNoKeyError(err):
				state.aiSearchResults = nil
			case err != nil:
				state.aiSearchErr = err
				state.aiSearchResults = nil
			default:
				state.aiSearchResults = verses
			}
			state.cancelAISearch = nil // this request is done; nothing to abandon
			state.refresh()            // → results / error
		})
	}
	state.retryAISearch = func() { runAsk(state.aiSearchQuery) }
	aiEntry.OnSubmitted = runAsk

	clearAsk := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		abandonAISearch(state) // cancel the REQUEST (invalidates the session too)
		aiEntry.SetText("")
		state.aiSearchQuery = ""
		state.aiSearchResults = nil
		state.aiSearchErr = nil
		state.aiSearchCancelled = false
		if state.IsSearching {
			state.refresh() // → prompt
		}
	})
	clearAsk.Importance = widget.LowImportance
	askBtn := widget.NewButtonWithIcon("", theme.SearchIcon(), func() { runAsk(aiEntry.Text) })
	askBtn.Importance = widget.LowImportance
	askRow := container.NewBorder(nil, nil, nil, container.NewHBox(clearAsk, askBtn), inputFrame(withCaret(state, aiEntry), pal.Border))

	// Notes: the third mode's filter field. Its own query, for the reason
	// AppState.NotesQuery documents. The results themselves are rendered by
	// buildSearchResultsView, which this surface already drives, so the sidebar
	// needs only the field.
	notesEntry := widget.NewEntry()
	notesEntry.SetPlaceHolder("Search your notes…")
	notesEntry.SetText(state.NotesQuery)
	notesEntry.OnChanged = func(q string) {
		state.NotesQuery = q
		state.refresh() // → buildSearchResultsView → the notes browser
	}
	clearNotes := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		notesEntry.SetText("") // fires OnChanged → refresh
	})
	clearNotes.Importance = widget.LowImportance
	notesRow := container.NewBorder(nil, nil, nil, clearNotes,
		inputFrame(withCaret(state, notesEntry), pal.Border))
	notesCaption := caption("Notes people share with you. Tap one to open its passage.")

	// Field + caption swap with the Search/Find/Notes toggle.
	fieldHost := container.NewStack()
	captionHost := container.NewStack()
	keywordCaption := caption("Keyword, or a reference like John 3:16.")
	aiCaption := caption("These results are generated by AI and may be inaccurate or incomplete. Always read each passage in its full context and confirm it in Scripture.")
	applyMode := func() {
		switch searchModeOf(state) {
		case modeNotes:
			fieldHost.Objects = []fyne.CanvasObject{notesRow}
			captionHost.Objects = []fyne.CanvasObject{notesCaption}
		case modeFind:
			fieldHost.Objects = []fyne.CanvasObject{askRow}
			captionHost.Objects = []fyne.CanvasObject{aiCaption}
		default:
			fieldHost.Objects = []fyne.CanvasObject{searchRow}
			captionHost.Objects = []fyne.CanvasObject{keywordCaption}
		}
		fieldHost.Refresh()
		captionHost.Refresh()
	}
	// Shared by the Search/Find pair and the notes button, so a mode switch means
	// the same thing however the reader got there.
	applyModeSwitch := func(mode searchMode) {
		ai := mode == modeFind
		wasNotes := searchModeOf(state) == modeNotes // BEFORE the flags move
		// Mirror the mobile twin: abandon any in-flight Find and clear its
		// progress state, or a completion dropped at the aiSearchActive guard
		// would leak aiSearchLoading=true and toggling back to Find would show
		// a permanent "Searching with AI…" pane (implementation verification).
		// Toggling away mid-flight IS an abandonment, so it must land on the
		// cancelled state — clearing the flag instead left query set and
		// results nil, which falls through to "AI didn't find matching
		// passages": the very false zero-result claim Cancel was fixed to
		// avoid.
		inFlight := state.cancelAISearch != nil
		abandonAISearch(state) // cancels the request; invalidates the session
		state.aiSearchErr = nil
		state.aiSearchCancelled = inFlight
		state.aiSearchMode = ai
		state.aiSearchActive = ai
		state.NotesMode = mode == modeNotes
		// On desktop the results pane only exists while IsSearching — that is how
		// keyword results replace the reading view. Notes is a BROWSER, so it has
		// something to show the moment it is picked, with no query submitted; it
		// therefore claims the pane on entry and gives it back on the way out,
		// unless a real keyword search is still active and owns it.
		if mode == modeNotes {
			state.IsSearching = true
		} else if wasNotes && strings.TrimSpace(state.ActiveSearchQuery) == "" {
			state.IsSearching = false
		}
		applyMode()
		// Notes mode has its own results view, so it must repaint on the switch
		// even when no verse search is running — otherwise picking Notes leaves
		// the previous mode's pane on screen until something else refreshes.
		if state.IsSearching || mode == modeNotes {
			state.refresh() // re-render the results in the new mode's context
		}
	}
	toggle := buildSearchModeToggle(state, applyModeSwitch)
	// The notes bubble sits beside the pair, not inside it: a different corpus
	// deserves a different-looking control. rebuildWindow repaints both, so the
	// fill lands on whichever one is now active.
	notesBtn := buildNotesModeButton(state, func(mode searchMode) {
		applyModeSwitch(mode)
		rebuildWindow(state)
	})

	state.focusSearch = func() {
		if state.window == nil {
			return
		}
		if state.aiSearchMode {
			state.window.Canvas().Focus(aiEntry)
		} else {
			state.window.Canvas().Focus(searchEntry)
		}
	}

	// --- Book filter + list ---
	filtered := filterBooks(state.Bible.Books, state.BookFilterQuery)

	bookFilter := widget.NewEntry()
	bookFilter.SetPlaceHolder("Filter books")
	bookFilter.SetText(state.BookFilterQuery)

	list := widget.NewList(
		func() int { return len(filtered) },
		func() fyne.CanvasObject {
			label := canvas.NewText("", pal.Text)
			label.TextSize = 13
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
	applyRowHeights(list, len(filtered))

	scrollToCurrent := func() {
		if id := indexOfBook(filtered, state.CurrentBook); id >= 0 {
			list.ScrollTo(widget.ListItemID(id))
		}
	}
	scrollToCurrent()

	list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(filtered) {
			return
		}
		selectBook(state, filtered[id], true)
		state.refresh()
	}

	bookFilter.OnChanged = func(s string) {
		state.BookFilterQuery = s
		filtered = filterBooks(state.Bible.Books, s)
		applyRowHeights(list, len(filtered))
		list.UnselectAll()
		list.Refresh()
		scrollToCurrent()
	}

	// syncSidebar re-highlights the current book without disturbing the entries.
	state.syncSidebar = func() {
		list.UnselectAll()
		list.Refresh()
		scrollToCurrent()
	}

	headerItems := []fyne.CanvasObject{sectionLabel("READ", pal)}
	if aiOn || notesFeatureOn(state) {
		// Each control collapses to a zero-height spacer when its own feature is
		// off, so this row shows whichever of them exist.
		headerItems = append(headerItems, container.NewBorder(nil, nil, nil, notesBtn, toggle))
	}
	headerItems = append(headerItems, fieldHost, captionHost, spacer(10))
	if b := incompleteBibleBanner(state); b != nil {
		headerItems = append(headerItems, b, spacer(8))
	}
	headerItems = append(headerItems,
		sectionLabel("BOOKS", pal),
		inputFrame(withCaret(state, bookFilter), pal.Border),
	)
	header := container.NewVBox(headerItems...)
	applyMode() // initialise the field + caption to the persisted mode

	body := container.NewBorder(header, nil, nil, nil, list)
	return surface(body, pal.SurfaceAlt, pal.Border, fyne.NewSize(210, 0))
}

func applyRowHeights(list *widget.List, n int) {
	for i := 0; i < n; i++ {
		list.SetItemHeight(widget.ListItemID(i), bookRowHeight)
	}
}

func sectionLabel(text string, pal palette) fyne.CanvasObject {
	t := canvas.NewText(text, pal.TextMuted)
	t.TextSize = 11
	t.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	return t
}

// caption is a small, muted hint that wraps to the sidebar width so it never
// clips, regardless of how narrow the panel is.
func caption(text string) fyne.CanvasObject {
	rt := widget.NewRichText(&widget.TextSegment{
		Text:  text,
		Style: widget.RichTextStyle{SizeName: theme.SizeNameCaptionText, ColorName: colorNameMuted},
	})
	rt.Wrapping = fyne.TextWrapWord
	return rt
}

// centeredCaption is caption() with its text centred — for the calm, centred
// wait/empty states, where a left-ragged caption under a centred heading reads
// as a misalignment.
func centeredCaption(text string) fyne.CanvasObject {
	rt := widget.NewRichText(&widget.TextSegment{
		Text: text,
		Style: widget.RichTextStyle{
			SizeName:  theme.SizeNameCaptionText,
			ColorName: colorNameMuted,
			Alignment: fyne.TextAlignCenter,
		},
	})
	rt.Wrapping = fyne.TextWrapWord
	return rt
}

// captionHeightFor sizes a bounded caption box for n wrapped lines, so a
// GridWrap can give the caption a fixed measure (keeping neighbouring elements
// centred on one axis) without hard-coding pixels at each call site.
func captionHeightFor(lines int) float32 {
	if lines < 1 {
		lines = 1
	}
	return float32(lines) * (theme.CaptionTextSize() + theme.Padding()*2)
}

func spacer(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(0, h))
	return r
}

// wrappedParagraph lays out a word-wrapping label at a fixed width, reserving its
// TRUE wrapped height. A widget.Label reports only its single-line MinSize until it
// has been laid out at some width, so the naive
// container.NewGridWrap(fyne.NewSize(w, lbl.MinSize().Height), lbl) reserves just one
// line — and any following sibling (a button, more text) then overlaps the wrapped
// text. We first Resize the label to the target width (RichText.Resize recomputes the
// wrap), then read the now-correct height for the GridWrap cell.
func wrappedParagraph(lbl *widget.Label, width float32) fyne.CanvasObject {
	lbl.Resize(fyne.NewSize(width, lbl.MinSize().Height))
	return container.NewGridWrap(fyne.NewSize(width, lbl.MinSize().Height), lbl)
}

func hgap(w float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(w, 0))
	return r
}
