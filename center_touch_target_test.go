//go:build !race

package bibletext

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func centerTouchTestApp(t *testing.T, mobile bool) {
	t.Helper()
	app := test.NewTempApp(t)
	if mobile {
		fyne.SetCurrentApp(phoneTestApp{app})
	}
}

func centerTouchTestWindow(t *testing.T, content fyne.CanvasObject, size fyne.Size) fyne.Window {
	t.Helper()
	window := test.NewWindow(content)
	window.SetPadded(false)
	window.Resize(size)
	t.Cleanup(window.Close)
	return window
}

// The probe records even disabled invocations, so the proxy's own guard is
// tested rather than relying on a Button to reject a forwarded disabled tap.
type centerTouchProbe struct {
	widget.BaseWidget
	minimum  fyne.Size
	disabled bool
	events   []fyne.PointEvent
}

func newCenterTouchProbe() *centerTouchProbe {
	p := &centerTouchProbe{minimum: fyne.NewSize(20, 20)}
	p.ExtendBaseWidget(p)
	return p
}

func (p *centerTouchProbe) CreateRenderer() fyne.WidgetRenderer {
	r := canvas.NewRectangle(color.NRGBA{R: 170, G: 90, B: 40, A: 255})
	r.SetMinSize(p.minimum)
	return widget.NewSimpleRenderer(r)
}
func (p *centerTouchProbe) Tapped(event *fyne.PointEvent) { p.events = append(p.events, *event) }
func (p *centerTouchProbe) Disable()                      { p.disabled = true }
func (p *centerTouchProbe) Enable()                       { p.disabled = false }
func (p *centerTouchProbe) Disabled() bool                { return p.disabled }

func TestCenterTouchTargetPreservesCenteredPixelsAndGeometry(t *testing.T) {
	for _, dense := range []bool{false, true} {
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprintf("dense=%v/dark=%v", dense, dark), func(t *testing.T) {
				centerTouchTestApp(t, true)
				var th fyne.Theme = theme.LightTheme()
				if dark {
					th = theme.DarkTheme()
				}
				if dense {
					th = browseRowTheme{Theme: th}
				}
				fyne.CurrentApp().Settings().SetTheme(th)
				for _, allocation := range []fyne.Size{
					fyne.NewSize(19, 80), fyne.NewSize(36, 60), fyne.NewSize(80, 80), fyne.NewSize(19, 22),
				} {
					capture := func(enlarge bool) (image.Image, fyne.Size, fyne.Position, fyne.Size) {
						button := widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)
						button.Importance = widget.LowImportance
						var slot fyne.CanvasObject = container.NewCenter(button)
						if enlarge {
							slot = centerTouchTarget(button)
						}
						root := container.NewWithoutLayout(slot)
						window := centerTouchTestWindow(t, root, fyne.NewSize(140, 140))
						slot.Move(fyne.NewPos(20, 20))
						slot.Resize(allocation)
						return window.Canvas().Capture(), slot.MinSize(), button.Position(), button.Size()
					}
					before, oldMin, oldPosition, oldSize := capture(false)
					after, newMin, newPosition, newSize := capture(true)
					if oldMin != newMin || oldPosition != newPosition || oldSize != newSize {
						t.Errorf("allocation %v changed geometry: min %v→%v, position %v→%v, size %v→%v",
							allocation, oldMin, newMin, oldPosition, newPosition, oldSize, newSize)
					}
					if before.Bounds() != after.Bounds() {
						t.Fatalf("capture bounds changed: %v→%v", before.Bounds(), after.Bounds())
					}
					for y := before.Bounds().Min.Y; y < before.Bounds().Max.Y; y++ {
						for x := before.Bounds().Min.X; x < before.Bounds().Max.X; x++ {
							if color.NRGBAModel.Convert(before.At(x, y)) != color.NRGBAModel.Convert(after.At(x, y)) {
								t.Fatalf("allocation %v changed pixel at (%d,%d)", allocation, x, y)
							}
						}
					}
				}
			})
		}
	}
}

func TestCenterTouchTargetCanvasTapsStayWithinOwnedSpace(t *testing.T) {
	centerTouchTestApp(t, true)
	probe := newCenterTouchProbe()
	slot := centerTouchTarget(probe)
	parentTaps := 0
	parent := newTapBox(slot, func() { parentTaps++ })
	window := centerTouchTestWindow(t, parent, fyne.NewSize(80, 80))

	// The newly captured ring lies outside the unchanged visible20×20 object.
	for _, point := range []fyne.Position{
		fyne.NewPos(40, 18), fyne.NewPos(40, 61), fyne.NewPos(18, 40), fyne.NewPos(61, 40),
	} {
		before := len(probe.events)
		test.TapCanvas(window.Canvas(), point)
		if len(probe.events) != before+1 || parentTaps != 0 {
			t.Fatalf("ring tap %v reached probe %d times / parent %d times", point, len(probe.events)-before, parentTaps)
		}
		event := probe.events[len(probe.events)-1]
		if event.AbsolutePosition != point || event.Position != fyne.NewPos(10, 10) {
			t.Errorf("ring event = %+v; want original absolute point and valid control center", event)
		}
	}

	// The proxy is behind the original widget, whose relative event survives.
	test.TapCanvas(window.Canvas(), fyne.NewPos(35, 36))
	if event := probe.events[len(probe.events)-1]; event.Position != fyne.NewPos(5, 6) {
		t.Errorf("the original widget's tap was intercepted: %+v", event)
	}

	before := len(probe.events)
	for _, point := range []fyne.Position{
		fyne.NewPos(40, 17), fyne.NewPos(40, 62), fyne.NewPos(17, 40), fyne.NewPos(62, 40),
	} {
		test.TapCanvas(window.Canvas(), point)
	}
	if len(probe.events) != before || parentTaps != 4 {
		t.Errorf("outside44 taps were swallowed: probe=%d, parent=%d", len(probe.events)-before, parentTaps)
	}
}

