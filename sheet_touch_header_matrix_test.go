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
	"sort"
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

// inkPartly names each control the sheet covers in part: some of its drawn
// pixels lie under the sheet and some outside it, beside it or above it,
// each by more than a pixel at the scale it is read. A top edge partway down
// a control is one way; a side edge through one is the other.
func inkPartly(ink []inkControl, sp sheetSpan) map[string]bool {
	const px = 1.0 / inkScale
	out := map[string]bool{}
	for _, c := range ink {
		under, outside := false, false
		for x, tb := range c.cols {
			switch {
			case x > sp.x0+px && x < sp.x1-px:
				if tb[1] > sp.top+px && tb[0] < sp.bottom-px {
					under = true
				}
				if tb[0] < sp.top-px || tb[1] > sp.bottom+px {
					outside = true
				}
			case x < sp.x0-px || x > sp.x1+px:
				outside = true
			}
		}
		if under && outside {
			out[c.name] = true
		}
	}
	return out
}

// newlyPartly names each control after covers in part that before left
// wholly alone or covered whole.
func newlyPartly(ink []inkControl, before, after sheetSpan) []string {
	was := inkPartly(ink, before)
	var names []string
	for name := range inkPartly(ink, after) {
		if !was[name] {
			names = append(names, "the "+name)
		}
	}
	sort.Strings(names)
	return names
}

// keptBody is what a sheet whose scrolling part was body tall must keep of
// it to move: two thirds, and 120 units, or all of it if it had less.
func keptBody(body float32) float32 { return max(body*2/3, min(body, 120)) }

// noBetterPlace says why a sheet that starts partway down a control with the
// rule off has nowhere better to go, from what the test measured rather than
// what the rule computes, or "" when it has somewhere. Covering the header
// takes it from the header's top edge to as far above the canvas's foot, and
// is closed where that foot is under the bottom inset, where the taller
// sheet's sides pass through a control it left alone, or where the sheet,
// opened covering the header (o.cover), shows less of its scrolling part
// than keptBody. Below the header it ends as far above the foot as the
// header's foot plus sheetHeaderGap is below the top, and that is closed
// where its content does not fit, or where its scrolling part keeps less
// than keptBody.
func noBetterPlace(s touchScreen, o touchSheetOpening, ink []inkControl) string {
	var why []string
	cover := sheetSpan{x0: o.before.x0, x1: o.before.x1, top: o.headerTop, bottom: s.h - o.headerTop}
	switch newly := newlyPartly(ink, o.before, cover); {
	case cover.bottom > s.h-s.bottom+0.5:
		why = append(why, fmt.Sprintf("covering the header would end it at %.1f, under the bottom inset (%.1f)", cover.bottom, s.h-s.bottom))
	case len(newly) > 0:
		why = append(why, "covering the header would pass its side through "+strings.Join(newly, " and "))
	case o.before.body > 0 && o.cover.body < keptBody(o.before.body)-0.5:
		why = append(why, fmt.Sprintf("covering the header would leave its scroll %.1f of %.1f", o.cover.body, o.before.body))
	default:
		return ""
	}
	below := s.h - 2*(o.headerBottom+sheetHeaderGap)
	height := o.before.bottom - o.before.top
	switch kept := o.before.body - (height - below); {
	case o.before.body > 0 && kept < keptBody(o.before.body)-0.5:
		why = append(why, fmt.Sprintf("below the header its scroll would keep %.1f of %.1f", kept, o.before.body))
	case o.before.body == 0 && below < o.before.least-0.5:
		why = append(why, fmt.Sprintf("below the header it would be %.1f tall, shorter than its content (%.1f)", below, o.before.least))
	default:
		return ""
	}
	return strings.Join(why, "; ")
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
// with it, and, where it starts partway down a control before the rule,
// made to cover the header (openSheetCovering).
type touchSheetOpening struct {
	before, after, cover    sheetSpan
	headerTop, headerBottom float32
	cutBefore               bool
}

// openSheetCovering opens sh on st's window with the rule on and made to
// cover the header wherever it would start partway down a control
// (touchSheetsAlwaysCover), reads it as openSheetAs does, and closes it:
// what covering the header would leave the sheet, as laid out.
func openSheetCovering(t *testing.T, st *AppState, sh touchSheet) sheetSpan {
	t.Helper()
	prev := touchSheetsAlwaysCover
	touchSheetsAlwaysCover = true
	defer func() { touchSheetsAlwaysCover = prev }()
	return openSheetAs(t, st, sh, true)
}

// openTouchOpening opens sh on st's window, on s, with the rule off, on and,
// where it starts partway down a control with the rule off, made to cover
// the header, which must then start at the header's top edge.
func openTouchOpening(t *testing.T, s touchScreen, st *AppState, ink []inkControl, sh touchSheet) touchSheetOpening {
	t.Helper()
	top, bottom := headerEdges(st)
	o := touchSheetOpening{before: openSheetAs(t, st, sh, false), after: openSheetAs(t, st, sh, true), headerTop: top, headerBottom: bottom}
	o.cutBefore = len(inkCut(ink, o.before)) > 0
	if o.cutBefore {
		o.cover = openSheetCovering(t, st, sh)
		if math.Abs(float64(o.cover.top-top)) > 0.5 {
			t.Errorf("%s, %s: control: made to cover the header, the sheet starts at %.1f, not at the header's top edge (%.1f)",
				s.name, sh.name, o.cover.top, top)
		}
	}
	return o
}

// screenNamed is the screen of touchScreens called name.
func screenNamed(t *testing.T, name string) touchScreen {
	t.Helper()
	for _, s := range touchScreens {
		if s.name == name {
			return s
		}
	}
	t.Fatalf("no screen %q", name)
	return touchScreen{}
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
			out[s.name] = map[string]touchSheetOpening{}
			for _, sh := range sheets {
				out[s.name][sh.name] = openTouchOpening(t, s, st, ink, sh)
			}
		})
	}
	if !t.Failed() {
		touchOpenings = out
	}
	return out
}

