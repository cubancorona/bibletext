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
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func landscapeTouchMode(t *testing.T, active bool) {
	t.Helper()
	original := phoneLandscapeReading
	phoneLandscapeReading = func() bool { return active }
	t.Cleanup(func() { phoneLandscapeReading = original })
}

func landscapeTouchWindow(t *testing.T, root fyne.CanvasObject, size fyne.Size) fyne.Window {
	t.Helper()
	w := test.NewWindow(root)
	w.SetPadded(false)
	w.Resize(size)
	t.Cleanup(w.Close)
	return w
}

type landscapeTouchCapture struct {
	pixels          image.Image
	minimum         fyne.Size
	contentPosition fyne.Position
	contentSize     fyne.Size
	arrows          [2]fyne.Position
	arrowSizes      [2]fyne.Size
	glyphs          [2]fyne.Position
	glyphSizes      [2]fyne.Size
}

func TestLandscapeTouchTargetsPreservePixelsAndLayout(t *testing.T) {
	for _, width := range []float32{568, 874} {
		for _, notice := range []bool{false, true} {
			t.Run(fmt.Sprintf("width=%v/notice=%v", width, notice), func(t *testing.T) {
				th := coverApp(t, true)
				landscapeTouchMode(t, true)
				setNotesEnabled(true)
				capture := func(expand bool) landscapeTouchCapture {
					st := fullScreenRowState()
					st.theme = th
					if notice {
						st.NoteNotice = noteDamagedMessage
					}
					content := container.NewStack(canvas.NewRectangle(st.pal().Background),
						canvas.NewText("Synthetic reading content", st.pal().Text))
					var root fyne.CanvasObject
					if expand {
						root = fullScreenReadingBody(st, content)
					} else {
						root = container.NewPadded(container.NewBorder(fullScreenTop(st), nil, nil, nil, content))
					}
					w := landscapeTouchWindow(t, root, fyne.NewSize(width, 240))
					out := landscapeTouchCapture{pixels: w.Canvas().Capture(), minimum: root.MinSize()}
					tree := drawnTree(t, root)
					c := drawnAs(t, tree, content)
					out.contentPosition, out.contentSize = c.pos, c.obj.Size()
					buttons := findDrawn(tree, isType[*iconTapButton])
					if len(buttons) != 2 {
						t.Fatalf("got %d arrows, want 2", len(buttons))
					}
					for i, button := range buttons {
						out.arrows[i], out.arrowSizes[i] = button.pos, button.obj.Size()
						position, size, ok := button.obj.(*iconTapButton).glyphBounds()
						if !ok {
							t.Fatal("arrow glyph was not rendered")
						}
						out.glyphs[i], out.glyphSizes[i] = button.pos.Add(position), size
					}
					if expand {
						l := root.(*fyne.Container).Layout.(*landscapeReadingTouchLayout)
						below := drawnAs(t, tree, l.below)
						for _, p := range l.proxies {
							if p.Size() != fyne.NewSize(36, 36) {
								t.Errorf("landscape target = %v, want 36 by 36", p.Size())
							}
							if p.Position().Y+p.Size().Height > below.pos.Y {
								t.Error("target crosses into notice or reading content")
							}
						}
					}
					return out
				}
				before, after := capture(false), capture(true)
				landscapeTouchSamePixels(t, before.pixels, after.pixels)
				before.pixels, after.pixels = nil, nil
				if before != after {
					t.Fatalf("visible geometry or layout minimum changed: before=%+v after=%+v", before, after)
				}
				for i := range after.arrowSizes {
					if after.arrowSizes[i] != fyne.NewSize(36, 22) || after.glyphSizes[i] != fyne.NewSize(20, 20) {
						t.Errorf("arrow %d visual dimensions changed: box=%v glyph=%v", i, after.arrowSizes[i], after.glyphSizes[i])
					}
				}
			})
		}
	}
}

func landscapeTouchSamePixels(t *testing.T, before, after image.Image) {
	t.Helper()
	if before.Bounds() != after.Bounds() {
		t.Fatalf("capture bounds changed: %v -> %v", before.Bounds(), after.Bounds())
	}
	for y := before.Bounds().Min.Y; y < before.Bounds().Max.Y; y++ {
		for x := before.Bounds().Min.X; x < before.Bounds().Max.X; x++ {
			if color.NRGBAModel.Convert(before.At(x, y)) != color.NRGBAModel.Convert(after.At(x, y)) {
				t.Fatalf("pixel changed at (%d, %d)", x, y)
			}
		}
	}
}

