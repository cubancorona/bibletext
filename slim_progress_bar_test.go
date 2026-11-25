package bibletext

import (
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func findTreeInfiniteBar(root fyne.CanvasObject) *widget.ProgressBarInfinite {
	var found *widget.ProgressBarInfinite
	walkTree(root, func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.ProgressBarInfinite); ok && found == nil {
			found = b
		}
	})
	return found
}

func TestAllProgressBarsAreSlimAndStoppable(t *testing.T) {
	app := test.NewTempApp(t)
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)

	t.Run("cold start loading view", func(t *testing.T) {
		state := sampleState()
		state.app = app
		state.theme = th
		win := app.NewWindow("loading")
		t.Cleanup(win.Close)
		win.Resize(fyne.NewSize(402, 874))
		view := buildLoadingView(state)
		t.Cleanup(state.stopLoadingBar)
		win.SetContent(view)

		bar := findTreeInfiniteBar(view)
		if bar == nil {
			t.Fatal("cold-start loading view has no ProgressBarInfinite")
		}
		if math.Abs(float64(bar.Size().Height-4)) > 0.05 || math.Abs(float64(bar.Size().Width-220)) > 0.05 {
			t.Fatalf("cold-start bar size = %v, want 220x4", bar.Size())
		}
		bx := app.Driver().AbsolutePositionForObject(bar).X + bar.Size().Width/2
		wantX := app.Driver().AbsolutePositionForObject(view).X + view.Size().Width/2
		if math.Abs(float64(bx-wantX)) > 1.0 {
			t.Fatalf("cold-start bar center X = %.1f, want %.1f", bx, wantX)
		}
		stopInfiniteBars(view)
		if bar.Running() {
			t.Fatal("stopInfiniteBars did not halt cold-start ProgressBarInfinite")
		}
	})

	t.Run("translation download modal", func(t *testing.T) {
		state := sampleState()
		state.app = app
		state.theme = th
		win := app.NewWindow("download")
		t.Cleanup(win.Close)
		win.Resize(fyne.NewSize(402, 874))
		state.window = win
		win.SetContent(canvas.NewRectangle(lightPalette.Background))

		dismiss := showVersionLoading(state, "Berean Standard Bible")
		defer dismiss()
		pop := topPopup(t, win)
		test.WidgetRenderer(pop).Layout(pop.Size())

		bar := findTreeInfiniteBar(pop)
		if bar == nil {
			t.Fatal("translation download modal has no ProgressBarInfinite")
		}
		if math.Abs(float64(bar.Size().Height-4)) > 0.05 || bar.Size().Width > 240.05 || bar.Size().Width < 200 {
			t.Fatalf("translation download bar size = %v, want <=240 x 4", bar.Size())
		}
		px := app.Driver().AbsolutePositionForObject(pop.Content).X + pop.Content.Size().Width/2
		bx := app.Driver().AbsolutePositionForObject(bar).X + bar.Size().Width/2
		if math.Abs(float64(bx-px)) > 1.0 {
			t.Fatalf("translation download bar center X = %.1f, card center = %.1f", bx, px)
		}
		stopInfiniteBars(pop)
		if bar.Running() {
			t.Fatal("stopInfiniteBars did not halt translation download ProgressBarInfinite")
		}
	})

	t.Run("inline Treasury loading row", func(t *testing.T) {
		bar := widget.NewProgressBarInfinite()
		t.Cleanup(bar.Stop)
		row := crossRefTreasuryLoading(bar)
		win := app.NewWindow("treasury")
		t.Cleanup(win.Close)
		win.SetPadded(false)
		win.Resize(fyne.NewSize(340, 120))
		win.SetContent(container.NewVBox(row))

		if math.Abs(float64(bar.Size().Height-4)) > 0.05 || math.Abs(float64(bar.Size().Width-240)) > 0.05 {
			t.Fatalf("inline Treasury bar size = %v, want 240x4", bar.Size())
		}
		barX := app.Driver().AbsolutePositionForObject(bar).X
		rowX := app.Driver().AbsolutePositionForObject(row).X
		if math.Abs(float64(barX-rowX)) > 0.5 {
			t.Fatalf("inline Treasury bar X = %.1f, row X = %.1f (want leading-aligned)", barX, rowX)
		}
		stopInfiniteBars(row)
		if bar.Running() {
			t.Fatal("stopInfiniteBars did not halt inline Treasury ProgressBarInfinite")
		}
	})

	t.Run("cross-references full-panel wait and framed offline Try again", func(t *testing.T) {
		withPublisherCrossRefs(t, "nkjv", false)
		prevRun, prevLoad := crossRefsRun, crossRefsLoad
		t.Cleanup(func() { crossRefsRun, crossRefsLoad = prevRun, prevLoad })
		var held func()
		crossRefsRun = func(work func()) { held = work }
		withCrossRefIndex(t, "")
		crossRefsLoad = func() error { return errOfflineForTest }

		bd := xrefBible(map[string]map[int]int{"Ruth": {1: 5}})
		win := app.NewWindow("xrefs")
		t.Cleanup(win.Close)
		win.Resize(fyne.NewSize(402, 874))
		win.SetContent(canvas.NewRectangle(lightPalette.Background))
		state := &AppState{Bible: bd, CurrentBook: "Ruth", CurrentChapter: 1, CurrentVersion: "web", app: app, window: win, theme: th}

		showCrossRefs(state, "Synthetic selection", selSpan{lo: 1, hi: 1})
		pop := topPopup(t, win)
		t.Cleanup(pop.Hide)
		test.WidgetRenderer(pop).Layout(pop.Size())

		bar := findTreeInfiniteBar(pop)
		if bar == nil {
			t.Fatal("cross-references waiting state has no ProgressBarInfinite")
		}
		if math.Abs(float64(bar.Size().Height-4)) > 0.05 || math.Abs(float64(bar.Size().Width-240)) > 0.05 {
			t.Fatalf("cross-references waiting bar size = %v, want 240x4", bar.Size())
		}

		if held == nil {
			t.Fatal("expected deferred crossRefsRun work")
		}
		held()
		retry := findTreeButton(pop, "Try again")
		if retry == nil {
			t.Fatal("cross-references offline Try again did not appear")
		}
		framed := false
		walkTree(pop, func(o fyne.CanvasObject) {
			c, ok := o.(*fyne.Container)
			if !ok || len(c.Objects) != 2 || c.Objects[0] != fyne.CanvasObject(retry) {
				return
			}
			if r, ok := c.Objects[1].(*canvas.Rectangle); ok && r.StrokeWidth == 1 && r.CornerRadius == 10 {
				framed = true
			}
		})
		if !framed {
			t.Fatal("cross-references offline Try again is missing its 1pt rounded inputFrame")
		}
	})
}
