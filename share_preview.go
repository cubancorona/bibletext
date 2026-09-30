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

// shareMail is what a mail carrying a shared picture says.
type shareMail struct{ subject, body string }

// shareImageMail is what a mail carrying the last card the preview handed to
// the platform says, for Email… on the desktop confirmation, whose share seam
// takes the file's path alone (fallbackShareImage): the citation as the
// subject, "John 3:16 (World English Bible)", and as the text the quote and
// its citation — what Share with citation would have copied — so that no
// route that drops the picture leaves a blank message. Set before every
// hand-off; UI goroutine only.
var shareImageMail shareMail

// shareImageOut hands the rendered card to the platform. A variable, like
// shareTextOut, so a test can drive the preview's Share into the desktop
// fallback on a Mac, whose own route opens the system picker.
var shareImageOut = func(path string) { nativeShareImage(path) }

// previewCardRender renders the card the preview shows. A variable, so a test
// that opens the preview many times over with the same passage can render
// its card once: the card is a function of its arguments alone, and drawing
// and encoding its 1080-pixel square is most of what opening the preview
// costs.
var previewCardRender = renderVerseImage

func showShareImagePreview(state *AppState, quote, cite, version string) {
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

	ps := sheetPanelSize(state, cnv)
	side := shareImageSide(ps, headerClearance(state) > 0)

	// The card is square; scale it to fit the preview box.
	img := &canvas.Image{FillMode: canvas.ImageFillContain}
	imgBox := container.NewGridWrap(fyne.NewSize(side, side), img)

	variant := 0
	curPath := ""
	render := func() {
		path, err := previewCardRender(state, quote, cite, version, variant)
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
	sub := canvas.NewText(cite+" ("+version+")", pal.Accent)
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
			shareImageMail.subject = cite + " (" + version + ")"
			shareImageMail.body = composeShareText(quote, cite, version)
			shareImageOut(p)
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
	// On a phone or tablet, clear of the header's controls or over them
	// (touchSheetHeight). The image keeps its side there, so the least the
	// sheet can be is what it measures.
	resize := func() {
		h := minF(ps.Height, side+220)
		h = touchSheetHeight(state, popup, ps.Width, h, ps.Height <= side+220, func() float32 { return popup.MinSize().Height })
		popup.Resize(fyne.NewSize(ps.Width, h))
	}
	resize()
	// Sized again, image and all, when a desktop window changes size
	// (sheet_refit.go).
	registerSheetRefit(state, popup, func() {
		ps.Height = sheetPanelSize(state, cnv).Height
		side = shareImageSide(ps, headerClearance(state) > 0)
		imgBox.Layout = layout.NewGridWrapLayout(fyne.NewSize(side, side))
		imgBox.Refresh()
		resize()
	})
}

// shareImageSide is the side of the preview image in a share sheet of size
// ps: the height left under the title and above the buttons, no wider than
// the sheet.
//
// On a phone or tablet it is at least 200pt, so the card stays legible and
// the sheet grows past its size to hold it. On a desktop window the sheet's
// height is capped to open below the header (clearOfHeader), and it can only
// keep to that cap if the image follows it down: with the 200pt floor, a
// window 440pt tall centred the sheet's 347pt of content 43pt down, partway
// down the Go to chip. There the image only keeps a side of at least one
// point; the shortest window the app allows still leaves it about 38pt.
func shareImageSide(ps fyne.Size, belowHeader bool) float32 {
	side := minF(ps.Width-44, ps.Height-190)
	floor := float32(200)
	if belowHeader {
		floor = 1
	}
	if side < floor {
		side = floor
	}
	return side
}