func TestLandscapeTouchTargetsKeepGapsContentAndOverlays(t *testing.T) {
	for _, bannerPresent := range []bool{false, true} {
		t.Run(fmt.Sprint(bannerPresent), func(t *testing.T) {
			th := coverApp(t, true)
			landscapeTouchMode(t, true)
			st := fullScreenRowState()
			st.theme = th
			prev, next := chapterArrowButtons(st)
			arrows := [2]*iconTapButton{prev, next}
			fired := [2]int{}
			for i, arrow := range arrows {
				arrow.onTapped = func() { fired[i]++ }
			}
			row := fullScreenExitRowWithControls(st, container.NewHBox(prev, next))
			contentTaps, bannerTaps, parentTaps := 0, 0, 0
			content := newTapBox(canvas.NewRectangle(st.pal().Background), func() { contentTaps++ })
			var banner fyne.CanvasObject
			if bannerPresent {
				rect := canvas.NewRectangle(st.pal().SurfaceAlt)
				rect.SetMinSize(fyne.NewSize(100, 30))
				banner = newTapBox(rect, func() { bannerTaps++ })
			}
			owner := landscapeReadingTouchBody(row, banner, content, arrows)
			holder := container.NewWithoutLayout(owner)
			parent := newTapBox(holder, func() { parentTaps++ })
			w := landscapeTouchWindow(t, parent, fyne.NewSize(940, 280))
			owner.Move(fyne.NewPos(17, 19))
			owner.Resize(fyne.NewSize(874, 220))
			w.Canvas().Capture()
			l := owner.(*fyne.Container).Layout.(*landscapeReadingTouchLayout)
			tree := drawnTree(t, parent)
			for i, proxy := range l.proxies {
				p, a := drawnAs(t, tree, proxy), drawnAs(t, tree, arrows[i])
				for _, point := range []fyne.Position{
					p.pos.Add(fyne.NewPos(p.obj.Size().Width/2, 0.5)),
					p.pos.Add(fyne.NewPos(p.obj.Size().Width/2, p.obj.Size().Height-0.5)),
					a.pos.Add(fyne.NewPos(a.obj.Size().Width/2, a.obj.Size().Height/2)),
				} {
					old := fired
					test.TapCanvas(w.Canvas(), point)
					if fired[i] != old[i]+1 || fired[1-i] != old[1-i] || parentTaps != 0 || contentTaps != 0 || bannerTaps != 0 {
						t.Fatalf("tap at %v dispatched incorrectly: arrows=%v parent=%d content=%d banner=%d", point, fired, parentTaps, contentTaps, bannerTaps)
					}
				}
				center := a.pos.Add(fyne.NewPos(a.obj.Size().Width/2, a.obj.Size().Height/2))
				found, ok := hitAt(tree, center, driverMatchers[2].match)
				if !ok || found.obj != arrows[i] {
					t.Fatal("proxy intercepted the original arrow rectangle")
				}
			}
			before := fired
			left, right := drawnAs(t, tree, l.proxies[0]), drawnAs(t, tree, l.proxies[1])
			gap := (left.pos.X + left.obj.Size().Width + right.pos.X) / 2
			for _, point := range []fyne.Position{
				fyne.NewPos(gap, left.pos.Y+0.5),
				fyne.NewPos(gap, left.pos.Y+18),
				fyne.NewPos(left.pos.X-0.5, left.pos.Y+0.5),
				fyne.NewPos(right.pos.X+right.obj.Size().Width+0.5, right.pos.Y+0.5),
				fyne.NewPos(left.pos.X+18, left.pos.Y-0.5),
				owner.Position().Add(fyne.NewPos(theme.Padding()+1, theme.Padding()+5)),
			} {
				old := parentTaps
				test.TapCanvas(w.Canvas(), point)
				if parentTaps != old+1 || fired != before {
					t.Fatalf("unowned point %v did not retain parent routing: arrows=%v parent=%d", point, fired, parentTaps)
				}
			}
			below := drawnAs(t, tree, l.below)
			test.TapCanvas(w.Canvas(), fyne.NewPos(left.pos.X+18, below.pos.Y+0.5))
			if fired != before || (!bannerPresent && contentTaps != 1) || (bannerPresent && bannerTaps != 1) {
				t.Fatal("arrow stole the first point in reading content or banner")
			}
			overlayTaps := 0
			overlay := newTapBox(canvas.NewRectangle(color.Transparent), func() { overlayTaps++ })
			w.Canvas().Overlays().Add(overlay)
			overlay.Move(fyne.Position{})
			overlay.Resize(w.Canvas().Size())
			test.TapCanvas(w.Canvas(), left.pos.Add(fyne.NewPos(18, 1)))
			if overlayTaps != 1 || fired != before {
				t.Fatal("arrow stole a tap from an overlay")
			}
			w.Canvas().Overlays().Remove(overlay)
		})
	}
}

