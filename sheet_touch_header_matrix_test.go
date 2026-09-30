//go:build !race

package bibletext

// The whole matrix: every sheet on every screen, opened with the rule off
// and on, against the header's controls as drawn. Over five hundred sheet
// openings and fifteen renderings at 3x take under ten seconds, and about
// two minutes under the race detector, which has nothing to find in layout
// arithmetic on the test's own goroutine; CI's run without the detector
// runs them. The harness, and the checks that are cheap under the
// detector, are in sheet_touch_header_test.go.

import (
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// inkScale is the rendering scale the header's drawn parts are read at.
const inkScale = 3

// inkControl is one header control as drawn: for each pixel column its
// drawn pixels reach, the first and last rows they reach, in canvas units.
type inkControl struct {
	name string
	cols map[float32][2]float32 // column centre -> top, bottom
}

// band is the rows c's drawn pixels reach in the columns between x0 and x1.
func (c inkControl) band(x0, x1 float32) (top, bottom float32, ok bool) {
	top, bottom = float32(math.Inf(1)), float32(math.Inf(-1))
	for x, tb := range c.cols {
		if x < x0 || x > x1 {
			continue
		}
		ok = true
		top, bottom = min(top, tb[0]), max(bottom, tb[1])
	}
	return top, bottom, ok
}

// headerMarkNames names state.headerMarks, in the order buildHeader
// records them.
var headerMarkNames = []string{"title", "translation line", "Go to chip", "sparkle", "gear"}

// headerInk renders the window at inkScale with no sheet open and reads
// where each header control's pixels are: those in and just around its box
// that differ from the header's ground.
func headerInk(t *testing.T, st *AppState, w fyne.Window) []inkControl {
	t.Helper()
	if len(st.headerMarks) != len(headerMarkNames) {
		t.Fatalf("the header recorded %d controls, want %d", len(st.headerMarks), len(headerMarkNames))
	}
	wc, ok := w.Canvas().(test.WindowlessCanvas)
	if !ok {
		t.Fatal("the test window's canvas cannot be rendered at a scale")
	}
	wc.SetScale(inkScale)
	img := w.Canvas().Capture()
	wc.SetScale(1)
	gr, gg, gb, _ := st.pal().SurfaceAlt.RGBA()
	drawn := func(c color.Color) bool {
		r, g, b, _ := c.RGBA()
		d := math.Abs(float64(r)-float64(gr)) + math.Abs(float64(g)-float64(gg)) + math.Abs(float64(b)-float64(gb))
		return d > 3*0x1000 // about 16 of 255 a channel, summed
	}
	var out []inkControl
	for i, m := range st.headerMarks {
		p := onCanvasAt(m)
		sz := m.Size()
		// A stroke is centred on its edge, so look one unit outside the box.
		x0, y0 := int((p.X-1)*inkScale), int((p.Y-1)*inkScale)
		x1, y1 := int((p.X+sz.Width+1)*inkScale), int((p.Y+sz.Height+1)*inkScale)
		b := img.Bounds()
		c := inkControl{name: headerMarkNames[i], cols: map[float32][2]float32{}}
		for x := max(x0, b.Min.X); x < min(x1, b.Max.X); x++ {
			first, last := -1, -1
			for y := max(y0, b.Min.Y); y < min(y1, b.Max.Y); y++ {
				if drawn(img.At(x, y)) {
					if first < 0 {
						first = y
					}
					last = y
				}
			}
			if first >= 0 {
				c.cols[(float32(x)+0.5)/inkScale] = [2]float32{float32(first) / inkScale, float32(last+1) / inkScale}
			}
		}
		if len(c.cols) == 0 {
			t.Fatalf("control: no pixel of the %s is drawn in its box", c.name)
		}
		out = append(out, c)
	}
	return out
}

// headerInkFor reads the header of a window on s, once per screen.
var headerInkCache = map[string][]inkControl{}

func headerInkFor(t *testing.T, s touchScreen) []inkControl {
	t.Helper()
	if ink, ok := headerInkCache[s.name]; ok {
		return ink
	}
	st, w := touchAppWindow(t, s)
	ink := headerInk(t, st, w)
	headerInkCache[s.name] = ink
	return ink
}

// inkCut names each control the sheet starts partway down: one it spans
// across, whose drawn pixels reach both above and below the sheet's top
// edge. A control is read whole, as the reader reads it — a word's lower-case
// letters under a sheet's edge are still the word the edge cuts — with a
// pixel's slack either way, at the scale it is read.
func inkCut(ink []inkControl, sp sheetSpan) []string {
	const px = 1.0 / inkScale
	var cut []string
	for _, c := range ink {
		if _, _, spans := c.band(sp.x0, sp.x1); !spans {
			continue
		}
		top, bottom, _ := c.band(float32(math.Inf(-1)), float32(math.Inf(1)))
		if sp.top > top+px && sp.top < bottom-px {
			cut = append(cut, fmt.Sprintf("the %s (drawn %.1f..%.1f)", c.name, top, bottom))
		}
	}
	return cut
}

// openTouchSheet opens sh on a fresh window on s and returns where the sheet
// lies and where the header's edges are.
func openTouchSheet(t *testing.T, s touchScreen, sh touchSheet) (sp sheetSpan, headerTop, headerBottom float32) {
	t.Helper()
	st, _ := touchAppWindow(t, s)
	popup := pickerPopup(t, st, func(a *AppState) { sh.open(t, a) })
	defer popup.Hide()
	headerTop, headerBottom = headerEdges(st)
	return sheetSpanOf(t, popup), headerTop, headerBottom
}

// touchSheetOpening is one sheet opened on one screen, before the rule and
// with it.
type touchSheetOpening struct {
	before, after           sheetSpan
	headerTop, headerBottom float32
	cutBefore               bool
}

// openTouchSheets opens every sheet on a window on each screen, with the
// rule off, which is how every sheet opened before it, and on. One window a
// screen: a sheet opened and closed leaves the window as it found it, and
// the sheets that change the reader's state come last (touchSheets). Each
// screen is a subtest of its own, so what a sheet registered for the rest
// of the test (withMoreTranslations) is gone before the next screen's
// sheets open. Read once for every test that asks.
var touchOpenings map[string]map[string]touchSheetOpening

func openTouchSheets(t *testing.T) map[string]map[string]touchSheetOpening {
	t.Helper()
	if touchOpenings != nil {
		return touchOpenings
	}
	sheets := touchSheets(t)
	out := map[string]map[string]touchSheetOpening{}
	for _, s := range touchScreens {
		t.Run("open on "+s.name, func(t *testing.T) {
			ink := headerInkFor(t, s)
			st, _ := touchAppWindow(t, s)
			top, bottom := headerEdges(st)
			out[s.name] = map[string]touchSheetOpening{}
			for _, sh := range sheets {
				before := openSheetAs(t, st, sh, false)
				after := openSheetAs(t, st, sh, true)
				out[s.name][sh.name] = touchSheetOpening{before, after, top, bottom, len(inkCut(ink, before)) > 0}
			}
		})
	}
	if !t.Failed() {
		touchOpenings = out
	}
	return out
}

// NO SHEET STARTS PARTWAY DOWN A HEADER CONTROL, AND ONE THAT DID NOT DOES
// NOT MOVE. Every sheet, on every phone and tablet screen, opened with the
// rule off and on: with it on, none starts partway down a control but one
// that has no room over or below the header, which stays where it was;
// wherever a sheet did not with the rule off, the two are the same box; and
// each that moved lies within the safe area, since a sheet centred over the
// header ends the header's depth above the canvas's foot, which can be under
// a deep bottom inset. Mutations guarded: the rule doing nothing, as before
// it (Settings, the cross-references, the long verse of the day and the
// audio source menu cut the title's letters at 360x803, the chapter picker
// the Go to chip at 320x568, the centred sheets the chip on a phone on its
// side); the header found only through containers (a phone's window holds
// the page in a root widget, so the rule never saw a header there); a
// covering sheet started 8pt below the header's top edge (the title's
// letters show above the translation picker at 320x568); the drawn parts
// taken as the controls' whole boxes (sheets whose edge misses every drawn
// pixel, such as the long verse of the day on a 667x375 phone on its side,
// are moved); covering whatever the bottom inset (Settings ends under the
// navigation bar with three-button navigation).
func TestTouchSheetsKeepClearOfTheHeaderControls(t *testing.T) {
	opened := openTouchSheets(t)
	moved := 0
	for _, s := range touchScreens {
		ink := headerInkFor(t, s)
		for _, sh := range touchSheets(t) {
			o := opened[s.name][sh.name]
			// No place holds it: covering the header would end it under the
			// bottom inset, and it cannot be as short as the room below. It
			// is left where it was.
			noRoom := o.headerTop < s.bottom && s.h-2*(o.headerBottom+sheetHeaderGap) < o.after.least
			if cut := inkCut(ink, o.after); len(cut) > 0 {
				if noRoom && sameSpan(o.before, o.after) {
					t.Logf("%s, %s: no room over or below the header; left at %.1f", s.name, sh.name, o.after.top)
					continue
				}
				t.Errorf("%s, %s: the sheet's top edge (%.1f) is partway down %s", s.name, sh.name, o.after.top, strings.Join(cut, " and "))
			}
			if o.cutBefore {
				moved++
				if o.after.top < s.top-0.5 || o.after.bottom > s.h-s.bottom+0.5 {
					t.Errorf("%s, %s: moved, the sheet spans %.1f..%.1f, outside the safe area's %.1f..%.1f",
						s.name, sh.name, o.after.top, o.after.bottom, s.top, s.h-s.bottom)
				}
			} else if !sameSpan(o.before, o.after) {
				t.Errorf("%s, %s: clear of the header's controls or over them, the sheet spanned %v; it now spans %v",
					s.name, sh.name, o.before, o.after)
			}
		}
	}
	if moved == 0 {
		t.Error("control: no sheet started partway down a control with the rule off, so nothing here could move")
	}
}

// WHICH WAY A MOVED SHEET GOES. A sheet as tall as the screen lets it be
// covers the header, as the tall sheets do on an iPhone; one sized to its
// content opens below it and scrolls, as the translation picker does on an
// iPhone; with too little room below the header (a phone on its side) it
// covers the header too; and where covering would end it under the bottom
// inset (three-button navigation, an iPhone on its side) it opens below.
// The header's edges are the lines: at or above its top, or sheetHeaderGap
// or more below its bottom. Each case starts partway down a control with the
// rule off, and ends on the screen.
func TestTouchSheetsMovedGoOverOrBelowTheHeader(t *testing.T) {
	opened := openTouchSheets(t)
	heights := map[string]float32{}
	for _, s := range touchScreens {
		heights[s.name] = s.h
	}
	for _, tc := range []struct {
		screen, sheet string
		over          bool
	}{
		{"360x803", "Settings", true},
		{"360x803", "verse of the day, long", true},
		{"360x803", "translation picker, more translations", true},
		{"360x803", "cross-references, listed", true},
		{"360x803", "audio source menu", false},
		{"320x568", "chapter picker", false},
		{"375x667", "translation picker", false},
		{"375x667", "AI answer, waiting", false},
		{"440x956", "cross-references, listed", true},
		{"393x852", "audio source menu", false},
		{"568x320", "translation picker", true},
		{"568x320", "link notice", true},
		{"956x440", "audio source menu", false},
		// Covering would end these under the bottom inset.
		{"360x803, 3-button navigation", "Settings", false},
		{"360x803, 3-button navigation", "cross-references, listed", false},
		{"852x393", "Settings", false},
		{"852x393", "translation picker", false},
	} {
		o, ok := opened[tc.screen][tc.sheet]
		switch {
		case !ok:
			t.Errorf("no sheet %q opened on %s", tc.sheet, tc.screen)
		case !o.cutBefore:
			t.Errorf("%s, %s: control: with the rule off the sheet (top %.1f) starts partway down no control", tc.screen, tc.sheet, o.before.top)
		case tc.over && o.after.top > o.headerTop+0.5:
			t.Errorf("%s, %s: the sheet starts at %.1f; it should cover the header, which starts at %.1f", tc.screen, tc.sheet, o.after.top, o.headerTop)
		case !tc.over && o.after.top < o.headerBottom+sheetHeaderGap-0.5:
			t.Errorf("%s, %s: the sheet starts at %.1f; it should open below the header, which ends at %.1f", tc.screen, tc.sheet, o.after.top, o.headerBottom)
		case o.after.top < -0.5 || o.after.bottom > heights[tc.screen]+0.5:
			t.Errorf("%s, %s: the sheet spans %.1f..%.1f, off the canvas", tc.screen, tc.sheet, o.after.top, o.after.bottom)
		}
	}
}

// THE RULE'S IDEA OF EACH CONTROL HOLDS ITS PIXELS. The rule cannot render
// the header, so it takes each control's drawn part from its box
// (drawnPart). Every drawn pixel lies within what it takes, so the rule
// never leaves a sheet alone whose edge crosses a control's pixels.
func TestHeaderDrawnPartsHoldTheirPixels(t *testing.T) {
	for _, s := range touchScreens {
		ink := headerInkFor(t, s)
		st, _ := touchAppWindow(t, s)
		band, ok := touchHeaderBand(st)
		if !ok || len(band.marks) != len(ink) {
			t.Fatalf("%s: the rule reads %d controls in the header (ok %v); %d are drawn", s.name, len(band.marks), ok, len(ink))
		}
		for i, c := range ink {
			m := band.marks[i]
			top, bottom, _ := c.band(float32(math.Inf(-1)), float32(math.Inf(1)))
			if top < m.y0-0.5 || bottom > m.y1+0.5 {
				t.Errorf("%s: the %s is drawn over %.1f..%.1f, outside the %.1f..%.1f the rule takes for it",
					s.name, c.name, top, bottom, m.y0, m.y1)
			}
			var left, right float32 = float32(math.Inf(1)), float32(math.Inf(-1))
			for x := range c.cols {
				left, right = min(left, x), max(right, x)
			}
			if left < m.x0-1 || right > m.x1+1 {
				t.Errorf("%s: the %s is drawn across %.1f..%.1f, outside the %.1f..%.1f the rule takes for it",
					s.name, c.name, left, right, m.x0, m.x1)
			}
		}
	}
}

// TestTouchSheetSurvey writes where every sheet's top edge lands against the
// header's controls on every screen, as JSON and as a table, into the
// directory BIBLETEXT_SHEET_SURVEY_DIR names. A tool for looking, not a
// check.
func TestTouchSheetSurvey(t *testing.T) {
	dir := os.Getenv("BIBLETEXT_SHEET_SURVEY_DIR")
	if dir == "" {
		t.Skip("set BIBLETEXT_SHEET_SURVEY_DIR to write the survey")
	}
	type row struct {
		Screen, Sheet           string
		Left, Right             float32
		Top, Bottom             float32
		HeaderTop, HeaderBottom float32
		Where                   string
		Cut                     []string `json:",omitempty"`
	}
	var rows []row
	sheets := touchSheets(t)
	for _, s := range touchScreens {
		ink := headerInkFor(t, s)
		for _, sh := range sheets {
			t.Run(s.name+"/"+sh.name, func(t *testing.T) {
				sp, top, bottom := openTouchSheet(t, s, sh)
				cut := inkCut(ink, sp)
				where := "partway down a control"
				switch {
				case len(cut) > 0:
				case sp.top >= bottom-0.5:
					where = "below the header"
				case sp.top <= top+0.5:
					where = "over the header"
				default:
					where = "in the header's margin, clear of its controls or over them"
				}
				rows = append(rows, row{s.name, sh.name, sp.x0, sp.x1, sp.top, sp.bottom, top, bottom, where, cut})
			})
		}
	}
	b, err := json.MarshalIndent(rows, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "survey.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	var tbl strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&tbl, "%-10s %-38s top %6.1f  header %5.1f..%5.1f  %s %s\n",
			r.Screen, r.Sheet, r.Top, r.HeaderTop, r.HeaderBottom, r.Where, strings.Join(r.Cut, ", "))
	}
	if err := os.WriteFile(filepath.Join(dir, "survey.txt"), []byte(tbl.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
