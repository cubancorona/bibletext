package main

// The generated page sets Scripture in the same face as the app, so it has to
// open it up by the same amount. If these drift, a chapter read on the web is a
// different size from the same chapter read in the app — which is precisely the
// defect the optical scale exists to remove, reintroduced one surface over.

import (
	"strconv"
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

func testCSS() string {
	return readerCSS(webFonts{
		uiRegular:        "ui-r.woff2",
		uiBold:           "ui-b.woff2",
		scriptureRegular: "sc-r.woff2",
		scriptureBold:    "sc-b.woff2",
	})
}

func TestScriptureIsSetAtTheOpticallyCorrectedSize(t *testing.T) {
	css := testCSS()
	scale := bibletext.ReadingOpticalScale()

	if scale < 1.10 || scale > 1.20 {
		t.Fatalf("the optical scale is %.4f — far enough from the measured 1.1518 that "+
			"this test's expectations are no longer the right ones", scale)
	}

	want := remSize(webScriptureBaseRem) // 1.5115rem
	if !strings.Contains(css, "font-size:"+want+";") {
		t.Errorf("the stylesheet does not set the scripture body at %s", want)
	}

	// The control. The old, uncorrected size must be gone — otherwise the check
	// above would pass on a page that still carried it somewhere and the reader
	// would get whichever rule won the cascade.
	if strings.Contains(css, "font-size:1.3125rem") {
		t.Error("the uncorrected 1.3125rem scripture size is still in the stylesheet")
	}

	// The paragraph gap is a body's worth of air (reading_spacing.go), so it
	// has to follow the type. A literal left behind here would show as
	// paragraphs that no longer sit the app's gap apart.
	gap := strconv.FormatFloat(bibletext.ReadingParaGapEm(), 'f', -1, 64)
	lead := strconv.FormatFloat(bibletext.ReadingLinePitchEm(), 'f', 4, 64)
	if !strings.Contains(css, "--pgap:calc("+gap+" * "+want+")") {
		t.Errorf("the paragraph gap is not figured from the corrected size %s at the "+
			"app's gap %s", want, gap)
	}

	// The leading is a shared number now, not a value measured off a screenshot
	// once and copied. If the site drifts from the app the two disagree on the
	// page's rhythm, which is the same class of defect as disagreeing on its size.
	if !strings.Contains(css, "line-height:"+lead+";") {
		t.Errorf("the stylesheet does not set the scripture leading to %s", lead)
	}
	if strings.Contains(css, "line-height:1.3175") {
		t.Error("the old measured-once leading 1.3175 is still in the stylesheet")
	}
}

// The measure is the one quantity the scale must NOT reach: holding the column
// still while the glyphs grow is what puts the line back to its old character
// count. If this ever becomes em-based, or picks up the scale, the web page's
// lines get 15% longer than the app's.
func TestTheWebMeasureDoesNotTakeTheOpticalScale(t *testing.T) {
	css := testCSS()
	for _, want := range []string{".wrap{max-width:40rem;", ".foot{max-width:40rem;"} {
		if !strings.Contains(css, want) {
			t.Errorf("the reading column is no longer %q — a measure must stay in root rem "+
				"and must not move when the face does", want)
		}
	}
	if strings.Contains(css, "max-width:40em") {
		t.Error("the reading column has become em-based, so it now grows with the type")
	}
}

// Headings are set exactly as the app panes set them (reading.go, p.sec): at the
// body's size, bold, with 1.1em above and .35em below. The web once gave them a
// size a step under the body and margins figured from the paragraph gap — zero
// on the reporter page — so a desktop reader saw a three-quarter-size heading
// with 6px above and nothing below. Mutations: a size of the heading's own
// (rem or px); the size left out, which lets the browser's 1.5em h2 default
// in; margins figured from --pgap again.
func TestHeadingsAreSetAsThePanesSetThem(t *testing.T) {
	css := testCSS()
	i := strings.Index(css, ".text .sec{")
	if i < 0 {
		t.Fatal("the section heading rule is gone")
	}
	rule := css[i : strings.Index(css[i:], "}")+i]
	// font-size:1em, stated: the heading is an <h2>, whose browser default is
	// 1.5em, so leaving the size out is not the same as inheriting the body's.
	if !strings.Contains(rule, "font-size:1em;") {
		t.Errorf("the heading rule does not pin the body size (font-size:1em); an <h2> defaults to 1.5em: %s", rule)
	}
	if strings.Contains(rule, "rem") || strings.Contains(rule, "px") {
		t.Errorf("the heading rule sets a size of its own; the panes set headings at the body size: %s", rule)
	}
	want := "margin:" + bibletext.EmCSS(bibletext.ReadingHeadLeadEm()) + " 0 " + bibletext.EmCSS(bibletext.ReadingHeadTailEm())
	if !strings.Contains(rule, want) {
		t.Errorf("the heading margins are not the app's (%s): %s", want, rule)
	}
	if strings.Contains(rule, "--pgap") {
		t.Errorf("the heading margins are figured from the paragraph gap again, which is zero on the reporter page: %s", rule)
	}
	if !strings.Contains(rule, "font-weight:700") {
		t.Errorf("the heading is no longer bold: %s", rule)
	}
	// The control: the body rule still carries the corrected size, so a missing
	// font-size on the heading means inheritance, not a stylesheet that lost
	// its sizes altogether.
	if !strings.Contains(css, "font-size:"+remSize(webScriptureBaseRem)) {
		t.Fatal("the body size is gone from the stylesheet, so the check above proves nothing")
	}
}
