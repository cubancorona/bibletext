package main

import (
	"image/color"

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

	// --- Search ---
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search…")
	searchEntry.SetText(state.SearchQuery)

	// Live search runs synchronously on the UI goroutine for every keystroke
	// (the corpus is small and indexed, so this is fast and race-free). It only
	// lists matches; pressing Enter additionally jumps to an exact verse ref.
	searchEntry.OnChanged = func(s string) {
		searchResultsOnly(state, s)
	}
	searchEntry.OnSubmitted = func(s string) {
		executeSearch(state, s)
	}

	clearSearch := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		searchEntry.SetText("")
	})
	clearSearch.Importance = widget.LowImportance
	searchRow := container.NewBorder(nil, nil, nil, clearSearch, inputFrame(searchEntry, pal.Border))

	state.focusSearch = func() {
		if state.window != nil {
			state.window.Canvas().Focus(searchEntry)
		}
	}
	state.setSearchText = func(s string) {
		searchEntry.SetText(s)
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

	header := container.NewVBox(
		sectionLabel("READ", pal),
		searchRow,
		caption("Keyword, or a reference like John 3:16."),
		spacer(10),
		sectionLabel("BOOKS", pal),
		inputFrame(bookFilter, pal.Border),
	)

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

func spacer(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(0, h))
	return r
}

func hgap(w float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(w, 0))
	return r
}
