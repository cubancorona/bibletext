//go:build !race

package bibletext

// Pixel-level and theme-variant gates for the AI waiting column and speed
// choices: single-clip ink column verification, borderless/fill-less perimeter
// verification in both themes with a framed-button control, and an opt-in
// snapshot gallery.

import (
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func colorDiff(a, b color.Color) int {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	d := func(x, y uint32) int {
		v := int(x>>8) - int(y>>8)
		if v < 0 {
			return -v
		}
		return v
	}
	return max(d(ar, br), max(d(ag, bg), d(ab, bb)))
}

func countNonGroundPerimeterPixels(img image.Image, x0, y0, x1, y1 int, ground color.Color) int {
	b := img.Bounds()
	x0 = max(b.Min.X, x0)
	y0 = max(b.Min.Y, y0)
	x1 = min(b.Max.X-1, x1)
	y1 = min(b.Max.Y-1, y1)
	count := 0
	check := func(x, y int) {
		if colorDiff(img.At(x, y), ground) > 6 {
			count++
		}
	}
	for x := x0; x <= x1; x++ {
		check(x, y0)
		check(x, y1)
	}
	for y := y0 + 1; y < y1; y++ {
		check(x0, y)
		check(x1, y)
	}
	return count
}

// In both Light and Dark themes, choice rows and Faster options carry no
// resting background fill or border stroke; sampling the perimeter pixels finds
// pure ground, whereas the same sampler on a framed Cancel finds its stroke.
func TestAIWaitChoiceRowsHaveNoRestingBoxInEitherTheme(t *testing.T) {
	armAISpeedClock(t)
	studies := stubAIActionParked(t)
	sc := insetScreen{"402x874", 402, 874, 62, 34, 0, 0}

	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		name := "light"
		if variant == theme.VariantDark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			origVariant := appearanceVariant
			appearanceVariant = func(*AppState) fyne.ThemeVariant { return variant }
			t.Cleanup(func() { appearanceVariant = origVariant })

			st, w := seedKeyedStudyWindow(t, sc, providerGemini, "gemini-3-pro", "high", []string{"low", "medium", "high"})
			switchThemeUnder(st.app, variant)
			showAIPanel(st, aiActionExplain, "For God so loved the world", "", selSpan{})
			waitParked(t, studies, "study perimeter")
			popup := topPopup(t, w)
			defer popup.Hide()
			sheetBox(t, popup)

			disc := waitingDisclosure(popup)
			if disc == nil {
				t.Fatal("missing Faster options")
			}
			// Tree check on closed disclosure.
			if disc.wash.FillColor != color.Transparent || disc.ring.StrokeWidth != 0 {
				t.Fatalf("closed disclosure has resting fill/stroke: fill=%v stroke=%v", disc.wash.FillColor, disc.ring.StrokeWidth)
			}

			disc.Tapped(nil)
			_, rows := collectSpeedRows(popup)
			if len(rows) != 2 {
				t.Fatalf("expected 2 rows, got %d", len(rows))
			}
			for i, r := range rows {
				if r.wash.FillColor != color.Transparent || r.ring.StrokeWidth != 0 {
					t.Fatalf("row[%d] has resting fill/stroke: fill=%v stroke=%v", i, r.wash.FillColor, r.ring.StrokeWidth)
				}
			}

			img := w.Canvas().Capture()
			cnvSz := w.Canvas().Size()
			scale := float64(img.Bounds().Dx()) / float64(cnvSz.Width)
			toPx := func(pt float32) int { return int(math.Round(float64(pt) * scale)) }

			drv := fyne.CurrentApp().Driver()
			pal := st.pal()
			ground := pal.SurfaceAlt

			for _, obj := range []fyne.CanvasObject{disc, rows[0], rows[1]} {
				p := drv.AbsolutePositionForObject(obj)
				s := obj.Size()
				nonGround := countNonGroundPerimeterPixels(
					img,
					toPx(p.X)+1,
					toPx(p.Y)+1,
					toPx(p.X+s.Width)-2,
					toPx(p.Y+s.Height)-2,
					ground,
				)
				if nonGround != 0 {
					t.Errorf("%s %T perimeter had %d non-ground pixels (expected 0)", name, obj, nonGround)
				}
			}

			// Control: framed Cancel button has a visible border stroke along its straight top/bottom edges.
			cancelBtn := findTreeButton(popup, "Cancel")
			if cancelBtn == nil {
				t.Fatal("missing framed Cancel button")
			}
			cp := drv.AbsolutePositionForObject(cancelBtn)
			cs := cancelBtn.Size()
			strokeHits := 0
			for x := toPx(cp.X + 12); x <= toPx(cp.X+cs.Width-12); x++ {
				for dy := -1; dy <= 2; dy++ {
					if colorDiff(img.At(x, toPx(cp.Y)+dy), ground) > 6 {
						strokeHits++
						break
					}
				}
			}
			if strokeHits < 40 {
				t.Fatalf("control failed: framed Cancel top edge found only %d border columns", strokeHits)
			}
		})
	}
}

func countInkColumns(img image.Image, ground color.Color) []bool {
	b := img.Bounds()
	cols := make([]bool, b.Dx())
	for x := b.Min.X; x < b.Max.X; x++ {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			if colorDiff(img.At(x, y), ground) > 12 {
				cols[x-b.Min.X] = true
				break
			}
		}
	}
	return cols
}

