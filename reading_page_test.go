package bibletext

import (
	"math"
	"testing"
)

// The reading page's one rule (reading_page.go), stated as numbers a reader could
// check against a ruler. R is the reading size before the optical scale: 21 at
// Normal, 24.15 at Large, 27.3 at Extra large.
func TestReadingPageFor(t *testing.T) {
	for _, tc := range []struct {
		name          string
		width, ref    float64
		book          bool
		measure, side float64
	}{
		{"unsized", 0, 21, true, 577.5, 0},
		{"the switch, exactly", 607.5, 21, true, 577.5, 15},
		{"a hair under it", 607.4, 21, false, 577.4, 15},
		{"a wide window", 1200, 21, true, 577.5, 311},
		{"iPhone portrait", 375, 21, false, 345, 15},
		{"iPhone landscape", 734, 21, true, 577.5, 78},
		{"iPad mini portrait", 744, 21, true, 577.5, 83},
		{"iPad mini portrait, Large", 744, 24.15, true, 664.125, 39},
		{"iPad mini portrait, Extra large", 744, 27.3, false, 714, 15},
		{"Large, at the switch", 694.125, 24.15, true, 664.125, 15},
		{"Large, under it", 694.0, 24.15, false, 664, 15},
		{"Extra large, at the switch", 780.75, 27.3, true, 750.75, 15},
		{"Extra large, under it", 780.7, 27.3, false, 750.7, 15},
		{"Extra large, wide", 1400, 27.3, true, 750.75, 324},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := readingPageFor(tc.width, tc.ref)
			if p.Book() != tc.book {
				t.Fatalf("page = %v, want book=%v", p.Kind, tc.book)
			}
			if math.Abs(p.Measure-tc.measure) > 1e-6 || math.Abs(p.Side-tc.side) > 1e-6 {
				t.Errorf("measure %.4f side %.4f, want %.4f and %.4f", p.Measure, p.Side, tc.measure, tc.side)
			}
			if p.Book() {
				if p.PitchEm != readingBookPitchEm || p.ParaGapEm != 0 || p.IndentEm != reporterIndentEm {
					t.Errorf("book page ems %+v", p)
				}
			} else if p.PitchEm != readingPhonePitchEm || p.ParaGapEm != readingParaGapEm || p.IndentEm != 0 {
				t.Errorf("phone page ems %+v", p)
			}
		})
	}
}

// Every width, at every text size: the page fits the pane, the book page's
// column is the measure, the phone page's margin is the minimum, and the page
// never flips back as the pane widens.
func TestReadingPageInvariants(t *testing.T) {
	for _, ref := range []float64{21, 24.15, 27.3} {
		wasBook := false
		for w := 1.0; w <= 2000; w++ {
			p := readingPageFor(w, ref)
			if p.Book() {
				if p.Side < readingPageSideMin || math.Abs(p.Measure-reporterMeasureEm*ref) > 1e-9 || 2*p.Side+p.Measure > w+1 {
					t.Fatalf("R=%v w=%v: book page %+v does not fit", ref, w, p)
				}
				wasBook = true
			} else {
				if wasBook {
					t.Fatalf("R=%v w=%v: the page went back to phone as the pane widened", ref, w)
				}
				if p.Side != readingPageSideMin || math.Abs(p.Measure-math.Max(0, w-2*readingPageSideMin)) > 1e-9 {
					t.Fatalf("R=%v w=%v: phone page %+v", ref, w, p)
				}
			}
		}
		if !wasBook {
			t.Fatalf("R=%v: no width reached the book page", ref)
		}
	}
}

// The measure is figured from the REFERENCE size, never the size the type is set
// at (docs/READING_TYPOGRAPHY.md, "The optical scale, and the one thing it must
// not touch"). Handing the set size in would widen the column 15%; the control
// shows the check can tell.
func TestReadingPageMeasureIsTheReferences(t *testing.T) {
	ref := readingReferencePx()
	p := readingPageFor(2000, ref)
	if math.Abs(p.Measure-reporterMeasureEm*ref) > 1e-9 {
		t.Errorf("measure %.3f, want 27.5 × the reference %.3f", p.Measure, ref)
	}
	if wrong := readingPageFor(2000, readingGlyphPx()); math.Abs(wrong.Measure-p.Measure) < 1 {
		t.Fatal("the control did not move: a measure from the set size must differ, or this test proves nothing")
	}
}

// The sizes inside the page, pinned where the spec states them.
func TestReadingPageSizeTable(t *testing.T) {
	for _, tc := range []struct {
		name      string
		got, want float64
	}{
		{"verse numeral", readingNumeralEm, 0.66},
		{"verse numeral lift", readingNumeralLiftEm, 1.0 / 3.0},
		{"omitted-verse mark", readingGapMarkEm, 0.66},
		{"footnote entries", readingFootnoteEm, 0.85},
		{"air under the footnote rule", readingFootnoteRuleGapEm, 0.33},
		{"air between footnotes", readingFootnoteEntryGapEm, 0.2},
		{"note body", noteBodySize, 15},
		{"note byline and pills", noteWhoSize, 11},
		{"side minimum", readingPageSideMin, 15},
		{"book pitch", readingBookPitchEm, readingLinePitchEm},
		{"phone pitch", readingPhonePitchEm, readingLinePitchEm},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

// The dev override forces a page on a pane of any width, and narrows a forced
// book page to fit rather than overflow it.
func TestReadingPageOverride(t *testing.T) {
	prev := readingPageOverride
	t.Cleanup(func() { readingPageOverride = prev })

	readingPageOverride = func() (readingPageKind, bool) { return readingPageBook, true }
	if p := readingPageAt(400, 21); !p.Book() || p.Measure != 370 || p.Side != 15 {
		t.Errorf("forced book at 400 = %+v, want book, measure 370, side 15", p)
	}
	readingPageOverride = func() (readingPageKind, bool) { return readingPagePhone, true }
	if p := readingPageAt(1200, 21); p.Book() || p.Measure != 1170 {
		t.Errorf("forced phone at 1200 = %+v, want phone, measure 1170", p)
	}
	readingPageOverride = func() (readingPageKind, bool) { return 0, false }
	if p := readingPageAt(1200, 21); !p.Book() {
		t.Errorf("an override that declines must leave the rule alone, got %+v", p)
	}
}