func TestCenterTouchTargetNarrowSlotsKeepNeighbouringGapsInert(t *testing.T) {
	centerTouchTestApp(t, true)
	left, right := newCenterTouchProbe(), newCenterTouchProbe()
	row := container.NewHBox(centerTouchTarget(left), centerTouchTarget(right))
	parentTaps := 0
	parent := newTapBox(row, func() { parentTaps++ })
	width := 40 + theme.Padding()
	window := centerTouchTestWindow(t, parent, fyne.NewSize(width, 80))

	test.TapCanvas(window.Canvas(), fyne.NewPos(10, 20))
	test.TapCanvas(window.Canvas(), fyne.NewPos(30+theme.Padding(), 20))
	if len(left.events) != 1 || len(right.events) != 1 || parentTaps != 0 {
		t.Fatalf("bounded slot taps: left=%d right=%d parent=%d", len(left.events), len(right.events), parentTaps)
	}
	test.TapCanvas(window.Canvas(), fyne.NewPos(20+theme.Padding()/2, 40))
	if len(left.events) != 1 || len(right.events) != 1 || parentTaps != 1 {
		t.Error("the inter-control gap activated a neighbouring target")
	}
	if left.Size() != left.minimum || right.Size() != right.minimum {
		t.Error("the visible widgets grew")
	}
}

func TestCenterTouchTargetClipsToShortAllocation(t *testing.T) {
	centerTouchTestApp(t, true)
	probe := newCenterTouchProbe()
	probe.minimum = fyne.NewSize(20, 10)
	slot := centerTouchTarget(probe)
	root := container.NewWithoutLayout(slot)
	parentTaps := 0
	parent := newTapBox(root, func() { parentTaps++ })
	window := centerTouchTestWindow(t, parent, fyne.NewSize(80, 80))
	slot.Move(fyne.NewPos(30, 25))
	slot.Resize(fyne.NewSize(20, 30))

	for _, point := range []fyne.Position{fyne.NewPos(40, 25), fyne.NewPos(40, 54)} {
		test.TapCanvas(window.Canvas(), point)
	}
	if len(probe.events) != 2 || parentTaps != 0 {
		t.Fatalf("short cell ring did not reach control: probe=%d parent=%d", len(probe.events), parentTaps)
	}
	for _, point := range []fyne.Position{fyne.NewPos(40, 24), fyne.NewPos(40, 55), fyne.NewPos(29, 40), fyne.NewPos(50, 40)} {
		test.TapCanvas(window.Canvas(), point)
	}
	if len(probe.events) != 2 || parentTaps != 4 {
		t.Errorf("target escaped its allocated cell: probe=%d parent=%d", len(probe.events), parentTaps)
	}
}

func TestCenterTouchTargetRejectsDisabledAndHiddenControls(t *testing.T) {
	centerTouchTestApp(t, true)
	probe := newCenterTouchProbe()
	parentTaps := 0
	parent := newTapBox(centerTouchTarget(probe), func() { parentTaps++ })
	window := centerTouchTestWindow(t, parent, fyne.NewSize(80, 80))

	probe.Disable()
	test.TapCanvas(window.Canvas(), fyne.NewPos(40, 20))
	if len(probe.events) != 0 || parentTaps != 1 {
		t.Errorf("disabled margin did not return to parent: probe=%d parent=%d", len(probe.events), parentTaps)
	}
	probe.Enable()
	test.TapCanvas(window.Canvas(), fyne.NewPos(40, 20))
	if len(probe.events) != 1 {
		t.Fatal("a re-enabled control did not receive its tap")
	}
	probe.Hide()
	test.TapCanvas(window.Canvas(), fyne.NewPos(40, 20))
	if len(probe.events) != 1 || parentTaps != 2 {
		t.Errorf("hidden control retained a hit area: probe=%d parent=%d", len(probe.events), parentTaps)
	}
}

func TestCenterTouchTargetDesktopAndNonTappableContentStayUnchanged(t *testing.T) {
	for _, mobile := range []bool{false, true} {
		t.Run(fmt.Sprintf("mobile=%v", mobile), func(t *testing.T) {
			centerTouchTestApp(t, mobile)
			var content fyne.CanvasObject = canvas.NewRectangle(color.Black)
			if !mobile {
				content = newCenterTouchProbe()
			}
			slot := centerTouchTarget(content).(*fyne.Container)
			if _, ok := slot.Layout.(*centerTouchLayout); ok {
				t.Error("content that needs no proxy acquired one")
			}
			if len(slot.Objects) != 1 || slot.Objects[0] != content || slot.MinSize() != layout.NewCenterLayout().MinSize([]fyne.CanvasObject{content}) {
				t.Error("desktop or non-tappable NewCenter behaviour changed")
			}
		})
	}
}
