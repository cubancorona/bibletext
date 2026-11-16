//go:build !race

package bibletext

import (
	"fmt"
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func votdTouchCard(t *testing.T, mobile bool, width float32, variant fyne.ThemeVariant) (fyne.Window, *widget.PopUp, *iconTapButton) {
	t.Helper()
	th := coverApp(t, mobile)
	previousVariant := appearanceVariant
	appearanceVariant = func(*AppState) fyne.ThemeVariant { return variant }
	t.Cleanup(func() { appearanceVariant = previousVariant })
	fyne.CurrentApp().Settings().SetTheme(forcedVariant{Theme: th, v: variant})
	votdSynchronousRemeasure(t)
	w := test.NewTempWindow(t, canvas.NewRectangle(th.palette().Background))
	w.Resize(fyne.NewSize(width, 568))
	st := sampleState()
	st.window, st.theme = w, th
	verse := Verse{BookName: "John", Chapter: 3, Verse: 16, Text: "Synthetic passage for the card touch target."}
	st.Bible = &BibleData{Books: []string{"John"}, Verses: map[string]map[int][]Verse{"John": {3: {verse}}}}
	showVerseOfDayCard(st, dayVerse{Book: "John", Chapter: 3, Lo: 16, Hi: 16, Verses: []Verse{verse}})
	p := topPopup(t, w)
	test.WidgetRenderer(p).Layout(p.Size())
	w.Canvas().Capture()
	share := findIconTapButton(p.Content, theme.MailSendIcon().Name())
	if share == nil {
		t.Fatal("the production card has no Share control")
	}
	return w, p, share
}

// This uses the production card's construction: adding a helper without
// connecting it to the card must leave the upper-padding tap failing.
func TestVOTDShareUpperPaddingUsesOriginalAnchor(t *testing.T) {
	w, p, share := votdTouchCard(t, true, 390, theme.VariantLight)
	shares := catchAnchoredShares(t)
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(share)
	if share.Size() != fyne.NewSize(33, 22) {
		t.Fatalf("Share geometry changed: %v", share.Size())
	}
	test.TapCanvas(w.Canvas(), pos.Add(fyne.NewPos(16.5, -8)))
	if len(*shares) != 1 {
		t.Fatalf("the production card's upper padding handed over %d shares, want 1", len(*shares))
	}
	want := shareAnchor{x: pos.X, y: pos.Y, w: 33, h: 22}
	if got := (*shares)[0]; !got.ok || got.at != want {
		t.Fatalf("padding share anchor = %+v; want original control %+v", got, want)
	}
	if _, ok := shareAnchorNow(); ok {
		t.Fatal("padding share left an anchor for a later share")
	}
	if !p.Visible() {
		t.Fatal("sharing closed the card")
	}
}

func TestVOTDShareTouchPreservesProductionCardPixelsAndLayout(t *testing.T) {
	for _, width := range []float32{320, 390, 900} {
		for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
			t.Run(fmt.Sprintf("width=%g/variant=%d", width, variant), func(t *testing.T) {
				w, p, share := votdTouchCard(t, true, width, variant)
				owner, ok := p.Content.(*fyne.Container)
				if !ok {
					t.Fatal("the card has no touch owner")
				}
				l, ok := owner.Layout.(*votdShareTouchLayout)
				if !ok {
					t.Fatal("the production card has no Share padding target")
				}
				before := w.Canvas().Capture()
				minimum, size, pos := p.Content.MinSize(), p.Content.Size(), p.Content.Position()
				type geometry struct {
					kind string
					pos  fyne.Position
					size fyne.Size
				}
				geometryOf := func(tree []drawnObject) []geometry {
					var result []geometry
					for _, o := range tree {
						if o.obj == l.card || o.under(l.card) {
							result = append(result, geometry{fmt.Sprintf("%T", o.obj), o.pos, o.obj.Size()})
						}
					}
					return result
				}
				beforeGeometry := geometryOf(drawnTree(t, p))
				if share.Size() != fyne.NewSize(33, 22) || l.proxy.Size() != fyne.NewSize(33, 40) {
					t.Fatalf("unexpected Share/target sizes: %v / %v", share.Size(), l.proxy.Size())
				}
				_, glyphSize, ok := share.glyphBounds()
				if !ok || glyphSize != fyne.NewSize(17, 17) {
					t.Fatalf("Share glyph changed: %v", glyphSize)
				}
				// Reopen the exact original card without the touch owner. The
				// popup receives the same outer size, including its own padding.
				popupPadding := p.MinSize().Subtract(minimum)
				p.Hide()
				legacy := widget.NewModalPopUp(l.card, w.Canvas())
				t.Cleanup(legacy.Hide)
				legacy.Show()
				legacy.Resize(size.Add(popupPadding))
				test.WidgetRenderer(legacy).Layout(legacy.Size())
				touchSamePixels(t, before, w.Canvas().Capture())
				if legacy.Content.MinSize() != minimum || legacy.Content.Size() != size || legacy.Content.Position() != pos {
					t.Fatal("touch owner changed the card's minimum or allocated geometry")
				}
				// Popup teardown may recreate renderer objects. Their displayed
				// types, positions and sizes must stay identical, not their pointers.
				afterGeometry := geometryOf(drawnTree(t, legacy))
				if len(afterGeometry) != len(beforeGeometry) {
					t.Fatalf("visible object count changed: %d -> %d", len(beforeGeometry), len(afterGeometry))
				}
				for i, before := range beforeGeometry {
					if before != afterGeometry[i] {
						t.Fatalf("visible object %d changed: %+v -> %+v", i, before, afterGeometry[i])
					}
				}
			})
		}
	}
}

