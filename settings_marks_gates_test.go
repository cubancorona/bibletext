//go:build !race

package bibletext

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/software"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type gatesVariantApp struct {
	fyne.App
	variant fyne.ThemeVariant
	mobile  bool
}

func (a gatesVariantApp) Settings() fyne.Settings {
	return gatesVariantSettings{a.App.Settings(), a.variant}
}

func (a gatesVariantApp) Driver() fyne.Driver {
	if a.mobile {
		return phoneTestDriver{a.App.Driver()}
	}
	return a.App.Driver()
}

type gatesVariantSettings struct {
	fyne.Settings
	variant fyne.ThemeVariant
}

func (s gatesVariantSettings) ThemeVariant() fyne.ThemeVariant { return s.variant }

func startGatesApp(t *testing.T, dark, mobile bool) *bibleTheme {
	t.Helper()
	app := test.NewTempApp(t)
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	v := theme.VariantLight
	if dark {
		v = theme.VariantDark
	}
	fyne.SetCurrentApp(gatesVariantApp{App: app, variant: v, mobile: mobile})
	app.Settings().SetTheme(th)
	return th
}

func gatesAbsRect(o fyne.CanvasObject) (x, y, w, h float32) {
	d := fyne.CurrentApp().Driver()
	p := d.AbsolutePositionForObject(o)
	if c := d.CanvasForObject(o); c != nil {
		inset, _ := c.InteractiveArea()
		p = p.Add(inset)
	}
	s := o.Size()
	return p.X, p.Y, s.Width, s.Height
}

