package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// CreateMainUI assembles the whole window. The sidebar and the split are built
// once and stay put; navigation swaps only the reading pane's content, so the
// search and filter fields never lose focus. Toggling the theme rebuilds the
// tree with the new palette.
func CreateMainUI(app fyne.App, state *AppState, window fyne.Window) fyne.CanvasObject {
	state.app = app
	state.window = window
	if state.theme == nil {
		state.theme = &bibleTheme{fonts: loadBookFonts()}
	}
	app.Settings().SetTheme(state.theme)
	pal := state.pal()

	sidebar := buildSidebar(state)

	readingHost := container.NewStack(buildReadingPane(state))
	state.showReading = func() {
		readingHost.Objects = []fyne.CanvasObject{buildReadingPane(state)}
		readingHost.Refresh()
	}

	split := container.NewHSplit(sidebar, readingHost)
	split.SetOffset(0.2)

	body := container.NewBorder(buildHeader(state), nil, nil, nil, split)

	base := canvas.NewRectangle(pal.Background)
	root := container.NewStack(base, body)

	installShortcuts(state)
	return root
}

func buildHeader(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	title := canvas.NewText("Holy Bible", pal.Text)
	title.TextSize = 21
	title.TextStyle = fyne.TextStyle{Bold: true}

	subtitle := canvas.NewText("World English Bible · Public Domain", pal.TextMuted)
	subtitle.TextSize = 11

	toggleLabel := "Dark mode"
	if state.theme.dark {
		toggleLabel = "Light mode"
	}
	themeToggle := widget.NewButton(toggleLabel, func() {
		toggleTheme(state)
	})
	themeToggle.Importance = widget.LowImportance

	left := container.NewVBox(title, subtitle)
	right := container.NewVBox(layout.NewSpacer(), themeToggle, layout.NewSpacer())
	row := container.NewBorder(nil, nil, left, right, nil)

	rule := canvas.NewLine(pal.Border)
	rule.StrokeWidth = 1

	bg := canvas.NewRectangle(pal.SurfaceAlt)
	content := container.NewVBox(container.NewPadded(row), rule)
	return container.NewStack(bg, content)
}

func toggleTheme(state *AppState) {
	if state.theme == nil || state.app == nil || state.window == nil {
		return
	}
	state.theme.dark = !state.theme.dark
	state.app.Settings().SetTheme(state.theme)
	state.window.SetContent(CreateMainUI(state.app, state, state.window))
}

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
