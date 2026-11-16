package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
)

// This canvas object exercises forwarding without renderer or app settings.
type centerTouchDispatchProbe struct {
	position fyne.Position
	size     fyne.Size
	hidden   bool
	disabled bool
	events   []fyne.PointEvent
}

func (p *centerTouchDispatchProbe) MinSize() fyne.Size          { return p.size }
func (p *centerTouchDispatchProbe) Move(position fyne.Position) { p.position = position }
func (p *centerTouchDispatchProbe) Position() fyne.Position     { return p.position }
func (p *centerTouchDispatchProbe) Resize(size fyne.Size)       { p.size = size }
func (p *centerTouchDispatchProbe) Size() fyne.Size             { return p.size }
func (p *centerTouchDispatchProbe) Hide()                       { p.hidden = true }
func (p *centerTouchDispatchProbe) Visible() bool               { return !p.hidden }
func (p *centerTouchDispatchProbe) Show()                       { p.hidden = false }
func (p *centerTouchDispatchProbe) Refresh()                    {}
func (p *centerTouchDispatchProbe) Disable()                    { p.disabled = true }
func (p *centerTouchDispatchProbe) Enable()                     { p.disabled = false }
func (p *centerTouchDispatchProbe) Disabled() bool              { return p.disabled }
func (p *centerTouchDispatchProbe) Tapped(event *fyne.PointEvent) {
	p.events = append(p.events, *event)
}

func TestCenterTouchProxyForwardsCoordinatesWithoutMutatingEvent(t *testing.T) {
	content := &centerTouchDispatchProbe{
		position: fyne.NewPos(-3, -5),
		size:     fyne.NewSize(20, 30),
	}
	proxy := &centerTouchProxy{content: content}
	for _, tc := range []struct {
		name  string
		event *fyne.PointEvent
		want  fyne.PointEvent
	}{
		{"inside", &fyne.PointEvent{Position: fyne.NewPos(2, 4), AbsolutePosition: fyne.NewPos(92, 104)},
			fyne.PointEvent{Position: fyne.NewPos(5, 9), AbsolutePosition: fyne.NewPos(92, 104)}},
		{"margin", &fyne.PointEvent{Position: fyne.NewPos(21, 4), AbsolutePosition: fyne.NewPos(111, 104)},
			fyne.PointEvent{Position: fyne.NewPos(10, 15), AbsolutePosition: fyne.NewPos(111, 104)}},
		{"nil", nil, fyne.PointEvent{Position: fyne.NewPos(10, 15)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var original fyne.PointEvent
			if tc.event != nil {
				original = *tc.event
			}
			before := len(content.events)
			proxy.Tapped(tc.event)
			if len(content.events) != before+1 || content.events[before] != tc.want {
				t.Fatalf("forwarded events = %v; want one additional event %+v", content.events, tc.want)
			}
			if tc.event != nil && *tc.event != original {
				t.Errorf("forwarding mutated the caller's event: %+v -> %+v", original, *tc.event)
			}
		})
	}
}

func TestCenterTouchProxyRejectsUnavailableContentWithoutRenderer(t *testing.T) {
	content := &centerTouchDispatchProbe{size: fyne.NewSize(20, 30)}
	proxy := &centerTouchProxy{content: content}
	content.Disable()
	proxy.Tapped(nil)
	content.Enable()
	content.Hide()
	proxy.Tapped(nil)
	if len(content.events) != 0 {
		t.Fatal("unavailable content received a tap")
	}
	content.Show()
	proxy.Tapped(nil)
	if len(content.events) != 1 {
		t.Fatal("restored content did not receive one tap")
	}
}
