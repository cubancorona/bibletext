package bibletext

import "math"

// THE READING PAGE — docs/READING_TYPOGRAPHY.md, "The reading page".
//
// The page is specified once, here, beside the sizes (reading_face_scale.go)
// and the vertical air (reading_spacing.go) it is built from. Every surface
// gets its page from readingPageAt, or has that function's answers pushed or
// substituted into it, the way the spacing constants reach the Apple
// stylesheet, the canvas pane, Android's Java and the web's CSS.
//
// Two pages, and one rule for choosing between them: the WIDTH the text has.
// The book page — the U.S. Reports set, a centred column of reporterMeasureEm
// with first-line indents and no gap between paragraphs — is used whenever the
// column and the phone page's own side margins fit. Otherwise the phone page:
// the full width less those margins, with a gap between paragraphs and no
// indent. Never by device, orientation or idiom, so a narrow Mac or iPad
// window reads like a phone and a wide Android tablet like a book.
//
// Widths are in the surface's own logical unit: Apple points, Fyne units (a
// pixel at 100% on Windows and Linux), Android dp, CSS pixels at a 16px root.
// The ems a page carries are multiplied by the size each surface actually sets
// the body at.

type readingPageKind uint8

const (
	readingPagePhone readingPageKind = iota
	readingPageBook
)

func (k readingPageKind) String() string {
	if k == readingPageBook {
		return "book"
	}
	return "phone"
}

// readingPageSideMin is the distance from the pane's edge to the ink, each
// side, below which the book page gives way to the phone page — and the phone
// page's own margin. It is the iPhone's left ink margin (its inset of 10 plus
// UIKit's line-fragment padding of 5). Fixed: it does not follow the reader's
// text size, because it is the page's edge, not its type.
const readingPageSideMin = 15.0

// Line pitch per page, as a multiple of the size the body is SET at. One value
// on both pages, the one every surface but the canvas pane already used: the
// canvas pane's phone page set 1.55 and its book page 1.3 until the page was
// specified here. Two names, so a surface asks for its page's pitch and one
// constant carries any later decision to all five surfaces.
const (
	readingBookPitchEm  = readingLinePitchEm
	readingPhonePitchEm = readingLinePitchEm
)

// Text sizes inside the page, in ems of the body as set (numeral lift: ems of
// the body, measured baseline to baseline).
const (
	readingNumeralEm          = 0.66
	readingNumeralLiftEm      = 1.0 / 3.0
	readingGapMarkEm          = 0.66
	readingFootnoteEm         = 0.85
	readingFootnoteRuleGapEm  = 0.33
	readingFootnoteEntryGapEm = 0.2
)

// The note card's text, in units. The card is the app's furniture, not
// Scripture, so it does not follow the reader's text size (notes_bubble.go).
const (
	noteBodySize = 15.0
	noteWhoSize  = 11.0
)

// readingPage is what a surface needs to lay its page out.
type readingPage struct {
	Kind      readingPageKind
	Measure   float64 // the ink line's width; from the REFERENCE size, never the set size
	Side      float64 // pane edge to ink, each side; 0 when the pane has no width yet
	PitchEm   float64 // line pitch, ems of the body as set
	ParaGapEm float64 // air between paragraphs, ems of the body as set
	IndentEm  float64 // first-line indent of a prose paragraph, ems of the body as set
}

// Book reports whether this is the book page.
func (p readingPage) Book() bool { return p.Kind == readingPageBook }

// readingBookPageFits is THE switch: the measure plus the phone page's margins
// fits in the width the text has. reference is the reading size before the
// optical scale (readingReferencePx on the natives, the canvas pane's
// referenceSize) — a measure is figured from it and from nothing else.
func readingBookPageFits(paneWidth, reference float64) bool {
	return paneWidth >= reporterMeasureEm*reference+2*readingPageSideMin
}

// readingPageFor is the page for a pane of this width. A width of zero or less
// — a pane not yet laid out — answers the book page with no side. The canvas
// pane never asks it so (it lays out from a real width); the native panes ask
// currentReadingPage (reading_page_width.go), which answers a push made with no
// width at all from the device instead.
func readingPageFor(paneWidth, reference float64) readingPage {
	if paneWidth <= 0 || readingBookPageFits(paneWidth, reference) {
		return readingPageOf(readingPageBook, paneWidth, reference)
	}
	return readingPageOf(readingPagePhone, paneWidth, reference)
}

// readingPageOf builds a page of the given kind for this width.
func readingPageOf(kind readingPageKind, paneWidth, reference float64) readingPage {
	m := reporterMeasureEm * reference // REFERENCE, never the set size
	p := readingPage{Kind: kind}
	if kind == readingPageBook {
		p.PitchEm, p.IndentEm = readingBookPitchEm, reporterIndentEm
	} else {
		m = paneWidth - 2*readingPageSideMin
		p.PitchEm, p.ParaGapEm = readingPhonePitchEm, readingParaGapEm
	}
	if paneWidth <= 0 {
		p.Measure = math.Max(0, m)
		return p
	}
	// A no-op when the book page fits; it only narrows a book page forced on a
	// pane too narrow for it (readingPageOverride).
	p.Measure = math.Max(0, math.Min(m, paneWidth-2*readingPageSideMin))
	p.Side = math.Max(readingPageSideMin, math.Floor((paneWidth-p.Measure)/2))
	return p
}

// readingPageOverride forces a page, for looking at one on a pane of any width.
// Nil in release builds; the dev build sets it.
var readingPageOverride func() (readingPageKind, bool)

// readingPageAt is readingPageFor with the dev override applied. Surfaces call
// this one.
func readingPageAt(paneWidth, reference float64) readingPage {
	if readingPageOverride != nil {
		if k, ok := readingPageOverride(); ok {
			return readingPageOf(k, paneWidth, reference)
		}
	}
	return readingPageFor(paneWidth, reference)
}

// The same numbers for the website generator, which lives in another package.
func ReadingBodyBase() float64           { return readingBodyBase }
func ReadingReporterMeasureEm() float64  { return reporterMeasureEm }
func ReadingPageSideMin() float64        { return readingPageSideMin }
func ReadingBookPitchEm() float64        { return readingBookPitchEm }
func ReadingPhonePitchEm() float64       { return readingPhonePitchEm }
func ReadingNumeralEm() float64          { return readingNumeralEm }
func ReadingNumeralLiftEm() float64      { return readingNumeralLiftEm }
func ReadingFootnoteEm() float64         { return readingFootnoteEm }
func ReadingGapMarkEm() float64          { return readingGapMarkEm }
func ReadingFootnoteEntryGapEm() float64 { return readingFootnoteEntryGapEm }
func NoteBodySize() float64              { return noteBodySize }
func NoteWhoSize() float64               { return noteWhoSize }