func TestLandscapeTouchTargetsReleaseDisabledAndHiddenArrows(t *testing.T) {
	for _, chapter := range []int{1, 3} {
		t.Run(fmt.Sprint(chapter), func(t *testing.T) {
			th := coverApp(t, true)
			landscapeTouchMode(t, true)
			st := fullScreenRowState()
			st.theme, st.CurrentChapter = th, chapter
			owner := fullScreenReadingBody(st, canvas.NewRectangle(st.pal().Background))
			parentTaps := 0
			parent := newTapBox(owner, func() { parentTaps++ })
			w := landscapeTouchWindow(t, parent, fyne.NewSize(874, 220))
			l := owner.(*fyne.Container).Layout.(*landscapeReadingTouchLayout)
			fired := 0
			for _, arrow := range l.arrows {
				arrow.onTapped = func() { fired++ }
			}
			off := 0
			if chapter == 3 {
				off = 1
			}
			point := l.proxies[off].Position().Add(fyne.NewPos(18, 0.5))
			test.TapCanvas(w.Canvas(), point)
			if fired != 0 || parentTaps != 1 || l.proxies[off].Visible() {
				t.Fatal("disabled arrow retained its added margin")
			}
			active := 1 - off
			point = l.proxies[active].Position().Add(fyne.NewPos(18, 0.5))
			l.arrows[active].Hide()
			test.TapCanvas(w.Canvas(), point)
			if fired != 0 || parentTaps != 2 {
				t.Fatal("hidden arrow retained a stale proxy")
			}
			l.arrows[active].Show()
			owner.Refresh()
			point = l.proxies[active].Position().Add(fyne.NewPos(18, 0.5))
			// Hide an ancestor after layout: the proxy cannot wait for a later
			// layout pass before relinquishing the old target.
			ancestors := l.proxies[active].ancestors
			ancestors[len(ancestors)-2].Hide()
			test.TapCanvas(w.Canvas(), point)
			if fired != 0 || parentTaps != 3 {
				t.Fatal("a hidden arrow ancestor retained a stale proxy")
			}
		})
	}
}

func TestLandscapeTouchTargetsFollowResizeAndRejectInvalidAllocations(t *testing.T) {
	th := coverApp(t, true)
	landscapeTouchMode(t, true)
	st := fullScreenRowState()
	st.theme = th
	owner := fullScreenReadingBody(st, canvas.NewRectangle(st.pal().Background))
	holder := container.NewWithoutLayout(owner)
	w := landscapeTouchWindow(t, holder, fyne.NewSize(1000, 260))
	l := owner.(*fyne.Container).Layout.(*landscapeReadingTouchLayout)
	fired := 0
	l.arrows[1].onTapped = func() { fired++ }
	var oldPoint fyne.Position
	for _, width := range []float32{874, 568, 874} {
		owner.Resize(fyne.NewSize(width, 220))
		w.Canvas().Capture()
		p := l.proxies[1]
		point := p.Position().Add(fyne.NewPos(p.Size().Width/2, 0.5))
		if oldPoint.X > width {
			before := fired
			test.TapCanvas(w.Canvas(), oldPoint)
			if fired != before {
				t.Fatal("old wide-layout target survived a narrower resize")
			}
		}
		before := fired
		test.TapCanvas(w.Canvas(), point)
		if fired != before+1 {
			t.Fatal("resized target missed its arrow")
		}
		oldPoint = point
	}
	for _, size := range []fyne.Size{fyne.NewSize(50, 18), fyne.NewSize(0, 0)} {
		owner.Resize(size)
		for _, p := range l.proxies {
			if p.Size().Width == 0 || p.Size().Height == 0 {
				continue
			}
			pos, bounds := p.Position(), p.Size()
			if pos.X < 0 || pos.Y < 0 || pos.X+bounds.Width > size.Width || pos.Y+bounds.Height > size.Height {
				t.Fatalf("proxy escaped undersized owner %v: position=%v size=%v", size, pos, bounds)
			}
		}
	}
}

func TestLandscapeTouchTargetsLeaveOtherModesAndUnloadedStateAlone(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		mobile, landscape, loaded bool
	}{
		{"chosen fullscreen", true, false, true},
		{"desktop", false, true, true},
		{"unloaded", true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			th := coverApp(t, tc.mobile)
			landscapeTouchMode(t, tc.landscape)
			st := fullScreenRowState()
			st.theme = th
			if !tc.loaded {
				st.Bible = nil
			}
			root := fullScreenReadingBody(st, canvas.NewRectangle(st.pal().Background))
			if _, expanded := root.(*fyne.Container).Layout.(*landscapeReadingTouchLayout); expanded {
				t.Fatal("a mode outside loaded phone-landscape reading gained targets")
			}
			if !tc.landscape && (countWidgetButtons(root) != 1 || countTapButtons(root) != 0) {
				t.Fatal("chosen full-screen no longer offers only its restore button")
			}
			if !tc.loaded && countTapButtons(root) != 0 {
				t.Fatal("unloaded Bible gained arrows")
			}
		})
	}
}
