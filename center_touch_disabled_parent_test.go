//go:build !race

package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func TestCenterTouchTargetDisabledMarginReturnsToParent(t *testing.T) {
	centerTouchTestApp(t, true)
	controlTaps, parentTaps := 0, 0
	button := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() { controlTaps++ })
	parent := newTapBox(centerTouchTarget(button), func() { parentTaps++ })
	window := centerTouchTestWindow(t, parent, fyne.NewSize(80, 80))
	margin := fyne.NewPos(40, 18.5)
	if button.Position().Y <= margin.Y {
		t.Fatal("fixture has no added margin above the button")
	}
	button.Disable()
	test.TapCanvas(window.Canvas(), margin)
	if controlTaps != 0 || parentTaps != 1 {
		t.Fatalf("disabled margin: control=%d parent=%d; want 0, 1", controlTaps, parentTaps)
	}
	// The existing disabled button keeps owning its original rectangle.
	test.TapCanvas(window.Canvas(), fyne.NewPos(40, 40))
	if controlTaps != 0 || parentTaps != 1 {
		t.Fatalf("disabled button: control=%d parent=%d; want 0, 1", controlTaps, parentTaps)
	}
	button.Enable()
	test.TapCanvas(window.Canvas(), margin)
	if controlTaps != 1 || parentTaps != 1 {
		t.Fatalf("enabled margin: control=%d parent=%d; want 1, 1", controlTaps, parentTaps)
	}
}