func gatesColourDist(a, b color.Color) float64 {
	r1, g1, b1, _ := a.RGBA()
	r2, g2, b2, _ := b.RGBA()
	dr := float64(r1>>8) - float64(r2>>8)
	dg := float64(g1>>8) - float64(g2>>8)
	db := float64(b1>>8) - float64(b2>>8)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

func gatesInkBounds(img image.Image, rx, ry, rw, rh, scale float32, ground color.Color, tol float64) (minX, minY, maxX, maxY float32, ok bool) {
	px := image.Rect(
		int(math.Floor(float64(rx*scale))),
		int(math.Floor(float64(ry*scale))),
		int(math.Ceil(float64((rx+rw)*scale))),
		int(math.Ceil(float64((ry+rh)*scale))),
	).Intersect(img.Bounds())
	ix0, iy0, ix1, iy1 := math.MaxInt, math.MaxInt, -1, -1
	for y := px.Min.Y; y < px.Max.Y; y++ {
		for x := px.Min.X; x < px.Max.X; x++ {
			if gatesColourDist(img.At(x, y), ground) > tol {
				ix0, iy0 = min(ix0, x), min(iy0, y)
				ix1, iy1 = max(ix1, x), max(iy1, y)
			}
		}
	}
	if ix1 < 0 {
		return 0, 0, 0, 0, false
	}
	return float32(ix0) / scale, float32(iy0) / scale, float32(ix1+1) / scale, float32(iy1+1) / scale, true
}

// gatesCaptureScroll renders w while isolating scroll on its own origin-aligned
// software canvas so nested Entry scrollers cannot replace the outer scroll
// clip in Fyne's software painter.
func gatesCaptureScroll(w fyne.Window, scroll *container.Scroll) image.Image {
	c := w.Canvas()
	if scroll == nil || !scroll.Visible() {
		return c.Capture()
	}
	sx, sy, _, _ := gatesAbsRect(scroll)
	scroll.Hide()
	base := c.Capture()
	scroll.Show()

	proxy := software.NewTransparentCanvas()
	proxy.SetPadded(false)
	proxy.SetScale(c.Scale())
	shifter := container.NewWithoutLayout(scroll)
	shifter.Move(fyne.NewPos(-scroll.Position().X, -scroll.Position().Y))
	root := container.NewWithoutLayout(shifter)
	proxy.SetContent(root)
	proxy.Resize(scroll.Size())
	body := proxy.Capture()
	shifter.Objects = nil

	out := image.NewNRGBA(base.Bounds())
	draw.Draw(out, out.Bounds(), base, image.Point{}, draw.Src)
	s := c.Scale()
	x0, y0 := int(math.Round(float64(sx*s))), int(math.Round(float64(sy*s)))
	draw.Draw(out, body.Bounds().Add(image.Pt(x0, y0)), body, image.Point{}, draw.Over)
	return out
}

func gatesFirstObject[T fyne.CanvasObject](root fyne.CanvasObject, match func(T) bool) T {
	var found T
	done := false
	walkTree(root, func(o fyne.CanvasObject) {
		if done {
			return
		}
		if v, ok := o.(T); ok && match(v) {
			found, done = v, true
		}
	})
	return found
}

// TestSettingsPickerRowsGeometryAndAlignment verifies Settings picker rows,
// key-field outlines, and modal picker chrome across all nine target widths in
// both light and dark appearances:
//   - zero mid-glyph clipping on Model and Reasoning effort / Thinking budget
//     for Recommended, gemini-4-carbon, and a 60-character model identifier;
//   - no container.Clip or container.ThemeOverride inside modelRows;
//   - both dropdown triangles share one right edge within 0.5pt of Get a key's
//     outward arrow right ink edge;
//   - modelCaption sits below Reasoning effort as a direct VBox sibling at the
//     same bottom-note spacing as READING's caption;
//   - both API key inputs carry a 1pt pal.Border inputFrame and withCaret;
//   - the modal picker sheet uses an 18pt bold title, header separator,
//     SurfaceAlt fill, and trailing ConfirmIcon on the active row without
//     overlapping long model identifiers at 320pt.
func TestSettingsPickerRowsGeometryAndAlignment(t *testing.T) {
	prevList := aiSettingsListModels
	aiSettingsListModels = func(context.Context, providerInfo, string) ([]discoveredModel, error) {
		return nil, errors.New("offline")
	}
	t.Cleanup(func() { aiSettingsListModels = prevList })

	const longModel60 = "gemini-4-experimental-reasoning-preview-2026-11-25-extended0"
	if len(longModel60) != 60 {
		t.Fatalf("synthetic 60-character id has length %d", len(longModel60))
	}

	widths := []struct {
		w, h   float32
		mobile bool
	}{
		{320, 568, true},
		{360, 800, true},
		{375, 667, true},
		{393, 852, true},
		{402, 874, true},
		{440, 956, true},
		{834, 1194, true},
		{1032, 768, false},
		{1280, 800, false},
	}

	cases := []struct {
		name     string
		provider string
		model    string
		effort   string
		seed     []discoveredModel
	}{
		{"recommended", providerGemini, "", "", []discoveredModel{{id: geminiModel, efforts: []string{"low", "medium", "high"}}}},
		{"carbon-high", providerGemini, "gemini-4-carbon", "high", nil},
		{"long60-xhigh", providerOpenAI, longModel60, "xhigh", []discoveredModel{{id: longModel60, efforts: []string{"low", "medium", "high", "xhigh"}}}},
		{"budget-32768", providerAnthropic, "claude-sonnet-4-5", "32768", []discoveredModel{{id: "claude-sonnet-4-5", efforts: []string{"1024", "4096", "16384", "32768"}}}},
	}

	for _, dark := range []bool{false, true} {
		look := "light"
		if dark {
			look = "dark"
		}
		for _, sz := range widths {
			t.Run(fmt.Sprintf("%s-%.0f", look, sz.w), func(t *testing.T) {
				th := startGatesApp(t, dark, sz.mobile)
				pal := th.palette()
				pixelCheckWidth := sz.w == 320 || sz.w == 375 || sz.w == 402 || sz.w == 834 || sz.w == 1280

				for _, tc := range cases {
					win := test.NewWindow(canvas.NewRectangle(pal.Background))
					scale := float32(1)
					if pixelCheckWidth && tc.name == "carbon-high" {
						scale = 2
					}
					win.Canvas().(test.WindowlessCanvas).SetScale(scale)
					win.Resize(fyne.NewSize(sz.w, sz.h))

					state := sampleState()
					state.window, state.theme = win, th
					state.aiKeys = newKeyStoreWith(newFakePrefs())
					store := state.keys()
					store.setActiveProvider(tc.provider)
					store.setAPIKey(tc.provider, "synthetic-key-000000000000000000000000000")
					if tc.seed != nil {
						rememberModelEfforts(store.prefs, tc.provider, tc.seed)
					}
					if tc.model != "" {
						store.setOverrideModel(tc.provider, tc.model)
					}
					activeModel := activeModelFor(store, tc.provider)
					if tc.effort != "" {
						setAIReasoningEffort(store.prefs, tc.provider, activeModel, tc.effort)
					}

					showAISettings(state)
					sheet := settingsPopup(t, state)

					modelRows := gatesFirstObject(sheet, func(c *fyne.Container) bool {
						_, ok := c.Layout.(*modelSettingsLayout)
						return ok
					})
					if modelRows == nil {
						t.Fatalf("%s: missing modelRows container", tc.name)
					}
					layout := modelRows.Layout.(*modelSettingsLayout)

					// No Clip or ThemeOverride inside modelRows.
					walkTree(modelRows, func(o fyne.CanvasObject) {
						switch o.(type) {
						case *container.Clip:
							t.Errorf("%s: unexpected container.Clip inside modelRows", tc.name)
						case *container.ThemeOverride:
							t.Errorf("%s: unexpected container.ThemeOverride inside modelRows", tc.name)
						}
					})

					// Locate card inner container, Get a key link, and modelCaption.
					var assistantCard *fyne.Container
					walkTree(sheet, func(o fyne.CanvasObject) {
						c, ok := o.(*fyne.Container)
						if !ok || len(c.Objects) != 2 || assistantCard != nil {
							return
						}
						if fr, ok := c.Objects[0].(*canvas.Rectangle); ok && fr.FillColor == pal.SurfaceAlt && fr.StrokeWidth == 1 {
							if gatesFirstObject(c, func(x *fyne.Container) bool { return x == modelRows }) != nil {
								assistantCard = c
							}
						}
					})
					if assistantCard == nil {
						t.Fatalf("%s: missing ASSISTANT card", tc.name)
					}
					cardInner := assistantCard.Objects[1].(*fyne.Container).Objects[0]
					ix, _, iw, _ := gatesAbsRect(cardInner)
					cardLeft, cardRight := ix, ix+iw

					getKey := gatesFirstObject(assistantCard, func(l *outboundLink) bool { return l.Text == "Get a key" })
					if getKey == nil {
						t.Fatalf("%s: missing Get a key outboundLink", tc.name)
					}
					kx, _, _, _ := gatesAbsRect(getKey)
					arrowAt, arrowSz := getKey.arrowBox()
					// arrow_outward.svg reaches x=23/24 inside its box.
					getKeyArrowInkRight := kx + arrowAt.X + arrowSz.Width*(23.0/24.0)

					// Both rows: check full un-ellipsised Button.Text, zero mid-glyph cut,
					// value right edge left of dropdown icon box, and right-edge alignment.
					var markInkRights []float32
					for _, row := range []*settingsPickerRow{layout.model, layout.effort} {
						if !row.Visible() {
							t.Fatalf("%s: expected %s row visible", tc.name, row.label.Text)
						}
						rx, _, rw, rh := gatesAbsRect(row)
						if rh < minTapTarget-0.01 {
							t.Errorf("%s %s: row height %.2f < 44pt", tc.name, row.label.Text, rh)
						}
						if math.Abs(float64(rx-cardLeft)) > 0.5 || math.Abs(float64((rx+rw)-cardRight)) > 0.5 {
							t.Errorf("%s %s: row %.1f..%.1f does not match card inner %.1f..%.1f",
								tc.name, row.label.Text, rx, rx+rw, cardLeft, cardRight)
						}

						labelSizeName := row.label.SizeName
						if labelSizeName == "" {
							labelSizeName = theme.SizeNameText
						}
						if th.Size(labelSizeName) != 18 || row.label.TextStyle != (fyne.TextStyle{}) {
							t.Errorf("%s %s: label size=%v style=%+v, want 18pt regular",
								tc.name, row.label.Text, th.Size(labelSizeName), row.label.TextStyle)
						}
						if row.value.TextSize != 15 || row.value.TextStyle != (fyne.TextStyle{Bold: true}) {
							t.Errorf("%s %s: value style size=%v style=%+v, want 15 bold", tc.name, row.label.Text, row.value.TextSize, row.value.TextStyle)
						}

						vx, _, vw, _ := gatesAbsRect(row.value)
						mx, _, mw, _ := gatesAbsRect(row.mark)
						measuredW := fyne.MeasureText(row.value.Text, 15, fyne.TextStyle{Bold: true}).Width
						if math.Abs(float64(measuredW-vw)) > 0.5 {
							t.Errorf("%s %s: value box %.2f disagrees with measured %.2f", tc.name, row.label.Text, vw, measuredW)
						}
						if vx < cardLeft-0.5 || vx+vw > mx+0.01 {
							t.Errorf("%s %s: value %.2f..%.2f crosses card left %.2f or icon box left %.2f",
								tc.name, row.label.Text, vx, vx+vw, cardLeft, mx)
						}
						if row.value.Text != row.Text {
							if !strings.HasSuffix(row.value.Text, "…") {
								t.Errorf("%s %s: shortened value %q does not end with ellipsis", tc.name, row.label.Text, row.value.Text)
							}
							if tc.name == "long60-xhigh" && row == layout.model && row.Text != longModel60 {
								t.Errorf("%s: Button.Text mutated to %q, want full 60-char id", tc.name, row.Text)
							}
						}
						markInkRight := mx + mw*dropMarkInkRight
						markInkRights = append(markInkRights, markInkRight)
						if math.Abs(float64(markInkRight-getKeyArrowInkRight)) > 0.5 {
							t.Errorf("%s %s: dropdown mark right ink %.2f differs from Get a key arrow %.2f by >0.5pt",
								tc.name, row.label.Text, markInkRight, getKeyArrowInkRight)
						}
					}
					if len(markInkRights) == 2 && math.Abs(float64(markInkRights[0]-markInkRights[1])) > 0.05 {
						t.Errorf("%s: Model and effort dropdown marks do not share one right edge: %.2f vs %.2f",
							tc.name, markInkRights[0], markInkRights[1])
					}

					// Verify modelCaption sits directly below modelRows as a VBox sibling
					// at the exact same native VBox gap as READING's caption.
					modelCap := gatesFirstObject(assistantCard, func(r *widget.RichText) bool {
						return strings.HasPrefix(segmentText(r.Segments), "Recommended keeps itself current")
					})
					readingCap := gatesFirstObject(sheet, func(r *widget.RichText) bool {
						return strings.HasPrefix(segmentText(r.Segments), "Verse of the day:")
					})
					textSizeLbl := gatesFirstObject(sheet, func(l *widget.Label) bool {
						return l.Text == "Text size"
					})
					if modelCap == nil || readingCap == nil || textSizeLbl == nil {
						t.Fatalf("%s: missing modelCaption (%v), readingCap (%v), or Text size label (%v)",
							tc.name, modelCap != nil, readingCap != nil, textSizeLbl != nil)
					}
					_, mry, _, mrh := gatesAbsRect(modelRows)
					_, mcy, _, _ := gatesAbsRect(modelCap)
					modelVBoxGap := mcy - (mry + mrh)
					if math.Abs(float64(modelVBoxGap-7.0)) > 0.5 {
						t.Errorf("%s: modelCaption VBox gap = %.2f, want 7.00", tc.name, modelVBoxGap)
					}
					// Compare visible bottom-note distance against READING's inline label->caption rhythm.
					if sz.w >= 834 {
						_, ty, _, thLbl := gatesAbsRect(textSizeLbl)
						_, rcy, _, _ := gatesAbsRect(readingCap)
						readingGap := rcy - (ty + thLbl)
						if math.Abs(float64(modelVBoxGap-readingGap)) > 0.5 {
							t.Errorf("%s: modelCaption gap %.2f differs from READING caption gap %.2f",
								tc.name, modelVBoxGap, readingGap)
						}
					}

					// Pixel cross-check on 2x software canvas for carbon-high.
					if scale == 2 {
						formScroll := gatesFirstObject(sheet, func(s *container.Scroll) bool {
							c, ok := s.Content.(*fyne.Container)
							if !ok {
								return false
							}
							_, sq := c.Layout.(squeezeWidthLayout)
							return sq
						})
						_, scY, _, _ := gatesAbsRect(formScroll.Content)
						_, cardY, _, cardH := gatesAbsRect(assistantCard)
						view := formScroll.Size().Height
						most := max(float32(0), formScroll.Content.Size().Height-view)
						off := max(float32(0), min(cardY-scY+cardH-view+8, most))
						formScroll.Offset = fyne.NewPos(0, off)
						formScroll.Refresh()

						img := gatesCaptureScroll(win, formScroll)
						mx1, my1, mw1, mh1 := gatesAbsRect(layout.model.mark)
						mx2, my2, mw2, mh2 := gatesAbsRect(layout.effort.mark)
						_, _, r1, _, ok1 := gatesInkBounds(img, mx1, my1, mw1, mh1, scale, pal.SurfaceAlt, 24)
						_, _, r2, _, ok2 := gatesInkBounds(img, mx2, my2, mw2, mh2, scale, pal.SurfaceAlt, 24)
						if !ok1 || !ok2 {
							t.Errorf("%s: dropdown mark ink missing in 2x render (model=%v effort=%v)", tc.name, ok1, ok2)
						} else if math.Abs(float64(r1-r2)) > 0.5 {
							t.Errorf("%s: painted dropdown right edges differ: %.2f vs %.2f", tc.name, r1, r2)
						}
					}

					// Verify both PasswordEntry inputs sit inside inputFrame + withCaret,
					// and open the Model picker to check its header, separator, fill, and
					// active-only ConfirmIcon.
					if tc.name == "recommended" && sz.w == 402 {
						entries := 0
						framedEntries := 0
						walkTree(sheet, func(o fyne.CanvasObject) {
							c, ok := o.(*fyne.Container)
							if !ok || len(c.Objects) != 2 {
								return
							}
							rect, ok := c.Objects[1].(*canvas.Rectangle)
							if !ok || rect.StrokeWidth != 1 || rect.StrokeColor != pal.Border || rect.CornerRadius != 10 {
								return
							}
							if gatesFirstObject(c.Objects[0], func(*widget.Entry) bool { return true }) != nil {
								framedEntries++
							}
						})
						walkTree(sheet, func(o fyne.CanvasObject) {
							if _, ok := o.(*widget.Entry); ok {
								entries++
							}
						})
						if entries != 2 || framedEntries != 2 {
							t.Errorf("Settings password entries = %d, framed with 1pt pal.Border = %d, want 2 and 2", entries, framedEntries)
						}

						layout.model.OnTapped()
						picker := state.window.Canvas().Overlays().Top().(*widget.PopUp)
						if picker == sheet {
							t.Fatal("Model tap did not open picker popup")
						}
						titleTxt := gatesFirstObject(picker, func(tx *canvas.Text) bool { return tx.Text == "Model" })
						sep := gatesFirstObject(picker, func(*widget.Separator) bool { return true })
						surfaceRect := gatesFirstObject(picker, func(r *canvas.Rectangle) bool {
							return r.FillColor == pal.SurfaceAlt && r.StrokeWidth == 1
						})
						if titleTxt == nil || titleTxt.TextSize != 18 || !titleTxt.TextStyle.Bold || sep == nil || surfaceRect == nil {
							t.Errorf("picker chrome mismatch: title=%v sep=%v surfaceAlt=%v",
								titleTxt != nil && titleTxt.TextSize == 18, sep != nil, surfaceRect != nil)
						}
						list := settingsPickerList(t, state)
						item0 := list.CreateItem()
						list.UpdateItem(0, item0)
						activeCheck := gatesFirstObject(item0, func(ic *widget.Icon) bool { return ic.Visible() })
						if activeCheck == nil {
							t.Error("active picker row is missing visible ConfirmIcon")
						}
						if list.Length() > 1 {
							item1 := list.CreateItem()
							list.UpdateItem(1, item1)
							if gatesFirstObject(item1, func(ic *widget.Icon) bool { return ic.Visible() }) != nil {
								t.Error("unselected picker row unexpectedly shows ConfirmIcon")
							}
						}
						picker.Hide()
					}

					// At 320pt with a long chosen model ID, verify the active picker row
					// truncates with an ellipsis before ConfirmIcon.
					if tc.name == "long60-xhigh" && sz.w == 320 {
						layout.model.OnTapped()
						picker := state.window.Canvas().Overlays().Top().(*widget.PopUp)
						_ = win.Canvas().Capture()
						list := settingsPickerList(t, state)
						item := list.CreateItem()
						item.Resize(fyne.NewSize(picker.Content.Size().Width-2*theme.Padding(), 44))
						list.UpdateItem(1, item)
						chosenText := gatesFirstObject(item, func(x *canvas.Text) bool { return x.TextStyle.Bold })
						chosenCheck := gatesFirstObject(item, func(x *widget.Icon) bool { return x.Visible() })
						if chosenText == nil || chosenCheck == nil {
							t.Fatal("320pt long60 picker missing active bold row text or visible ConfirmIcon")
						}
						win.SetContent(container.NewWithoutLayout(item))
						_ = win.Canvas().Capture()
						txX, _, _, _ := gatesAbsRect(chosenText)
						ckX, _, _, _ := gatesAbsRect(chosenCheck)
						txW := fyne.MeasureText(chosenText.Text, chosenText.TextSize, chosenText.TextStyle).Width
						if !strings.HasSuffix(chosenText.Text, "…") || txX+txW > ckX+0.01 {
							t.Errorf("320pt active picker text %q (%.1f..%.1f) collides with ConfirmIcon at %.1f",
								chosenText.Text, txX, txX+txW, ckX)
						}
						picker.Hide()
					}

					sheet.Hide()
					win.Close()
				}
			})
		}
	}
}

// TestDisclosureAndDropdownMarksAcrossSurfaces verifies vector disclosure and
// dropdown marks across Cross-References, the header version selector, the
// audio source chip, and Ask/Share composers (including trailing placement in
// landscape keyboard viewports and framed composer Cancel buttons).
func TestDisclosureAndDropdownMarksAcrossSurfaces(t *testing.T) {
	th := startGatesApp(t, true, true)

	// 1. Cross-References fold chevrons: 26-unit SVG stroke chevrons, never MenuExpandIcon or MenuDropDownIcon.
	closedRes, openRes := crossRefGroupIcon(false), crossRefGroupIcon(true)
	if closedRes == openRes {
		t.Fatal("closed and open cross-ref fold icons must be distinct")
	}
	for _, res := range []fyne.Resource{closedRes, openRes} {
		low := strings.ToLower(res.Name())
		if strings.Contains(low, "expand") || strings.Contains(low, "drop") {
			t.Fatalf("crossRefGroupIcon still references triangle icons: %s", res.Name())
		}
	}
	if !strings.Contains(string(closedRes.Content()), `viewBox="0 0 26 26"`) ||
		!strings.Contains(string(openRes.Content()), `viewBox="0 0 26 26"`) {
		t.Fatal("cross-ref chevron resources do not carry 26x26 SVG viewBox")
	}

	// 2. Header versionSelector and audio labeledTapChip carry vector MenuDropDownIcon,
	// no triangle/chevron rune, and clear the audio card's close button.
	st := sampleState()
	st.theme = th
	sel := versionSelector(st)
	anchor, _ := sel.(*versionPickerAnchor)
	if anchor == nil {
		t.Fatal("expected *versionPickerAnchor")
	}
	dropSVG := string(theme.NewColoredResource(theme.MenuDropDownIcon(), theme.ColorNamePlaceHolder).Content())
	anchorDrop := false
	for _, obj := range test.WidgetRenderer(anchor).Objects() {
		walkTree(obj, func(o fyne.CanvasObject) {
			if tx, ok := o.(*canvas.Text); ok && strings.ContainsAny(tx.Text, "▾▸▼⌄⌃") {
				t.Errorf("versionPickerAnchor still carries arrow/chevron rune in text: %q", tx.Text)
			}
			if im, ok := o.(*canvas.Image); ok && im.Resource != nil && string(im.Resource.Content()) == dropSVG {
				anchorDrop = true
			}
		})
	}
	if !anchorDrop {
		t.Error("versionPickerAnchor does not draw MenuDropDownIcon")
	}

	for _, synthetic := range []bool{false, true} {
		for _, kind := range []audioKind{audioTTS, audioRecorded} {
			card := buildAudioCard(st, kind, synthetic, false, false, kind == audioRecorded, audioCardCallbacks{})
			win := test.NewTempWindow(t, container.NewVBox(container.NewGridWrap(card.MinSize(), card)))
			win.Resize(fyne.NewSize(400, 200))
			chip := findLabeledChip(card)
			closeCell := findTappableArea(card)
			if chip == nil || closeCell == nil {
				t.Fatal("missing audio chip or close cell")
			}
			if strings.ContainsAny(chip.label, "▾▸▼⌄⌃") {
				t.Errorf("audio chip label still carries arrow/chevron rune: %q", chip.label)
			}
			chipDrop := false
			for _, obj := range test.WidgetRenderer(chip).Objects() {
				walkTree(obj, func(o fyne.CanvasObject) {
					if im, ok := o.(*canvas.Image); ok && im.Resource != nil && string(im.Resource.Content()) == dropSVG {
						chipDrop = true
					}
				})
			}
			if !chipDrop {
				t.Errorf("audio chip %q does not draw MenuDropDownIcon", chip.label)
			}
			cx, cy, cw, ch := gatesAbsRect(chip)
			xx, xy, xw, xh := gatesAbsRect(closeCell)
			if cx+cw > xx && xx+xw > cx && cy+ch > xy && xy+xh > cy {
				t.Errorf("audio chip %q (%.1f..%.1f) overlaps close button (%.1f..%.1f)", chip.label, cx, cx+cw, xx, xx+xw)
			}
		}
	}

	// 3. Ask and Share composers: trailing 14pt chevron icon on Show/Hide passage,
	// consistent trailing placement in an 874x402 landscape sheet with the soft
	// keyboard open, and framed Cancel button.
	for _, ask := range []bool{true, false} {
		name := "Ask"
		if !ask {
			name = "Share"
		}
		t.Run(name, func(t *testing.T) {
			compState := psalm23NoteState()
			compState.theme = th
			win := noteSheetWindow(t, compState, true, 874, 402)
			held := holdNoteSheetTimers(t)
			softKeyboard(t, false)
			prevArea, prevFoot := canvasArea, keyboardFreeFoot
			keyboardFreeFoot.known = false
			up := false
			canvasArea = func(c fyne.Canvas) (fyne.Position, fyne.Size) {
				h := float32(381)
				if up {
					h = 178
				}
				return fyne.NewPos(62, 0), fyne.NewSize(c.Size().Width-124, h)
			}
			t.Cleanup(func() { canvasArea, keyboardFreeFoot = prevArea, prevFoot })

			if ask {
				promptAskQuestion(compState, "A synthetic selected passage for landscape testing.", verseTwo)
			} else {
				promptShareNote(compState, drawnSelection(t, compState, 1, 2), verseTwo)
			}
			popup := topPopup(t, win)

			// Verify Cancel sits inside a 1pt pal.Border inputFrame.
			cancelBtn := findTreeButton(popup, "Cancel")
			if cancelBtn == nil {
				t.Fatal("missing composer Cancel button")
			}
			compPal := compState.pal()
			cancelFramed := false
			walkTree(popup, func(o fyne.CanvasObject) {
				c, ok := o.(*fyne.Container)
				if !ok || len(c.Objects) != 2 || c.Objects[0] != cancelBtn {
					return
				}
				if rect, ok := c.Objects[1].(*canvas.Rectangle); ok && rect.StrokeWidth == 1 && rect.StrokeColor == compPal.Border {
					cancelFramed = true
				}
			})
			if !cancelFramed {
				t.Error("composer Cancel button is missing inputFrame(..., pal.Border)")
			}

			up = true
			noteSoftKeyboard(224)
			runHeld(held)

			showBtn := findTreeButton(popup, "Show passage")
			if showBtn == nil {
				t.Fatal("missing Show passage button in landscape keyboard view")
			}
			if showBtn.IconPlacement != widget.ButtonIconTrailingText || showBtn.Icon != iconDisclosureExpand {
				t.Fatalf("Show passage icon = %v placement = %v, want iconDisclosureExpand trailing", showBtn.Icon, showBtn.IconPlacement)
			}
			if got := showBtn.Theme().Size(theme.SizeNameInlineIcon); got != 14 {
				t.Errorf("composer disclosure inline icon size = %g, want 14", got)
			}
			sxBefore, _, _, _ := gatesAbsRect(showBtn)
			midCanvas := float32(874) / 2
			if sxBefore < midCanvas {
				t.Errorf("collapsed Show passage starts at x=%.1f on leading half of 874pt landscape sheet", sxBefore)
			}

			test.Tap(showBtn)
			runHeld(held)

			hideBtn := findTreeButton(popup, "Hide passage")
			if hideBtn == nil {
				t.Fatal("tapping Show passage did not switch to Hide passage")
			}
			if hideBtn.IconPlacement != widget.ButtonIconTrailingText || hideBtn.Icon != iconDisclosureCollapse {
				t.Fatalf("Hide passage icon = %v placement = %v, want iconDisclosureCollapse trailing", hideBtn.Icon, hideBtn.IconPlacement)
			}
			sxAfter, _, _, _ := gatesAbsRect(hideBtn)
			if sxAfter < midCanvas {
				t.Errorf("expanded Hide passage jumped to leading half (x=%.1f < %.1f)", sxAfter, midCanvas)
			}

			test.Tap(cancelBtn)
			runHeld(held)
		})
	}
}

type animRecordDriver struct {
	fyne.Driver
	mu     sync.Mutex
	active map[*fyne.Animation]bool
	starts int
}

func (d *animRecordDriver) StartAnimation(a *fyne.Animation) {
	d.mu.Lock()
	d.active[a] = true
	d.starts++
	d.mu.Unlock()
	d.Driver.StartAnimation(a)
}

func (d *animRecordDriver) StopAnimation(a *fyne.Animation) {
	d.mu.Lock()
	delete(d.active, a)
	d.mu.Unlock()
	d.Driver.StopAnimation(a)
}

func (d *animRecordDriver) count() (active, starts int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.active), d.starts
}