// NO SHEET STARTS PARTWAY DOWN A HEADER CONTROL WHERE IT HAS A BETTER PLACE,
// AND ONE THAT DID NOT DOES NOT MOVE. Every sheet, on every phone and tablet
// screen, opened with the rule off and on: with it on, none starts partway
// down a control but one with no better place (noBetterPlace), which stays
// exactly where it was; wherever a sheet did not with the rule off, the two
// are the same box; and each that moved lies within the safe area, since a
// sheet centred over the header ends the header's depth above the canvas's
// foot, which can be under a deep bottom inset. Mutations guarded: the rule
// doing nothing, as before it (Settings, the cross-references, the long
// verse of the day and the audio source menu cut the title's letters at
// 360x803, the chapter picker the Go to chip at 320x568, the centred sheets
// the chip on a phone on its side); the header found only through
// containers (a phone's window holds the page in a root widget, so the rule
// never saw a header there); a covering sheet started 8pt below the header's
// top edge (the title's letters show above the translation picker at
// 320x568); the drawn parts taken as the controls' whole boxes (sheets whose
// edge misses every drawn pixel, such as the long verse of the day on a
// 667x375 phone on its side, are moved); covering whatever the bottom inset
// (Settings ends under the navigation bar with three-button navigation).
func TestTouchSheetsKeepClearOfTheHeaderControls(t *testing.T) {
	opened := openTouchSheets(t)
	moved, left := 0, 0
	for _, s := range touchScreens {
		ink := headerInkFor(t, s)
		for _, sh := range touchSheets(t) {
			o := opened[s.name][sh.name]
			if cut := inkCut(ink, o.after); len(cut) > 0 {
				why := noBetterPlace(s, o, ink)
				switch {
				case !sameSpan(o.before, o.after):
					t.Errorf("%s, %s: moved, the sheet's top edge (%.1f) is still partway down %s", s.name, sh.name, o.after.top, strings.Join(cut, " and "))
				case why == "":
					t.Errorf("%s, %s: the sheet's top edge (%.1f) is partway down %s, and it had a better place", s.name, sh.name, o.after.top, strings.Join(cut, " and "))
				default:
					left++
					t.Logf("%s, %s: left at %v, %.1f tall, across %s: %s",
						s.name, sh.name, o.after, o.after.bottom-o.after.top, strings.Join(cut, " and "), why)
				}
				continue
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
	t.Logf("%d sheets moved, %d left where they were", moved, left)
}

// A MOVE KEEPS WHAT THE SHEET SHOWS. Below the header a centred sheet is
// shorter by twice the distance its top edge moves down, and its scroll
// gives up that height. Every sheet that moved keeps at least two thirds of
// what its scroll showed, and at least 120 units of it, or all of it if it
// showed less. Mutation guarded: opening below the header whatever it costs
// (on an iPhone 16 Pro Max on its side the translation picker kept 36 of
// its 120 units, a clipped part of one row, and Settings on a 852x393 iPhone
// its footnotes switch and little else; with landscape reading turned off an
// AI answer kept 32 units of text).
func TestTouchSheetsMovedKeepWhatTheyShow(t *testing.T) {
	opened := openTouchSheets(t)
	shrank := 0
	for _, s := range touchScreens {
		for _, sh := range touchSheets(t) {
			o := opened[s.name][sh.name]
			if sameSpan(o.before, o.after) || o.before.body == 0 {
				continue
			}
			if o.after.body < o.before.body-0.5 {
				shrank++
			}
			if need := keptBody(o.before.body); o.after.body < need-0.5 {
				t.Errorf("%s, %s: moved, its scroll shows %.1f of the %.1f it showed; it should keep %.1f",
					s.name, sh.name, o.after.body, o.before.body, need)
			}
		}
	}
	if shrank == 0 {
		t.Error("control: no sheet that moved gave up any of its scroll, so nothing here holds the floor")
	}
}

// A MOVE CUTS NOTHING THE SHEET LEFT ALONE. A sheet is narrower than the
// header, so one grown to cover it can pass its sides through a control it
// had cleared: a card on a 568x320 phone, clear of the title with its top
// edge across the Go to chip, left "BibleT" beside it when it covered the
// header. Every sheet that moved covers in part, per the pixels drawn, only
// controls it covered in part before. (A tall sheet's left edge passing
// through the title's first letter on a phone held upright is how the
// iPhone's tall sheets have always sat; those sheets cut the title before
// they moved as well.) Mutation guarded: covering whatever the sheet's sides
// then pass through.
func TestTouchSheetsMovedCutNothingTheyLeftAlone(t *testing.T) {
	opened := openTouchSheets(t)
	for _, s := range touchScreens {
		ink := headerInkFor(t, s)
		for _, sh := range touchSheets(t) {
			o := opened[s.name][sh.name]
			if sameSpan(o.before, o.after) {
				continue
			}
			if newly := newlyPartly(ink, o.before, o.after); len(newly) > 0 {
				t.Errorf("%s, %s: moved to %v, the sheet covers part of %s, which it left alone at %v",
					s.name, sh.name, o.after, strings.Join(newly, " and "), o.before)
			}
		}
	}
}

// WHERE A SHEET GOES. A sheet as tall as the screen lets it be covers the
// header, as the tall sheets do on an iPhone; one sized to its content opens
// below it and scrolls, as the translation picker does on an iPhone; and
// where covering would end it under the bottom inset (three-button
// navigation) or pass its side through a control it had cleared (the title,
// on a tall iPhone and on a phone on its side) it opens below the header if
// it keeps what it shows there. With no such place it stays where it was:
// on an iPhone on its side, where the header is a fifth of the screen, and
// on the smallest iPhones on their sides, whose sheets are narrower than
// the header. The header's edges are the lines: at or above its top, or
// sheetHeaderGap or more below its bottom. Each case starts partway down a
// control with the rule off, and ends on the screen.
//
// Whether a sheet is sized to its content or stands at its cap can be data.
// The translation picker on a 375x667 iPhone is sized to its rows and its
// sentences, and opens below the header; with a translation under
// evaluation compiled in (the nrsv and lsb builds), the evaluation sentence
// makes it stand at its cap, from 40 units down, and it covers the header
// like any sheet at its cap. That case follows what was measured: over
// where the sheet's scroll holds more than it shows with the rule off,
// below where it holds no more.
func TestTouchSheetsMovedGoOverOrBelowTheHeader(t *testing.T) {
	opened := openTouchSheets(t)
	heights := map[string]float32{}
	for _, s := range touchScreens {
		heights[s.name] = s.h
	}
	const over, below, stays, byCap = "over", "below", "stays", "over at its cap, below at its content's height"
	for _, tc := range []struct{ screen, sheet, where string }{
		{"360x803", "Settings", over},
		{"360x803", "verse of the day, long", over},
		{"360x803", "translation picker, more translations", over},
		{"360x803", "cross-references, listed", over},
		{"360x803", "audio source menu", below},
		{"320x568", "chapter picker", below},
		{"375x667", "translation picker", byCap},
		{"375x667", "AI answer, waiting", below},
		{"393x852", "audio source menu", below},
		{"568x320", "Settings", over},
		{"956x440", "audio source menu", below},
		// Covering would end these under the bottom inset, and below the
		// header they keep at least two thirds of what they show.
		{"360x803, 3-button navigation", "Settings", below},
		{"360x803, 3-button navigation", "cross-references, listed", below},
		{"956x440", "chapter picker", below},
		// Covering would pass the panel's side through the title.
		{"440x956", "cross-references, listed", below},
		// Covering would end these under the bottom inset, and below the
		// header they would keep 82 of 181, 36 of 120 and 64 of 148 units.
		{"852x393", "Settings", stays},
		{"956x440", "translation picker", stays},
		{"956x440", "AI answer, waiting", stays},
		// Covering would pass the card's side through the title, or the
		// translation line, and it cannot be as short as the room below.
		{"568x320", "link notice", stays},
		{"667x375", "note link offer", stays},
	} {
		o, ok := opened[tc.screen][tc.sheet]
		where := tc.where
		if ok && where == byCap {
			where = below
			if o.before.overflows {
				where = over
			}
			t.Logf("%s, %s: with the rule off its scroll shows %.1f, and its content overflows it: %v; it should go %s",
				tc.screen, tc.sheet, o.before.body, o.before.overflows, where)
		}
		switch {
		case !ok:
			t.Errorf("no sheet %q opened on %s", tc.sheet, tc.screen)
		case !o.cutBefore:
			t.Errorf("%s, %s: control: with the rule off the sheet (top %.1f) starts partway down no control", tc.screen, tc.sheet, o.before.top)
		case where == stays:
			if !sameSpan(o.before, o.after) {
				t.Errorf("%s, %s: the sheet spanned %v; it should stay there, and spans %v", tc.screen, tc.sheet, o.before, o.after)
			}
		case where == over && o.after.top > o.headerTop+0.5:
			t.Errorf("%s, %s: the sheet starts at %.1f; it should cover the header, which starts at %.1f", tc.screen, tc.sheet, o.after.top, o.headerTop)
		case where == below && o.after.top < o.headerBottom+sheetHeaderGap-0.5:
			t.Errorf("%s, %s: the sheet starts at %.1f; it should open below the header, which ends at %.1f", tc.screen, tc.sheet, o.after.top, o.headerBottom)
		case o.after.top < -0.5 || o.after.bottom > heights[tc.screen]+0.5:
			t.Errorf("%s, %s: the sheet spans %.1f..%.1f, off the canvas", tc.screen, tc.sheet, o.after.top, o.after.bottom)
		}
	}
}

// THE TRANSLATION PICKER ON A 1080-PIXEL ANDROID PHONE AT 420 DPI. On its
// full 1080x2410 screen, with gesture or three-button navigation, and at 2:1
// (1080x2160), with the translations this build compiles in and with more
// than any build has and every notice at once, the picker's top edge is at
// or below the header's bottom, or at or above its top: never between,
// unless it starts partway down a control with nowhere better to go
// (noBetterPlace), when it is where it was with the rule off. With more
// translations it started 12pt down the header, partway down the title.
// With three-button navigation and a translation under evaluation compiled
// in, the picker stands at its cap from 40 units down, across the Go to
// chip, and stays: covering the header would end it under the navigation
// bar, and below the header its rows would keep 222.7 of their 348.9 units.
func TestTranslationPickerOnA420DpiAndroidPhoneIsClearOfTheHeader(t *testing.T) {
	opened := openTouchSheets(t)
	checked := 0
	for _, s := range touchScreens {
		if s.w != 360 {
			continue
		}
		ink := headerInkFor(t, s)
		for _, sh := range touchSheets(t) {
			if !strings.HasPrefix(sh.name, "translation picker") {
				continue
			}
			checked++
			o := opened[s.name][sh.name]
			if o.after.top <= o.headerTop+0.5 || o.after.top >= o.headerBottom-0.5 {
				continue
			}
			why := ""
			if o.cutBefore && sameSpan(o.before, o.after) {
				why = noBetterPlace(s, o, ink)
			}
			if why == "" {
				t.Errorf("%s, %s: the picker starts at %.1f, inside the header (%.1f..%.1f)", s.name, sh.name, o.after.top, o.headerTop, o.headerBottom)
				continue
			}
			t.Logf("%s, %s: left at %.1f, inside the header (%.1f..%.1f): %s", s.name, sh.name, o.after.top, o.headerTop, o.headerBottom, why)
		}
	}
	if checked == 0 {
		t.Error("control: no translation picker was opened on a 360-unit-wide screen")
	}
}

// A COVER THAT TAKES THE PICKER'S ROWS IS NOT A BETTER PLACE. The translation
// picker pins its sentences under the rows wherever the sheet has room for
// them and moves them into the scroll after the rows where it has not. On a
// 667x375 phone on its side with a translation under evaluation, the picker
// stands at its cap from 40 units down, across the Go to chip, its sentences
// in the scroll; grown to cover the header it had room to pin them, and the
// scroll that shows its rows went from 123 units to 54. It stays where it
// was, as it would if covering were closed to it: below the header its rows
// would keep less than two thirds. Every build: where none is compiled in,
// the test registers a translation under evaluation. Mutations guarded:
// covering without asking the picker how it lays out at the taller height,
// in the rule (touchSheetHeight) or at the picker (no relayout passed).
func TestTranslationPickerCoversOnlyWhereItsRowsKeepTheirRoom(t *testing.T) {
	s := screenNamed(t, "667x375")
	var picker touchSheet
	for _, sh := range touchSheets(t) {
		if sh.name == "translation picker" {
			picker = sh
		}
	}
	if picker.open == nil {
		t.Fatal("no translation picker among the sheets")
	}
	if len(lockedVersionNames(false)) == 0 {
		withRegisteredVersion(t, BibleVersion{
			ID: "sample", Name: "Sample Translation", Abbrev: "SAMPLE",
			Publisher: "Sample Publisher — license required",
			source:    newLicensedSource("sample"),
		})
	}
	ink := headerInkFor(t, s)
	st, _ := touchAppWindow(t, s)
	o := openTouchOpening(t, s, st, ink, picker)
	switch why := noBetterPlace(s, o, ink); {
	case !o.cutBefore:
		t.Fatalf("control: with the rule off the picker (%v) starts partway down no control", o.before)
	case o.cover.body >= keptBody(o.before.body)-0.5:
		t.Fatalf("control: covering the header, the picker's scroll shows %.1f of %.1f, enough to keep; nothing here holds the floor",
			o.cover.body, o.before.body)
	case !sameSpan(o.before, o.after):
		t.Errorf("the picker spanned %v; it moved to %v, where its scroll shows %.1f of the %.1f it showed",
			o.before, o.after, o.after.body, o.before.body)
	case why == "":
		t.Errorf("the picker stays at %v, and measured, it had a better place", o.after)
	default:
		t.Logf("the picker stays at %v: %s", o.after, why)
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
