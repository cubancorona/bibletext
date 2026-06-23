package bibletext

// Share-as-image preview. Rendering a verse card is cheap and offline, so before
// anything leaves the app we show the card in a modal: the reader can Regenerate
// (cycle the colour treatment) until they like it, then Share it to the OS share
// sheet — or Cancel. Mirrors the cross-reference / AI panels' overlay dance.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func showShareImagePreview(state *AppState, quote, cite, abbrev string) {
	if state == nil || state.window == nil {
		return
	}
	cnv := state.window.Canvas()
	if cnv == nil {
		return
	}
	pal := state.pal()

	// The native reading overlay floats above the Fyne canvas, so it must go down
	// while the modal is up (same as every other popup).
	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	restore := func() {
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	}

	ps := aiPanelSize(cnv.Size())
	side := minF(ps.Width-44, ps.Height-190)
	if side < 200 {
		side = 200
	}

	// The card is square; scale it to fit the preview box.
	img := &canvas.Image{FillMode: canvas.ImageFillContain}
	imgBox := container.NewGridWrap(fyne.NewSize(side, side), img)

	variant := 0
	curPath := ""
	render := func() {
		path, err := renderVerseImage(state, quote, cite, abbrev, variant)
		if err != nil {
			return
		}
		curPath = path
		img.Resource = nil
		img.File = path
		img.Refresh()
	}
	render() // initial card (variant 0 = the verse's default treatment)

	var popup *widget.PopUp
	closePanel := func() {
		if popup != nil {
			popup.Hide()
		}
		restore()
	}

	title := canvas.NewText("Share as image", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 22
	sub := canvas.NewText(cite+" ("+abbrev+")", pal.Accent)
	sub.TextStyle = fyne.TextStyle{Bold: true}
	sub.TextSize = subheadingTextSize
	header := container.NewVBox(title, sub, widget.NewSeparator())

	regen := widget.NewButtonWithIcon("Regenerate", theme.ViewRefreshIcon(), func() {
		variant++
		render()
	})
	regen.Importance = widget.LowImportance

	shareBtn := widget.NewButtonWithIcon("Share", theme.MailSendIcon(), func() {
		p := curPath
		closePanel()
		if p != "" {
			nativeShareImage(p)
		}
	})
	shareBtn.Importance = widget.HighImportance

	cancel := widget.NewButton("Cancel", closePanel)

	footer := container.NewVBox(
		widget.NewSeparator(),
		container.NewHBox(cancel, layout.NewSpacer(), regen, shareBtn),
	)

	content := container.NewBorder(header, footer, nil, nil, container.NewCenter(imgBox))
	popup = widget.NewModalPopUp(
		surface(container.NewPadded(content), pal.SurfaceAlt, pal.Border, fyne.Size{}),
		cnv,
	)
	popup.Show()
	popup.Resize(fyne.NewSize(ps.Width, minF(ps.Height, side+220)))
}