type animRecordApp struct {
	fyne.App
	drv *animRecordDriver
}

func (a animRecordApp) Driver() fyne.Driver { return a.drv }

// TestAudioBufferingSpinnerStopsOnEveryExit verifies that the audio card's
// buffering widget.Activity registers an animation while buffering and stops
// it cleanly on card rebuilds, buffering->idle transitions, card close, and
// rebuildWindow's stopInfiniteBars sweep.
func TestAudioBufferingSpinnerStopsOnEveryExit(t *testing.T) {
	baseApp := test.NewTempApp(t)
	rec := &animRecordDriver{
		Driver: baseApp.Driver(),
		active: map[*fyne.Animation]bool{},
	}
	app := animRecordApp{App: baseApp, drv: rec}
	fyne.SetCurrentApp(app)
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)

	savedOpen := audioPanelOpen
	savedTTS := ttsSupported
	ttsSupported = func() bool { return true }
	t.Cleanup(func() {
		stopAudioSpinner()
		audioPanelOpen = savedOpen
		ttsSupported = savedTTS
	})

	state := sampleState()
	state.app = app
	state.theme = th
	win := app.NewWindow("Audio Spinner Lifecycle")
	defer win.Close()
	state.window = win
	win.Resize(fyne.NewSize(800, 600))

	// 1. Direct buffering rebuilds and transition out of buffering inside audioControl.
	gAudio.mu.Lock()
	origLoaded := gAudio.loaded
	origLoadedFP := gAudio.loadedFP
	origKind := gAudio.kind
	origRecID := gAudio.loadedRecID
	origState := gAudio.state
	gAudio.mu.Unlock()
	t.Cleanup(func() {
		gAudio.mu.Lock()
		gAudio.loaded = origLoaded
		gAudio.loadedFP = origLoadedFP
		gAudio.kind = origKind
		gAudio.loadedRecID = origRecID
		gAudio.state = origState
		gAudio.mu.Unlock()
	})

	audioPanelOpen = true
	fp := chapterAudioFingerprint(state)
	setBuffering := func(on bool) {
		gAudio.mu.Lock()
		gAudio.loaded = on
		if on {
			gAudio.loadedFP = fp
			gAudio.state = audioBuffering
		} else {
			gAudio.loadedFP = ""
			gAudio.state = audioIdle
		}
		gAudio.mu.Unlock()
	}
	setBuffering(true)

	ctl := audioControl(state, 34, nil)
	win.SetContent(container.NewCenter(ctl))
	_ = win.Canvas().Capture()

	active, starts := rec.count()
	if starts == 0 || active != 1 {
		t.Fatalf("expected 1 active buffering animation after open, got active=%d starts=%d", active, starts)
	}

	// Trigger a second rebuild while still buffering: previous spinner must stop.
	gAudio.onChange()
	_ = win.Canvas().Capture()
	if active, _ = rec.count(); active != 1 {
		t.Fatalf("after repeated buffering rebuild, active animations = %d, want 1", active)
	}

	// Transition out of buffering: spinner must stop completely.
	setBuffering(false)
	gAudio.onChange()
	_ = win.Canvas().Capture()
	if active, _ = rec.count(); active != 0 {
		t.Fatalf("after leaving buffering state, active animations = %d, want 0", active)
	}

	// Re-enter buffering, then close card via ✕: spinner must stop completely.
	setBuffering(true)
	gAudio.onChange()
	_ = win.Canvas().Capture()
	if active, _ = rec.count(); active != 1 {
		t.Fatalf("expected 1 active animation on second buffering start, got %d", active)
	}
	closeCell := findTappableArea(ctl)
	if closeCell == nil {
		t.Fatal("missing close button on buffering audio card")
	}
	test.Tap(closeCell)
	_ = win.Canvas().Capture()
	if active, _ = rec.count(); active != 0 {
		t.Fatalf("closing buffering audio card left %d active animations, want 0", active)
	}

	// 2. Standalone started widget.Activity in window content is stopped by rebuildWindow.
	setBuffering(false)
	orphanSpin := widget.NewActivity()
	orphanSpin.Start()
	win.SetContent(container.NewCenter(orphanSpin))
	_ = win.Canvas().Capture()
	if active, _ = rec.count(); active != 1 {
		t.Fatalf("control: standalone Activity did not register animation (active=%d)", active)
	}
	rebuildWindow(state)
	if active, _ = rec.count(); active != 0 {
		t.Fatalf("rebuildWindow did not stop widget.Activity in replaced content (active=%d)", active)
	}
}

