package bibletext

// THE OPTICAL SCALE.
//
// Two faces set at the same nominal size do not draw the same size. What the eye
// reads as "how big is this text" is the x-height, and the share of the em a face
// spends on it varies enormously between designs. The face this app now sets
// Scripture in is a Renaissance revival and spends much less of its em on the
// lowercase than the screen serif it replaced: 0.418 of the em against 0.4814.
// Swapping one for the other at an unchanged nominal size therefore shrank the
// text by 13.2% without a single size in the code changing. Measured on a phone,
// the drawn x-height fell from 30 device pixels to 26.
//
// The scale below buys that back. Almost everything takes it: the size handed to
// a rasteriser, a CSS font-size, and every length reckoned in ems of the type as
// it is set — leading, first-line indents, the space between paragraphs. Those
// are marks made in the type's own units and they grow with it, which is also
// what keeps the deeper descenders of this face from crowding the line below.
//
// ONE KIND OF QUANTITY DOES NOT TAKE IT: a MEASURE, the physical width of the
// column. That is pinned to the unscaled reference size, and the reason is worth
// stating because it is not obvious. A measure exists to hold a line to a
// readable number of characters, and this face's average lowercase advance is
// 0.8691 of the reference face's, so at the same column width it already fits
// more characters than the column was cut for — 63 where the reporter measure
// was set for 55. But
//
//	0.8691 × 1.1518 = 1.0009
//
// so holding the column at its old physical width while the glyphs inside it
// grow puts the line back to its old character count within a tenth of a percent.
// The two errors cancel. Scale the measure as well and both survive: the column
// widens 15%, the line keeps the characters it should not have, and the reading
// column overtakes the list column that readable_column.go exists to keep it in
// sympathy with — on an iPad at the largest text size it would no longer fit the
// page at all.
//
// The two ratios are properties of the font binaries, not preferences. They are
// checked against the shipped files by TestOpticalScaleMatchesTheShippedFaces,
// which reads the outlines rather than the OS/2 fields — those disagree with the
// outline in three of the faces measured here, and always in the same direction.

// Fractions of the em, read off the 'x' outline.
//
// referenceXHeightEm is the face the reading surfaces set before the change: the
// stylesheet's first-named family on the Apple panes and the generated site, and
// what the desktop pane's own lookup preferred. The faces that stood in for it
// where it was absent — a serif fallback on Android, another on Linux — were
// larger still, so anchoring here restores the intended size on the surfaces that
// named a face and brings the two that never did into line with them.
const referenceXHeightEm = 0.4814453125

// readingXHeightEm is the face they set now.
const readingXHeightEm = 0.418

// readingBodyBase is the "Normal" reading size in points, before the reader's own
// text-size setting and before the optical scale. The Apple panes, the Android
// overlay and the generated site are all figured from it.
const readingBodyBase = 21.0

// readingOpticalScale is what a nominal size must be multiplied by for the shipped
// face to draw at the apparent size the reference face drew at. 1.1518.
func readingOpticalScale() float64 {
	return referenceXHeightEm / readingXHeightEm
}

// readingReferencePx is the reading size a MEASURE is figured from — the physical
// width of the reading column, and nothing else. It is deliberately blind to which
// face is set, so that changing the face never moves the column.
func readingReferencePx() float64 {
	return readingBodyBase * readingTextScale()
}

// readingGlyphPx is the size the face is actually set at: the reference size opened
// up until the lowercase draws as tall as the old face's did.
func readingGlyphPx() float64 {
	return readingReferencePx() * readingOpticalScale()
}

// readingGlyphSize applies the same correction to a surface that reckons its
// reading size from its own base rather than from readingBodyBase — the desktop
// canvas pane, whose base is the toolkit's text size.
func readingGlyphSize(reference float32) float32 {
	return reference * float32(readingOpticalScale())
}

// ReadingOpticalScale is the correction as the site generator needs it. The
// generated stylesheet sets Scripture in the same face as the app and has to open
// it up by the same amount, or a chapter read on the web is visibly smaller than
// the same chapter read in the app.
func ReadingOpticalScale() float64 { return readingOpticalScale() }

// THE LEADING, AS A MULTIPLE OF THE SIZE THE TYPE IS SET AT.
//
// It is a chosen number now. It was not before: the stylesheet asked for a
// unitless line-height, the importer turned that into a minimum line height on
// each RUN, and the panes take a paragraph's leading from its FIRST run — which
// is always the 0.66em verse numeral. So the drawn pitch was 2.0 × 0.66 × the
// body, an accident that would have moved if the numeral were ever resized, and
// that no reader of the stylesheet could have predicted.
//
// 1.2222 sets a 24pt body on a 29.33pt line — 88 device pixels on a 3× phone.
// Judged against the face's REAL ink rather than its declared line box, which
// overstates what it draws by 27%: at this pitch the deepest descender clears the
// next line's tallest ascender by 5.4pt. The face it replaced had 7.2pt at its
// own smaller size, and leaving this at the old accidental value gave 7.7pt.
const readingLinePitchEm = 1.2222

// ReadingLinePitchEm is the same number for the generated site, whose CSS
// line-height multiplies the font size directly — no numeral in the way.
func ReadingLinePitchEm() float64 { return readingLinePitchEm }