func TestVOTDShareTouchKeepsCanvasEdgesAndOverlays(t *testing.T) {
	w, p, share := votdTouchCard(t, true, 390, theme.VariantLight)
	owner := p.Content.(*fyne.Container)
	l := owner.Layout.(*votdShareTouchLayout)
	tree := drawnTree(t, p)
	a, target := drawnAs(t, tree, share), drawnAs(t, tree, l.proxy)
	fired := 0
	share.onTapped = func() { fired++ }
	for _, point := range []fyne.Position{
		target.pos.Add(fyne.NewPos(0.5, 0.5)),
		target.pos.Add(fyne.NewPos(32.5, 0.5)),
		a.pos.Add(fyne.NewPos(16.5, -0.5)),
		a.pos.Add(fyne.NewPos(16.5, 0.5)),
		a.pos.Add(fyne.NewPos(16.5, 21.5)),
	} {
		old := fired
		test.TapCanvas(w.Canvas(), point)
		if fired != old+1 {
			t.Fatalf("Share did not receive exactly one tap at %v", point)
		}
	}
	center := a.pos.Add(fyne.NewPos(16.5, 11))
	if hit, ok := hitAt(tree, center, driverMatchers[2].match); !ok || hit.obj != share {
		t.Fatal("proxy intercepted the original Share rectangle")
	}
	before := fired
	for _, point := range []fyne.Position{
		target.pos.Add(fyne.NewPos(16.5, -0.5)),
		target.pos.Add(fyne.NewPos(-0.5, 0.5)),
		target.pos.Add(fyne.NewPos(33.5, 0.5)),
		a.pos.Add(fyne.NewPos(-0.5, 11)),
		a.pos.Add(fyne.NewPos(33.5, 11)),
		a.pos.Add(fyne.NewPos(16.5, 22.5)),
	} {
		test.TapCanvas(w.Canvas(), point)
		if fired != before {
			t.Fatalf("Share stole an outside tap at %v", point)
		}
	}
	body := findReadingParagraph(p.Content)
	bodyAt := drawnAs(t, tree, body)
	test.TapCanvas(w.Canvas(), bodyAt.pos.Add(fyne.NewPos(1, 1)))
	if fired != before {
		t.Fatal("Share stole a passage tap")
	}
	overlayTaps := 0
	overlay := newTapBox(canvas.NewRectangle(color.Transparent), func() { overlayTaps++ })
	w.Canvas().Overlays().Add(overlay)
	overlay.Move(fyne.Position{})
	overlay.Resize(w.Canvas().Size())
	test.TapCanvas(w.Canvas(), target.pos.Add(fyne.NewPos(16.5, 1)))
	w.Canvas().Overlays().Remove(overlay)
	if overlayTaps != 1 || fired != before {
		t.Fatal("Share stole a higher overlay's tap")
	}
}

func TestVOTDShareTouchReleasesHiddenAndDisabledControls(t *testing.T) {
	w, p, share := votdTouchCard(t, true, 390, theme.VariantLight)
	l := p.Content.(*fyne.Container).Layout.(*votdShareTouchLayout)
	ancestor := l.proxy.ancestors[len(l.proxy.ancestors)-2]
	point := fyne.CurrentApp().Driver().AbsolutePositionForObject(share).Add(fyne.NewPos(16.5, -8))
	fired := 0
	share.onTapped = func() { fired++ }
	check := func() {
		t.Helper()
		if l.proxy.Visible() {
			t.Fatal("inactive Share still exposes a padding target")
		}
		test.TapCanvas(w.Canvas(), point)
		l.proxy.Tapped(&fyne.PointEvent{})
		if fired != 0 {
			t.Fatal("inactive Share received a padding tap")
		}
	}
	share.disabled = true
	check()
	share.disabled = false
	share.Hide()
	check()
	share.Show()
	ancestor.Hide()
	check()
	ancestor.Show()
	p.Hide()
	test.TapCanvas(w.Canvas(), point)
	if fired != 0 {
		t.Fatal("dismissed card retained a canvas target")
	}
}

func TestVOTDShareTouchLeavesDesktopCardUnwrapped(t *testing.T) {
	_, p, share := votdTouchCard(t, false, 900, theme.VariantLight)
	if got := votdShareTouchTarget(p.Content, share); got != p.Content {
		t.Fatal("desktop card was wrapped or resized")
	}
	if c, ok := p.Content.(*fyne.Container); ok {
		if _, expanded := c.Layout.(*votdShareTouchLayout); expanded {
			t.Fatal("desktop production card gained a touch proxy")
		}
	}
}
