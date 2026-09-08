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

	// The paragraph gap is a line of the type, so it has to follow the type. A
	// literal left behind here would show as paragraphs that no longer sit a
	// line apart.
	lead := strconv.FormatFloat(bibletext.ReadingLinePitchEm(), 'f', 4, 64)
	if !strings.Contains(css, "--pgap:calc("+lead+" * "+want+")") {
		t.Errorf("the paragraph gap is not figured from the corrected size %s at the "+
			"chosen leading %s", want, lead)
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

// Headings are set in the scripture face, so they take the same correction — the
// point is to preserve the ratio between heading and body, not to redesign it.
func TestHeadingsTakeTheSameCorrection(t *testing.T) {
	css := testCSS()
	want := remSize(webHeadingBaseRem)
	if !strings.Contains(css, "font-size:"+want+";") {
		t.Errorf("the section heading is not set at the corrected %s", want)
	}
	ratio := webHeadingBaseRem / webScriptureBaseRem
	if ratio < 0.7 || ratio > 0.85 {
		t.Errorf("the heading/body ratio has moved to %.3f; it was 0.762 and the "+
			"correction is not supposed to change it", ratio)
	}
}
