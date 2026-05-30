package holybible

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// Shared UI helpers used by both the desktop and mobile entry points. The
// platform-specific layout (HSplit + keyboard shortcuts vs. bottom tabs + drawer
// with touch-sized rows) lives in ui_desktop.go and ui_mobile.go, selected by
// build tag — `CreateMainUI` is defined in exactly one of them per build.

func buildHeader(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	// The chrome should defer to the reading text — small serif title, muted
	// subtitle, no in-app theme toggle (light vs. dark follows the system
	// appearance via the variant Fyne hands bibleTheme.Color).
	title := canvas.NewText("Holy Bible", pal.Text)
	title.TextSize = 17
	title.TextStyle = fyne.TextStyle{Bold: true}

	subtitle := canvas.NewText("World English Bible · Public Domain", pal.TextMuted)
	subtitle.TextSize = 10

	left := container.NewVBox(title, subtitle)
	row := container.NewBorder(nil, nil, left, nil, nil)

	rule := canvas.NewLine(pal.Border)
	rule.StrokeWidth = 1

	bg := canvas.NewRectangle(pal.SurfaceAlt)
	content := container.NewVBox(container.NewPadded(row), rule)
	return container.NewStack(bg, content)
}
