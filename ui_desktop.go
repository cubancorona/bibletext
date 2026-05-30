//go:build !ios && !android

package holybible

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
)

// CreateMainUI (desktop) assembles the whole window. The sidebar and the split
// are built once and stay put; navigation swaps only the reading pane's content,
// so the search and filter fields never lose focus. Toggling the theme rebuilds
// the tree with the new palette.
func CreateMainUI(app fyne.App, state *AppState, window fyne.Window) fyne.CanvasObject {
	state.app = app
	state.window = window
	registerAIState(state)
	if state.theme == nil {
		state.theme = &bibleTheme{fonts: loadBookFonts()}
	}
	app.Settings().SetTheme(state.theme)
	pal := state.pal()

	readingHost := container.NewStack(buildReadingPane(state))
	state.showReading = func() {
		readingHost.Objects = []fyne.CanvasObject{buildReadingPane(state)}
		readingHost.Refresh()
	}

	// Distraction-free reading: drop the sidebar and the app header so the
	// reading column gets the whole window. The chapter toolbar stays (so you
	// can still navigate chapters and toggle back out via its focus button).
	if state.IsFullScreen {
		// No sidebar means no search field; keep the hooks safe no-ops.
		state.syncSidebar = func() {}
		state.focusSearch = func() {}
		state.setSearchText = func(string) {}
		base := canvas.NewRectangle(pal.Background)
		root := container.NewStack(base, readingHost)
		installShortcuts(state)
		return root
	}

	sidebar := buildSidebar(state)

	split := container.NewHSplit(sidebar, readingHost)
	split.SetOffset(0.2)

	body := container.NewBorder(buildHeader(state), nil, nil, nil, split)

	base := canvas.NewRectangle(pal.Background)
	root := container.NewStack(base, body)

	installShortcuts(state)
	return root
}

// afterRebuild is a no-op on desktop — there's no native overlay to re-pin
// after the window content is swapped (see the iOS build for the real one).
func afterRebuild(*AppState) {}

// installShortcuts wires keyboard shortcuts on the window canvas. The canvas
// stores shortcuts in a map keyed by name, so re-installing after a theme
// rebuild is harmless. Handlers read state hooks lazily so they always target
// the current widgets.
func installShortcuts(state *AppState) {
	if state.window == nil {
		return
	}
	cnv := state.window.Canvas()

	cnv.AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyF,
		Modifier: fyne.KeyModifierShortcutDefault,
	}, func(fyne.Shortcut) {
		if state.focusSearch != nil {
			state.focusSearch()
		}
	})

	cnv.SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if ev.Name != fyne.KeyEscape {
			return
		}
		if !state.IsSearching && !state.HasHighlightedVerse {
			return
		}
		if state.setSearchText != nil {
			state.setSearchText("")
		}
		clearSearchState(state)
		state.refresh()
	})
}
