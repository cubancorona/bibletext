package holybible

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// Shared UI helpers used by both the desktop and mobile entry points. The
// platform-specific layout (HSplit + keyboard shortcuts vs. bottom tabs + drawer
// with touch-sized rows) lives in ui_desktop.go and ui_mobile.go, selected by
// build tag — `CreateMainUI` is defined in exactly one of them per build.

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

// toggleTheme flips light/dark and rebuilds the window content so every
// palette-coloured canvas object is recreated against the new colours.
func toggleTheme(state *AppState) {
	if state.theme == nil || state.app == nil || state.window == nil {
		return
	}
	state.theme.dark = !state.theme.dark
	state.app.Settings().SetTheme(state.theme)
	state.window.SetContent(CreateMainUI(state.app, state, state.window))
}