// Pixel cross-check: a choice row mounted inside a VScroll paints the same
// horizontal ink columns as a lone unclipped copy (with >=120 ink columns in
// the lone copy as control).
func TestAIWaitChoiceRowInScrollMatchesUnclippedInk(t *testing.T) {
	app := test.NewTempApp(t)
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	pal := lightPalette

	makeRow := func() *aiSwitchActionRow {
		return newAISwitchAction(nil, "Use less reasoning this time", "24,576 tokens", func() {})
	}

	// Lone copy on wide 260x44 ground.
	lone := makeRow()
	loneBg := canvas.NewRectangle(pal.SurfaceAlt)
	wLone := app.NewWindow("lone")
	wLone.SetPadded(false)
	wLone.SetContent(container.NewStack(loneBg, lone.box))
	wLone.Resize(fyne.NewSize(260, 44))
	t.Cleanup(wLone.Close)
	loneImg := wLone.Canvas().Capture()
	loneCols := countInkColumns(loneImg, pal.SurfaceAlt)
	inkTotal := 0
	for _, on := range loneCols {
		if on {
			inkTotal++
		}
	}
	if inkTotal < 120 {
		t.Fatalf("control: lone row had only %d ink columns", inkTotal)
	}

	// In-situ inside VScroll of width 260.
	inSitu := makeRow()
	sc := container.NewVScroll(inSitu.box)
	inSituBg := canvas.NewRectangle(pal.SurfaceAlt)
	wSitu := app.NewWindow("situ")
	wSitu.SetPadded(false)
	wSitu.SetContent(container.NewStack(inSituBg, sc))
	wSitu.Resize(fyne.NewSize(260, 44))
	t.Cleanup(wSitu.Close)
	situImg := wSitu.Canvas().Capture()
	situCols := countInkColumns(situImg, pal.SurfaceAlt)

	mismatched := 0
	for i := 0; i < min(len(loneCols), len(situCols)); i++ {
		if loneCols[i] != situCols[i] {
			mismatched++
		}
	}
	if mismatched > 3 {
		t.Errorf("in-situ row mismatched %d ink columns vs lone copy (limit 3)", mismatched)
	}
}

// Opt-in visual gallery writing PNGs to BIBLETEXT_PANE_SNAPSHOT_DIR across
// canvases, themes and states after asserting layout invariants on each.
func TestTheAIWaitGallery(t *testing.T) {
	dir := os.Getenv("BIBLETEXT_PANE_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("BIBLETEXT_PANE_SNAPSHOT_DIR is unset")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	armAISpeedClock(t)
	reqs := stubCapturedStudyRequests(t)
	stubAIGenerate(t)

	origVariant := appearanceVariant
	t.Cleanup(func() { appearanceVariant = origVariant })

	screens := []insetScreen{
		{"320x568", 320, 568, 20, 0, 0, 0},
		{"402x874", 402, 874, 62, 34, 0, 0},
		{"1280x800", 1280, 800, 0, 0, 0, 0},
	}
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		vName := "light"
		if variant == theme.VariantDark {
			vName = "dark"
		}
		appearanceVariant = func(*AppState) fyne.ThemeVariant { return variant }

		for _, sc := range screens {
			// 1. Study waiting (closed + open) and Study timeout.
			st, w := seedKeyedStudyWindow(t, sc, providerGemini, "gemini-3-pro", "high", []string{"low", "medium", "high"})
			switchThemeUnder(st.app, variant)
			showAIPanel(st, aiActionExplain, "For God so loved the world, that he gave his one and only Son.", "", selSpan{})
			req := nextCapturedStudyRequest(t, reqs, "gallery-study")
			reportAIModel(req.ctx, speedTargetReport(st.keys().prefs, providerGemini, "gemini-3-pro"))
			popup := topPopup(t, w)
			sheetBox(t, popup)
			writeRenderPNG(t, filepath.Join(dir, "study-wait-closed-"+sc.name+"-"+vName+".png"), w.Canvas().Capture())

			waitingDisclosure(popup).Tapped(nil)
			writeRenderPNG(t, filepath.Join(dir, "study-wait-open-"+sc.name+"-"+vName+".png"), w.Canvas().Capture())

			finishCapturedStudyRequest(t, req, "", syntheticAITimeout())
			sheetBox(t, popup)
			writeRenderPNG(t, filepath.Join(dir, "study-timeout-"+sc.name+"-"+vName+".png"), w.Canvas().Capture())
			popup.Hide()

			// 2. Find waiting (open).
			stF, wF := seedKeyedFindWindow(t, sc, providerGemini, "gemini-3-pro", "high", []string{"low", "medium", "high"})
			switchThemeUnder(stF.app, variant)
			submitCompactFind(t, stF, "where mercy and truth meet")
			_ = nextCtx(t, aiCtxSeen)
			showFindModel(stF, stF.askSession.gen, speedTargetReport(stF.keys().prefs, providerGemini, "gemini-3-pro"))
			waitingDisclosure(compactPaneOf(wF)).Tapped(nil)
			writeRenderPNG(t, filepath.Join(dir, "find-wait-open-"+sc.name+"-"+vName+".png"), wF.Canvas().Capture())
		}
	}
}
