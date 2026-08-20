package bibletext

import (
	"fmt"
	"image/color"
	"image/png"
	"os"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

// Candidate bins, all stroked (never a solid slab), on a 24x24 viewBox.
func binSVG(variant int, c color.Color) fyne.Resource {
	hex, a := noteSVGHex(c)
	var body string
	switch variant {
	case 1: // straight can, 3 ridges
		body = `<path d="M4 6h16M10 3h4a1 1 0 0 1 1 1v2h-6V4a1 1 0 0 1 1-1z` +
			`M6 6v14a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2V6" />` +
			`<path d="M10 10v8M14 10v8" />`
	case 2: // tapered can, 3 ridges, wider lid
		body = `<path d="M3.5 6h17M9.5 3.5h5a1 1 0 0 1 1 1V6h-7V4.5a1 1 0 0 1 1-1z` +
			`M5.5 6l1 14.2a2 2 0 0 0 2 1.8h7a2 2 0 0 0 2-1.8L18.5 6" />` +
			`<path d="M9.6 9.8l.35 8.4M12 9.8v8.4M14.4 9.8l-.35 8.4" />`
	case 3: // lighter weight, 2 ridges
		body = `<path d="M4 6.5h16M9.5 3.5h5a1 1 0 0 1 1 1v2h-7v-2a1 1 0 0 1 1-1z` +
			`M6 6.5v13.5a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2V6.5" />` +
			`<path d="M10.2 10.2v8M13.8 10.2v8" />`
	}
	sw := "1.5"
	if variant == 3 {
		sw = "1.2"
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24">`+
		`<g fill="none" stroke="%s" stroke-opacity="%.3f" stroke-width="%s" `+
		`stroke-linecap="round" stroke-linejoin="round">%s</g></svg>`, hex, a, sw, body)
	return fyne.NewStaticResource(fmt.Sprintf("bin%d.svg", variant), []byte(svg))
}

func TestZZIcons(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	pal := lightPalette

	row := container.NewHBox()
	// Fyne's current one, then the three candidates, each at 16pt.
	add := func(res fyne.Resource) {
		i := canvas.NewImageFromResource(res)
		i.FillMode = canvas.ImageFillContain
		i.SetMinSize(fyne.NewSize(16, 16))
		row.Add(container.NewPadded(i))
	}
	add(theme.DeleteIcon())
	for v := 1; v <= 3; v++ {
		add(binSVG(v, pal.TextMuted))
	}
	w := test.NewWindow(container.NewPadded(row))
	defer w.Close()
	w.Resize(fyne.NewSize(200, 60))
	f, _ := os.Create(os.Getenv("SHOT"))
	defer f.Close()
	png.Encode(f, w.Canvas().Capture())
}
