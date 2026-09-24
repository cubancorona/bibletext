//go:build bibletextdev

package bibletext

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// THE TEXT-SIZE SLIDER — a continuous stand-in for the reader's three text
// sizes, for watching the reading page (reading_page.go) follow the size on the
// pane itself: the type, the measure, the leading and the switch between the
// two pages. DEVELOPMENT BUILDS ONLY: nothing in a release build sets
// readingTextScaleOverride, so a reader gets the three named sizes and nothing
// between them.
//
// On the Read tab rather than the Links tab, because the Links tab takes the
// reading pane's place: a slider there could only move a pane nobody can see.
// The Links tab's "Text-size slider on the Read tab" puts the strip above the
// pane; the slider moves the size, and the pane is re-rendered in place a
// moment after the thumb stops. "Setting" hands the size back to the reader's
// own choice, and turning the strip off does the same.
//
// BIBLETEXT_DEV_TEXT_SCALE seeds it at launch, for a scripted look: "on" shows
// the strip at the setting's size, a number (1.45) shows it at that size.

var (
	devTextScaleStripOn, devTextScale = devTextScaleSeed(os.Getenv("BIBLETEXT_DEV_TEXT_SCALE"))
	devTextScaleTimer                 *time.Timer
)

// devTextScaleSeed reads the launch seed: whether the strip is on, and the size
// it starts at (0: the setting's).
func devTextScaleSeed(v string) (bool, float64) {
	v = strings.TrimSpace(v)
	if strings.EqualFold(v, "on") {
		return true, 0
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil && f >= devTextScaleMin && f <= devTextScaleMax {
		return true, f
	}
	return false, 0
}

const (
	devTextScaleMin = 0.8
	devTextScaleMax = 1.6
	// devTextScaleSettle is how long the thumb must rest before the pane is
	// re-rendered: a native pane re-imports its chapter at every new size.
	devTextScaleSettle = 60 * time.Millisecond
)

func init() {
	readingTextScaleOverride = func() (float64, bool) { return devTextScale, devTextScale > 0 }
}

// devTextScaleStrip is the strip above the reading pane, or nil while it is
// off. pane is the reading slot, whose width the numbers read.
func devTextScaleStrip(state *AppState, pane fyne.CanvasObject) fyne.CanvasObject {
	if !devTextScaleStripOn {
		return nil
	}
	numbers := widget.NewLabel("")
	numbers.TextStyle = fyne.TextStyle{Monospace: true}
	show := func() { numbers.SetText(devTextScaleNumbers(float64(pane.Size().Width))) }

	rerender := func() {
		if devTextScaleTimer != nil {
			devTextScaleTimer.Stop()
		}
		devTextScaleTimer = time.AfterFunc(devTextScaleSettle, func() {
			fyne.Do(func() {
				state.refreshReadingOnly()
				show()
			})
		})
	}

	slider := widget.NewSlider(devTextScaleMin, devTextScaleMax)
	slider.Step = 0.01
	slider.SetValue(readingTextScale())
	settingBack := false
	slider.OnChanged = func(v float64) {
		if settingBack {
			return
		}
		devTextScale = v
		show()
		rerender()
	}
	setting := widget.NewButton("Setting", func() {
		devTextScale = 0
		settingBack = true
		slider.SetValue(readingTextScale())
		settingBack = false
		show()
		rerender()
	})
	show()
	// The numbers are figured from the pane's width, which it has only once
	// laid out; ask again when it has.
	time.AfterFunc(150*time.Millisecond, func() { fyne.Do(show) })

	// The app's theme sets the input border to zero (theme.go), and a Fyne
	// slider draws its track two borders tall, so the track is given one here.
	track := container.NewThemeOverride(slider, devSliderTheme{fyne.CurrentApp().Settings().Theme()})
	return container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel("Text size"), setting, track),
		numbers,
	)
}

// devSliderTheme is the app's theme with an input border wide enough to draw
// the slider's track.
type devSliderTheme struct{ fyne.Theme }

func (t devSliderTheme) Size(n fyne.ThemeSizeName) float32 {
	if n == theme.SizeNameInputBorder {
		return 1.5
	}
	return t.Theme.Size(n)
}

// devTextScaleNumbers is the spec's arithmetic at the slider's size, for the
// pane's width: what the page will be, and where it switches.
func devTextScaleNumbers(paneWidth float64) string {
	if readingPaneWidth > 0 {
		paneWidth = readingPaneWidth // a native pane's own report is the width it uses
	}
	ref := readingReferencePx()
	page := readingPageAt(paneWidth, ref)
	return fmt.Sprintf("×%.2f   size %.1f (set %.1f)   measure %.1f   book page from %.1f   pane %.0f: %v page",
		readingTextScale(), ref, readingGlyphPx(), reporterMeasureEm*ref,
		reporterMeasureEm*ref+2*readingPageSideMin, paneWidth, page.Kind)
}
