package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// frameProbe stands where the phone and tablet panes' host stands, under the
// chapter header, and records every frame it is laid out at and every frame
// nativeHostFrame lets through to the pane, as the hosts' Resize does
// (reading_ios.go, reading_android.go).
type frameProbe struct {
	widget.BaseWidget
	laid, pushed []fyne.Size
}

func newFrameProbe() *frameProbe {
	p := &frameProbe{}
	p.ExtendBaseWidget(p)
	return p
}

func (p *frameProbe) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(layout.NewSpacer())
}

func (p *frameProbe) MinSize() fyne.Size { return fyne.NewSize(1, 1) }

func (p *frameProbe) Resize(s fyne.Size) {
	p.BaseWidget.Resize(s)
	p.laid = append(p.laid, s)
	if _, size, _, ok := nativeHostFrame(p); ok {
		p.pushed = append(p.pushed, size)
	}
}

// A paint after the window content's minimum changed lays the content out at
// its minimum first, when that is wider than the screen, and at the screen's
// size straight after (EnsureMinSize, then sizeContent, in the mobile
// driver's handlePaint). The phone header's minimum is wider than a phone, so
// the host under it is laid out wider than the canvas for a moment, and the
// pane must not be handed that frame: on Android it left a reflow armed that
// dropped every scroll after it. Only the settled frame reaches the pane.
func TestAReadingHostWiderThanItsCanvasIsNotHandedToThePane(t *testing.T) {
	th := coverApp(t, true)
	paints := 0
	for _, c := range []struct {
		book              string
		chapter, chapters int
	}{{"John", 3, 21}, {"Psalms", 119, 150}, {"Song of Solomon", 8, 8}} {
		for _, width := range []float32{320, 360, 375, 402} {
			st, nums := coverState(th, c.book, c.chapter, c.chapters)
			probe := newFrameProbe()
			pad := theme.Padding()
			band := container.New(layout.NewCustomPaddedLayout(0, 0, pad, 0), container.NewVBox(chapterHeaderMobile(st, nums)))
			page := container.New(layout.NewCustomPaddedLayout(0, pad, 0, pad), container.NewBorder(band, nil, nil, nil, probe))
			win := coverWindow(t, st, page, width)
			screen := win.Canvas().Size()
			settled := page.Size()
			minimum := page.MinSize()
			if minimum.Width <= screen.Width {
				// Nothing grows past this screen, so there is no transient to
				// keep from the pane here.
				continue
			}
			paints++
			t.Logf("%s %d at %.0f: content minimum %.1f on a canvas %.1f wide", c.book, c.chapter, width, minimum.Width, screen.Width)
			probe.laid, probe.pushed = nil, nil
			// The paint: the minimum-size pass, then the content sized to
			// the screen again.
			page.Resize(minimum.Max(settled))
			page.Resize(settled)
			wide := false
			for _, s := range probe.laid {
				wide = wide || s.Width > probe.laid[len(probe.laid)-1].Width
			}
			if !wide {
				t.Fatalf("%s %d at %.0f: the host was never laid out wider than it settles (laid out at %v); the paint is not reproduced",
					c.book, c.chapter, width, probe.laid)
			}
			if len(probe.pushed) == 0 {
				t.Fatalf("%s %d at %.0f: no frame reached the pane at all; laid out at %v", c.book, c.chapter, width, probe.laid)
			}
			for _, s := range probe.pushed {
				if s.Width > screen.Width {
					t.Errorf("%s %d at %.0f: the pane was handed a frame %.1f wide on a canvas %.1f wide (content minimum %.1f); it settles at %.1f",
						c.book, c.chapter, width, s.Width, screen.Width, minimum.Width, probe.laid[len(probe.laid)-1].Width)
				}
			}
			if last := probe.pushed[len(probe.pushed)-1]; last != probe.laid[len(probe.laid)-1] {
				t.Errorf("%s %d at %.0f: the last frame the pane took is %v, want the settled %v", c.book, c.chapter, width, last, probe.laid[len(probe.laid)-1])
			}
		}
	}
	if paints == 0 {
		t.Fatal("no header was wider than its canvas, so no paint was reproduced")
	}
}

// The test above is only as good as its claim that the header asks for more
// than a phone has: were the header ever to fit, it would pass with nothing
// to stop. A Pixel 7's canvas is 360 units wide, and John 3's header, the
// chapter the reader in the defect was on, does not fit in it.
func TestThePhoneHeaderAsksForMoreThanAPhoneCanvas(t *testing.T) {
	th := coverApp(t, true)
	st, nums := coverState(th, "John", 3, 21)
	header := chapterHeaderMobile(st, nums)
	if w := header.MinSize().Width; w <= 360 {
		t.Fatalf("John 3's phone header asks for %.1f units, which fits a 360-unit canvas; the frame test above no longer reproduces anything", w)
	}
}

func TestHostFitsCanvas(t *testing.T) {
	canvas := fyne.NewSize(360, 780)
	for _, c := range []struct {
		pos  fyne.Position
		size fyne.Size
		want bool
	}{
		{fyne.NewPos(7, 231), fyne.NewSize(339, 420), true},     // settled: 346 of 360
		{fyne.NewPos(7, 231), fyne.NewSize(425.4, 420), false},  // the minimum-size pass
		{fyne.NewPos(7, 231), fyne.NewSize(339, 600), false},    // grown past the bottom
		{fyne.NewPos(0, -45.3), fyne.NewSize(339, 420), true},   // a new window's first placement
		{fyne.NewPos(7, 231), fyne.NewSize(353.6, 549.6), true}, // to the edge, rounding
	} {
		if got := hostFitsCanvas(c.pos, c.size, canvas); got != c.want {
			t.Errorf("hostFitsCanvas(%v, %v, %v) = %v, want %v", c.pos, c.size, canvas, got, c.want)
		}
	}
	if !hostFitsCanvas(fyne.NewPos(7, 231), fyne.NewSize(425.4, 420), fyne.Size{}) {
		t.Error("a canvas not yet sized refused a frame; the pane would get none")
	}
}