// TestRenderSettingsMarksVerificationScreenshots writes full-window verification
// PNGs when BIBLETEXT_PANE_SNAPSHOT_DIR is set, and skips otherwise.
func TestRenderSettingsMarksVerificationScreenshots(t *testing.T) {
	outDir := os.Getenv("BIBLETEXT_PANE_SNAPSHOT_DIR")
	if outDir == "" {
		t.Skip("BIBLETEXT_PANE_SNAPSHOT_DIR not set")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	savePNG := func(name string, img image.Image) {
		t.Helper()
		f, err := os.Create(filepath.Join(outDir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}

	prevList := aiSettingsListModels
	aiSettingsListModels = func(context.Context, providerInfo, string) ([]discoveredModel, error) {
		return nil, errors.New("offline")
	}
	t.Cleanup(func() { aiSettingsListModels = prevList })

	// 1. Settings ASSISTANT card at 375x667 light (gemini-4-carbon, High effort,
	// framed API key input, trailing dropdown triangles, bottom modelCaption).
	{
		th := startGatesApp(t, false, true)
		pal := th.palette()
		win := test.NewWindow(canvas.NewRectangle(pal.Background))
		win.Canvas().(test.WindowlessCanvas).SetScale(2)
		win.Resize(fyne.NewSize(375, 667))
		st := sampleState()
		st.window, st.theme = win, th
		st.aiKeys = newKeyStoreWith(newFakePrefs())
		store := st.keys()
		store.setActiveProvider(providerGemini)
		store.setAPIKey(providerGemini, "synthetic-key-000000000000000000000000000")
		store.setOverrideModel(providerGemini, "gemini-4-carbon")
		showAISettings(st)
		sheet := settingsPopup(t, st)
		formScroll := gatesFirstObject(sheet, func(s *container.Scroll) bool {
			c, ok := s.Content.(*fyne.Container)
			if !ok {
				return false
			}
			_, sq := c.Layout.(squeezeWidthLayout)
			return sq
		})
		keyLbl := gatesFirstObject(sheet, func(l *widget.Label) bool { return l.Text == "Gemini key" })
		_, scY, _, _ := gatesAbsRect(formScroll.Content)
		_, ky, _, _ := gatesAbsRect(keyLbl)
		most := max(float32(0), formScroll.Content.Size().Height-formScroll.Size().Height)
		formScroll.Offset = fyne.NewPos(0, max(float32(0), min(ky-scY-10, most)))
		formScroll.Refresh()
		savePNG("verify-settings-375x667-light.png", gatesCaptureScroll(win, formScroll))
		sheet.Hide()
		win.Close()
	}

	// 2. Settings ASSISTANT card + open Model picker sheet at 402x874 dark.
	{
		th := startGatesApp(t, true, true)
		pal := th.palette()
		win := test.NewWindow(canvas.NewRectangle(pal.Background))
		win.Canvas().(test.WindowlessCanvas).SetScale(2)
		win.Resize(fyne.NewSize(402, 874))
		st := sampleState()
		st.window, st.theme = win, th
		st.aiKeys = newKeyStoreWith(newFakePrefs())
		store := st.keys()
		store.setActiveProvider(providerAnthropic)
		store.setAPIKey(providerAnthropic, "synthetic-key-000000000000000000000000000")
		rememberModelEfforts(store.prefs, providerAnthropic, []discoveredModel{
			{id: "claude-opus-5-5-20261001", efforts: []string{"low", "medium", "high", "xhigh"}},
			{id: "claude-sonnet-4-6", efforts: []string{"low", "medium", "high"}},
			{id: "claude-haiku-4-5", efforts: []string{"1024", "4096", "8192"}},
		})
		store.setOverrideModel(providerAnthropic, "claude-opus-5-5-20261001")
		setAIReasoningEffort(store.prefs, providerAnthropic, "claude-opus-5-5-20261001", "xhigh")
		showAISettings(st)
		sheet := settingsPopup(t, st)
		formScroll := gatesFirstObject(sheet, func(s *container.Scroll) bool {
			c, ok := s.Content.(*fyne.Container)
			if !ok {
				return false
			}
			_, sq := c.Layout.(squeezeWidthLayout)
			return sq
		})
		keyLbl := gatesFirstObject(sheet, func(l *widget.Label) bool { return l.Text == "Claude key" })
		_, scY, _, _ := gatesAbsRect(formScroll.Content)
		_, ky, _, _ := gatesAbsRect(keyLbl)
		most := max(float32(0), formScroll.Content.Size().Height-formScroll.Size().Height)
		formScroll.Offset = fyne.NewPos(0, max(float32(0), min(ky-scY-10, most)))
		formScroll.Refresh()
		baseSheet := gatesCaptureScroll(win, formScroll)

		modelRows := gatesFirstObject(sheet, func(c *fyne.Container) bool {
			_, ok := c.Layout.(*modelSettingsLayout)
			return ok
		})
		modelRows.Layout.(*modelSettingsLayout).model.OnTapped()
		picker := win.Canvas().Overlays().Top().(*widget.PopUp)
		_ = win.Canvas().Capture()
		px, py, pw, ph := gatesAbsRect(picker.Content)
		if px <= 0 && py <= 0 {
			px, py = (402-pw)/2, (874-ph)/2
		}
		sheet.Hide()
		overlayOnly := software.NewTransparentCanvas()
		overlayOnly.SetPadded(false)
		overlayOnly.SetScale(2)
		scrim := canvas.NewRectangle(color.NRGBA{0, 0, 0, 110})
		holder := container.NewWithoutLayout(scrim, picker.Content)
		scrim.Resize(fyne.NewSize(402, 874))
		picker.Content.Move(fyne.NewPos(px, py))
		picker.Content.Resize(fyne.NewSize(pw, ph))
		overlayOnly.SetContent(holder)
		overlayOnly.Resize(fyne.NewSize(402, 874))
		topImg := overlayOnly.Capture()
		holder.Objects = nil

		comp := image.NewNRGBA(baseSheet.Bounds())
		draw.Draw(comp, comp.Bounds(), baseSheet, image.Point{}, draw.Src)
		draw.Draw(comp, comp.Bounds(), topImg, image.Point{}, draw.Over)
		savePNG("verify-picker-402x874-dark.png", comp)
		picker.Hide()
		win.Close()
	}

	// 3. Cross-References panel at 402x874 dark showing 14pt stroke chevrons
	// (closed group pointing down, opened group and note pointing up).
	{
		th := startGatesApp(t, true, true)
		pal := th.palette()
		licenseNKJVForTest(t)
		withPublisherCrossRefs(t, "nkjv", true)
		withCrossRefIndex(t, "John.3.16\tRom.5.8\t10\n")
		prevRun, prevLoad := crossRefsRun, crossRefsLoad
		crossRefsRun = func(work func()) { work() }
		crossRefsLoad = func() error { return nil }
		t.Cleanup(func() { crossRefsRun, crossRefsLoad = prevRun, prevLoad })

		win := test.NewWindow(canvas.NewRectangle(pal.Background))
		win.Canvas().(test.WindowlessCanvas).SetScale(2)
		win.Resize(fyne.NewSize(402, 874))
		st, _ := groupListState(t)
		st.theme, st.window = th, win
		st.window.SetContent(container.NewBorder(buildHeader(st), nil, nil, nil, canvas.NewRectangle(pal.Background)))
		showCrossRefs(st, "", selSpan{lo: 16, hi: 16})
		popup := win.Canvas().Overlays().Top().(*widget.PopUp)
		var groups []*crossRefGroupRow
		var scroll *container.Scroll
		walkTree(popup, func(o fyne.CanvasObject) {
			switch v := o.(type) {
			case *crossRefGroupRow:
				groups = append(groups, v)
			case *container.Scroll:
				if scroll == nil {
					scroll = v
				}
			}
		})
		if len(groups) >= 2 {
			test.Tap(groups[1].header)
		}
		savePNG("verify-crossref-402x874-dark.png", gatesCaptureScroll(win, scroll))
		popup.Hide()
		win.Close()
	}
}
