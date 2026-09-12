package bibletext

import "strconv"

// THE AIR OF THE READING PANE, ONCE.
//
// Everything that stacks vertically inside the reading pane — paragraphs, the
// publisher's section headings, a psalm's title, the reporter page's indent —
// takes its measure from the five numbers here, in ems of the size the type
// is set at. Every surface derives what it needs from them: the Apple panes
// format their stylesheet from them, the Fyne pane reads them, the website
// substitutes them, and the Android bridge's own constants are held equal to
// them by a test (android/BtBridge.java cannot import Go). Change a number
// here and every surface moves together; change one surface's copy and its
// test fails.
//
// They were five copies before, and they had drifted: the Apple panes gave a
// paragraph a fixed 24px that did not scale with the reader's text size, the
// web gave it a full line, the Fyne pane 0.65 of a 1.55 line, Android a
// blank line of its pitch; a psalm's title stood 14px, 0.45 of a line, or a
// whole paragraph gap above its psalm depending on the platform. See
// docs/READING_TYPOGRAPHY.md, "Vertical spacing of the reading pane".
//
// The note band — the air reserved above and below a note's card — is the
// one vertical quantity that lives elsewhere: noteMetrics() in
// notes_bubble.go, in points, because the card is chrome rather than text.
const (
	// readingParaGapEm is the air between paragraphs on the gapped (phone)
	// page. The reporter page has none and indents instead.
	readingParaGapEm = 1.0
	// readingHeadLeadEm and readingHeadTailEm stand above and below a
	// publisher's section heading, which is set at the body size, bold.
	readingHeadLeadEm = 1.1
	readingHeadTailEm = 0.35
	// readingTitleGapEm stands between a psalm's superscription and verse 1.
	readingTitleGapEm = 0.55
	// reporterIndentEm is the reporter page's first-line indent. It is the
	// width the em-space and en-space pair used to draw, kept so the page did
	// not change when the characters stopped being characters.
	reporterIndentEm = 1.5
)

// The same numbers for the website generator, which lives in another package.
func ReadingParaGapEm() float64        { return readingParaGapEm }
func ReadingHeadLeadEm() float64       { return readingHeadLeadEm }
func ReadingHeadTailEm() float64       { return readingHeadTailEm }
func ReadingTitleGapEm() float64       { return readingTitleGapEm }
func ReadingReporterIndentEm() float64 { return reporterIndentEm }

// emCSS writes an em value the way a stylesheet wants it: 1 → "1em",
// 0.35 → "0.35em". Shortest exact form, so a test can match it verbatim.
func emCSS(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) + "em" }

// EmCSS is emCSS for the website generator.
func EmCSS(v float64) string { return emCSS(v) }
