//go:build bibletextdev

package bibletext

import (
	"math"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// devStripParts finds the slider, the Setting button and the numbers line in
// the strip.
func devStripParts(t *testing.T, o fyne.CanvasObject) (s *widget.Slider, b *widget.Button, numbers *widget.Label) {
	t.Helper()
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch w := o.(type) {
		case *widget.Slider:
			s = w
		case *widget.Button:
			b = w
		case *widget.Label:
			if w.TextStyle.Monospace {
				numbers = w
			}
		case *fyne.Container:
			for _, c := range w.Objects {
				walk(c)
			}
		case *container.ThemeOverride:
			walk(w.Content)
		}
	}
	walk(o)
	if s == nil || b == nil || numbers == nil {
		t.Fatalf("the strip is missing a part: slider %v, button %v, numbers %v", s != nil, b != nil, numbers != nil)
	}
	return s, b, numbers
}

// The slider moves the reading size continuously, re-renders the pane in place
// once the thumb rests, and says what the page is at that size — down to the
// page switching when the measure no longer fits. "Setting" hands the size back.
func TestTheTextSizeSliderMovesThePaneLive(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	prevOn, prevScale, prevW := devTextScaleStripOn, devTextScale, readingPaneWidth
	t.Cleanup(func() { devTextScaleStripOn, devTextScale, readingPaneWidth = prevOn, prevScale, prevW })
	devTextScaleStripOn, devTextScale, readingPaneWidth = true, 0, 0

	st := sampleState()
	repaints := make(chan struct{}, 16)
	st.showReading = func() { repaints <- struct{}{} }
	pane := widget.NewLabel("the pane")
	pane.Resize(fyne.NewSize(900, 600))
	slider, setting, numbers := devStripParts(t, devTextScaleStrip(st, pane))

	if got := readingTextScale(); got != 1.0 {
		t.Fatalf("the strip moved the size before the slider did: %v", got)
	}
	wait := func(what string) {
		select {
		case <-repaints:
		case <-time.After(2 * time.Second):
			t.Fatalf("the pane was not re-rendered after %s", what)
		}
	}

	before := chapterRenderFingerprint(st)
	slider.SetValue(1.4)
	wait("moving the slider")
	if got := readingTextScale(); math.Abs(got-1.4) > 1e-9 {
		t.Errorf("the reading size is ×%v, want the slider's ×1.4", got)
	}
	if chapterRenderFingerprint(st) == before {
		t.Error("the chapter's fingerprint did not change with the size, so a native pane would skip the re-render")
	}
	// 27.5 × 21 × 1.4 = 808.5; with 15 a side the book page needs 838.5.
	for _, want := range []string{"×1.40", "measure 808.5", "book page from 838.5", "pane 900: book page"} {
		if !strings.Contains(numbers.Text, want) {
			t.Errorf("the numbers do not say %q: %s", want, numbers.Text)
		}
	}
	// ×1.6: a 924 measure needs 954, more than the pane has.
	slider.SetValue(1.6)
	wait("moving the slider past the switch")
	if !strings.Contains(numbers.Text, "pane 900: phone page") {
		t.Errorf("the page did not switch when the measure stopped fitting: %s", numbers.Text)
	}

	test.Tap(setting)
	wait("handing the size back")
	if devTextScale != 0 || readingTextScale() != 1.0 || math.Abs(slider.Value-1.0) > 1e-9 {
		t.Errorf("Setting left the size at ×%v (override %v, slider %v), want the setting's ×1", readingTextScale(), devTextScale, slider.Value)
	}
}

// The launch seed: "on" shows the strip at the setting's size, a size in the
// slider's range shows it at that size, and anything else leaves it off.
func TestTheTextSizeSeed(t *testing.T) {
	for _, tc := range []struct {
		in    string
		on    bool
		scale float64
	}{
		{"", false, 0}, {"on", true, 0}, {"ON", true, 0}, {"1.45", true, 1.45},
		{"3", false, 0}, {"0.5", false, 0}, {"large", false, 0},
	} {
		if on, scale := devTextScaleSeed(tc.in); on != tc.on || scale != tc.scale {
			t.Errorf("seed %q = %v, %v; want %v, %v", tc.in, on, scale, tc.on, tc.scale)
		}
	}
}
